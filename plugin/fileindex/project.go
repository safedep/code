package fileindex

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/code/plugin/callgraph"
	sitter "github.com/smacker/go-tree-sitter"
)

// maxInstanceHops bounds the walk from an instance key, as Parser//this//m,
// through its synthetic edges to the method.
const maxInstanceHops = 4

// definitionKinds maps the tree-sitter type of a definition to its kind. A
// function in a class becomes a method. The root of the file is the module.
var definitionKinds = map[string]fidx.DefKind{
	"class_declaration":              fidx.DefClass,
	"enum_declaration":               fidx.DefClass,
	"interface_declaration":          fidx.DefInterface,
	"abstract_class_declaration":     fidx.DefClass,
	"class_definition":               fidx.DefClass,
	"class":                          fidx.DefClass,
	"module":                         fidx.DefClass,
	"struct_declaration":             fidx.DefClass,
	"record_declaration":             fidx.DefClass,
	"trait_declaration":              fidx.DefClass,
	"function_declaration":           fidx.DefFunction,
	"generator_function_declaration": fidx.DefFunction,
	"function_definition":            fidx.DefFunction,
	"function_expression":            fidx.DefFunction,
	"arrow_function":                 fidx.DefFunction,
	"function_item":                  fidx.DefFunction,
	"local_function_statement":       fidx.DefFunction,
	"method_definition":              fidx.DefMethod,
	"method_declaration":             fidx.DefMethod,
	"method":                         fidx.DefMethod,
	"singleton_method":               fidx.DefMethod,
	"constructor_declaration":        fidx.DefConstructor,
}

// constructorNames name the method that makes an instance of a class.
var constructorNames = map[core.LanguageCode]string{
	core.LanguageCodePython:     "__init__",
	core.LanguageCodeJavascript: "constructor",
	core.LanguageCodeTypescript: "constructor",
	core.LanguageCodePHP:        "__construct",
	core.LanguageCodeRuby:       "initialize",
}

// projection projects the call graph of one tree into an index.
type projection struct {
	cg       *callgraph.CallGraph
	tree     core.ParseTree
	language core.Language
	source   []byte
	ix       *fidx.Index

	// defs holds the definitions by key. syntax holds the tree node of each
	// definition, and spans the key of each such node. bodies holds the key
	// of each definition that the call graph does not know and that takes
	// the calls of its node, as a function that a JavaScript property or
	// argument holds. exports holds the definition keys that an export
	// names, and bindings the names that the imports bind.
	defs     map[string]*fidx.Definition
	syntax   map[string]*sitter.Node
	spans    map[span]string
	bodies   map[span]string
	exports  map[string]bool
	bindings map[string]bool

	// exportStatements are the export statements and the CommonJS export
	// assignments of the file.
	exportStatements []*sitter.Node
}

// project builds the call graph of tree and projects it into an index.
func project(ctx context.Context, tree core.ParseTree) (*fidx.Index, error) {
	language, err := tree.Language()
	if err != nil {
		return nil, fmt.Errorf("failed to get language: %w", err)
	}
	data, err := tree.Data()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree data: %w", err)
	}
	cg, err := callgraph.Build(ctx, tree)
	if err != nil {
		return nil, err
	}

	p := &projection{
		cg:       cg,
		tree:     tree,
		language: language,
		source:   *data,
		ix: &fidx.Index{
			Version:     fidx.Version,
			Path:        cg.FileName,
			Language:    language.Meta().Code,
			ContentHash: fidx.HashContent(*data),
		},
		defs:     map[string]*fidx.Definition{},
		syntax:   map[string]*sitter.Node{},
		spans:    map[span]string{},
		bodies:   map[span]string{},
		exports:  map[string]bool{},
		bindings: map[string]bool{},
	}

	switch {
	case p.isGo():
		p.ix.Package = p.goPackage()
	case p.isJava():
		p.ix.Package = p.javaPackage()
	}
	p.addImports()
	p.addDefinitions()
	p.addExports()
	p.addTruncations()
	p.addGoSignatures()
	p.addJavaSignatures()
	p.walkTree(ctx)
	p.addDeclaredTypes()
	if err := p.addClasses(ctx); err != nil {
		return nil, err
	}
	p.addCalls()
	p.addUnresolved()
	for _, def := range p.defs {
		def.Exported = p.exported(def)
		p.ix.Definitions = append(p.ix.Definitions, *def)
	}

	p.ix.Sort()
	return p.ix, nil
}

func (p *projection) addImports() {
	for _, imp := range p.cg.Imports {
		p.ix.Imports = append(p.ix.Imports, fidx.Import{
			Line:      int(imp.Position.StartLine),
			Module:    imp.Module,
			Item:      imp.Item,
			Alias:     imp.Alias,
			Wildcard:  imp.Wildcard,
			Kind:      p.importKind(imp),
			Namespace: imp.Namespace,
		})
		if imp.Alias != "" {
			p.bindings[imp.Alias] = true
		}
	}
}

// importKind tells a static import from a dynamic import that a string
// names, as await import('x'), and from a TypeScript import type.
func (p *projection) importKind(imp callgraph.Import) fidx.ImportKind {
	for node := p.descendant(imp.Position); node != nil; node = node.Parent() {
		switch node.Type() {
		case "call_expression":
			if fn := node.ChildByFieldName("function"); fn != nil && fn.Type() == "import" {
				return fidx.ImportDynamicString
			}
		case "import_statement":
			if typeOnly(node) || imp.Item != "" && p.typeOnlySpecifier(node, imp.Item) {
				return fidx.ImportTypeOnly
			}
			return fidx.ImportStatic
		}
	}
	return fidx.ImportStatic
}

// typeOnly reports whether a TypeScript import, export or specifier binds
// only a type, as import type { A } or import { type A }.
func typeOnly(node *sitter.Node) bool {
	return hasKeyword(node, "type", "typeof")
}

// hasKeyword reports whether one of the anonymous children of the node is
// one of the words.
func hasKeyword(node *sitter.Node, words ...string) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		if child := node.Child(i); !child.IsNamed() && slices.Contains(words, child.Type()) {
			return true
		}
	}
	return false
}

// typeOnlySpecifier reports whether the import statement imports the item
// by a type-only specifier.
func (p *projection) typeOnlySpecifier(statement *sitter.Node, item string) bool {
	for _, clause := range ts.NamedChildren(statement) {
		if clause.Type() != "import_clause" {
			continue
		}
		for _, imports := range ts.NamedChildren(clause) {
			if imports.Type() != "named_imports" {
				continue
			}
			for _, specifier := range ts.NamedChildren(imports) {
				if p.text(specifier.ChildByFieldName("name")) == item && typeOnly(specifier) {
					return true
				}
			}
		}
	}
	return false
}

func (p *projection) addDefinitions() {
	file := p.cg.FileName
	for key, node := range p.cg.Nodes {
		kind, ok := definitionKinds[node.Kind]
		switch {
		case key == file:
			kind = fidx.DefModule
		case !ok || !strings.HasPrefix(key, file+fidx.Separator):
			continue
		}

		p.defs[key] = &fidx.Definition{
			Key:       key,
			Kind:      kind,
			Name:      definitionName(key, file),
			StartLine: int(node.Position.StartLine),
			EndLine:   int(node.Position.EndLine),
		}
		p.addSyntax(key, p.nodeAt(node.Position, node.Kind))
	}
	p.addInterfaces()
	p.addGoTypes()

	for _, def := range p.defs {
		p.fillDefinition(def)
	}
	p.addVariables()
}

// addVariables adds each name that the top level of the module assigns and
// that no other definition holds, with the values of the name. A variable
// has no tree node in syntax, so a flag in its value lands on the module.
func (p *projection) addVariables() {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		for _, name := range p.assignedNames(statement) {
			key := p.local(p.text(name))
			if _, defined := p.defs[key]; defined || p.bindings[p.text(name)] {
				continue
			}
			p.defs[key] = &fidx.Definition{
				Key:       key,
				Kind:      fidx.DefVariable,
				Name:      p.text(name),
				StartLine: lineOf(statement),
				EndLine:   int(statement.EndPoint().Row) + 1,
				Values:    p.targets(p.cg.Values(key)),
			}
		}
	}
}

// assignedNames returns the name nodes that a top-level statement assigns,
// as x in x = f() or const x = f().
func (p *projection) assignedNames(statement *sitter.Node) []*sitter.Node {
	if statement.Type() == "export_statement" {
		if statement = statement.ChildByFieldName("declaration"); statement == nil {
			return nil
		}
	}
	var names []*sitter.Node
	switch statement.Type() {
	case "expression_statement":
		if p.ix.Language != core.LanguageCodePython || statement.NamedChildCount() == 0 {
			return nil
		}
		if assignment := statement.NamedChild(0); assignment.Type() == "assignment" {
			names = append(names, assignment.ChildByFieldName("left"))
		}
	case "lexical_declaration", "variable_declaration":
		for _, declarator := range ts.NamedChildren(statement) {
			names = append(names, declarator.ChildByFieldName("name"))
		}
	}
	return slices.DeleteFunc(names, func(n *sitter.Node) bool { return n == nil || n.Type() != "identifier" })
}

// definitionName returns the last segment of a key. The module is the base
// name of the file with no extension, as parse for src/parse.py.
func definitionName(key, file string) string {
	if key == file {
		base := path.Base(file)
		return strings.TrimSuffix(base, path.Ext(base))
	}
	return fidx.Last(key)
}

// fillDefinition sets the parent class, the method kind and the decorators.
func (p *projection) fillDefinition(def *fidx.Definition) {
	if def.Kind == fidx.DefModule {
		return
	}
	// A function with an anonymous name in a class body, as an argument in
	// a static block, is no method of the class.
	scope := fidx.Parent(def.Key)
	if parent, ok := p.defs[scope]; ok && (parent.Kind == fidx.DefClass || parent.Kind == fidx.DefInterface) && !fidx.IsAnonymous(def.Name) {
		def.Parent = scope
		if def.Kind == fidx.DefFunction {
			def.Kind = fidx.DefMethod
		}
		if def.Kind == fidx.DefMethod && def.Name == constructorNames[p.ix.Language] {
			def.Kind = fidx.DefConstructor
		}
	}
	if p.isJava() {
		def.Decorators = p.javaAnnotations(p.syntax[def.Key])
		return
	}
	def.Decorators = p.decorators(p.syntax[def.Key])
}

// decorators returns the decorators of a definition as written, with no
// arguments, as @app.route for @app.route("/"). A Python decorated
// definition and a JavaScript method hold their decorators. A TypeScript
// class body holds the decorators of a method before the method.
func (p *projection) decorators(node *sitter.Node) []string {
	if node == nil {
		return nil
	}
	var nodes []*sitter.Node
	switch parent := node.Parent(); {
	case parent == nil:
	case parent.Type() == "decorated_definition":
		nodes = ts.NamedChildren(parent)
	case parent.Type() == "class_body":
		for sibling := node.PrevNamedSibling(); sibling != nil && sibling.Type() == "decorator"; sibling = sibling.PrevNamedSibling() {
			nodes = append(nodes, sibling)
		}
		slices.Reverse(nodes)
	}
	nodes = append(nodes, ts.NamedChildren(node)...)

	var decorators []string
	for _, decorator := range nodes {
		if decorator.Type() != "decorator" || decorator.NamedChildCount() == 0 {
			continue
		}
		expression := decorator.NamedChild(0)
		if callee := expression.ChildByFieldName("function"); callee != nil {
			expression = callee
		}
		decorators = append(decorators, "@"+expression.Content(p.source))
	}
	return decorators
}

// addClasses adds each class definition with its methods and bases. The
// bases come from the class resolver of the language, and from the class
// heritage of a JavaScript class.
func (p *projection) addClasses(ctx context.Context) error {
	bases, err := p.classBases(ctx)
	if err != nil {
		return err
	}

	methods := map[string][]string{}
	for key, def := range p.defs {
		if def.Parent != "" {
			methods[def.Parent] = append(methods[def.Parent], key)
		}
	}
	for key, def := range p.defs {
		node := p.syntax[key]
		switch {
		case def.Kind == fidx.DefInterface:
			p.ix.Classes = append(p.ix.Classes, fidx.Class{Key: key, Bases: p.interfaceBases(node)})
		case def.Kind != fidx.DefClass:
		case node == nil:
			p.ix.Classes = append(p.ix.Classes, fidx.Class{Key: key, Methods: methods[key]})
		default:
			p.ix.Classes = append(p.ix.Classes, fidx.Class{Key: key, Bases: bases[node.StartByte()], Methods: methods[key], Fields: p.fields(node)})
		}
	}
	return nil
}

// classBases returns the bases of each class by the start byte of the class
// node.
func (p *projection) classBases(ctx context.Context) (map[uint32][]string, error) {
	bases := map[uint32][]string{}
	resolvers, ok := p.language.Resolvers().(core.ObjectOrientedLanguageResolvers)
	if !ok {
		for _, node := range p.syntax {
			switch {
			case node == nil:
			case node.Type() == "class_declaration":
				bases[node.StartByte()] = p.heritage(node)
			case p.isGo():
				bases[node.StartByte()] = p.goBases(node)
			}
		}
		return bases, nil
	}
	if p.isJava() {
		for _, node := range p.syntax {
			if node != nil {
				bases[node.StartByte()] = p.javaBases(node)
			}
		}
		return bases, nil
	}

	classes, err := resolvers.ResolveClasses(ctx, p.tree)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve classes: %w", err)
	}
	for _, class := range classes {
		name := class.GetClassNameNode()
		if name == nil || name.Parent() == nil || !class.HasInheritance() {
			continue
		}
		bases[name.Parent().StartByte()] = class.BaseClasses()
	}
	return bases, nil
}

// heritage returns the bases that the class_heritage of a class names, as
// Base in class A extends Base.
func (p *projection) heritage(class *sitter.Node) []string {
	var bases []string
	for i := 0; i < int(class.NamedChildCount()); i++ {
		child := class.NamedChild(i)
		if child.Type() != "class_heritage" {
			continue
		}
		for j := 0; j < int(child.NamedChildCount()); j++ {
			bases = append(bases, child.NamedChild(j).Content(p.source))
		}
	}
	return bases
}

func (p *projection) addCalls() {
	for key, node := range p.cg.Nodes {
		for _, ref := range node.CallsTo {
			kind, ok := callKind(ref.Kind)
			if !ok {
				continue
			}
			if p.isRequire(ref) {
				continue
			}
			targets := p.targets(ref.Targets)
			if ref.Unbound {
				targets = p.rebind(key, ref.Method)
				// A Java name that the file does not define keeps its
				// name with no type arguments, as Box for new Box<>().
				if len(targets) == 0 && p.isJava() {
					targets = []string{ref.CalleeNamespace}
				}
			}
			targets = p.throughRequire(ref.Receiver, targets)
			p.ix.Calls = append(p.ix.Calls, fidx.Call{
				Caller:   p.caller(key, ref.Site),
				Line:     int(ref.Site.StartLine),
				Column:   int(ref.Site.StartColumn),
				Callee:   ref.Text,
				Targets:  targets,
				Receiver: ref.Receiver,
				Method:   ref.Method,
				Kind:     kind,
				Args:     p.arguments(key, ref.Site),
			})
		}
	}
}

// caller returns the definition that runs the call site that the
// definition key holds. A function that a JavaScript property or argument
// holds is a definition that the call graph does not know, so it takes the
// calls of its body. A JavaScript static field or static block runs when the class
// is defined, so its calls belong to the scope that defines the class. A
// function value in a static field runs only when a call runs it. A flag in
// a static block stays on the class.
func (p *projection) caller(key string, site ast.NodePosition) string {
	if !p.ix.Language.IsECMAScript() {
		return key
	}
	def, ok := p.defs[key]
	class := ok && def.Kind == fidx.DefClass
	if !class && len(p.bodies) == 0 {
		return key
	}
	var own span
	if node := p.syntax[key]; node != nil {
		own = spanOf(node)
	}
	for node := p.descendant(site); node != nil && spanOf(node) != own; node = node.Parent() {
		if body, ok := p.bodies[spanOf(node)]; ok {
			return body
		}
		if !class {
			continue
		}
		switch node.Type() {
		case "class_body":
			return key
		case "arrow_function", "function_expression", "function", "generator_function":
			return key
		case "class_static_block":
			return fidx.Parent(key)
		case "field_definition", "public_field_definition":
			if hasKeyword(node, "static") {
				return fidx.Parent(key)
			}
		}
	}
	return key
}

// isRequire reports whether a call is a CommonJS require of the builtin
// require. The import record of the call names the module, so the call
// itself makes no call record.
func (p *projection) isRequire(ref callgraph.CallReference) bool {
	return p.ix.Language.IsECMAScript() && ref.Text == "require" && slices.Equal(ref.Targets, []string{"require"})
}

// requireCall matches a receiver that starts with a require of a string, as
// require('js-yaml') in require('js-yaml').load.
var requireCall = regexp.MustCompile("^require\\(\\s*['\"`]([^'\"`]+)['\"`]\\s*\\)")

// throughRequire gives a member of a require call the namespace of the
// module, as js-yaml//load for require('js-yaml').load. The call graph keys
// the member under require.
func (p *projection) throughRequire(receiver string, targets []string) []string {
	if !p.ix.Language.IsECMAScript() {
		return targets
	}
	m := requireCall.FindStringSubmatch(receiver)
	if m == nil {
		return targets
	}
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		if rest, ok := strings.CutPrefix(t, "require"+fidx.Separator); ok {
			t = fidx.Join(fidx.Namespace(p.ix.Language, m[1]), rest)
		}
		out = append(out, t)
	}
	return out
}

// callKind maps the kind of a call graph edge to the kind of a call. An
// edge that no call site makes has no call.
func callKind(kind callgraph.CallKind) (fidx.CallKind, bool) {
	switch kind {
	case callgraph.CallCall:
		return fidx.CallCall, true
	case callgraph.CallNew:
		return fidx.CallNew, true
	case callgraph.CallImport:
		return fidx.CallImport, true
	default:
		return "", false
	}
}

// targets replaces an instance key, as Parser//this//parse, with the method
// that its synthetic edge reaches, so a target is a definition key.
func (p *projection) targets(targets []string) []string {
	var resolved []string
	for _, target := range targets {
		resolved = append(resolved, p.throughInstance(target, 0)...)
	}
	slices.Sort(resolved)
	return slices.Compact(resolved)
}

func (p *projection) throughInstance(target string, hops int) []string {
	node, ok := p.cg.Nodes[target]
	if !ok || node.Kind != "" || len(node.CallsTo) == 0 || hops >= maxInstanceHops {
		return []string{target}
	}
	var methods []string
	for _, ref := range node.CallsTo {
		if ref.Kind != callgraph.CallSynthetic {
			return []string{target}
		}
		methods = append(methods, p.throughInstance(ref.CalleeNamespace, hops+1)...)
	}
	return methods
}

// rebind binds a name that the scope chain of the build did not bind to a
// definition of the file, as a function that the file declares after the
// call. It looks from the scope of the caller out to the module, and skips
// a class scope, as the languages do. A Java class scope holds the methods
// and the nested classes that the class calls with no receiver. A name
// with no definition has no target.
func (p *projection) rebind(caller, name string) []string {
	if name == "" || !fidx.HasKeyPrefix(caller, p.cg.FileName) {
		return nil
	}
	for scope := caller; ; {
		if def, ok := p.defs[scope]; ok && (def.Kind != fidx.DefClass || scope == caller || p.isJava()) {
			if _, bound := p.defs[fidx.Join(scope, name)]; bound {
				return []string{fidx.Join(scope, name)}
			}
		}
		if scope == p.cg.FileName {
			return nil
		}
		scope = fidx.Parent(scope)
	}
}

func (p *projection) addUnresolved() {
	for _, u := range p.cg.Unresolved() {
		p.ix.Unresolved = append(p.ix.Unresolved, fidx.Unresolved{
			Caller:   p.caller(u.Caller, u.Site),
			Line:     int(u.Site.StartLine),
			Column:   int(u.Site.StartColumn),
			Text:     u.Text,
			Receiver: u.Receiver,
			Method:   u.Method,
			Reason:   fidx.UnresolvedReason(u.Reason),
		})
	}
}

func (p *projection) addTruncations() {
	for _, t := range p.cg.Truncations() {
		p.truncate(fidx.TruncationKind(t.Kind), t.Namespace, t.Line)
	}
}

// truncate records the first hit of a kind of limit.
func (p *projection) truncate(kind fidx.TruncationKind, namespace string, line int) {
	if slices.ContainsFunc(p.ix.Truncated, func(t fidx.Truncation) bool { return t.Kind == kind }) {
		return
	}
	p.ix.Truncated = append(p.ix.Truncated, fidx.Truncation{Kind: kind, Namespace: namespace, Line: line})
}

// descendant returns the smallest named node that spans position.
func (p *projection) descendant(position ast.NodePosition) *sitter.Node {
	start := sitter.Point{Row: position.StartLine - 1, Column: position.StartColumn - 1}
	end := sitter.Point{Row: position.EndLine - 1, Column: position.EndColumn - 1}
	return p.tree.Tree().RootNode().NamedDescendantForPointRange(start, end)
}

// nodeAt returns the node of type kind at position. A node of the same
// range can wrap it, so the walk goes up to the node of the type.
func (p *projection) nodeAt(position ast.NodePosition, kind string) *sitter.Node {
	node := p.descendant(position)
	for ; node != nil; node = node.Parent() {
		if node.Type() == kind && node.StartByte() == position.StartByte {
			return node
		}
	}
	return nil
}

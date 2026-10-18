package indexer

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	"github.com/safedep/code/plugin/callgraph"
	sitter "github.com/smacker/go-tree-sitter"
)

// maxInstanceHops bounds the walk from an instance key, as Parser//this//m,
// through its synthetic edges to the method.
const maxInstanceHops = 4

// staticInitName names the static initializer of a class, as the JVM does.
// The call graph of Kotlin and C# keys the code that runs when a class
// loads under it.
const staticInitName = "<clinit>"

// projection projects the call graph of one tree into an index.
type projection struct {
	cg       *callgraph.CallGraph
	tree     core.ParseTree
	language core.Language
	source   []byte
	ix       *fileindex.Index

	// defs holds the definitions by key. syntax holds the tree node of each
	// definition, and spans the key of each such node. bodies holds the key
	// of each definition that the call graph does not know and that takes
	// the calls of its node, as a function that a JavaScript property or
	// argument holds. exports holds the definition keys that an export
	// names, and bindings the names that the imports bind.
	defs     map[string]*fileindex.Definition
	syntax   map[string]*sitter.Node
	spans    map[span]string
	bodies   map[span]string
	exports  map[string]bool
	bindings map[string]bool
	// members holds the line and the column of each call that names a
	// member of an object of the file.
	members map[[2]uint32]bool

	// exportStatements are the export statements and the CommonJS export
	// assignments of the file.
	exportStatements []*sitter.Node
}

// project builds the call graph of tree and projects it into an index.
func project(ctx context.Context, tree core.ParseTree) (*fileindex.Index, error) {
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
		ix: &fileindex.Index{
			Version:     fileindex.Version,
			Path:        cg.FileName,
			Language:    language.Meta().Code,
			ContentHash: fileindex.HashContent(*data),
			Unit:        unitOf(cg.FileName),
		},
		defs:     map[string]*fileindex.Definition{},
		syntax:   map[string]*sitter.Node{},
		spans:    map[span]string{},
		bodies:   map[span]string{},
		exports:  map[string]bool{},
		bindings: map[string]bool{},
		members:  map[[2]uint32]bool{},
	}

	rules := p.rules()
	if rules.packageOf != nil {
		p.ix.Package = rules.packageOf(p)
	}
	p.addImports()
	p.addDefinitions()
	p.flagModuleGetattr()
	p.run(rules.exports)
	p.addTruncations()
	p.addSignatures()
	p.run(rules.inits)
	p.walkTree(ctx)
	p.run(rules.afterWalk)
	p.addCalls()
	p.addUnresolved()
	p.run(rules.prune)
	if err := p.addClasses(ctx); err != nil {
		return nil, err
	}
	for _, def := range p.defs {
		def.Exported = p.exported(def)
		p.ix.Definitions = append(p.ix.Definitions, *def)
	}

	p.ix.Sort()
	return p.ix, nil
}

// run runs a pass of the rules of the language, when the language has it.
func (p *projection) run(pass func(p *projection)) {
	if pass != nil {
		pass(p)
	}
}

// unitOf returns the package unit of the name of a file of dependency code.
func unitOf(name string) string {
	unit, _ := fileindex.SplitUnit(name)
	return unit
}

func (p *projection) addImports() {
	for _, imp := range p.cg.Imports {
		kind, require := p.importForm(imp)
		p.ix.Imports = append(p.ix.Imports, fileindex.Import{
			Line:      int(imp.Position.StartLine),
			Module:    imp.Module,
			Item:      imp.Item,
			Alias:     imp.Alias,
			Wildcard:  imp.Wildcard,
			Kind:      kind,
			Namespace: imp.Namespace,
			Require:   require,
		})
		if imp.Alias != "" {
			p.bindings[imp.Alias] = true
		}
	}
}

// importForm tells a static import from a dynamic import that a string
// names, as await import('x'), and from a TypeScript import type. It
// reports whether a require call makes the import.
func (p *projection) importForm(imp callgraph.Import) (fileindex.ImportKind, bool) {
	for node := p.descendant(imp.Position); node != nil; node = node.Parent() {
		switch node.Type() {
		case "call_expression":
			switch fn := node.ChildByFieldName("function"); {
			case fn == nil:
			case fn.Type() == "import":
				return fileindex.ImportDynamicString, false
			case p.text(fn) == requireFunction:
				return fileindex.ImportStatic, true
			}
		case "import_statement":
			if typeOnly(node) || imp.Item != "" && p.typeOnlySpecifier(node, imp.Item) {
				return fileindex.ImportTypeOnly, false
			}
			return fileindex.ImportStatic, false
		}
	}
	return fileindex.ImportStatic, false
}

// requireFunction names the CommonJS function that loads a module.
const requireFunction = "require"

// typeOnly reports whether a TypeScript import, export or specifier binds
// only a type, as import type { A } or import { type A }.
func typeOnly(node *sitter.Node) bool {
	return ts.HasKeyword(node, "type", "typeof")
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

// addDefinitions adds each node of the call graph whose node type is a
// definition of the language. The root of the file is the module.
func (p *projection) addDefinitions() {
	file := p.cg.FileName
	rules := p.rules()
	for key, node := range p.cg.Nodes {
		kind, ok := rules.kinds[node.Kind]
		switch {
		case key == file:
			kind = fileindex.DefModule
		case !strings.HasPrefix(key, file+fileindex.Separator):
			continue
		case fileindex.Last(key) == staticInitName && node.Kind != "":
			kind = fileindex.DefMethod
		case !ok:
			continue
		}

		treeNode := p.nodeAt(node.Position, node.Kind)
		if rules.kindOf != nil {
			kind = rules.kindOf(p, treeNode, kind)
		}
		p.defs[key] = &fileindex.Definition{
			Key:       key,
			Kind:      kind,
			Name:      definitionName(key, file),
			StartLine: int(node.Position.StartLine),
			EndLine:   int(node.Position.EndLine),
		}
		p.addSyntax(key, treeNode)
	}
	p.run(rules.definitions)

	for _, def := range p.defs {
		p.fillDefinition(def)
	}
	p.addVariables()
}

// addVariables adds each name that the top level of the module assigns and
// that no other definition holds, with the values of the name. A variable
// has no tree node in syntax, so a flag in its value lands on the module.
func (p *projection) addVariables() {
	assigned := p.rules().assigned
	if assigned == nil {
		return
	}
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		for _, name := range assigned(p, statement) {
			if name == nil || name.Type() != "identifier" {
				continue
			}
			key := p.local(p.text(name))
			if _, defined := p.defs[key]; defined || p.bindings[p.text(name)] {
				continue
			}
			p.defs[key] = &fileindex.Definition{
				Key:       key,
				Kind:      fileindex.DefVariable,
				Name:      p.text(name),
				StartLine: lineOf(statement),
				EndLine:   int(statement.EndPoint().Row) + 1,
				Values:    p.targets(p.cg.Values(key)),
			}
		}
	}
}

// definitionName returns the last segment of a key. The module is the base
// name of the file with no extension, as parse for src/parse.py.
func definitionName(key, file string) string {
	if key == file {
		base := path.Base(file)
		return strings.TrimSuffix(base, path.Ext(base))
	}
	return fileindex.Last(key)
}

// fillDefinition sets the parent class, the method kind and the decorators.
func (p *projection) fillDefinition(def *fileindex.Definition) {
	if def.Kind == fileindex.DefModule {
		return
	}
	// A function with an anonymous name in a class body, as an argument in
	// a static block, is no method of the class.
	scope := fileindex.Parent(def.Key)
	if parent, ok := p.defs[scope]; ok && (parent.Kind == fileindex.DefClass || parent.Kind == fileindex.DefInterface) && !fileindex.IsAnonymous(def.Name) {
		def.Parent = scope
		if def.Kind == fileindex.DefFunction {
			def.Kind = fileindex.DefMethod
		}
		if def.Kind == fileindex.DefMethod && def.Name == p.rules().constructor {
			def.Kind = fileindex.DefConstructor
		}
	}
	if decorators := p.rules().decorators; decorators != nil {
		if node := p.syntax[def.Key]; node != nil {
			def.Decorators = decorators(p, node)
		}
	}
}

// decorators returns the decorators of a definition as written, with no
// arguments, as @app.route for @app.route("/"). A Python decorated
// definition and a JavaScript method hold their decorators. A TypeScript
// class body holds the decorators of a method before the method.
func decorators(p *projection, node *sitter.Node) []string {
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

// addClasses adds each class definition with its methods, its bases and
// its fields, and each interface with its bases and the methods that it
// declares. The bases come from the class node, as the decorators do.
func (p *projection) addClasses(_ context.Context) error {
	methods := map[string][]string{}
	for key, def := range p.defs {
		if def.Parent != "" {
			methods[def.Parent] = append(methods[def.Parent], key)
		}
	}
	for key, def := range p.defs {
		node := p.syntax[key]
		switch {
		case def.Kind == fileindex.DefInterface:
			p.ix.Classes = append(p.ix.Classes, fileindex.Class{Key: key, Bases: p.bases(node), Abstract: p.abstract(node)})
		case def.Kind != fileindex.DefClass:
		case node == nil:
			p.ix.Classes = append(p.ix.Classes, fileindex.Class{Key: key, Methods: methods[key]})
		default:
			p.ix.Classes = append(p.ix.Classes, fileindex.Class{Key: key, Bases: p.bases(node), Methods: methods[key], Fields: p.fields(node)})
		}
	}
	return nil
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
				if len(targets) == 0 && p.rules().rawUnbound {
					targets = []string{ref.CalleeNamespace}
				}
			}
			targets = p.throughRequire(ref.Receiver, targets)
			if local, ok := p.objectMember(key, targets); ok {
				targets = local
				p.members[[2]uint32{ref.Site.StartLine, ref.Site.StartColumn}] = true
			}
			p.ix.Calls = append(p.ix.Calls, fileindex.Call{
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

// objectMember returns the definition that a JavaScript call of a member of
// an object of the file names, as api//run for api.run() after
// const api = { run() {} }. The call graph keeps no value of an object
// literal, so its target has no unit. Only a target whose whole key is a
// definition changes.
func (p *projection) objectMember(caller string, targets []string) ([]string, bool) {
	if !p.ix.Language.IsECMAScript() || len(targets) != 1 || fileindex.HasKeyPrefix(targets[0], p.cg.FileName) {
		return nil, false
	}
	bound := p.rebind(caller, fileindex.Unit(targets[0]))
	if len(bound) != 1 || len(fileindex.Segments(targets[0])) == 0 {
		return nil, false
	}
	local := fileindex.Join(bound[0], fileindex.Segments(targets[0])...)
	if def := p.defs[local]; def == nil || def.Kind != fileindex.DefMethod && def.Kind != fileindex.DefFunction {
		return nil, false
	}
	return []string{local}, true
}

// caller returns the definition that runs the call site that the
// definition key holds. A function that a JavaScript property or argument
// holds, and the static initializer of a Java class, are definitions that
// the call graph does not know, so they take the calls of their nodes. A
// JavaScript static field or static block runs when the class is defined,
// so its calls belong to the scope that defines the class. A function
// value in a static field runs only when a call runs it. A flag in
// a static block stays on the class.
func (p *projection) caller(key string, site ast.NodePosition) string {
	def, ok := p.defs[key]
	class := ok && def.Kind == fileindex.DefClass && p.ix.Language.IsECMAScript()
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
			return fileindex.Parent(key)
		case "field_definition", "public_field_definition":
			if ts.HasKeyword(node, "static") {
				return fileindex.Parent(key)
			}
		}
	}
	return key
}

// isRequire reports whether a call is a CommonJS require of the builtin
// require. The import record of the call names the module, so the call
// itself makes no call record.
func (p *projection) isRequire(ref callgraph.CallReference) bool {
	return p.ix.Language.IsECMAScript() && ref.Text == requireFunction && slices.Equal(ref.Targets, []string{requireFunction})
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
		if rest, ok := strings.CutPrefix(t, "require"+fileindex.Separator); ok {
			t = fileindex.Join(fileindex.Namespace(p.ix.Language, m[1]), rest)
		}
		out = append(out, t)
	}
	return out
}

// callKind maps the kind of a call graph edge to the kind of a call. An
// edge that no call site makes has no call.
func callKind(kind callgraph.CallKind) (fileindex.CallKind, bool) {
	switch kind {
	case callgraph.CallCall:
		return fileindex.CallCall, true
	case callgraph.CallNew:
		return fileindex.CallNew, true
	case callgraph.CallImport:
		return fileindex.CallImport, true
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
	if name == "" || !fileindex.HasKeyPrefix(caller, p.cg.FileName) {
		return nil
	}
	for scope := caller; ; {
		if def, ok := p.defs[scope]; ok && (def.Kind != fileindex.DefClass || scope == caller || p.rules().memberScopes) {
			if _, bound := p.defs[fileindex.Join(scope, name)]; bound {
				return []string{fileindex.Join(scope, name)}
			}
		}
		if scope == p.cg.FileName {
			return nil
		}
		scope = fileindex.Parent(scope)
	}
}

func (p *projection) addUnresolved() {
	for _, u := range p.cg.Unresolved() {
		if p.members[[2]uint32{u.Site.StartLine, u.Site.StartColumn}] && u.Reason == callgraph.UnresolvedUnknownReceiver {
			continue
		}
		p.ix.Unresolved = append(p.ix.Unresolved, fileindex.Unresolved{
			Caller:   p.caller(u.Caller, u.Site),
			Line:     int(u.Site.StartLine),
			Column:   int(u.Site.StartColumn),
			Text:     u.Text,
			Receiver: u.Receiver,
			Method:   u.Method,
			Reason:   fileindex.UnresolvedReason(u.Reason),
		})
	}
}

func (p *projection) addTruncations() {
	for _, t := range p.cg.Truncations() {
		p.truncate(fileindex.TruncationKind(t.Kind), t.Namespace, t.Line)
	}
}

// truncate records the first hit of a kind of limit.
func (p *projection) truncate(kind fileindex.TruncationKind, namespace string, line int) {
	if slices.ContainsFunc(p.ix.Truncated, func(t fileindex.Truncation) bool { return t.Kind == kind }) {
		return
	}
	p.ix.Truncated = append(p.ix.Truncated, fileindex.Truncation{Kind: kind, Namespace: namespace, Line: line})
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

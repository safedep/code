package indexer

import (
	"context"
	"fmt"
	"path"
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

// project builds the call graph of tree and projects it into an index. It
// returns the call graph too, so the run can give it to a GraphPlugin.
func project(ctx context.Context, tree core.ParseTree) (*fileindex.Index, *callgraph.CallGraph, error) {
	language, err := tree.Language()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get language: %w", err)
	}
	data, err := tree.Data()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tree data: %w", err)
	}
	cg, err := callgraph.Build(ctx, tree)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}
	for _, def := range p.defs {
		def.Exported = p.exported(def)
		p.ix.Definitions = append(p.ix.Definitions, *def)
	}

	p.ix.Sort()
	return p.ix, cg, nil
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

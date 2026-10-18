package indexer

import (
	"slices"
	"strings"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// maxReferenceSegments bounds the member chain of a reference, as a.b.c.
const maxReferenceSegments = 8

// referenceSyntax names the node types of one language that hold a value.
// A holder maps to the field of its value, or to "" when each named child
// is a value.
type referenceSyntax struct {
	holders          map[string]string
	member           string
	object, property string
	identifiers      map[string]bool
	instances        map[string]bool
	// logical holds the operators of a binary node whose operands are
	// values, as a || b.
	logical map[string]bool
	// bases maps the type of a class to the field that names its bases, as
	// the argument list of class A(Base). A base is no value.
	bases map[string]string
	// lists hold values with no name of their own, as the expression list
	// of a, b := f, g. The last child of each list holds the value, so the
	// key of a keyed element is no value.
	lists map[string]bool
	// rawNames marks a language whose files name the classes of other
	// files of a unit with no import, as C#. A name that the file does not
	// bind keeps its text, and the link step resolves it or drops it.
	rawNames bool
}

// referenceVisitor adds each function, method, class or import that the
// file uses as a value and does not call, as yaml.load in run(yaml.load).
// The code that gets the value can call it. A name in an export statement
// or a CommonJS export is an export record, not a reference.
func referenceVisitor(p *projection) visitor {
	syntax := p.rules().references
	if syntax == nil {
		return nil
	}
	kinds := p.rules().kinds
	type open struct {
		key string
		end uint32
	}
	stack := []open{{key: p.cg.FileName, end: ^uint32(0)}}
	type site struct {
		line, column int
		caller, text string
	}
	seen := map[site]bool{}
	// bases holds the start of each list of class bases. The walk visits a
	// class before its bases, and a parent lookup costs a walk from the root.
	bases := map[uint32]bool{}
	return func(node *sitter.Node, kind string) {
		start := node.StartByte()
		for len(stack) > 1 && start >= stack[len(stack)-1].end {
			stack = stack[:len(stack)-1]
		}
		if _, definition := kinds[kind]; definition {
			if key, ok := p.spans[span{start, node.EndByte(), kind}]; ok {
				stack = append(stack, open{key: key, end: node.EndByte()})
			}
		}
		if field, ok := syntax.bases[kind]; ok {
			if list := node.ChildByFieldName(field); list != nil {
				bases[list.StartByte()] = true
			}
		}
		field, ok := syntax.holders[kind]
		switch {
		case !ok || bases[start]:
			return
		case kind == "binary_expression" && !syntax.logical[p.text(node.ChildByFieldName("operator"))]:
			return
		}
		values := ts.NamedChildren(node)
		if field != "" {
			values = []*sitter.Node{node.ChildByFieldName(field)}
		}
		caller := stack[len(stack)-1].key
		for _, value := range syntax.unlist(values) {
			target, ok := p.reference(*syntax, caller, value)
			if !ok {
				continue
			}
			at := ast.GetNodePosition(value)
			if k := (site{int(at.StartLine), int(at.StartColumn), caller, p.text(value)}); !seen[k] {
				seen[k] = true
				p.addReference(value, caller, p.targets([]string{target}))
			}
		}
	}
}

// addReference adds a reference from the definition that runs the code at
// the node to the targets. The caller is the definition that holds the
// node, and a static initializer or a property function that the call
// graph does not know takes it from there, as a call does. The text is
// bounded as the text of a record is.
func (p *projection) addReference(node *sitter.Node, caller string, targets []string) {
	site := ast.GetNodePosition(node)
	p.ix.References = append(p.ix.References, fileindex.Reference{
		Caller:  p.caller(caller, site),
		Line:    int(site.StartLine),
		Column:  int(site.StartColumn),
		Text:    clipText(p.text(node)),
		Targets: targets,
	})
}

// clipText bounds the text of a record as the call graph does.
func clipText(text string) string {
	const maxText = 120
	if len(text) <= maxText {
		return text
	}
	return strings.ToValidUTF8(text[:maxText], "")
}

// unlist replaces each list of the values with the value that it holds.
func (syntax referenceSyntax) unlist(values []*sitter.Node) []*sitter.Node {
	var out []*sitter.Node
	for _, value := range values {
		for depth := 0; value != nil && syntax.lists[value.Type()] && depth < maxReferenceSegments; depth++ {
			children := ts.NamedChildren(value)
			if value.Type() == "expression_list" {
				out = append(out, syntax.unlist(children)...)
				value = nil
				break
			}
			value = nil
			if len(children) > 0 {
				value = children[len(children)-1]
			}
		}
		if value != nil {
			out = append(out, value)
		}
	}
	return out
}

// reference returns the per-file target of a value. The value is a name
// or a member chain, and a definition, an import or the instance of a
// class binds its first name.
func (p *projection) reference(syntax referenceSyntax, caller string, value *sitter.Node) (string, bool) {
	if value == nil || p.inExport(value) && syntax.identifiers[value.Type()] {
		return "", false
	}
	var names []string
	root := value
	for root != nil && root.Type() == syntax.member && len(names) < maxReferenceSegments {
		property := root.ChildByFieldName(syntax.property)
		if property == nil {
			return "", false
		}
		names = append(names, p.text(property))
		root = root.ChildByFieldName(syntax.object)
	}
	if root == nil || !syntax.identifiers[root.Type()] && !syntax.instances[p.text(root)] {
		return "", false
	}
	slices.Reverse(names)

	unit, ok := p.valueOf(syntax, caller, p.text(root))
	switch {
	case !ok && syntax.rawNames && syntax.identifiers[root.Type()]:
		unit = p.text(root)
	case !ok:
		return "", false
	}
	// A Go parameter or local of a declared type holds a value of the
	// type, and a value is no function.
	if _, typed := p.typedScope(caller, p.text(root)); typed && len(names) == 0 {
		return "", false
	}
	return fileindex.Join(unit, names...), true
}

// valueOf returns the per-file target of a name: the definition that the
// scope chain binds, the namespace of an import, or the class of self or
// this. A typed parameter or local variable of a Go function is a name
// under the function, as the call graph keys it. Any other local variable
// or parameter has no target.
func (p *projection) valueOf(syntax referenceSyntax, caller, name string) (string, bool) {
	if syntax.instances[name] {
		for scope := caller; scope != p.cg.FileName; scope = fileindex.Parent(scope) {
			if def := p.defs[scope]; def != nil && def.Parent != "" {
				return def.Parent, true
			}
		}
		return "", false
	}
	if targets := p.rebind(caller, name); len(targets) > 0 {
		return targets[0], true
	}
	if scope, ok := p.typedScope(caller, name); ok {
		return fileindex.Join(scope, name), true
	}
	for _, imp := range p.ix.Imports {
		if imp.Alias == name && imp.Namespace != "" && imp.Kind != fileindex.ImportTypeOnly {
			return imp.Namespace, true
		}
	}
	return "", false
}

// inExport reports whether the node is part of an export statement or a
// CommonJS export assignment.
func (p *projection) inExport(node *sitter.Node) bool {
	return slices.ContainsFunc(p.exportStatements, func(statement *sitter.Node) bool {
		return node.StartByte() >= statement.StartByte() && node.EndByte() <= statement.EndByte()
	})
}

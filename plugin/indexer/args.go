package indexer

import (
	"github.com/safedep/code/core/ast"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

// argumentSyntax names the node types of the calls and the arguments of
// one language.
type argumentSyntax struct {
	calls    map[string]string
	skipped  map[string]bool
	strings  map[string]bool
	literals map[string]bool
}

// arguments returns the positional arguments of the call whose callee is at
// the site. A name or a member has the per-file targets of a reference, and
// a function the key of its definition when the index has one.
func (p *projection) arguments(caller string, site ast.NodePosition) []fileindex.Arg {
	rules := p.rules()
	if rules.arguments == nil || rules.references == nil {
		return nil
	}
	syntax, refs := *rules.arguments, *rules.references
	call := p.descendant(site)
	for call != nil {
		if field, ok := syntax.calls[call.Type()]; ok {
			if callee := call.ChildByFieldName(field); callee != nil && callee.StartByte() == site.StartByte {
				break
			}
		}
		if call.StartByte() != site.StartByte {
			return nil
		}
		call = call.Parent()
	}
	if call == nil {
		return nil
	}
	list := call.ChildByFieldName("arguments")
	if list == nil {
		return nil
	}
	var args []fileindex.Arg
	for _, node := range ts.NamedChildren(list) {
		if syntax.skipped[node.Type()] {
			continue
		}
		arg := fileindex.Arg{Shape: p.shape(syntax, refs, node)}
		switch arg.Shape {
		case fileindex.ArgName, fileindex.ArgMember:
			if target, ok := p.reference(refs, caller, node); ok {
				arg.Targets = p.targets([]string{target})
			}
		case fileindex.ArgFunction:
			if key, ok := p.spans[spanOf(node)]; ok {
				arg.Targets = []string{key}
			}
		}
		args = append(args, arg)
	}
	return args
}

// shape returns the shape of an argument. A string with an interpolation is
// a template.
func (p *projection) shape(syntax argumentSyntax, refs referenceSyntax, node *sitter.Node) fileindex.ArgShape {
	kind := node.Type()
	switch {
	case syntax.strings[kind]:
		if _, whole := p.stringPrefix(node); whole || kind == "concatenated_string" {
			return fileindex.ArgString
		}
		return fileindex.ArgTemplate
	case refs.identifiers[kind]:
		return fileindex.ArgName
	case kind == refs.member:
		return fileindex.ArgMember
	case functionValues[kind] || kind == "lambda":
		return fileindex.ArgFunction
	case syntax.literals[kind]:
		return fileindex.ArgLiteral
	}
	return fileindex.ArgComputed
}

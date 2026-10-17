package fileindex

import (
	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	fidx "github.com/safedep/code/fileindex"
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

var ecmascriptArguments = argumentSyntax{
	calls:    map[string]string{"call_expression": "function", "new_expression": "constructor"},
	skipped:  map[string]bool{"comment": true},
	strings:  map[string]bool{"string": true, "template_string": true},
	literals: map[string]bool{"number": true, "true": true, "false": true, "null": true, "undefined": true, "array": true, "object": true, "regex": true},
}

var argumentSyntaxes = map[core.LanguageCode]argumentSyntax{
	core.LanguageCodeGo: {
		calls:    map[string]string{"call_expression": "function"},
		skipped:  map[string]bool{"comment": true},
		strings:  map[string]bool{"interpreted_string_literal": true, "raw_string_literal": true},
		literals: map[string]bool{"int_literal": true, "float_literal": true, "imaginary_literal": true, "rune_literal": true, "true": true, "false": true, "nil": true, "composite_literal": true},
	},
	core.LanguageCodeJavascript: ecmascriptArguments,
	core.LanguageCodeTypescript: ecmascriptArguments,
	core.LanguageCodePython: {
		calls: map[string]string{"call": "function"},
		// A keyword argument has no position, so a rule that names a
		// position does not read it.
		skipped:  map[string]bool{"comment": true, "keyword_argument": true, "dictionary_splat": true},
		strings:  map[string]bool{"string": true, "concatenated_string": true},
		literals: map[string]bool{"integer": true, "float": true, "true": true, "false": true, "none": true, "list": true, "dictionary": true, "tuple": true, "set": true},
	},
}

// arguments returns the positional arguments of the call whose callee is at
// the site. A name or a member has the per-file targets of a reference, and
// a function the key of its definition when the index has one.
func (p *projection) arguments(caller string, site ast.NodePosition) []fidx.Arg {
	syntax, ok := argumentSyntaxes[p.ix.Language]
	refs, hasRefs := referenceSyntaxes[p.ix.Language]
	if !ok || !hasRefs {
		return nil
	}
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
	var args []fidx.Arg
	for _, node := range ts.NamedChildren(list) {
		if syntax.skipped[node.Type()] {
			continue
		}
		arg := fidx.Arg{Shape: p.shape(syntax, refs, node)}
		switch arg.Shape {
		case fidx.ArgName, fidx.ArgMember:
			if target, ok := p.reference(refs, caller, node); ok {
				arg.Targets = p.targets([]string{target})
			}
		case fidx.ArgFunction:
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
func (p *projection) shape(syntax argumentSyntax, refs referenceSyntax, node *sitter.Node) fidx.ArgShape {
	kind := node.Type()
	switch {
	case syntax.strings[kind]:
		if _, whole := p.stringPrefix(node); whole || kind == "concatenated_string" {
			return fidx.ArgString
		}
		return fidx.ArgTemplate
	case refs.identifiers[kind]:
		return fidx.ArgName
	case kind == refs.member:
		return fidx.ArgMember
	case functionValues[kind] || kind == "lambda":
		return fidx.ArgFunction
	case syntax.literals[kind]:
		return fidx.ArgLiteral
	}
	return fidx.ArgComputed
}

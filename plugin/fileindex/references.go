package fileindex

import (
	"context"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	fidx "github.com/safedep/code/fileindex"
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
	// bases holds the parent types of a holder that names the bases of a
	// class, as the argument list of class A(Base). A base is no value.
	bases map[string]bool
}

var referenceSyntaxes = map[core.LanguageCode]referenceSyntax{
	core.LanguageCodePython: {
		holders: map[string]string{
			"argument_list":            "",
			"keyword_argument":         "value",
			"list":                     "",
			"tuple":                    "",
			"set":                      "",
			"expression_list":          "",
			"pair":                     "value",
			"assignment":               "right",
			"return_statement":         "",
			"default_parameter":        "value",
			"typed_default_parameter":  "value",
			"conditional_expression":   "",
			"boolean_operator":         "",
			"parenthesized_expression": "",
		},
		member:      "attribute",
		object:      "object",
		property:    "attribute",
		identifiers: map[string]bool{"identifier": true},
		instances:   map[string]bool{"self": true, "cls": true},
		bases:       map[string]bool{"class_definition": true},
	},
	core.LanguageCodeJavascript: ecmascriptReferences,
	core.LanguageCodeTypescript: ecmascriptReferences,
}

var ecmascriptReferences = referenceSyntax{
	holders: map[string]string{
		"arguments":                "",
		"array":                    "",
		"object":                   "",
		"pair":                     "value",
		"variable_declarator":      "value",
		"assignment_expression":    "right",
		"assignment_pattern":       "right",
		"return_statement":         "",
		"ternary_expression":       "",
		"binary_expression":        "",
		"parenthesized_expression": "",
		"export_statement":         "value",
	},
	member:      "member_expression",
	object:      "object",
	property:    "property",
	identifiers: map[string]bool{"identifier": true, "shorthand_property_identifier": true},
	instances:   map[string]bool{"this": true},
	logical:     map[string]bool{"||": true, "??": true, "&&": true},
}

// addReferences adds each function, method, class or import that the file
// uses as a value and does not call, as yaml.load in run(yaml.load). The
// code that gets the value can call it. A name in an export statement or a
// CommonJS export is an export record, not a reference.
func (p *projection) addReferences(ctx context.Context) {
	syntax, ok := referenceSyntaxes[p.ix.Language]
	if !ok {
		return
	}
	type span struct {
		start, end uint32
		kind       string
	}
	spanOf := func(node *sitter.Node) span { return span{node.StartByte(), node.EndByte(), node.Type()} }
	defs := make(map[span]string, len(p.syntax))
	for key, node := range p.syntax {
		if node != nil {
			defs[spanOf(node)] = key
		}
	}

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
	err := ts.Walk(ctx, p.tree.Tree().RootNode(), ts.WalkOptions{}, func(node *sitter.Node) bool {
		for len(stack) > 1 && node.StartByte() >= stack[len(stack)-1].end {
			stack = stack[:len(stack)-1]
		}
		if key, ok := defs[spanOf(node)]; ok {
			stack = append(stack, open{key: key, end: node.EndByte()})
		}
		field, ok := syntax.holders[node.Type()]
		switch {
		case !ok:
			return true
		case node.Type() == "binary_expression" && !syntax.logical[p.text(node.ChildByFieldName("operator"))]:
			return true
		case node.Parent() != nil && syntax.bases[node.Parent().Type()]:
			return true
		}
		values := ts.NamedChildren(node)
		if field != "" {
			values = []*sitter.Node{node.ChildByFieldName(field)}
		}
		caller := stack[len(stack)-1].key
		for _, value := range values {
			ref, ok := p.reference(syntax, caller, value)
			if k := (site{ref.Line, ref.Column, ref.Caller, ref.Text}); ok && !seen[k] {
				seen[k] = true
				p.ix.References = append(p.ix.References, ref)
			}
		}
		return true
	})
	p.truncateWalk(err)
}

// reference returns the reference of a value. The value is a name or a
// member chain, and a definition, an import or the instance of a class
// binds its first name.
func (p *projection) reference(syntax referenceSyntax, caller string, value *sitter.Node) (fidx.Reference, bool) {
	if value == nil || p.inExport(value) && syntax.identifiers[value.Type()] {
		return fidx.Reference{}, false
	}
	var names []string
	root := value
	for root != nil && root.Type() == syntax.member && len(names) < maxReferenceSegments {
		property := root.ChildByFieldName(syntax.property)
		if property == nil {
			return fidx.Reference{}, false
		}
		names = append(names, p.text(property))
		root = root.ChildByFieldName(syntax.object)
	}
	if root == nil || !syntax.identifiers[root.Type()] && !syntax.instances[p.text(root)] {
		return fidx.Reference{}, false
	}
	slices.Reverse(names)

	unit, ok := p.valueOf(syntax, caller, p.text(root))
	if !ok {
		return fidx.Reference{}, false
	}
	site := ast.GetNodePosition(value)
	return fidx.Reference{
		Caller:  p.caller(caller, site),
		Line:    int(site.StartLine),
		Column:  int(site.StartColumn),
		Text:    p.text(value),
		Targets: p.targets([]string{fidx.Join(unit, names...)}),
	}, true
}

// valueOf returns the per-file target of a name: the definition that the
// scope chain binds, the namespace of an import, or the class of self or
// this. A local variable or a parameter has no target.
func (p *projection) valueOf(syntax referenceSyntax, caller, name string) (string, bool) {
	if syntax.instances[name] {
		for scope := caller; scope != p.cg.FileName; scope = fidx.Parent(scope) {
			if def := p.defs[scope]; def != nil && def.Parent != "" {
				return def.Parent, true
			}
		}
		return "", false
	}
	if targets := p.rebind(caller, name); len(targets) > 0 {
		return targets[0], true
	}
	for _, imp := range p.ix.Imports {
		if imp.Alias == name && imp.Namespace != "" && imp.Kind != fidx.ImportTypeOnly {
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

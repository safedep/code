package indexer

import (
	"regexp"
	"slices"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/pkg/ts"
	sitter "github.com/smacker/go-tree-sitter"
)

var pythonRules = languageRules{
	kinds: map[string]fileindex.DefKind{
		"class_definition":    fileindex.DefClass,
		"function_definition": fileindex.DefFunction,
	},
	constructor: "__init__",
	bases:       pythonBases,
	decorators:  decorators,
	assigned:    pythonAssigned,
	exports:     pythonExports,
	visitors:    []func(p *projection) visitor{flagVisitor, mainGuardVisitor, referenceVisitor},
	dynamic: &dynamicSyntax{
		call:           "call",
		assignment:     "assignment",
		subscript:      "subscript",
		subscriptIndex: "subscript",
		members:        map[string]string{"attribute": "object", "subscript": "value"},
		wrappers:       map[string]bool{"parenthesized_expression": true},
		eval:           map[string]bool{"eval": true, "exec": true},
		importers:      map[string]bool{"importlib.import_module": true, "import_module": true, "__import__": true},
		reflection:     func(callee string) bool { return slices.Contains([]string{"getattr", "setattr", "delattr"}, callee) },
		reflectionName: 1,
		moduleGetattr:  "__getattr__",
		// A Python subscript is a lookup in a dict, so a string index
		// computes the callee too.
		stringIndexComputes: true,
	},
	references: &referenceSyntax{
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
		bases:       map[string]string{"class_definition": "superclasses"},
	},
	arguments: &argumentSyntax{
		calls: map[string]string{"call": "function"},
		// A keyword argument has no position, so a rule that names a
		// position does not read it.
		skipped:  map[string]bool{"comment": true, "keyword_argument": true, "dictionary_splat": true},
		strings:  map[string]bool{"string": true, "concatenated_string": true},
		literals: map[string]bool{"integer": true, "float": true, "true": true, "false": true, "none": true, "list": true, "dictionary": true, "tuple": true, "set": true},
	},
	mainGuard: &mainGuard{statement: "if_statement", condition: regexp.MustCompile(`^(__name__\s*==\s*["']__main__["']|["']__main__["']\s*==\s*__name__)$`)},
}

// pythonBases returns the bases of a Python class as written, as Base and
// mod.Base in class A(Base, mod.Base, metaclass=M). A keyword argument, as
// the metaclass, is no base.
func pythonBases(p *projection, class *sitter.Node) []string {
	if class.Type() != "class_definition" {
		return nil
	}
	var bases []string
	if list := class.ChildByFieldName("superclasses"); list != nil {
		for _, base := range ts.NamedChildren(list) {
			if base.Type() != "keyword_argument" {
				bases = append(bases, p.text(base))
			}
		}
	}
	return bases
}

// pythonAssigned returns the name of a top-level assignment, as x in x = f().
func pythonAssigned(_ *projection, statement *sitter.Node) []*sitter.Node {
	if statement.Type() != "expression_statement" || statement.NamedChildCount() == 0 {
		return nil
	}
	if assignment := statement.NamedChild(0); assignment.Type() == "assignment" {
		return []*sitter.Node{assignment.ChildByFieldName("left")}
	}
	return nil
}

// pythonExports adds the names of the __all__ of the file as its exports.
func pythonExports(p *projection) {
	for _, statement := range ts.NamedChildren(p.tree.Tree().RootNode()) {
		if statement.Type() != "expression_statement" || statement.NamedChildCount() == 0 {
			continue
		}
		assignment := statement.NamedChild(0)
		left, right := assignment.ChildByFieldName("left"), assignment.ChildByFieldName("right")
		if left != nil && right != nil && left.Type() == "identifier" && p.text(left) == "__all__" {
			p.addAll(right, lineOf(statement))
		}
	}
}

// addAll adds the names of __all__ when it is a list or a tuple of string
// literals. A computed __all__ gives no export.
func (p *projection) addAll(value *sitter.Node, line int) {
	if value.Type() != "list" && value.Type() != "tuple" {
		return
	}
	var names []string
	for _, item := range ts.NamedChildren(value) {
		name, whole := p.stringPrefix(item)
		if !whole {
			return
		}
		names = append(names, name)
	}
	for _, name := range names {
		p.export(name, name, line)
	}
}

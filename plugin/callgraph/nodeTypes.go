package callgraph

import (
	"github.com/safedep/code/core"
	sitter "github.com/smacker/go-tree-sitter"
)

var literalNodeTypes = map[string]bool{
	"string":                         true,
	"encapsed_string":                true,
	"string_literal":                 true,
	"number":                         true,
	"decimal_integer_literal":        true,
	"hex_integer_literal":            true,
	"octal_integer_literal":          true,
	"binary_integer_literal":         true,
	"decimal_floating_point_literal": true,
	"hex_floating_point_literal":     true,
	"integer":                        true,
	"float":                          true,
	"double":                         true,
	"character":                      true,
	"character_literal":              true,
	"boolean":                        true,
	"null":                           true,
	"none":                           true,
	"undefined":                      true,
	"true":                           true,
	"false":                          true,
	// Go literals
	"int_literal":                true,
	"float_literal":              true,
	"imaginary_literal":          true,
	"rune_literal":               true,
	"raw_string_literal":         true,
	"interpreted_string_literal": true,
}

var initialisedDataStructures = map[string]bool{
	"list":       true,
	"set":        true,
	"dictionary": true,
	"tuple":      true,
	"array":      true,
	"map":        true,
	"object":     true,
	// Go data structures
	"composite_literal": true,
	"slice_expression":  true,
}

// pythonLiteralReceivers and jsLiteralReceivers map a literal node to the
// builtin type of its value, so that "a,b".split() calls String//split.
var pythonLiteralReceivers = map[string]string{
	"string":                   "str",
	"concatenated_string":      "str",
	"integer":                  "int",
	"float":                    "float",
	"list":                     "list",
	"list_comprehension":       "list",
	"dictionary":               "dict",
	"dictionary_comprehension": "dict",
	"set":                      "set",
	"set_comprehension":        "set",
	"tuple":                    "tuple",
}

var jsLiteralReceivers = map[string]string{
	"string":          "String",
	"template_string": "String",
	"number":          "Number",
	"array":           "Array",
	"object":          "Object",
	"regex":           "RegExp",
}

// transparentNodes are the nodes whose value is the value of one child, as
// (x) or await x. The resolvers read through them.
var transparentNodes = map[core.LanguageCode]map[string]bool{
	core.LanguageCodePython:     {"parenthesized_expression": true},
	core.LanguageCodeGo:         {"parenthesized_expression": true},
	core.LanguageCodeJavascript: jsTransparentNodes,
	core.LanguageCodeTypescript: jsTransparentNodes,
}

var jsTransparentNodes = map[string]bool{
	"parenthesized_expression": true,
	"await_expression":         true,
	"sequence_expression":      true,
	"non_null_expression":      true,
	"as_expression":            true,
	"satisfies_expression":     true,
}

// unwrap returns the node that gives the value of node. A sequence gives
// the value of its last expression, as fn in (0, fn)().
func (cg *CallGraph) unwrap(node *sitter.Node) *sitter.Node {
	transparent := transparentNodes[cg.languageCode]
	for depth := 0; node != nil && transparent[node.Type()] && node.NamedChildCount() > 0 && depth <= maxMemberExpressionDepth; depth++ {
		if node.Type() == "sequence_expression" {
			node = node.NamedChild(int(node.NamedChildCount()) - 1)
			continue
		}
		node = node.NamedChild(0)
	}
	return node
}

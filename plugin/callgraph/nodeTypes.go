package callgraph

import sitter "github.com/smacker/go-tree-sitter"

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
	"int_literal":                    true,
	"float_literal":                  true,
	"imaginary_literal":              true,
	"rune_literal":                   true,
	"raw_string_literal":             true,
	"interpreted_string_literal":     true,
}

// initialisedDataStructures are the literals of a collection. The items can
// hold calls, but the collection is no value of a name.
var initialisedDataStructures = map[string]bool{
	"list":       true,
	"set":        true,
	"dictionary": true,
	"tuple":      true,
	"array":      true,
	"map":        true,
	"object":     true,
}

// ignoredNodeTypes are the imports, comments and punctuation. The resolvers
// read the imports, and the others hold no call.
var ignoredNodeTypes = map[string]bool{
	"import_statement": true, "import": true, "import_from_statement": true, "import_declaration": true,
	"comment": true, "whitespace": true, "newline": true, "line_comment": true,
	"+": true, "-": true, "*": true, "/": true, "%": true, "**": true, "//": true,
	"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true,
	",": true, ":": true, ";": true, ".": true, "(": true, ")": true, "{": true, "}": true, "[": true, "]": true,
}

// unwrap returns the node that gives the value of node. A sequence gives
// the value of its last expression, as fn in (0, fn)().
func (cg *CallGraph) unwrap(node *sitter.Node) *sitter.Node {
	transparent := cg.profile.transparent
	for depth := 0; node != nil && transparent[node.Type()] && node.NamedChildCount() > 0 && depth <= maxMemberExpressionDepth; depth++ {
		if node.Type() == "sequence_expression" {
			node = node.NamedChild(int(node.NamedChildCount()) - 1)
			continue
		}
		node = node.NamedChild(0)
	}
	return node
}

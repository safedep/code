package callgraph

var literalNodeTypes = map[string]bool{
	"string":                         true,
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
}

var initialisedDataStructures = map[string]bool{
	"list":       true,
	"set":        true,
	"dictionary": true,
	"tuple":      true,
	"array":      true,
	"map":        true,
	"object":     true,
}

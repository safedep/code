package callgraph

var literalNodeTypes = map[string]bool{
	"string":         true,
	"string_literal": true,
	"number":         true,
	"integer":        true,
	"float":          true,
	"double":         true,
	"boolean":        true,
	"null":           true,
	"none":           true,
	"undefined":      true,
	"true":           true,
	"false":          true,
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

package callgraph

// TS nodes Ignored when parsing AST
// eg. comment is useless, imports are already resolved
var ignoredTypesList = []string{"comment"}
var ignoredTypes = make(map[string]bool)

func init() {
	for _, ignoredType := range ignoredTypesList {
		ignoredTypes[ignoredType] = true
	}
}

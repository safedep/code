package depsusage

// TS nodes Ignored when parsing AST
// eg. comment is useless, imports are already resolved
var ignoredTypesList = []string{"comment", "import_statement", "import_from_statement"}
var ignoredTypes = make(map[string]bool)

func init() {
	for _, ignoredType := range ignoredTypesList {
		ignoredTypes[ignoredType] = true
	}
}

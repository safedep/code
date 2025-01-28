package callgraph

import "strings"

// @TODO - Refactor this for a language agnostic approach

// GetBaseModuleName returns the base module name from the given module name.
// eg. for "os.path", the base module name is "os"
func GetBaseModuleName(moduleName string) string {
	parts := strings.Split(moduleName, ".")
	return parts[0]
}

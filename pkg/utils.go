package pkg

import "strings"

// generate docs -

// GetBaseModuleName returns the base module name from the given module name.
// eg. for "os.path", the base module name is "os"
func GetBaseModuleName(moduleName string) string {
	parts := strings.Split(moduleName, ".")
	return parts[0]
}

// GetFirstNonEmptyString returns the first non-empty string from the given list of strings.
// eg. for GetFirstNonEmptyString("", "", "foo", "bar"), it returns "foo"
func GetFirstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "" // return empty string if none are non-empty
}

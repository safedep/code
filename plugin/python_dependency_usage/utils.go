package python_dependency_usage

import (
	"sort"
	"strings"
)

func getBaseModuleName(moduleName string) string {
	parts := strings.Split(moduleName, ".")
	return parts[0] // Just return the first part, the base name
}

func getFirstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "" // return empty string if none are non-empty
}

// Only for debugging
func getSortedKeys[K comparable, V any](mapData map[K]V, less func(K, K) bool) []K {
	keys := make([]K, 0, len(mapData))
	for key := range mapData {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return less(keys[i], keys[j])
	})
	return keys
}

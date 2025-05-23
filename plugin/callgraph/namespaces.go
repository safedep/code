package callgraph

import "strings"

func resolveRootQualifier(namespace string) string {
	parts := strings.Split(namespace, namespaceSeparator)

	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

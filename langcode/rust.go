package langcode

import (
	"slices"
	"strings"
)

// RustFromFile writes a path that starts with self or super in an inline
// module as a path from the module of the file, as self::outer::x for
// super::x in mod outer { mod inner { .. } }. mods holds the names of the
// inline modules, the outermost first. The index reads such a path from the
// module of the file. So self::x in mod m named the x of the file, and a
// call through it lost its edge to m::x.
func RustFromFile(path string, mods []string) string {
	parts := strings.Split(path, "::")
	ups := 0
	for ups < len(parts) && parts[ups] == "super" {
		ups++
	}
	switch {
	case parts[0] == "self":
		return strings.Join(slices.Concat([]string{"self"}, mods, parts[1:]), "::")
	case ups == 0:
		return path
	case ups > len(mods):
		return strings.Join(slices.Concat(slices.Repeat([]string{"super"}, ups-len(mods)), parts[ups:]), "::")
	}
	return strings.Join(slices.Concat([]string{"self"}, mods[:len(mods)-ups], parts[ups:]), "::")
}

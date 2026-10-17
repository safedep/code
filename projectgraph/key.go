package projectgraph

import "strings"

// Separator joins the segments of a symbol key, as fileindex keys do.
const Separator = "//"

// Join returns the key of the segments under the unit.
func Join(unit string, segments ...string) string {
	if len(segments) == 0 {
		return unit
	}
	return unit + Separator + strings.Join(segments, Separator)
}

// Unit returns the first part of a key: a file path or an import namespace.
func Unit(key string) string {
	unit, _, _ := strings.Cut(key, Separator)
	return unit
}

// Segments returns the parts of a key after its unit.
func Segments(key string) []string {
	_, rest, ok := strings.Cut(key, Separator)
	if !ok {
		return nil
	}
	return strings.Split(rest, Separator)
}

// Last returns the last segment of a key, or the unit of a key with no
// segment.
func Last(key string) string {
	if i := strings.LastIndex(key, Separator); i >= 0 {
		return key[i+len(Separator):]
	}
	return key
}

// HasKeyPrefix reports whether key is prefix or a key under prefix.
func HasKeyPrefix(key, prefix string) bool {
	return key == prefix || strings.HasPrefix(key, prefix+Separator)
}

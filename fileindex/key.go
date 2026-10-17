package fileindex

import (
	"strconv"
	"strings"

	"github.com/safedep/code/langcode"
)

// Separator joins the segments of a symbol key, as plugin/callgraph does.
const Separator = "//"

// Join returns the key of the segments under the unit.
func Join(unit string, segments ...string) string {
	if len(segments) == 0 {
		return unit
	}
	return unit + Separator + strings.Join(segments, Separator)
}

// JoinNonEmpty returns the key of the segment under the unit, or the unit
// for an empty segment, as the whole module of a re-export with no item.
func JoinNonEmpty(unit, segment string) string {
	if segment == "" {
		return unit
	}
	return Join(unit, segment)
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

// Parent returns the key with no last segment, or the key itself when it
// has no segment.
func Parent(key string) string {
	if i := strings.LastIndex(key, Separator); i >= 0 {
		return key[:i]
	}
	return key
}

// HasKeyPrefix reports whether key is prefix or a key under prefix.
func HasKeyPrefix(key, prefix string) bool {
	return key == prefix || strings.HasPrefix(key, prefix+Separator)
}

// Namespace returns the key form of a module as the source writes it, as
// yaml//loader for yaml.loader in Python, java//util for java.util in Java,
// fs//promises for fs/promises in JavaScript or App//Kernel for \App\Kernel
// in PHP. It keeps the leading dots of a relative module.
func Namespace(lang langcode.Code, module string) string {
	sep := "/"
	switch lang {
	case langcode.Python, langcode.Java, langcode.Kotlin, langcode.CSharp:
		sep = "."
	case langcode.PHP:
		sep, module = `\`, strings.TrimPrefix(module, `\`)
	case langcode.Rust:
		sep = "::"
	case langcode.Ruby:
		module = strings.ReplaceAll(module, "::", "/")
	}
	return strings.ReplaceAll(module, sep, Separator)
}

// anonymousPrefix starts the name of a definition that has no name in the
// source.
const anonymousPrefix = "<anonymous:"

// Anonymous returns the name of the n-th definition from 1 with no name in
// the source that starts on the line, as <anonymous:12> or <anonymous:12#2>.
// The index names a function literal that a call takes as an argument, and
// a class expression, so a root rule can mark it and its calls have a
// caller of their own.
func Anonymous(line, n int) string {
	name := anonymousPrefix + strconv.Itoa(line)
	if n > 1 {
		name += "#" + strconv.Itoa(n)
	}
	return name + ">"
}

// IsAnonymous reports whether a name is a name that Anonymous returns.
func IsAnonymous(name string) bool {
	return strings.HasPrefix(name, anonymousPrefix)
}

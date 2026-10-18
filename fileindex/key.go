package fileindex

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

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

// Unit returns the first part of a key: a file path, an import namespace,
// or the package unit and the path of a file of dependency code, as
// npm:lodash@4.17.21//template.js.
func Unit(key string) string {
	unit, _ := cutUnit(key)
	return unit
}

// Segments returns the parts of a key after its unit.
func Segments(key string) []string {
	_, rest := cutUnit(key)
	if rest == "" {
		return nil
	}
	return strings.Split(rest, Separator)
}

// cutUnit returns the unit of a key and the segments after it, as one
// string.
func cutUnit(key string) (string, string) {
	first, rest, ok := strings.Cut(key, Separator)
	if !ok {
		return key, ""
	}
	if IsPackageUnit(first) {
		path, after, _ := strings.Cut(rest, Separator)
		return first + Separator + path, after
	}
	return first, rest
}

// Last returns the last segment of a key, or the unit of a key with no
// segment.
func Last(key string) string {
	unit, rest := cutUnit(key)
	if rest == "" {
		return unit
	}
	if i := strings.LastIndex(rest, Separator); i >= 0 {
		return rest[i+len(Separator):]
	}
	return rest
}

// Parent returns the key with no last segment, or the key itself when it
// has no segment.
func Parent(key string) string {
	unit, rest := cutUnit(key)
	if rest == "" {
		return unit
	}
	if i := strings.LastIndex(rest, Separator); i >= 0 {
		return unit + Separator + rest[:i]
	}
	return unit
}

// PackageUnit names one version of one package of dependency code, as
// npm:lodash@4.17.21, pypi:requests@2.31.0 or, for a Go module, as
// "go:github.com/google/uuid@v1.6.0". The name of an npm package can have
// a scope, as npm:@babel/core@7.24.0.
type PackageUnit struct {
	Ecosystem string
	Name      string
	Version   string
}

// String returns the unit as a key writes it.
func (u PackageUnit) String() string {
	return u.Ecosystem + ":" + u.Name + "@" + u.Version
}

// ParseUnit reads a package unit, as npm:lodash@4.17.21. The ecosystem is a
// lower-case word. The name has no "//" and no space. The version follows
// the last "@" and has no "/".
func ParseUnit(s string) (PackageUnit, error) {
	eco, rest, ok := strings.Cut(s, ":")
	at := strings.LastIndex(rest, "@")
	if !ok || !isEcosystem(eco) || at <= 0 {
		return PackageUnit{}, fmt.Errorf("unit %q is not ecosystem:name@version", s)
	}
	u := PackageUnit{Ecosystem: eco, Name: rest[:at], Version: rest[at+1:]}
	switch {
	case strings.HasPrefix(u.Name, ":") || strings.Contains(u.Name, Separator) || strings.ContainsFunc(u.Name, unicode.IsSpace):
		return PackageUnit{}, fmt.Errorf("unit %q has a bad name", s)
	case u.Version == "" || strings.ContainsAny(u.Version, "/@") || strings.ContainsFunc(u.Version, unicode.IsSpace):
		return PackageUnit{}, fmt.Errorf("unit %q has a bad version", s)
	}
	return u, nil
}

// IsPackageUnit reports whether s is a package unit, as ParseUnit reads it.
// A path of the project or an import namespace is not one, unless a file at
// the root of the project has a name of that form.
func IsPackageUnit(s string) bool {
	if !strings.Contains(s, "@") || !strings.Contains(s, ":") {
		return false
	}
	_, err := ParseUnit(s)
	return err == nil
}

func isEcosystem(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool { return (r < 'a' || r > 'z') && (r < '0' || r > '9') })
}

// UnitPath returns the path of a file of a package unit, as
// npm:lodash@4.17.21//template.js. A file of the project has no unit, and
// its path stays as it is.
func UnitPath(unit, path string) string {
	if unit == "" {
		return path
	}
	return unit + Separator + path
}

// SplitUnit returns the package unit of a path of dependency code and the
// path inside the package, as UnitPath joins them. A path of the project
// has no unit.
func SplitUnit(path string) (string, string) {
	if first, rest, ok := strings.Cut(path, Separator); ok && IsPackageUnit(first) {
		return first, rest
	}
	return "", path
}

// PackageOf returns the package unit of a key of dependency code, as
// npm:lodash@4.17.21 for npm:lodash@4.17.21//template.js//template, and ""
// for any other key.
func PackageOf(key string) string {
	unit, _ := SplitUnit(key)
	if unit == "" && IsPackageUnit(key) {
		return key
	}
	return unit
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
	switch lang {
	case langcode.PHP:
		module = strings.TrimPrefix(module, `\`)
	case langcode.Ruby:
		module = strings.ReplaceAll(module, "::", "/")
	}
	return strings.ReplaceAll(module, lang.ModuleSeparator(), Separator)
}

// WithoutTypeArguments removes each type argument list from a name, as in
// List<string>, Map<K, V>.Entry, serde_json::from_str::<Value> or
// Hmac::<Sha256>::new. The text around the arguments stays as written.
func WithoutTypeArguments(name string) string {
	if !strings.Contains(name, "<") {
		return name
	}
	var b strings.Builder
	depth := 0
	for _, r := range name {
		switch {
		case r == '<':
			depth++
		case r == '>' && depth > 0:
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
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

package projectgraph

import (
	"maps"
	"slices"
	"strings"

	"github.com/safedep/code/langcode"
)

// namespaces is the table of the units of the languages whose files declare
// a package or a namespace, as Java, C# and PHP. The files of one package
// are one unit. A qualified name names a unit by its longest prefix, and a
// class and a member under it.
type namespaces struct {
	sep   string
	units map[string]*unit
}

// addNamespaceUnits groups the files of the languages by the package of
// their index into the table of the family.
func (b *builder) addNamespaceUnits(family langcode.Code, sep string, langs ...langcode.Code) *namespaces {
	files := map[string][]string{}
	for _, p := range b.paths {
		if ix := b.byPath[p]; ix != nil && slices.Contains(langs, ix.Language) {
			files[ix.Package] = append(files[ix.Package], p)
		}
	}
	t := &namespaces{sep: sep, units: map[string]*unit{}}
	for _, pkg := range slices.Sorted(maps.Keys(files)) {
		t.units[pkg] = b.addUnit(string(family)+":"+pkg, files[pkg])
	}
	b.namespaces[family] = t
	return t
}

// resolveQualified resolves a qualified name of the family, as
// com.x.Service, com.x.Outer.Inner or App\Http\Kernel, to the unit of a
// package or to a class of the project and a member under it. The longest
// package that the name starts with wins. The default package has no name,
// so no import names it.
func (b *builder) resolveQualified(family langcode.Code, name string) (string, bool) {
	t := b.namespaces[family]
	if t == nil {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(name, t.sep), t.sep)
	for i := len(parts); i > 0; i-- {
		u := t.units[strings.Join(parts[:i], t.sep)]
		if u == nil {
			continue
		}
		if i == len(parts) {
			return u.key, true
		}
		// A top-level property of Kotlin has no value that a walk can
		// read, but an import names it.
		if keys := u.names[parts[i]]; i == len(parts)-1 && len(keys) > 0 {
			return keys[0], true
		}
		if r := b.walk(u.key, parts[i:]); r.fail == nil {
			return r.key, true
		}
		return "", false
	}
	return "", false
}

// unitMember returns what a name under a unit names when no definition of
// the unit has the name: a unit of a package under it, as com.x.y under
// com.x, or the target of a use of a file of the unit, as transmit for pub
// use hidden::send as transmit in Rust.
func (b *builder) unitMember(key, name string) (string, bool) {
	family, pkg, ok := strings.Cut(key, ":")
	t := b.namespaces[langcode.Code(family)]
	if !ok || t == nil {
		return "", false
	}
	if u := t.units[pkg+t.sep+name]; u != nil {
		return u.key, true
	}
	if !profileOf(langcode.Code(family)).unitImports {
		return "", false
	}
	for _, p := range b.units[key].files {
		sc := b.scopes[p]
		if sc == nil {
			continue
		}
		if imp, ok := sc.bindings[name]; ok {
			module, lead := importedModule(sc.ix.Language, imp)
			if mod, _, found := b.resolveModule(sc.ix, module); found {
				if r := b.walk(mod, lead); r.fail == nil {
					return r.key, true
				}
			}
		}
		if keys := b.wildcardImported(sc, name); len(keys) > 0 {
			return keys[0], true
		}
	}
	return "", false
}

// firstParty reports whether a qualified name starts with a package of the
// project. It looks up each prefix of the name, because a scan of the
// packages at each import grows with the square of the project.
func (t *namespaces) firstParty(name string) bool {
	if t == nil {
		return false
	}
	for prefix := strings.TrimPrefix(name, t.sep); prefix != ""; {
		if _, ok := t.units[prefix]; ok {
			return true
		}
		i := strings.LastIndex(prefix, t.sep)
		if i < 0 {
			return false
		}
		prefix = prefix[:i]
	}
	return false
}

// namespaceResolver resolves an import of a language of a namespace table
// to a class or a package of the project.
type namespaceResolver struct {
	b      *builder
	family langcode.Code
}

func (r namespaceResolver) Module(_, spec string) (string, string, bool) {
	if key, ok := r.b.resolveQualified(r.family, spec); ok {
		return key, "class", true
	}
	return "", "", false
}

// FirstParty reports whether a qualified name starts with a package of the
// project.
func (r namespaceResolver) FirstParty(spec string) bool {
	return r.b.namespaces[r.family].firstParty(spec)
}

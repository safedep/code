package projectgraph

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

// unit is a set of files that share one namespace, as a Go package. A name
// that a file of the unit defines at its top level is in scope in every
// file of the unit. The members of a type can come from more than one file
// of the unit, as the methods of a Go type. Two files of the unit can define
// one name, as two files under other build tags, and a call of the name
// then reaches both.
type unit struct {
	key     string
	files   []string
	names   map[string][]string
	members map[string]map[string][]string
}

func newUnit(key string, files []string) *unit {
	return &unit{key: key, files: files, names: map[string][]string{}, members: map[string]map[string][]string{}}
}

// addUnit adds a unit of the files and the names that they define.
func (b *builder) addUnit(key string, files []string) *unit {
	u := newUnit(key, files)
	for _, p := range files {
		b.unitOf[p] = u
		for _, def := range b.byPath[p].Definitions {
			switch segs := fileindex.Segments(def.Key); {
			case len(segs) == 1:
				u.names[segs[0]] = append(u.names[segs[0]], def.Key)
			case len(segs) == 2 && def.Kind == fileindex.DefMethod:
				if u.members[segs[0]] == nil {
					u.members[segs[0]] = map[string][]string{}
				}
				u.members[segs[0]][segs[1]] = append(u.members[segs[0]][segs[1]], def.Key)
			}
		}
	}
	b.units[key] = u
	return u
}

// addGoUnits groups the Go files of each directory by their package clause.
// The clause of the most files is the package that an import of the
// directory names. A test package, as p_test next to p in files that end in
// _test.go, is a unit of its own. Any other clause is a unit of its own
// with a truncation record, because the link step does not read build tags.
func (b *builder) addGoUnits() {
	clauses := map[string]map[string][]string{}
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || ix.Language != langcode.Go {
			continue
		}
		dir := modules.Dir(p)
		if clauses[dir] == nil {
			clauses[dir] = map[string][]string{}
		}
		clauses[dir][ix.Package] = append(clauses[dir][ix.Package], p)
	}
	dirs := make([]string, 0, len(clauses))
	for dir := range clauses {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	b.golang = modules.NewGolang(b.files, b.opts.Root, dirs)
	b.problems = append(b.problems, b.golang.Problems()...)

	for _, dir := range dirs {
		byClause := clauses[dir]
		names := make([]string, 0, len(byClause))
		for name := range byClause {
			names = append(names, name)
		}
		slices.SortFunc(names, func(x, y string) int {
			return cmp.Or(compareBool(isGoTestPackage(x, byClause), isGoTestPackage(y, byClause)), len(byClause[y])-len(byClause[x]), cmp.Compare(x, y))
		})
		for i, name := range names {
			u := b.addUnit(goUnitKey(dir, name), byClause[name])
			switch {
			case i == 0:
				b.goPackages[dir] = u
			case !isGoTestPackage(name, byClause):
				for _, p := range byClause[name] {
					b.truncations[Truncation{Kind: TruncationPackageClauses, Key: p, Site: Site{Path: p, Line: 1}}] = true
				}
			}
		}
	}
}

// isGoTestPackage reports whether a clause is the test package of another
// clause of the directory, in files that end in _test.go.
func isGoTestPackage(name string, byClause map[string][]string) bool {
	base, ok := strings.CutSuffix(name, "_test")
	if !ok || byClause[base] == nil {
		return false
	}
	return !slices.ContainsFunc(byClause[name], func(p string) bool { return !strings.HasSuffix(p, "_test.go") })
}

// addJavaUnits groups the Java files by their package clause. A Java
// package is the class table of its classes: an import names a class by
// its package and its name, and a class names the classes of its package
// with no import.
func (b *builder) addJavaUnits() {
	files := map[string][]string{}
	for _, p := range b.paths {
		if ix := b.byPath[p]; ix != nil && ix.Language == langcode.Java {
			files[ix.Package] = append(files[ix.Package], p)
		}
	}
	for _, pkg := range slices.Sorted(maps.Keys(files)) {
		b.javaPackages[pkg] = b.addUnit("java:"+pkg, files[pkg])
	}
}

// resolveJava resolves a qualified name, as com.x.Service or
// com.x.Outer.Inner, to the unit of a package or to a class of the project
// and a member under it. The longest package that the name starts with
// wins. The default package has no name, so no import names it.
func (b *builder) resolveJava(name string) (string, bool) {
	parts := strings.Split(name, ".")
	for i := len(parts); i > 0; i-- {
		u := b.javaPackages[strings.Join(parts[:i], ".")]
		if u == nil {
			continue
		}
		if i == len(parts) {
			return u.key, true
		}
		if r := b.walk(u.key, parts[i:]); r.fail == nil {
			return r.key, true
		}
		return "", false
	}
	return "", false
}

// javaFirstParty reports whether a qualified name starts with a package of
// the project. It looks up each dotted prefix of the name, because a scan
// of the packages at each import grows with the square of the project.
func (b *builder) javaFirstParty(name string) bool {
	for prefix := name; prefix != ""; {
		if _, ok := b.javaPackages[prefix]; ok {
			return true
		}
		i := strings.LastIndexByte(prefix, '.')
		if i < 0 {
			return false
		}
		prefix = prefix[:i]
	}
	return false
}

func goUnitKey(dir, clause string) string {
	return "go:" + dir + ":" + clause
}

// unitNames returns the definitions of the unit of the file with the name.
func (b *builder) unitNames(path, name string) []string {
	if u := b.unitOf[path]; u != nil {
		return u.names[name]
	}
	return nil
}

// member returns the method of a type that a file of the unit of the type
// defines.
func (b *builder) member(class, name string) (string, bool) {
	u := b.unitOf[fileindex.Unit(class)]
	segs := fileindex.Segments(class)
	if u == nil || len(segs) != 1 {
		return "", false
	}
	keys := u.members[segs[0]][name]
	if len(keys) == 0 {
		return "", false
	}
	return keys[0], true
}

// twins returns the other definitions of the unit with the name of a
// top-level definition or a member, as a function that two files under
// other build tags define.
func (b *builder) twins(key string) []string {
	u := b.unitOf[fileindex.Unit(key)]
	if u == nil {
		return nil
	}
	var keys []string
	switch segs := fileindex.Segments(key); len(segs) {
	case 1:
		keys = u.names[segs[0]]
	case 2:
		keys = u.members[segs[0]][segs[1]]
	}
	if len(keys) < 2 {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(keys), func(k string) bool { return k == key })
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

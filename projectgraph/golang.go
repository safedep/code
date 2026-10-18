package projectgraph

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

var goProfile = profile{
	code:               langcode.Go,
	resolver:           func(b *builder, sp *space) modules.Resolver { return goResolver{sp} },
	units:              (*builder).addGoUnits,
	dependencyCode:     true,
	link:               (*builder).linkInits,
	provide:            goProvide,
	narrowProviders:    goNarrowProviders,
	surface:            goSurface,
	implicitInterfaces: true,
	typeConversions:    true,
	scopeWildcards:     true,
	mainLoadsPackage:   true,
}

// goProvide returns the package unit of a Go module that an import path
// under its module path names.
func goProvide(_ *builder, sp *space, module string, _ modules.Form) []provision {
	rel, ok := strings.CutPrefix(module, sp.pkg.Name)
	if sp.pkg.Ecosystem != "go" || !ok || rel != "" && !strings.HasPrefix(rel, "/") {
		return nil
	}
	dir, ok := sp.golang.Resolve(module)
	if !ok {
		dir = strings.TrimPrefix(rel, "/")
	}
	if u := sp.goPackages[dir]; u != nil {
		return []provision{{sp: sp, module: u.key, via: "module"}}
	}
	return []provision{{sp: sp}}
}

// goNarrowProviders keeps the units of the longest module path, which holds
// the package, as the go command takes it for nested modules.
func goNarrowProviders(out []provision) []provision {
	longest := 0
	for _, pv := range out {
		longest = max(longest, len(pv.sp.pkg.Name))
	}
	return slices.DeleteFunc(out, func(pv provision) bool { return len(pv.sp.pkg.Name) < longest })
}

// goSurface adds the exported names and methods of each Go package of a
// unit that is not internal.
func goSurface(s *surfacer) {
	b, sp := s.b, s.sp
	for _, dir := range slices.Sorted(maps.Keys(sp.goPackages)) {
		importPath, ok := b.goImportPath(sp, dir)
		if !ok {
			continue
		}
		for _, p := range sp.goPackages[dir].files {
			if strings.HasSuffix(p, "_test.go") || b.byPath[p] == nil {
				continue
			}
			for _, def := range b.byPath[p].Definitions {
				segs := fileindex.Segments(def.Key)
				switch {
				case len(segs) == 1 && isGoExported(segs[0]):
					s.add(langcode.Go, importPath, modules.FormImport, segs[0])
				case len(segs) == 2 && def.Kind == fileindex.DefMethod && isGoExported(segs[0]) && isGoExported(segs[1]):
					s.add(langcode.Go, importPath, modules.FormImport, segs...)
				}
			}
		}
	}
}

// goImportPath returns the import path of a Go package of a unit. A package
// under an internal directory and a main package have no importer outside
// the module.
func (b *builder) goImportPath(sp *space, dir string) (string, bool) {
	u := sp.goPackages[dir]
	if slices.Contains(strings.Split(dir, "/"), "internal") || slices.ContainsFunc(u.files, func(p string) bool {
		ix := b.byPath[p]
		return ix != nil && ix.Package == "main"
	}) {
		return "", false
	}
	if p, ok := sp.golang.ImportPath(dir); ok {
		return p, true
	}
	return strings.TrimSuffix(sp.pkg.Name+"/"+dir, "/"), true
}

func isGoExported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// goState is the state of Go in a build: the methods of each package unit
// by name, and whether a type implements an interface.
type goState struct {
	methods     map[familyName][]string
	implemented map[[2]string]bool
}

func (b *builder) goState() *goState {
	s := langState[goState](b, langcode.Go)
	if s.implemented == nil {
		s.implemented = map[[2]string]bool{}
	}
	return s
}

// goResolver resolves a Go import path to the unit of the package that an
// import of its directory names, in one space.
type goResolver struct{ sp *space }

func (r goResolver) Module(_, spec string) (string, string, bool) {
	if dir, ok := r.sp.golang.Resolve(spec); ok {
		return r.sp.goPackages[dir].key, "module", true
	}
	return "", "", false
}

// FirstParty reports whether an import path is under the path of a module
// of the space.
func (r goResolver) FirstParty(spec string) bool {
	return r.sp.golang.FirstParty(spec)
}

// addGoUnits groups the Go files of each directory of each space by their
// package clause. The clause of the most files is the package that an
// import of the directory names. A test package, as p_test next to p in
// files that end in _test.go, is a unit of its own. Any other clause is a
// unit of its own with a truncation record, because the link step does not
// read build tags.
func (b *builder) addGoUnits() {
	files := map[*space][]string{}
	for _, p := range b.paths {
		if ix := b.byPath[p]; ix != nil && ix.Language == langcode.Go {
			sp := b.spaceOf(p)
			files[sp] = append(files[sp], p)
		}
	}
	for _, sp := range b.ordered {
		b.addSpaceGoUnits(sp, files[sp])
	}
}

func (b *builder) addSpaceGoUnits(sp *space, files []string) {
	clauses := map[string]map[string][]string{}
	for _, p := range files {
		ix := b.byPath[p]
		dir := modules.Dir(sp.inner(p))
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
	sp.golang = modules.NewGolang(sp.files, sp.root, dirs)
	b.addProblems(sp, sp.golang.Problems())

	for _, dir := range dirs {
		byClause := clauses[dir]
		names := make([]string, 0, len(byClause))
		for name := range byClause {
			names = append(names, name)
		}
		slices.SortFunc(names, func(x, y string) int {
			return cmp.Or(fileindex.CompareBool(isGoTestPackage(x, byClause), isGoTestPackage(y, byClause)), len(byClause[y])-len(byClause[x]), cmp.Compare(x, y))
		})
		for i, name := range names {
			u := b.addUnit(goUnitKey(sp.key(dir), name), byClause[name])
			switch {
			case i == 0:
				sp.goPackages[dir] = u
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

func goUnitKey(dir, clause string) string {
	return "go:" + dir + ":" + clause
}

// linkInits adds an edge from the module top level of a Go file to each
// init function of the file, because the program runs it when it loads
// the package.
func (b *builder) linkInits(ix *fileindex.Index) {
	key := fileindex.Join(ix.Path, "init")
	if b.kindOf(key) != NodeFunction {
		return
	}
	b.link(ix.Language, ix.Path, key, b.siteOf(key), EdgeDirect, ruleInit)
}

// implementations returns the Go methods that a call of a method of the
// interface can run: the methods with the name of the call and a signature
// that takes its arguments, of a type that implements the interface. A
// value of an interface of the project can hold a type of any unit of
// dependency code, as the value that a constructor of the unit returns. An
// interface of a unit holds only the types of its unit. One call as the
// only argument can give more values, so it takes any arity.
func (b *builder) implementations(iface string, call fileindex.Call) []string {
	s := b.goState()
	if s.methods == nil {
		s.methods = map[familyName][]string{}
		for _, p := range b.paths {
			ix := b.byPath[p]
			if ix == nil || ix.Language != langcode.Go {
				continue
			}
			for _, def := range ix.Definitions {
				if def.Kind == fileindex.DefMethod && len(fileindex.Segments(def.Key)) == 2 {
					k := familyName{unit: ix.Unit, name: def.Name}
					s.methods[k] = append(s.methods[k], def.Key)
				}
			}
		}
	}
	units := []string{fileindex.PackageOf(iface)}
	if units[0] == "" {
		for _, sp := range b.ordered {
			if sp.unit != "" {
				units = append(units, sp.unit)
			}
		}
	}
	args := len(call.Args)
	spread := args == 1 && call.Args[0].Shape == fileindex.ArgComputed
	var out []string
	for _, unit := range units {
		for _, key := range s.methods[familyName{unit: unit, name: call.Method}] {
			def := b.defs[key]
			takes := spread || def.Arity == args || def.Variadic && args >= def.Arity-1
			if takes && b.implements(fileindex.Parent(key), iface) {
				out = append(out, key)
			}
		}
	}
	slices.Sort(out)
	return out
}

// implements reports whether the method set of a Go type holds each method
// of a first-party interface by name and arity. The method set has the
// methods of the type and of the types that it embeds. A method that an
// embedded type of a dependency can give counts as held. The methods of an
// embedded interface of a dependency are not known, so the type needs none
// of them.
func (b *builder) implements(typ, iface string) bool {
	s := b.goState()
	k := [2]string{typ, iface}
	if held, ok := s.implemented[k]; ok {
		return held
	}
	held := true
	for _, sig := range b.abstract(iface, 0) {
		key, fail := b.method(typ, sig.Name)
		def := b.defs[key]
		if fail != nil || def != nil && (def.Arity != sig.Arity || def.Variadic != sig.Variadic) {
			held = false
			break
		}
	}
	s.implemented[k] = held
	return held
}

// abstract returns the methods that a first-party Go interface declares,
// with the methods of the first-party interfaces that it embeds.
func (b *builder) abstract(iface string, depth int) []fileindex.Signature {
	rec, ok := b.classes[iface]
	if !ok || depth > b.opts.maxExportHops {
		return nil
	}
	sigs := slices.Clone(rec.class.Abstract)
	for _, base := range b.typeBases(iface) {
		sigs = append(sigs, b.abstract(base, depth+1)...)
	}
	return sigs
}

package projectgraph

import (
	"cmp"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

var goProfile = profile{
	code:               langcode.Go,
	family:             langcode.Go,
	resolver:           func(b *builder) modules.Resolver { return goResolver{b} },
	units:              (*builder).addGoUnits,
	link:               (*builder).linkInits,
	implicitInterfaces: true,
	typeConversions:    true,
	scopeWildcards:     true,
	mainLoadsPackage:   true,
}

// goResolver resolves a Go import path to the unit of the package that an
// import of its directory names.
type goResolver struct{ b *builder }

func (r goResolver) Module(_, spec string) (string, string, bool) {
	if dir, ok := r.b.golang.Resolve(spec); ok {
		return r.b.goPackages[dir].key, "module", true
	}
	return "", "", false
}

// FirstParty reports whether an import path is under the path of a module
// of the project.
func (r goResolver) FirstParty(spec string) bool {
	return r.b.golang.FirstParty(spec)
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
			return cmp.Or(fileindex.CompareBool(isGoTestPackage(x, byClause), isGoTestPackage(y, byClause)), len(byClause[y])-len(byClause[x]), cmp.Compare(x, y))
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
	n := b.g.Nodes[key]
	b.addEdge(Edge{From: ix.Path, To: key, Site: Site{Path: ix.Path, Line: n.Line}, Kind: EdgeDirect, Resolver: resolverName(ix.Language, "init")})
}

// implementations returns the Go methods of the project that a call of a
// method of the interface can run: the methods with the name of the call
// and a signature that takes its arguments, of a type that implements the
// interface. One call as the only argument can give more values, so it
// takes any arity.
func (b *builder) implementations(iface string, call fileindex.Call) []string {
	if b.goMethods == nil {
		b.goMethods = map[string][]string{}
		for _, p := range b.paths {
			ix := b.byPath[p]
			if ix == nil || ix.Language != langcode.Go {
				continue
			}
			for _, def := range ix.Definitions {
				if def.Kind == fileindex.DefMethod && len(fileindex.Segments(def.Key)) == 2 {
					b.goMethods[def.Name] = append(b.goMethods[def.Name], def.Key)
				}
			}
		}
	}
	args := len(call.Args)
	spread := args == 1 && call.Args[0].Shape == fileindex.ArgComputed
	var out []string
	for _, key := range b.goMethods[call.Method] {
		def := b.defs[key]
		takes := spread || def.Arity == args || def.Variadic && args >= def.Arity-1
		if takes && b.implements(fileindex.Parent(key), iface) {
			out = append(out, key)
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
	k := [2]string{typ, iface}
	if held, ok := b.implemented[k]; ok {
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
	b.implemented[k] = held
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

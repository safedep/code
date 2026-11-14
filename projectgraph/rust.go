package projectgraph

import (
	"cmp"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

// rustProfile reads the module tree of a Rust crate: each file is a module
// whose path follows from its place under src, as tool::store::file for
// src/store/file.rs of the crate tool. A use and a path name a module of
// the crate by crate, self or super, or of another crate of the project by
// its name. The impl blocks of a type are one class with the type, and an
// impl of a trait makes the trait a base.
var rustProfile = profile{
	code:              langcode.Rust,
	resolver:          func(b *builder, sp *space) modules.Resolver { return rustResolver{b: b, sp: sp} },
	units:             (*builder).addRustUnits,
	dependencyCode:    true,
	aliases:           (*builder).joinImplBlocks,
	link:              (*builder).linkRust,
	scopeFallback:     (*builder).rustScopeFallback,
	provide:           rustProvide,
	importedModule:    rustImportedModule,
	symbol:            rustSymbol,
	surface:           rustSurface,
	unitTypes:         true,
	sameTypeMethods:   map[string]bool{"clone": true, "to_owned": true, "default": true, "from": true},
	abstractReceiver:  fileindex.UnresolvedTraitReceiver,
	dependencies:      func(b *builder) ([]string, []modules.Problem) { return b.cargoOf(b.project).Dependencies(), nil },
	typePath:          rustTypePath,
	instance:          []string{"self"},
	superModule:       true,
	unitImports:       true,
	scopeWildcards:    true,
	exactWildcards:    true,
	definitionExports: true,
	annotationsData:   true,
	itemPaths:         "::",
	implicitMethods:   rustImplicit,
	forwardMethod:     "deref",
	methodAliases:     rustConversions,
	helperProgram:     rustHelperProgram,
}

// rustHelperProgram reports whether a file is the build script, an example
// or a benchmark of a crate. A library crate with a build.rs has a main,
// but its users call its public items. Without this, the build script
// made the only root of the crate, and a call of the library read as
// not-found.
func rustHelperProgram(path string) bool {
	dir, file := "", path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		dir, file = path[:i], path[i+1:]
	}
	if file == "build.rs" {
		return true
	}
	return slices.ContainsFunc(strings.Split(dir, "/"), func(d string) bool { return d == "examples" || d == "benches" })
}

// rustConversions name the methods of the traits of std that a call of
// another name runs, as from for into or fmt for to_string.
var rustConversions = map[string][]string{
	"into":      {"from"},
	"try_into":  {"try_from"},
	"parse":     {"from_str"},
	"collect":   {"from_iter"},
	"to_string": {"fmt"},
	"to_owned":  {"clone"},
}

// linkRust adds the edges of the impl blocks, as C# does for a class body,
// and the name edges to the methods of an impl of a trait for a type of
// another crate, as an extension function.
func (b *builder) linkRust(ix *fileindex.Index) {
	b.linkClassBodies(ix)
	b.linkExtensions(ix, b.extensionsNamed)
	b.linkCrateRoutes(ix)
}

// linkCrateRoutes records each use of a path through a module into a
// crate that no unit gives, as use self::aes_gcm::Aes256Gcm after extern
// crate aes_gcm. The code of the crate is not in the graph, so the record
// has the crate as its import prefix, and only a target of the crate sees
// it. A graph with no unit links no crate, so it records nothing.
func (b *builder) linkCrateRoutes(ix *fileindex.Index) {
	if !b.hasUnits() {
		return
	}
	sc := b.scopes[ix.Path]
	for _, imp := range ix.Imports {
		if imp.Kind != fileindex.ImportStatic {
			continue
		}
		module, _ := importedModule(ix.Language, imp)
		path, file, ok := b.rustCratePath(ix.Path, module)
		if !ok {
			continue
		}
		crate, _ := rustCrateNames(path)
		if b.provides(origin{lang: ix.Language, unit: ix.Unit, module: b.packageModule(b.byPath[file], crate), form: modules.FormImport}) {
			continue
		}
		b.addUnresolved(UnresolvedSite{
			Caller:       b.runner(sc.enclosing(imp.Line)),
			Site:         Site{Path: ix.Path, Line: imp.Line},
			Text:         module,
			Reason:       fileindex.UnresolvedNoDependencySource,
			ImportPrefix: fileindex.Namespace(ix.Language, crate),
		})
	}
}

// rustImplicit name the methods of the traits of std that Rust code calls
// with no call of the name: an operator, a conversion of ? or of a for
// loop, a format of {} or {:?}, the drop at the end of a scope and the
// poll of .await.
var rustImplicit = map[string]bool{
	"drop": true, "fmt": true, "clone": true, "eq": true, "ne": true, "partial_cmp": true,
	"cmp": true, "hash": true, "deref": true, "deref_mut": true, "index": true, "index_mut": true,
	"add": true, "sub": true, "mul": true, "div": true, "rem": true, "neg": true, "not": true,
	"bitand": true, "bitor": true, "bitxor": true, "shl": true, "shr": true, "add_assign": true,
	"sub_assign": true, "mul_assign": true, "div_assign": true, "into_iter": true, "next": true,
	"from": true, "into": true, "try_from": true, "from_str": true, "default": true,
	"as_ref": true, "as_mut": true, "borrow": true, "poll": true, "call": true, "call_mut": true,
	"call_once": true, "serialize": true, "deserialize": true,
}

// rustUnwraps are the names that give the value inside an Option or a
// Result: unwrap, expect and the ? that the index records in a target. Any
// other name on a wrapped result is a method of the wrapper itself, as map
// or inspect of Option, and the link step does not know its result.
var rustUnwraps = map[string]bool{"unwrap": true, "expect": true, "unwrap_unchecked": true, fileindex.TryName: true}

// rustRoots are the first names of the paths of the crates of the
// toolchain, which no crate of the project defines.
var rustRoots = map[string]bool{"std": true, "core": true, "alloc": true}

// rustState is the state of Rust in a build: the crates of each space, the
// module path of each Rust file, and the paths that a resolution or a
// crate route reads now, by the file that writes them.
type rustState struct {
	cargo     map[*space]*modules.Cargo
	modules   map[string]string
	resolving map[[2]string]bool
	routing   map[[2]string]bool
	routes    map[[2]string]crateRoute
}

// crateRoute is the path in another crate that a path of a file names, and
// the file of the binding.
type crateRoute struct {
	path, file string
	ok         bool
}

func (b *builder) rustState() *rustState {
	return langState[rustState](b, langcode.Rust)
}

// cargoOf returns the crates of the Cargo.toml files of a space.
func (b *builder) cargoOf(sp *space) *modules.Cargo {
	s := b.rustState()
	if s.cargo == nil {
		s.cargo = map[*space]*modules.Cargo{}
	}
	c, ok := s.cargo[sp]
	if !ok {
		c = modules.NewCargo(sp.files, sp.root)
		s.cargo[sp] = c
		b.addProblems(sp, c.Problems())
	}
	return c
}

// addRustUnits makes each Rust file the unit of its module, in the table of
// the module paths.
func (b *builder) addRustUnits() {
	s := b.rustState()
	s.modules = map[string]string{}
	s.resolving = map[[2]string]bool{}
	s.routing = map[[2]string]bool{}
	s.routes = map[[2]string]crateRoute{}
	files := map[string][]string{}
	for _, p := range b.paths {
		if ix := b.byPath[p]; ix != nil && ix.Language == langcode.Rust {
			module := b.rustModule(p)
			s.modules[p] = module
			files[module] = append(files[module], p)
		}
	}
	t := &namespaces{sep: "::", units: map[string]*unit{}}
	for module, paths := range files {
		t.units[module] = b.addUnit(string(langcode.Rust)+":"+module, paths)
	}
	b.namespaces[langcode.Rust] = t
}

// rustModule returns the module path of a Rust file, as tool::store::file
// for src/store/file.rs and tool::store for src/store/mod.rs of the crate
// tool. The crate is the deepest Cargo.toml that holds the file, or crate
// with no Cargo.toml. src/lib.rs and src/main.rs are the root of the crate.
// A file of src/bin, tests, examples or benches, and build.rs, is the root
// of a crate of its own, which no path of another file names.
//
// The root of a crate of a unit of dependency code is the unit, as
// cargo:smallvec@1.13.2::alloc for src/alloc.rs of smallvec. So the
// modules of two versions of one crate, or of a crate of the project with
// the name of a dependency, stay apart.
func (b *builder) rustModule(p string) string {
	sp := b.spaceOf(p)
	inner := sp.inner(p)
	crate, ok := b.cargoOf(sp).Crate(inner)
	name, rel := "crate", inner
	if ok {
		name = crate.Name
		if crate.Dir != "" {
			rel = strings.TrimPrefix(inner, crate.Dir+"/")
		}
	}
	if sp.unit != "" {
		name = sp.unit
	}
	src, inSrc := strings.CutPrefix(rel, "src/")
	first, _, _ := strings.Cut(rel, "/")
	switch {
	case inSrc && strings.HasPrefix(src, "bin/"), !inSrc && (rel == "build.rs" || first == "tests" || first == "examples" || first == "benches"):
		return name + "#" + strings.TrimSuffix(rel, ".rs")
	case !inSrc:
		src = rel
	}
	segs := strings.Split(strings.TrimSuffix(src, ".rs"), "/")
	switch last := segs[len(segs)-1]; {
	case last == "mod":
		segs = segs[:len(segs)-1]
	case len(segs) == 1 && (last == "lib" || last == "main"):
		segs = nil
	}
	return strings.Join(append([]string{name}, segs...), "::")
}

// rustPaths returns the module paths that a path in a module can name. A
// path starts with crate, self, super, the name of a crate of the project
// or a name in the module, as a child module.
func (b *builder) rustPaths(module, written string) []string {
	parts := strings.Split(strings.TrimPrefix(written, "::"), "::")
	crate, _, _ := strings.Cut(module, "::")
	switch parts[0] {
	case "crate":
		return []string{joinPath(crate, parts[1:])}
	case "self":
		return []string{joinPath(module, parts[1:])}
	case "super":
		for len(parts) > 0 && parts[0] == "super" {
			i := strings.LastIndex(module, "::")
			if i < 0 {
				return nil
			}
			module, parts = module[:i], parts[1:]
		}
		return []string{joinPath(module, parts)}
	}
	out := []string{joinPath(module, parts)}
	switch sp := b.rustSpaceOf(module); {
	case sp.unit != "" && b.rustCrateName(sp) == parts[0]:
		out = append(out, joinPath(sp.unit, parts[1:]))
	case sp.unit == "" && b.cargoOf(sp).Has(parts[0]):
		out = append(out, written)
	}
	return out
}

// rustSpaceOf returns the space of a module path: the unit of its root, or
// the project.
func (b *builder) rustSpaceOf(module string) *space {
	root, _, _ := strings.Cut(module, "::")
	if sp := b.spaces[root]; sp != nil {
		return sp
	}
	return b.project
}

// rustCrateName returns the name that a path of another crate writes for
// the crate of a unit of dependency code, as tokio_postgres for the package
// tokio-postgres: the name of its root Cargo.toml, or else the name of its
// package.
func (b *builder) rustCrateName(sp *space) string {
	if crate, ok := b.cargoOf(sp).Crate("src/lib.rs"); ok && crate.Dir == "" {
		return crate.Name
	}
	return strings.ReplaceAll(sp.pkg.Name, "-", "_")
}

// rustProvide returns the item of a crate of a unit of dependency code that
// a path of another crate names, as the struct SmallVec of the unit
// cargo:smallvec@1.13.2 for smallvec::SmallVec.
func rustProvide(b *builder, sp *space, module string, _ modules.Form) []provision {
	parts := strings.Split(strings.TrimPrefix(module, "::"), "::")
	if sp.pkg.Ecosystem != "cargo" || b.rustCrateName(sp) != parts[0] {
		return nil
	}
	if key, ok := b.resolveQualified(langcode.Rust, joinPath(sp.unit, parts[1:])); ok {
		return []provision{{sp: sp, module: key, via: "crate"}}
	}
	return []provision{{sp: sp}}
}

// rustImportedModule returns the path of the package that a path of a file
// names. A path writes the name that the Cargo.toml of its crate gives a
// dependency, and the crate of the unit has the name of the package.
func rustImportedModule(b *builder, ix *fileindex.Index, module string) string {
	sp := b.spaceOf(ix.Path)
	first, rest, nested := strings.Cut(strings.TrimPrefix(module, "::"), "::")
	pkg := b.cargoOf(sp).Rename(sp.inner(ix.Path), first)
	switch {
	case pkg == first:
		return module
	case nested:
		return pkg + "::" + rest
	}
	return pkg
}

// rustSurface adds each pub item of the crate of a unit and each pub
// method of its types, under the path that another crate writes for it.
func rustSurface(s *surfacer) {
	b, sp := s.b, s.sp
	if sp.pkg.Ecosystem != "cargo" {
		return
	}
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || ix.Language != langcode.Rust || fileindex.PackageOf(p) != sp.unit {
			continue
		}
		for _, def := range ix.Definitions {
			segs := fileindex.Segments(def.Key)
			if !def.Exported || len(segs) == 0 || len(segs) > 2 || len(segs) == 2 && def.Kind != fileindex.DefMethod {
				continue
			}
			if module, names, ok := rustSymbol(b, sp, def.Key); ok {
				s.add(langcode.Rust, module, modules.FormImport, names...)
			}
		}
	}
}

// rustSymbol returns the path of the module of a Rust definition of a unit
// as another crate writes it and the names of the definition, as smallvec
// and SmallVec, insert. A type of a private module that the root of the
// crate gives on with pub use gets the shortest path, as a user of the
// crate writes it.
func rustSymbol(b *builder, sp *space, def string) (string, []string, bool) {
	module, ok := b.rustState().modules[fileindex.Unit(def)]
	if !ok {
		return "", nil, false
	}
	var segs []string
	if _, rest, ok := strings.Cut(module, "::"); ok {
		segs = strings.Split(rest, "::")
	}
	names := fileindex.Segments(def)
	if len(names) == 0 {
		return "", nil, false
	}
	top := fileindex.Join(fileindex.Unit(def), names[0])
	for i := range segs {
		if key, ok := b.resolveQualified(langcode.Rust, joinPath(sp.unit, append(slices.Clone(segs[:i]), names[0]))); ok && key == top {
			segs = segs[:i]
			break
		}
	}
	return joinPath(b.rustCrateName(sp), segs), names, true
}

func joinPath(module string, parts []string) string {
	return strings.Join(append([]string{module}, parts...), "::")
}

// resolveRust resolves a path that a file of the module writes to a
// definition or a module of the project.
//
// A use binds the last name of its path, and the walk of the path can read
// that binding, as the first smallvec of use smallvec::smallvec. Rust
// resolves the first name of a use path against the crates and the modules,
// and never against the binding of the same use. So a path that the file
// resolves now gives no answer on a repeat, and the walk tries the next
// candidate. Without the check the walk does not end.
func (b *builder) resolveRust(importer, written string) (string, bool) {
	s := b.rustState()
	k := [2]string{importer, written}
	if s.resolving[k] {
		return "", false
	}
	s.resolving[k] = true
	defer delete(s.resolving, k)
	for _, p := range b.rustPaths(s.modules[importer], written) {
		if key, ok := b.resolveQualified(langcode.Rust, p); ok {
			return key, true
		}
	}
	return "", false
}

// rustResolver resolves a use, which names a module, a type or a function
// with its path, in one space.
type rustResolver struct {
	b  *builder
	sp *space
}

func (r rustResolver) Module(importer, spec string) (string, string, bool) {
	t, ok := r.b.rustPath(importer, spec)
	switch {
	case !ok:
		return "", "", false
	case t.key != "":
		return t.key, "path", true
	}
	// A use can bind an item of the crate, as fl of use crate::ffi::flush
	// as fl after pub use dep as ffi. A call of fl links the key itself,
	// so the key needs its node. Without it the edge reached no node, and
	// the target read as not-found.
	return r.b.external(r.b.rustCrateKey(t.file, t.path)), "crate", true
}

// rustTarget is what a path of a file names: a definition or a module of
// the project, or a path in another crate and the file of the binding
// that names the crate.
type rustTarget struct {
	key, path, file string
}

// rustPath resolves a path that a file writes: in the project, or else
// through a name that a module binds to another crate.
func (b *builder) rustPath(importer, written string) (rustTarget, bool) {
	if key, ok := b.resolveRust(importer, written); ok {
		return rustTarget{key: key}, true
	}
	if path, file, ok := b.rustCratePath(importer, written); ok {
		return rustTarget{path: path, file: file}, true
	}
	return rustTarget{}, false
}

// rustCrateKey returns the external key of a path of another crate that a
// file names, and records the path as an origin of the key. It makes no
// node: a walk reads the key with no node, and a call of a name under it
// makes the node of that name. A node of a type that no call reaches would
// turn a target of the type from no-such-target into not-found.
func (b *builder) rustCrateKey(file, path string) string {
	crate, names := rustCrateNames(path)
	key := fileindex.Namespace(langcode.Rust, path)
	b.recordOrigin(b.byPath[file], key, crate, modules.FormImport, names)
	return key
}

// rustTypePath returns the type that a path of a file names, as a type
// alias of the file writes it: a type of the crate, or the key of a type of
// another crate, also through a binding of a module.
func rustTypePath(b *builder, sc *scope, written string) (string, bool) {
	if t, ok := b.rustPath(sc.ix.Path, written); ok {
		if t.key != "" {
			return b.deref(t.key), true
		}
		return b.rustCrateKey(t.file, t.path), true
	}
	if !strings.Contains(written, "::") || b.rustLocalRoot(sc.ix.Path, written) {
		return "", false
	}
	return b.rustCrateKey(sc.ix.Path, written), true
}

// rustCratePath returns the path in another crate that a path of a file
// names through a name that a module binds to that crate, and the file of
// the binding. extern crate rand in the module secure binds rand there, so
// crate::secure::rand::thread_rng is rand::thread_rng. pub use
// libsqlite3_sys as ffi binds ffi, so crate::ffi::ErrorCode is
// libsqlite3_sys::ErrorCode. The deepest module of the path that the crate
// has decides. A path through a use of another module of the crate, as
// self::rand after use crate::secure::rand, follows that use. So does a
// path through a use of the module of the file, as sub::f after use
// dep::sub.
//
// A walk asks for the route of each glob use of each module that it reads,
// so the route of an outer call stays for the build. The route reads only
// the tables of the build, so only the cycle guard of an outer route can
// change it.
func (b *builder) rustCratePath(importer, written string) (string, string, bool) {
	s := b.rustState()
	k := [2]string{importer, written}
	if r, ok := s.routes[k]; ok {
		return r.path, r.file, r.ok
	}
	if s.routing[k] {
		return "", "", false
	}
	outer := len(s.routing) == 0
	s.routing[k] = true
	path, file, ok := b.routeCrate(importer, written)
	delete(s.routing, k)
	if outer {
		s.routes[k] = crateRoute{path: path, file: file, ok: ok}
	}
	return path, file, ok
}

func (b *builder) routeCrate(importer, written string) (string, string, bool) {
	parts := strings.Split(strings.TrimPrefix(written, "::"), "::")
	for i := len(parts) - 1; i >= 0; i-- {
		u := b.rustModuleUnit(importer, cmp.Or(strings.Join(parts[:i], "::"), "self"))
		if u == nil {
			continue
		}
		for _, p := range u.files {
			for _, imp := range rustBindings(b.scopes[p], parts[i]) {
				module, _ := importedModule(langcode.Rust, imp)
				if i == 0 && rustSelfBinding(module, parts) {
					continue
				}
				if crate, file, ok := b.rustCrate(p, module); ok {
					return joinPath(crate, parts[i+1:]), file, true
				}
			}
		}
		return "", "", false
	}
	return "", "", false
}

// rustSelfBinding reports whether a use of the module of a path binds the
// first name of the path to the path itself: use rand binds rand, and use
// smallvec::smallvec binds smallvec. Rust never reads the binding of a use
// in its own path, so such a binding routes nothing.
func rustSelfBinding(module string, parts []string) bool {
	module = strings.TrimPrefix(module, "::")
	return module == parts[0] || module == strings.Join(parts, "::")
}

// rustModuleUnit returns the unit of the module that a path of a file
// names as written, with no walk through a use. A walk reads the uses and
// the glob uses of each module on the way, and each of them can ask for a
// crate route again, so a route through walks grows with the power of the
// path length.
func (b *builder) rustModuleUnit(importer, written string) *unit {
	t := b.namespaces[langcode.Rust]
	for _, p := range b.rustPaths(b.rustState().modules[importer], written) {
		if u := t.units[p]; u != nil {
			return u
		}
	}
	return nil
}

// rustBindings returns the uses and the extern crates of a file that bind
// a name. extern crate rand and use rand bind the name of the crate, and no
// alias holds it, so the scope keeps no binding for them. The scope keeps
// one binding of a name for the file, and an inline module, as mod tests,
// can bind the name again, so each one counts.
func rustBindings(sc *scope, name string) []fileindex.Import {
	var out []fileindex.Import
	for _, imp := range sc.imports {
		if !imp.Wildcard && (bindingName(langcode.Rust, imp) == name || imp.Item == "" && imp.Module == name) {
			out = append(out, imp)
		}
	}
	return out
}

// rustCrate returns the path in another crate that the module of a use of
// a file names, and the file that names the crate: the route of the module
// through a binding, or else the module itself when its first name names
// no module of the crate, as rand of extern crate rand.
func (b *builder) rustCrate(file, module string) (string, string, bool) {
	if path, via, ok := b.rustCratePath(file, module); ok {
		return path, via, true
	}
	if b.rustLocalRoot(file, module) {
		return "", "", false
	}
	return module, file, true
}

// rustLocalRoot reports whether the first name of a path of a file names
// the crate of the file or a module of it: crate, self, super, a crate of
// the project, a child module, or a module of the root of the crate, as a
// path of the 2015 edition names it. Any other first name names a crate,
// as schannel of extern crate schannel in src/imp/schannel.rs. A file of
// the name does not tell it, because the module of that file is no child.
func (b *builder) rustLocalRoot(file, path string) bool {
	first := rustFirstName(path)
	switch first {
	case "crate", "self", "super":
		return true
	}
	return b.cargoOf(b.spaceOf(file)).Has(first) || b.rustModuleUnit(file, first) != nil || b.rustModuleUnit(file, "crate::"+first) != nil
}

// rustFirstName returns the first name of a path, as std for ::std::io.
func rustFirstName(path string) string {
	first, _, _ := strings.Cut(strings.TrimPrefix(path, "::"), "::")
	return first
}

// rustCrateNames splits a path of a crate into the crate and the names
// under it.
func rustCrateNames(path string) (string, []string) {
	crate, rest, ok := strings.Cut(path, "::")
	if !ok {
		return crate, nil
	}
	return crate, strings.Split(rest, "::")
}

// FirstParty reports whether a path starts with crate, self or super, with
// the name of a crate of the project or with the name of a file or a
// directory of the project, as a child module.
func (r rustResolver) FirstParty(spec string) bool {
	first := rustFirstName(spec)
	switch {
	case first == "crate" || first == "self" || first == "super":
		return true
	case rustRoots[first]:
		return false
	}
	return r.b.cargoOf(r.sp).Has(first) || r.sp.files.HasName(first)
}

// FirstPartyFrom reports whether a path of a file can name a module of its
// crate, by the modules that the file sees, and not by the names of all
// the files of the project.
func (r rustResolver) FirstPartyFrom(importer, spec string) bool {
	return !rustRoots[rustFirstName(spec)] && r.b.rustLocalRoot(importer, spec)
}

// rustScopeFallback resolves a path that no use and no item of the module
// binds: a path of the crate, as crate::store::save or store::save for a
// child module, or else a path of another crate, as serde_json::to_string,
// whose external key the edge gets. The crate is the origin of the key, so a
// unit of dependency code that gives the crate links it. A single name stays a record, because
// a local closure can bind it.
func (b *builder) rustScopeFallback(sc *scope, _, first string, rest []string, target string) []resolution {
	written := strings.Join(append([]string{first}, rest...), "::")
	if t, ok := b.rustPath(sc.ix.Path, written); ok {
		if t.key != "" {
			return []resolution{b.walkNamespace(t.key)}
		}
		crate, names := rustCrateNames(t.path)
		return []resolution{b.importedWalk(b.byPath[t.file], fileindex.Namespace(langcode.Rust, t.path), crate, modules.FormImport, names)}
	}
	if len(rest) > 0 {
		return []resolution{b.importedWalk(sc.ix, target, first, modules.FormImport, rest)}
	}
	return nil
}

// joinImplBlocks makes an impl block of a type of another module one class
// with the type, as impl Store for FileStore after use crate::FileStore.
func (b *builder) joinImplBlocks() {
	var groups [][]string
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || ix.Language != langcode.Rust {
			continue
		}
		sc := b.scopes[p]
		for _, c := range ix.Classes {
			imp, ok := sc.bindings[fileindex.Last(c.Key)]
			if !ok || len(fileindex.Segments(c.Key)) != 1 {
				continue
			}
			module, _ := importedModule(ix.Language, imp)
			if key, _, found := b.resolveModule(ix, module); found && key != c.Key && b.isType(key) {
				groups = append(groups, []string{key, c.Key})
			}
		}
	}
	b.joinAliases(groups)
}

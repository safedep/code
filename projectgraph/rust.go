package projectgraph

import (
	"cmp"
	"math"
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
	nestedTypes:       true,
	receiverMethods:   true,
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
// module path of each Rust file, the paths that a resolution or a crate
// route reads now, by the file that writes them, the module paths of the
// glob uses of each file, the exported macros of each unit by name, and
// the names of the types of the inline modules.
type rustState struct {
	cargo     map[*space]*modules.Cargo
	modules   map[string]string
	resolving map[[2]string]bool
	routing   map[[2]string]bool
	routes    map[[2]string]crateRoute
	globs     map[string][]string
	macros    map[*space]map[string]string
	nested    map[string]bool
	glob      globMemo
	// useScopes holds the classes of each file that hold a use, which are
	// inline modules.
	useScopes map[string]map[string]bool
	// spaceMacros holds the names of the macro_rules! of each space.
	spaceMacros map[*space]map[string]bool
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
	s.globs = map[string][]string{}
	s.glob = globMemo{active: map[globKey]int{}, low: math.MaxInt, kept: map[globKey]globAnswer{}, visited: map[globKey]globAnswer{}}
	s.useScopes = map[string]map[string]bool{}
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

// rustScopePaths returns the module paths that a path of a file can name:
// the paths of rustPaths, with the path under the module of each glob use
// of the file after the path of a child item. use super::* in
// src/deep/worker.rs brings the module helpers of src/deep/mod.rs into
// scope, so helpers::cleanup names crate::deep::helpers::cleanup. Rust
// reads a name of a glob use after an item of the module and before a
// crate. Without the glob paths the first name reads as another crate, the
// edge goes to an external key with no record, and a target under it reads
// not-found.
func (b *builder) rustScopePaths(importer, written string) []string {
	out := b.rustPaths(b.rustState().modules[importer], written)
	if first := rustFirstName(written); first == "crate" || first == "self" || first == "super" || strings.HasPrefix(written, "::") {
		return out
	}
	parts := strings.Split(written, "::")
	var globs []string
	for _, p := range b.rustGlobModules(importer) {
		globs = append(globs, joinPath(p, parts))
	}
	return slices.Insert(out, 1, globs...)
}

// rustGlobModules returns the module paths of the glob uses of the module
// of a file. super in use super::* of an inline module, as mod tests, is
// the module of the file. Read from the module of the file, it names a
// sibling module, and a path of a crate with the name of that sibling
// loses its edge to the crate. So a glob use of an inline module stays out.
// The index gives an inline module the kind class, and an impl block holds
// no use, so a glob use whose innermost definition is a class sits in an
// inline module.
// The path of a glob use reads no glob use, so no cycle of glob uses can
// start.
func (b *builder) rustGlobModules(file string) []string {
	s := b.rustState()
	if mods, ok := s.globs[file]; ok {
		return mods
	}
	var mods []string
	if sc := b.scopes[file]; sc != nil {
		for _, imp := range sc.wildcards {
			if def := b.defs[sc.enclosing(imp.Line)]; def != nil && def.Kind == fileindex.DefClass {
				continue
			}
			mods = append(mods, b.rustPaths(s.modules[file], imp.Module)...)
		}
	}
	s.globs[file] = mods
	return mods
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
	// #[macro_export] puts a macro at the root of its crate, as from_sub of
	// src/macros.rs for dep::from_sub. A walk of the modules of the crate
	// does not find it.
	if len(parts) == 2 {
		exported, _ := b.rustMacros()
		if key, ok := exported[sp][parts[1]]; ok {
			return []provision{{sp: sp, module: key, via: "crate"}}
		}
	}
	return []provision{{sp: sp}}
}

// rustMacros returns the macros that #[macro_export] puts at the root of
// the crate of each unit, by name, and the names of the macro_rules! of
// each space.
func (b *builder) rustMacros() (exported map[*space]map[string]string, rules map[*space]map[string]bool) {
	s := b.rustState()
	if s.macros != nil {
		return s.macros, s.spaceMacros
	}
	s.macros = map[*space]map[string]string{}
	s.spaceMacros = map[*space]map[string]bool{}
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || ix.Language != langcode.Rust {
			continue
		}
		sp := b.spaceOf(p)
		for _, def := range ix.Definitions {
			if slices.Contains(def.Decorators, fileindex.RustMacroRules) {
				if s.spaceMacros[sp] == nil {
					s.spaceMacros[sp] = map[string]bool{}
				}
				s.spaceMacros[sp][def.Name] = true
			}
			if sp.unit == "" || !slices.Contains(def.Decorators, rustMacroExport) {
				continue
			}
			if s.macros[sp] == nil {
				s.macros[sp] = map[string]string{}
			}
			if _, taken := s.macros[sp][def.Name]; !taken {
				s.macros[sp][def.Name] = def.Key
			}
		}
	}
	return s.macros, s.spaceMacros
}

// rustMacroExport is the attribute that puts a macro at the root of its
// crate.
const rustMacroExport = "@macro_export"

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
	if b.asked != nil {
		b.asked[k] = true
	}
	if s.resolving[k] {
		b.cut()
		return "", false
	}
	s.resolving[k] = true
	defer delete(s.resolving, k)
	for _, p := range b.rustScopePaths(importer, written) {
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
	// the target read as not-found. A module of a unit, as html of
	// dep::html, runs no code, and its node would get a record of the unit
	// for a walk that ends on a module.
	key := r.b.rustCrateKey(t.file, t.path)
	if r.b.rustUnitModule(t.file, t.path) {
		return key, "crate", true
	}
	return r.b.external(key), "crate", true
}

// rustUnitModule reports whether a path of another crate that a file names
// is a module of a unit.
func (b *builder) rustUnitModule(file, path string) bool {
	ix := b.byPath[file]
	o := b.importOrigin(ix, path, modules.FormImport)
	return slices.ContainsFunc(b.providers(o), func(pv provision) bool { return b.units[pv.module] != nil })
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
	if t, ok := b.rustLocalPath(importer, written); ok {
		return t, true
	}
	return b.rustGlobPath(importer, written)
}

// rustLocalPath resolves a path that a file writes as rustPath does, with
// no glob use of another crate.
func (b *builder) rustLocalPath(importer, written string) (rustTarget, bool) {
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
		b.cut()
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
	for _, p := range b.rustScopePaths(importer, written) {
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
// a local closure or a parameter can bind it. A single name that a glob use
// of another crate or an exported macro of a dependency gives also gets the
// edge to that item.
func (b *builder) rustScopeFallback(sc *scope, caller, first string, rest []string, target string) []resolution {
	written := strings.Join(append([]string{first}, rest...), "::")
	record := resolution{fail: unknownReceiver(first)}
	// A glob use of the file brings no name into an inline module, which
	// reads its own uses.
	t, ok := b.rustLocalPath(sc.ix.Path, written)
	if !ok && !b.rustInUseScope(sc, caller) {
		t, ok = b.rustGlobPath(sc.ix.Path, written)
	}
	switch {
	case ok && t.key != "":
		return []resolution{b.walkNamespace(t.key)}
	case ok:
		crate, names := rustCrateNames(t.path)
		r := b.importedWalk(b.byPath[t.file], fileindex.Namespace(langcode.Rust, t.path), crate, modules.FormImport, names)
		if len(rest) > 0 {
			return []resolution{r}
		}
		return []resolution{r, record}
	case len(rest) > 0:
		return []resolution{b.importedWalk(sc.ix, target, first, modules.FormImport, rest)}
	}
	if crate, ok := b.rustMacroCrate(sc.ix, first); ok {
		return []resolution{b.importedWalk(sc.ix, fileindex.Join(crate, first), crate, modules.FormImport, []string{first}), record}
	}
	return nil
}

// rustInUseScope reports whether a key of a file sits in an inline module
// with a use of its own. The index keeps the uses of a file in one scope,
// and an impl block holds no use, so a class of the file that holds a use
// is an inline module.
func (b *builder) rustInUseScope(sc *scope, key string) bool {
	s := b.rustState()
	scopes, ok := s.useScopes[sc.ix.Path]
	if !ok {
		scopes = map[string]bool{}
		for _, imp := range sc.imports {
			if k := sc.enclosing(imp.Line); k != sc.ix.Path && b.kindOf(k) == NodeClass {
				scopes[k] = true
			}
		}
		s.useScopes[sc.ix.Path] = scopes
	}
	for k := fileindex.Parent(key); k != sc.ix.Path && fileindex.HasKeyPrefix(k, sc.ix.Path); k = fileindex.Parent(k) {
		if scopes[k] {
			return true
		}
	}
	return false
}

// rustMacroCrate returns a dependency of the crate of a file whose unit
// exports a macro of the name with #[macro_export], as dep for
// render_now!() after #[macro_use] extern crate dep. The index does not
// tell a call of a macro from a call of a function, so the caller keeps
// the record of the name too.
func (b *builder) rustMacroCrate(ix *fileindex.Index, name string) (string, bool) {
	// A macro_rules! of the crate of the file shadows a macro of a
	// dependency. A function or a method of the name does not, because a
	// macro call reads macros only. The rule does not read the textual
	// scope of the macro, so a macro of the crate that the call cannot see
	// shadows too.
	exported, rules := b.rustMacros()
	if rules[b.spaceOf(ix.Path)][name] {
		return "", false
	}
	found := false
	for _, macros := range exported {
		_, ok := macros[name]
		found = found || ok
	}
	if !found {
		return "", false
	}
	for _, dep := range b.cargoOf(b.spaceOf(ix.Path)).Dependencies() {
		crate := strings.ReplaceAll(dep, "-", "_")
		o := b.importOrigin(ix, crate+"::"+name, modules.FormImport)
		if slices.ContainsFunc(b.providers(o), func(pv provision) bool {
			def := b.defs[pv.module]
			return def != nil && slices.Contains(def.Decorators, rustMacroExport)
		}) {
			return crate, true
		}
	}
	return "", false
}

// rustGlobPath returns the target in another crate that a path of a file
// names through a glob use of a module of that crate, as dep::html::render
// for render after use dep::html::*, when a unit gives the crate and the
// first name of the path in the module. A glob use of a crate that no unit
// gives can bring any name, so the path then resolves with no glob path.
// docs/projectgraph.md has the rules of the cycles of glob uses.
func (b *builder) rustGlobPath(importer, written string) (rustTarget, bool) {
	sc := b.scopes[importer]
	first := rustFirstName(written)
	if sc == nil || len(sc.wildcards) == 0 || !b.hasUnits() || strings.HasPrefix(written, "::") {
		return rustTarget{}, false
	}
	switch first {
	case "crate", "self", "super":
		return rustTarget{}, false
	}
	a := b.rustState().glob.lookup(b, globKey{path: true, scope: importer, name: first}, func() globAnswer {
		return globAnswer{path: b.findGlobPath(sc, first)}
	})
	return a.path.join(written)
}

// rustPublic reports whether a provision gives an item that a glob use of
// another crate brings: a module, a key of a third crate, or a pub item. A
// private item stays out, as fn render of dep::html for use dep::html::*.
func (b *builder) rustPublic(pv provision) bool {
	if pv.module == "" {
		return false
	}
	def := b.defs[pv.module]
	return def == nil || def.Exported
}

// globPath is the path of the module of another crate that a glob use of
// a file brings a name from, and the file that names the crate.
type globPath struct {
	path, file string
	ok         bool
}

// join returns the target of a written path under the module.
func (g globPath) join(written string) (rustTarget, bool) {
	if !g.ok {
		return rustTarget{}, false
	}
	return rustTarget{path: joinPath(g.path, strings.Split(written, "::")), file: g.file}, true
}

func (b *builder) findGlobPath(sc *scope, first string) globPath {
	importer := sc.ix.Path
	for _, imp := range sc.wildcards {
		if def := b.defs[sc.enclosing(imp.Line)]; def != nil && def.Kind == fileindex.DefClass {
			continue
		}
		path, file, ok := b.rustCratePath(importer, imp.Module)
		switch {
		case ok:
		case b.rustLocalRoot(importer, imp.Module):
			continue
		default:
			path, file = imp.Module, importer
		}
		ix := b.byPath[file]
		o := b.importOrigin(ix, joinPath(path, []string{first}), modules.FormImport)
		if slices.ContainsFunc(b.providers(o), b.rustPublic) && b.rustGlobGives(ix, path, first) {
			return globPath{path: path, file: file, ok: true}
		}
	}
	return globPath{}
}

// rustGlobGives reports whether a glob use of the module of another crate
// at a path brings a name. Another crate sees only what is pub for every
// crate: a pub item of the module, a child module, or the binding of a pub
// use or of a pub glob use. A private use or a pub(crate) item of the module
// stays out, as render of use crate::inner::render in dep::html. A glob of
// what is no module, as an enum, brings the item that the provision gives.
func (b *builder) rustGlobGives(ix *fileindex.Index, path, name string) bool {
	o := b.importOrigin(ix, path, modules.FormImport)
	read := false
	for _, pv := range b.providers(o) {
		if b.units[pv.module] == nil {
			continue
		}
		read = true
		if b.rustExports(pv.module, name, map[string]bool{}) {
			return true
		}
	}
	return !read
}

// rustExports reports whether a module of the project or of a unit gives a
// name to another crate, as rustGlobGives says.
func (b *builder) rustExports(mod, name string, seen map[string]bool) bool {
	if seen[mod] {
		return false
	}
	seen[mod] = true
	u := b.units[mod]
	if slices.ContainsFunc(u.names[name], func(k string) bool { return b.defs[k] != nil && b.defs[k].Exported }) {
		return true
	}
	if _, pkg, ok := strings.Cut(mod, ":"); ok && b.namespaces[langcode.Rust].units[pkg+"::"+name] != nil {
		return true
	}
	for _, p := range u.files {
		sc := b.scopes[p]
		if sc == nil {
			continue
		}
		for _, imp := range sc.imports {
			if !imp.Exported || sc.enclosing(imp.Line) != p {
				continue
			}
			if !imp.Wildcard {
				if bindingName(langcode.Rust, imp) == name {
					return true
				}
				continue
			}
			if b.rustLocalRoot(p, imp.Module) {
				if m, _, ok := b.resolveModule(sc.ix, imp.Module); ok && b.units[m] != nil && b.rustExports(m, name, seen) {
					return true
				}
				continue
			}
			o := b.importOrigin(sc.ix, imp.Module, modules.FormImport)
			if slices.ContainsFunc(b.providers(o), func(pv provision) bool {
				return b.units[pv.module] != nil && b.rustExports(pv.module, name, seen)
			}) {
				return true
			}
		}
	}
	return false
}

// rustNestedName reports whether an inline module of the graph defines a
// type of the name. Most names in a Rust file name no such type, and a walk
// up the classes that hold each name cost a fifth of the link step.
func (b *builder) rustNestedName(name string) bool {
	s := b.rustState()
	if s.nested == nil {
		s.nested = map[string]bool{}
		for key := range b.g.Nodes {
			if b.isType(key) && b.isType(fileindex.Parent(key)) {
				s.nested[fileindex.Last(key)] = true
			}
		}
	}
	return s.nested[name]
}

// rustCrateMember returns the external key of a path of another crate that
// a use of a file of a unit binds, as inner::text of pub use inner::text in
// the prelude of dep. The walk then goes on in the unit of that crate.
// Without the key, dep::prelude::text named nothing, and text::fmt after
// use dep::prelude::* read as a crate text.
func (b *builder) rustCrateMember(sc *scope, path string) (string, bool) {
	if !b.rustUnitFile(sc.ix) || b.rustLocalRoot(sc.ix.Path, path) {
		return "", false
	}
	return b.rustCrateKey(sc.ix.Path, path), true
}

// rustGlobMember returns the external key of a name that a glob use of a
// file of a unit brings from another crate, as inner::ifn for ifn after
// pub use inner::*. A file of the project reads such a name in rustPath,
// which keeps the record of a single name.
func (b *builder) rustGlobMember(sc *scope, name string) (string, bool) {
	if !b.rustUnitFile(sc.ix) {
		return "", false
	}
	t, ok := b.rustGlobPath(sc.ix.Path, name)
	if !ok {
		return "", false
	}
	return b.rustCrateKey(t.file, t.path), true
}

// rustGlobUnitMember returns what a glob use of a module brings for a name
// that no definition of the module has: a child module, or the target of a
// use or of a glob use of the module, as leaf for use crate::sub::* with
// pub mod leaf in sub.
func (b *builder) rustGlobUnitMember(mod, name string) (string, bool) {
	a := b.rustState().glob.lookup(b, globKey{scope: mod, name: name}, func() globAnswer {
		key, ok := b.unitMember(mod, name)
		return globAnswer{key: key, ok: ok}
	})
	return a.key, a.ok
}

// globKey names a glob lookup: the glob path of a file and a first name, or
// the glob member of a module and a name.
type globKey struct {
	path        bool
	scope, name string
}

// globAnswer is the answer of a glob lookup, with the least depth that a cut
// under it hit.
type globAnswer struct {
	key  string
	ok   bool
	path globPath
	low  int
}

// found reports whether a glob lookup found its item.
func (a globAnswer) found() bool {
	return a.ok || a.path.ok
}

// globMemo holds the glob lookups of Rust that a build reads now and has
// read, as lookup says.
type globMemo struct {
	// active holds the depth of each glob path and glob member that a
	// lookup reads now. depth is the depth of the last one, and low the
	// least depth that a cut of the read in progress hit.
	active map[globKey]int
	depth  int
	low    int
	// kept holds the answers of the glob lookups that no cut of an outer
	// read gave, for the build. visited holds the other answers of the
	// outermost lookup in progress, and visits the order of their reads.
	kept    map[globKey]globAnswer
	visited map[globKey]globAnswer
	visits  []globKey
	// cuts counts the cuts of the glob guards, a part of b.cuts.
	cuts int
}

// lookup reads a glob lookup once. Modules and crates can bring each other
// with glob uses, so the outermost lookup reads each lookup once, and a
// lookup that it reads again gives its first answer. An answer stays for
// the build when each cut under it hit the lookup or a lookup under it, as
// the low link of Tarjan tells. docs/projectgraph.md has the rules for the
// visited answers under it.
func (g *globMemo) lookup(b *builder, k globKey, read func() globAnswer) globAnswer {
	if a, ok := g.kept[k]; ok {
		return a
	}
	if d, ok := g.active[k]; ok {
		g.cut(b, d)
		return globAnswer{}
	}
	if a, ok := g.visited[k]; ok {
		g.cut(b, a.low)
		return a
	}
	if len(g.active) == 0 {
		defer func() {
			clear(g.visited)
			g.visits = g.visits[:0]
		}()
	}
	g.depth++
	d := g.depth
	g.active[k] = d
	low, cuts, mark := g.low, b.cuts-g.cuts, len(g.visits)
	g.low = math.MaxInt
	a := read()
	a.low = g.low
	g.low = min(low, g.low)
	delete(g.active, k)
	g.depth--
	under := g.visits[mark:]
	if a.low >= d && b.cuts-g.cuts == cuts {
		remember(b, g.kept, k, a)
		for _, v := range under {
			if e, ok := g.visited[v]; ok && e.found() && e.low >= d {
				remember(b, g.kept, v, e)
			}
			delete(g.visited, v)
		}
		g.visits = g.visits[:mark]
		return a
	}
	for _, v := range under {
		if e, ok := g.visited[v]; ok && e.low > a.low {
			e.low = a.low
			g.visited[v] = e
		}
	}
	g.visits = append(g.visits, k)
	remember(b, g.visited, k, a)
	return a
}

// cut counts a read that the guard of a glob path or of a glob member cuts
// at the depth.
func (g *globMemo) cut(b *builder, depth int) {
	b.cut()
	g.cuts++
	g.low = min(g.low, depth)
}

// read starts a read that the cuts of the glob guards can make incomplete.
// The function that it returns ends the read and reports whether no cut of
// the read hit a module that a lookup read before the read began.
func (g *globMemo) read() func() bool {
	depth, low := g.depth, g.low
	g.low = math.MaxInt
	return func() bool {
		whole := g.low > depth
		g.low = min(low, g.low)
		return whole
	}
}

// rustUnitFile reports whether a file is Rust code of a unit of dependency
// code.
func (b *builder) rustUnitFile(ix *fileindex.Index) bool {
	return ix.Language == langcode.Rust && ix.Unit != ""
}

// joinImplBlocks makes an impl block of a type of another module one class
// with the type, as impl Store for FileStore after use crate::FileStore.
// Only a use at the root of the file counts. The root cannot define a type
// of a name that such a use binds, so the class is an impl block. A use of
// an inline module, as use crate::other::A in mod deep, binds A in deep
// only. Read for the root, it joined a struct A of the root with the A of
// the other module, and a call of a method of the root A went to the other
// A with no record.
func (b *builder) joinImplBlocks() {
	var groups [][]string
	for _, p := range b.paths {
		ix := b.byPath[p]
		if ix == nil || ix.Language != langcode.Rust {
			continue
		}
		sc := b.scopes[p]
		for _, c := range ix.Classes {
			if len(fileindex.Segments(c.Key)) != 1 {
				continue
			}
			for _, imp := range rustBindings(sc, fileindex.Last(c.Key)) {
				if imp.Item == "" && imp.Alias == "" || sc.enclosing(imp.Line) != p {
					continue
				}
				module, _ := importedModule(ix.Language, imp)
				if key, _, found := b.resolveModule(ix, module); found && key != c.Key && b.isType(key) {
					groups = append(groups, []string{key, c.Key})
				}
			}
		}
	}
	b.joinAliases(groups)
}

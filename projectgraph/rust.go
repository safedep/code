package projectgraph

import (
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
	resolver:          func(b *builder, _ *space) modules.Resolver { return rustResolver{b} },
	units:             (*builder).addRustUnits,
	aliases:           (*builder).joinImplBlocks,
	link:              (*builder).linkRust,
	scopeFallback:     (*builder).rustScopeFallback,
	dependencies:      func(b *builder) ([]string, []modules.Problem) { return b.rustState().cargo.Dependencies(), nil },
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

// rustRoots are the first names of the paths of the crates of the
// toolchain, which no crate of the project defines.
var rustRoots = map[string]bool{"std": true, "core": true, "alloc": true}

// rustState is the state of Rust in a build: the crates, the module path
// of each Rust file, and the paths that a resolution reads now, by the file
// that writes them.
type rustState struct {
	cargo     *modules.Cargo
	modules   map[string]string
	resolving map[[2]string]bool
}

func (b *builder) rustState() *rustState {
	return langState[rustState](b, langcode.Rust)
}

// addRustUnits makes each Rust file the unit of its module, in the table of
// the module paths.
func (b *builder) addRustUnits() {
	s := b.rustState()
	s.cargo = modules.NewCargo(b.project.files, b.opts.Root)
	b.problems = append(b.problems, s.cargo.Problems()...)
	s.modules = map[string]string{}
	s.resolving = map[[2]string]bool{}
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
func (b *builder) rustModule(p string) string {
	crate, ok := b.rustState().cargo.Crate(p)
	name, rel := "crate", p
	if ok {
		name = crate.Name
		if crate.Dir != "" {
			rel = strings.TrimPrefix(p, crate.Dir+"/")
		}
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
	if b.rustState().cargo.Has(parts[0]) {
		out = append(out, written)
	}
	return out
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
// with its path.
type rustResolver struct{ b *builder }

func (r rustResolver) Module(importer, spec string) (string, string, bool) {
	if key, ok := r.b.resolveRust(importer, spec); ok {
		return key, "path", true
	}
	return "", "", false
}

// FirstParty reports whether a path starts with crate, self or super, with
// the name of a crate of the project or with the name of a file or a
// directory of the project, as a child module.
func (r rustResolver) FirstParty(spec string) bool {
	first, _, _ := strings.Cut(strings.TrimPrefix(spec, "::"), "::")
	switch {
	case first == "crate" || first == "self" || first == "super":
		return true
	case rustRoots[first]:
		return false
	}
	return r.b.rustState().cargo.Has(first) || r.b.project.files.HasName(first)
}

// rustScopeFallback resolves a path that no use and no item of the module
// binds: a path of the crate, as crate::store::save or store::save for a
// child module, or else a path of another crate, as serde_json::to_string,
// whose external key the edge gets. A single name stays a record, because
// a local closure can bind it.
func (b *builder) rustScopeFallback(sc *scope, _, first string, rest []string, target string) []resolution {
	written := strings.Join(append([]string{first}, rest...), "::")
	if key, ok := b.resolveRust(sc.ix.Path, written); ok {
		return []resolution{b.walkNamespace(key)}
	}
	if len(rest) > 0 {
		return []resolution{b.externalResolution(target)}
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

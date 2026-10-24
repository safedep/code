package projectgraph

import (
	"cmp"
	"context"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

// space is the module resolution of the project or of one package unit of
// dependency code. The link step treats each unit as a project of its own:
// its paths are the paths inside the package, and its manifests are under
// the root that Options.Units gives it.
type space struct {
	unit string
	pkg  fileindex.PackageUnit
	root fs.FS

	files  *modules.Files
	python *modules.Python
	js     *modules.JavaScript
	golang *modules.Golang
	// goPackages holds the package that an import of each directory names.
	goPackages map[string]*unit
	resolvers  map[langcode.Code]modules.Resolver
}

// key returns the key of a path of the space.
func (sp *space) key(p string) string {
	return fileindex.UnitPath(sp.unit, p)
}

// inner returns the path of a key of the space inside its unit.
func (sp *space) inner(key string) string {
	if sp.unit == "" {
		return key
	}
	return strings.TrimPrefix(key, sp.unit+fileindex.Separator)
}

// keyed returns a resolver that takes and gives the keys of the space,
// around a resolver of its paths.
func (sp *space) keyed(r modules.Resolver) modules.Resolver {
	if sp.unit == "" {
		return r
	}
	return spaceResolver{sp: sp, r: r}
}

type spaceResolver struct {
	sp *space
	r  modules.Resolver
}

func (k spaceResolver) Module(importer, spec string) (string, string, bool) {
	mod, via, ok := k.r.Module(k.sp.inner(importer), spec)
	if !ok {
		return "", "", false
	}
	return k.sp.key(mod), via, true
}

// Modules resolves in the form when the resolver of the paths reads it.
func (k spaceResolver) Modules(importer, spec string, form modules.Form) ([]string, string, bool) {
	fr, ok := k.r.(modules.FormResolver)
	if !ok {
		mod, via, ok := k.Module(importer, spec)
		if !ok {
			return nil, "", false
		}
		return []string{mod}, via, true
	}
	mods, via, ok := fr.Modules(k.sp.inner(importer), spec, form)
	if !ok {
		return nil, "", false
	}
	for i, mod := range mods {
		mods[i] = k.sp.key(mod)
	}
	return mods, via, true
}

func (k spaceResolver) FirstParty(spec string) bool {
	return k.r.FirstParty(spec)
}

// FirstPartyFrom reads the importer when the resolver of the paths does.
func (k spaceResolver) FirstPartyFrom(importer, spec string) bool {
	if ir, ok := k.r.(modules.ImporterResolver); ok {
		return ir.FirstPartyFrom(k.sp.inner(importer), spec)
	}
	return k.r.FirstParty(spec)
}

// addSpaces makes the space of the project and the space of each unit that
// has a file. A pypi unit with an __init__.py at its root is its package
// directory, and its root is the package of the name of the unit.
func (b *builder) addSpaces() error {
	byUnit := map[string][]string{"": nil}
	for _, p := range b.paths {
		unit, inner := fileindex.SplitUnit(p)
		byUnit[unit] = append(byUnit[unit], inner)
	}
	b.hasProject = len(byUnit[""]) > 0
	for _, name := range slices.Sorted(maps.Keys(byUnit)) {
		sp := &space{unit: name, root: b.opts.Root, goPackages: map[string]*unit{}, resolvers: map[langcode.Code]modules.Resolver{}}
		sysPath := b.opts.SysPath
		if name != "" {
			pkg, err := fileindex.ParseUnit(name)
			if err != nil {
				return err
			}
			sp.pkg, sp.root, sysPath = pkg, b.opts.Units[name], nil
		}
		sp.files = modules.NewFiles(byUnit[name])
		sp.python = modules.NewPython(sp.files, sp.root, sysPath)
		if sp.pkg.Ecosystem == pypi && sp.files.Has(pythonInit) {
			sp.python.SetRootPackage(pythonImportName(sp.pkg.Name))
		}
		if sp.pkg.Ecosystem == pypi {
			b.readDistribution(sp)
		}
		sp.js = modules.NewJavaScript(sp.files, sp.root)
		b.addProblems(sp, sp.python.Problems())
		b.addProblems(sp, sp.js.Problems())
		b.spaces[name] = sp
		b.ordered = append(b.ordered, sp)
	}
	b.project = b.spaces[""]
	return nil
}

const (
	pypi       = "pypi"
	pythonInit = "__init__.py"
)

// pythonImportName returns the import name that the name of a pypi package
// gives with no other knowledge: in lower case, with _ for - and for ., as
// typing_extensions for typing-extensions. A package whose import name
// differs, as yaml for PyYAML, needs the .dist-info of its wheel at the
// root of the unit, or its package directory under an import directory.
func pythonImportName(name string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(name))
}

// readDistribution reads the .dist-info directories at the root of a pypi
// unit, as a site-packages directory or an unpacked wheel holds them, and
// gives the resolver of the unit the one of its package: the import names
// that it installs and its extension modules. A root with several
// distributions, as site-packages, matches by the normalized name, and by
// the version when two share the name.
func (b *builder) readDistribution(sp *space) {
	dists, problems := modules.PythonDistributions(sp.root)
	b.addProblems(sp, problems)
	name := modules.NormalizePythonName(sp.pkg.Name)
	var found *modules.Distribution
	for i := range dists {
		d := &dists[i]
		if modules.NormalizePythonName(d.Name) != name {
			continue
		}
		if found == nil || d.Version == sp.pkg.Version {
			found = d
		}
	}
	if found != nil {
		sp.python.SetDistribution(*found)
	}
}

// pythonTopLevel returns the import names that a pypi unit installs: the
// names of its .dist-info, or else the name of the package as an import
// writes it.
func pythonTopLevel(sp *space) []string {
	if names := sp.python.TopLevel(); len(names) > 0 {
		return names
	}
	return []string{pythonImportName(sp.pkg.Name)}
}

// checkUnits records a problem and a record of each pypi unit that no
// import names and that gives no module for any of its import names. The
// link step cannot tell which import would load such a unit, so the record
// holds for every search, and it makes uncertain every target that the
// code of the unit reaches. A build with no project file, as a summary,
// has no importer, so it skips the check.
func (b *builder) checkUnits() {
	if !b.hasProject {
		return
	}
	for _, sp := range b.ordered {
		if sp.unit == "" || sp.pkg.Ecosystem != pypi || b.named[sp.unit] {
			continue
		}
		names := pythonTopLevel(sp)
		if slices.ContainsFunc(names, func(name string) bool {
			return slices.ContainsFunc(b.provide(sp, langcode.Python, name, modules.FormImport), func(pv provision) bool { return pv.module != "" })
		}) {
			continue
		}
		text := strings.Join(names, ", ")
		b.problems = append(b.problems, modules.Problem{Path: sp.unit, Text: "no import names the unit, and it gives no module for the package " + text})
		b.addUnresolved(UnresolvedSite{Site: Site{Path: sp.unit}, Text: text, Reason: fileindex.UnresolvedNoDefinition, Unit: sp.unit})
	}
}

// addProblems adds the problems of a resolver of the space, by key.
func (b *builder) addProblems(sp *space, problems []modules.Problem) {
	for _, p := range problems {
		p.Path = sp.key(p.Path)
		b.problems = append(b.problems, p)
	}
}

// spaceOf returns the space of a key: the space of its unit, or the space
// of the project.
func (b *builder) spaceOf(key string) *space {
	if sp := b.spaces[fileindex.PackageOf(key)]; sp != nil {
		return sp
	}
	return b.project
}

// sameUnit reports whether two keys are of one unit, or both of the
// project.
func sameUnit(a, b string) bool {
	return fileindex.PackageOf(a) == fileindex.PackageOf(b)
}

// submodule returns the Python module of the name under a package module.
func (b *builder) submodule(module, name string) (string, bool) {
	sp := b.spaceOf(module)
	sub, ok := sp.python.Submodule(sp.inner(module), name)
	if !ok {
		return "", false
	}
	return sp.key(sub), true
}

// origin is an import that an external key comes from: the language and the
// unit of the importer, the module as the import writes it, the form of the
// import and the names from the module to the key.
type origin struct {
	lang   langcode.Code
	unit   string
	module string
	form   modules.Form
	names  []string
}

func (o origin) equal(other origin) bool {
	return o.lang == other.lang && o.unit == other.unit && o.module == other.module && o.form == other.form && slices.Equal(o.names, other.names)
}

// formOf returns the form of an import: a require, or else an import.
func formOf(imp fileindex.Import) modules.Form {
	if imp.Require {
		return modules.FormRequire
	}
	return modules.FormImport
}

// importedExternal returns the external node of a key that an import of the
// file gives, and records the import as its origin.
func (b *builder) importedExternal(ix *fileindex.Index, key, module string, form modules.Form, names []string) string {
	b.recordOrigin(ix, key, module, form, names)
	return b.external(key)
}

// recordOrigin records an import as an origin of the key. A graph with no
// unit of dependency code links no external key, so it records nothing.
func (b *builder) recordOrigin(ix *fileindex.Index, key, module string, form modules.Form, names []string) {
	if len(b.spaces) < 2 {
		return
	}
	o := origin{lang: ix.Language, unit: ix.Unit, module: b.packageModule(ix, module), form: form, names: slices.Clone(names)}
	if !slices.ContainsFunc(b.origins[key], o.equal) {
		b.origins[key] = append(b.origins[key], o)
	}
}

// packageModule returns the module of the package that an import of the
// file names, by the profile of its family.
func (b *builder) packageModule(ix *fileindex.Index, module string) string {
	if imported := profileOf(ix.Language.Family()).importedModule; imported != nil {
		return imported(b, ix, module)
	}
	return module
}

// deriveExternal returns the external node of the names under an external
// key. The origins of the key, with the names after their own, are the
// origins of the new key.
func (b *builder) deriveExternal(parent string, names []string) string {
	return b.external(b.deriveKey(parent, names))
}

// deriveKey returns the key of the names under an external key, with the
// origins of the key and the names after their own, and makes no node.
func (b *builder) deriveKey(parent string, names []string) string {
	key := fileindex.Join(parent, names...)
	for _, o := range b.origins[parent] {
		o.names = append(slices.Clone(o.names), names...)
		if !slices.ContainsFunc(b.origins[key], o.equal) {
			b.origins[key] = append(b.origins[key], o)
		}
	}
	return key
}

// provision is the module of a unit that gives the module of an origin. An
// empty module is a unit that gives the package but not the module, as a
// subpath that the package does not have, or an extension module with no
// Python source, which compiled marks.
type provision struct {
	sp       *space
	module   string
	via      string
	compiled bool
}

// provisionKey is what the providers of an origin depend on.
type provisionKey struct {
	family langcode.Code
	module string
	form   modules.Form
}

// providers returns the modules of the units that give the module of an
// origin, in the order of the units. A unit gives the module when its
// package has the name of the import: the npm name for JavaScript, the
// module path for Go, and the top-level package for Python. The answer
// depends only on the family, the module and the form, so the builder
// keeps it.
func (b *builder) providers(o origin) []provision {
	family := o.lang.Family()
	k := provisionKey{family: family, module: o.module, form: o.form}
	if out, ok := b.provisions[k]; ok {
		return out
	}
	var out []provision
	for _, sp := range b.ordered {
		if sp.unit != "" {
			out = append(out, b.provide(sp, family, o.module, o.form)...)
		}
	}
	if narrow := profileOf(family).narrowProviders; narrow != nil {
		out = narrow(out)
	}
	b.provisions[k] = out
	return out
}

// provide returns the modules of a unit that give a module of the family,
// as an import of the form names it, by the profile of the family.
func (b *builder) provide(sp *space, family langcode.Code, module string, form modules.Form) []provision {
	if provide := profileOf(family).provide; provide != nil {
		return provide(b, sp, module, form)
	}
	return nil
}

// linkPackages adds a package edge from each external key that an import
// gives to the definition of each unit that gives it, as from lodash//template
// to npm:lodash@4.17.21//template.js//template. The external node stays,
// because a symbol record names it, and the path goes on into the package.
// A walk into a package can reach more external keys, so the step runs
// until it reads no new one.
//
// A unit that gives the package and not the name, as a name that the
// package makes at run time, is a record of the unit on the external node.
// Any code of the unit can run there.
func (b *builder) linkPackages(ctx context.Context) error {
	if len(b.spaces) < 2 {
		return nil
	}
	for {
		var keys []string
		for k := range b.origins {
			if !b.linked[k] && b.kindOf(k) == NodeExternal {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 {
			return nil
		}
		slices.Sort(keys)
		for _, k := range keys {
			if err := ctx.Err(); err != nil {
				return err
			}
			b.linked[k] = true
			for _, o := range b.origins[k] {
				for _, pv := range b.providers(o) {
					b.linkProvision(k, o, pv)
				}
			}
		}
	}
}

func (b *builder) linkProvision(key string, o origin, pv provision) {
	r := rulePackage.via(cmp.Or(pv.via, "name"))
	fail := func(reason fileindex.UnresolvedReason) {
		b.addUnresolved(UnresolvedSite{Caller: key, Site: Site{Path: pv.sp.unit}, Text: key, Reason: reason, Unit: pv.sp.unit})
	}
	if pv.module == "" {
		fail(fileindex.UnresolvedNoDefinition)
		return
	}
	b.named[pv.sp.unit] = true
	names := o.names
	if len(names) == 0 {
		// The key of a whole module is the value of the module. Only the
		// default export of JavaScript gives that value code, and an
		// import edge runs the module top level.
		def := profileOf(o.lang).defaultExport
		if def == "" {
			return
		}
		names = []string{def}
	}
	res := b.walk(pv.module, names)
	if res.fail != nil {
		fail(res.fail.reason)
		return
	}
	n := b.g.Nodes[res.key]
	if res.key == key || n == nil {
		return
	}
	b.link(o.lang, key, res.key, Site{Path: cmp.Or(n.Path, pv.sp.unit), Line: n.Line}, EdgePackage, r)
}

// addSurfaces links an external node to each exported symbol of each unit,
// as an import of the package by its name would write it.
func (b *builder) addSurfaces() {
	for _, sp := range b.ordered {
		if sp.unit == "" {
			continue
		}
		for _, s := range b.surface(sp) {
			if !slices.ContainsFunc(b.origins[s.key], s.origin.equal) {
				b.origins[s.key] = append(b.origins[s.key], s.origin)
			}
			b.external(s.key)
		}
	}
}

type surfaceSymbol struct {
	key    string
	origin origin
}

// surface returns the exported symbols of a unit with the import that names
// each one, as the surface hook of each language family adds them.
func (b *builder) surface(sp *space) []surfaceSymbol {
	s := &surfacer{b: b, sp: sp}
	for _, p := range profiles {
		if p.surface != nil {
			p.surface(s)
		}
	}
	return s.out
}

// surfacer collects the exported symbols of one unit of dependency code.
type surfacer struct {
	b   *builder
	sp  *space
	out []surfaceSymbol
}

// add adds the symbol of the names under a module, as an import of the
// form names it.
func (s *surfacer) add(lang langcode.Code, module string, form modules.Form, names ...string) {
	key := fileindex.Join(fileindex.Namespace(lang, module), names...)
	s.out = append(s.out, surfaceSymbol{key: key, origin: origin{lang: lang, unit: s.sp.unit, module: module, form: form, names: names}})
}

// exported adds the name of a module and the public methods of the class
// that the name holds, as kit//Store//close. The default export of
// JavaScript is the value of the module, so its key has no name.
func (s *surfacer) exported(ix *fileindex.Index, module string, form modules.Form, name string) {
	b := s.b
	var names []string
	if name != profileOf(ix.Language).defaultExport {
		names = []string{name}
	}
	s.add(ix.Language, module, form, names...)
	key, fail := b.resolveName(ix.Path, name, 0)
	rec, ok := b.classes[b.deref(key)]
	if fail != nil || !ok {
		return
	}
	for _, m := range rec.class.Methods {
		if b.kindOf(m) == NodeMethod && !strings.HasPrefix(fileindex.Last(m), "_") {
			s.add(ix.Language, module, form, append(slices.Clone(names), fileindex.Last(m))...)
		}
	}
}

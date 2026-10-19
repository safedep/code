package projectgraph

import (
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph/modules"
)

// linkImports adds an import edge from the definition that holds each
// import, or the module of each re-export, to the module top level that it
// runs.
func (b *builder) linkImports(ix *fileindex.Index) {
	sc := b.scopes[ix.Path]
	for _, imp := range ix.Imports {
		if imp.Kind != fileindex.ImportStatic && imp.Kind != fileindex.ImportDynamicString {
			continue
		}
		from := b.runner(sc.enclosing(imp.Line))
		site := Site{Path: ix.Path, Line: imp.Line}
		mods, missing := b.importedModules(ix, imp.Module, formOf(imp))
		for _, unit := range missing {
			b.addUnresolved(UnresolvedSite{Caller: from, Site: site, Text: imp.Module, Reason: fileindex.UnresolvedNoDefinition, Unit: unit})
		}
		// The record has no method, so it makes every target uncertain
		// where the search reaches the import.
		if len(mods) == 0 && len(missing) == 0 && b.mayBeFirstParty(ix, imp.Module) {
			b.addUnresolved(UnresolvedSite{Caller: from, Site: site, Text: imp.Module, Reason: fileindex.UnresolvedNoDefinition})
		}
		for _, m := range mods {
			for _, to := range b.moduleFiles(m.key) {
				b.link(ix.Language, from, to, site, EdgeImport, ruleImport.via(m.via))
			}
			if !profileOf(ix.Language).moduleNames || imp.Item == "" || b.bindsName(m.key, imp.Item) {
				continue
			}
			if sub, ok := b.submodule(m.key, imp.Item); ok && b.isModule(sub) {
				b.link(ix.Language, from, sub, site, EdgeImport, ruleImport.via("submodule"))
			}
		}
	}
	for _, ex := range ix.Exports {
		if ex.From == "" {
			continue
		}
		mods, _ := b.importedModules(ix, ex.From, modules.FormImport)
		for _, m := range mods {
			if b.isModule(m.key) {
				site := Site{Path: ix.Path, Line: ex.Line}
				b.link(ix.Language, ix.Path, m.key, site, EdgeImport, ruleImport.via(m.via))
			}
		}
	}
}

// moduleRef is a module that an import names, and how the link step found
// it.
type moduleRef struct{ key, via string }

// importedModules returns the modules whose top level an import of the
// spec in the form runs: the modules of the space of the file, or else the
// module of each unit of dependency code that gives the package. A package
// edge does not stand for an import, because the external key of a
// JavaScript module is also the key of its default export, and the import
// does not call it. missing holds each unit that gives the package and not
// the module.
func (b *builder) importedModules(ix *fileindex.Index, spec string, form modules.Form) ([]moduleRef, []string) {
	if mods := b.resolveModules(ix, spec, form); len(mods) > 0 {
		return mods, nil
	}
	if len(b.spaces) < 2 {
		return nil, nil
	}
	var mods []moduleRef
	var missing []string
	for _, pv := range b.providers(origin{lang: ix.Language, unit: ix.Unit, module: b.packageModule(ix, spec), form: form}) {
		if pv.module == "" {
			missing = append(missing, pv.sp.unit)
			continue
		}
		b.named[pv.sp.unit] = true
		mods = append(mods, moduleRef{key: pv.module, via: "package"})
	}
	return mods, missing
}

// moduleFiles returns the module top levels that an import of the module
// runs: the module, or each file of a unit that runs code when the program
// loads it, as a Go file with an init function or a package variable that
// a call sets. A Go package can have hundreds of files, and an edge to each
// file with no such code adds nothing to a path.
func (b *builder) moduleFiles(mod string) []string {
	if u := b.units[mod]; u != nil {
		return slices.DeleteFunc(slices.Clone(u.files), func(p string) bool { return !b.runsAtLoad(b.byPath[p]) })
	}
	if b.isModule(mod) {
		return []string{mod}
	}
	return nil
}

// runsAtLoad reports whether the module top level of a file runs code: an
// init function, a call, a reference, an unresolved site, a flag, or an
// import of a package of the project that runs code at load.
func (b *builder) runsAtLoad(ix *fileindex.Index) bool {
	if runs, ok := b.loads[ix.Path]; ok {
		return runs
	}
	// The mark stops a cycle of imports.
	b.loads[ix.Path] = false
	atTop := func(caller string) bool { return caller == ix.Path }
	runs := b.kindOf(fileindex.Join(ix.Path, "init")) == NodeFunction ||
		slices.ContainsFunc(ix.Calls, func(c fileindex.Call) bool { return atTop(c.Caller) && c.Kind != fileindex.CallImport }) ||
		slices.ContainsFunc(ix.References, func(r fileindex.Reference) bool { return atTop(r.Caller) }) ||
		slices.ContainsFunc(ix.Unresolved, func(u fileindex.Unresolved) bool { return atTop(u.Caller) }) ||
		len(b.g.Nodes[ix.Path].Flags) > 0 ||
		slices.ContainsFunc(ix.Imports, func(imp fileindex.Import) bool {
			return slices.ContainsFunc(b.resolveModules(ix, imp.Module, formOf(imp)), func(m moduleRef) bool { return len(b.moduleFiles(m.key)) > 0 })
		})
	b.loads[ix.Path] = runs
	return runs
}

// importPrefix returns the key form of the static prefix of a computed
// import on the line. The Module of a computed import holds that prefix.
func (b *builder) importPrefix(ix *fileindex.Index, line int) string {
	for _, imp := range ix.Imports {
		if imp.Line != line || imp.Kind != fileindex.ImportDynamicComputed || imp.Module == "" {
			continue
		}
		spec := imp.Module
		if strings.HasPrefix(spec, ".") {
			trailing := strings.HasSuffix(spec, "/")
			if relative := profileOf(ix.Language).relativePath; relative != nil {
				trailing = trailing || strings.HasSuffix(spec, ".")
				spec = relative(spec)
			}
			sp := b.spaceOf(ix.Path)
			p, ok := modules.Join(modules.Dir(sp.inner(ix.Path)), spec)
			if !ok {
				return ""
			}
			if trailing {
				p += "/"
			}
			return sp.key(p)
		}
		return fileindex.Namespace(ix.Language, spec)
	}
	return ""
}

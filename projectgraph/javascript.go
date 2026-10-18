package projectgraph

import (
	"maps"
	"slices"
	"strings"

	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

var javascriptProfile = ecmascriptProfile(langcode.Javascript, false)

var typescriptProfile = ecmascriptProfile(langcode.Typescript, true)

func ecmascriptProfile(code langcode.Code, typescript bool) profile {
	p := profile{
		code:            code,
		resolver:        func(_ *builder, sp *space) modules.Resolver { return sp.keyed(sp.js.For(typescript)) },
		dependencyCode:  true,
		instance:        []string{"this"},
		superCalls:      true,
		literals:        map[string]bool{"undefined": true, "null": true, "true": true, "false": true, "NaN": true, "Infinity": true},
		defaultExport:   "default",
		openBases:       true,
		structuralTypes: true,
		memberFields:    true,
	}
	// The hooks of a family live on the profile of the family.
	if !typescript {
		p.provide = javascriptProvide
		p.surface = javascriptSurface
		p.dependencies = javascriptDependencies
	}
	return p
}

// javascriptDependencies returns the packages that the package.json files
// of the project depend on.
func javascriptDependencies(b *builder) ([]string, []modules.Problem) {
	var names []string
	for _, pj := range b.project.js.Packages() {
		names = append(names, pj.DependencyNames()...)
	}
	return names, nil
}

// javascriptProvide returns the files of an npm unit that an import of a
// module of its package names.
func javascriptProvide(_ *builder, sp *space, module string, form modules.Form) []provision {
	name, sub := modules.SplitPackage(module)
	if sp.pkg.Ecosystem != "npm" || sp.pkg.Name != name || strings.HasPrefix(module, ".") {
		return nil
	}
	files, via, ok := packageFiles(sp, sub, form)
	if !ok {
		return []provision{{sp: sp}}
	}
	var out []provision
	for _, file := range files {
		out = append(out, provision{sp: sp, module: sp.key(file), via: via})
	}
	return out
}

// packageFiles returns the files of a subpath of the npm package of a unit
// for an import of the form. A package runs its JavaScript, so a
// JavaScript file wins over a TypeScript declaration of the same path, as
// index.js over index.d.ts. A package with TypeScript sources only gives a
// TypeScript file.
func packageFiles(sp *space, sub string, form modules.Form) ([]string, string, bool) {
	if files, via, ok := sp.js.Package(sub, false, form); ok {
		return files, via, true
	}
	return sp.js.Package(sub, true, form)
}

// javascriptSurface adds the names that each subpath of an npm package
// exports, for an import and for a require.
func javascriptSurface(s *surfacer) {
	sp := s.sp
	if sp.pkg.Ecosystem != "npm" {
		return
	}
	for _, sub := range sp.js.Subpaths() {
		for _, form := range []modules.Form{modules.FormImport, modules.FormRequire} {
			files, _, ok := packageFiles(sp, sub, form)
			if !ok {
				continue
			}
			for _, file := range files {
				ix := s.b.byPath[sp.key(file)]
				if ix == nil {
					continue
				}
				for _, name := range s.b.exportNames(ix.Path, map[string]bool{}) {
					s.exported(ix, sp.pkg.Name+strings.TrimPrefix(sub, "."), form, name)
				}
			}
		}
	}
}

// exportNames returns the names that a JavaScript file exports, with the
// names of its export * from.
func (b *builder) exportNames(p string, seen map[string]bool) []string {
	sc := b.scopes[p]
	if sc == nil || seen[p] || len(seen) > b.opts.maxExportHops {
		return nil
	}
	seen[p] = true
	names := slices.Collect(maps.Keys(sc.exports))
	for _, st := range sc.stars {
		if mod, _, ok := b.resolveModule(sc.ix, st.spec); ok {
			names = append(names, slices.DeleteFunc(b.exportNames(mod, seen), func(n string) bool { return n == sc.profile.defaultExport })...)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// isDeclarationFile reports whether a TypeScript file only declares types.
func isDeclarationFile(p string) bool {
	return strings.HasSuffix(p, ".d.ts")
}

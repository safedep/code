package modules

import "strings"

// Resolver resolves the modules that the files of one language import.
type Resolver interface {
	// Module returns the module that the importer names with the spec, and
	// how the resolver found it. A resolver that reads the form of an
	// import takes FormImport.
	Module(importer, spec string) (string, string, bool)
	// FirstParty reports whether a spec that names no module can still
	// name a file of the project, through a search path root, an alias or
	// a build step.
	FirstParty(spec string) bool
}

// FormResolver is a Resolver whose answer depends on the form of the
// import, as the exports map of a package.json does.
type FormResolver interface {
	Resolver
	// Modules returns the modules that the importer names with the spec in
	// the form, and how the resolver found them. One import names one
	// module, unless the manifest of its package names no module for the
	// form and the resolver takes every module of the manifest.
	Modules(importer, spec string, form Form) ([]string, string, bool)
}

// Module returns the module of a Python import, found as relative or
// absolute.
func (p *Python) Module(importer, spec string) (string, string, bool) {
	mod, ok := p.Resolve(importer, spec)
	via := "absolute"
	if strings.HasPrefix(spec, ".") {
		via = "relative"
	}
	return mod, via, ok
}

// FirstParty reports whether a Python module is relative or starts with
// the name of a file or a directory of the project, or with the name of
// the package that the file set is.
func (p *Python) FirstParty(spec string) bool {
	first, _, _ := strings.Cut(spec, ".")
	return strings.HasPrefix(spec, ".") || p.files.HasName(first) || p.pkg != "" && first == p.pkg
}

// For returns the resolver of the JavaScript or the TypeScript importers.
func (j *JavaScript) For(typescript bool) Resolver {
	return jsResolver{j: j, typescript: typescript}
}

type jsResolver struct {
	j          *JavaScript
	typescript bool
}

func (r jsResolver) Module(importer, spec string) (string, string, bool) {
	mods, via, ok := r.Modules(importer, spec, FormImport)
	if !ok {
		return "", "", false
	}
	return mods[0], via, true
}

func (r jsResolver) Modules(importer, spec string, form Form) ([]string, string, bool) {
	return r.j.Resolve(importer, spec, r.typescript, form)
}

// FirstParty reports whether a module is relative or starts with the name
// of a file or a directory of the project.
func (r jsResolver) FirstParty(spec string) bool {
	first, _, _ := strings.Cut(spec, "/")
	return strings.HasPrefix(spec, ".") || r.j.files.HasName(first)
}

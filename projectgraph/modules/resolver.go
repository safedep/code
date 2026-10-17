package modules

import "strings"

// Resolver resolves the modules that the files of one language import.
type Resolver interface {
	// Module returns the module that the importer names with the spec, and
	// how the resolver found it.
	Module(importer, spec string) (string, string, bool)
	// FirstParty reports whether a spec that names no module can still
	// name a file of the project, through a search path root, an alias or
	// a build step.
	FirstParty(spec string) bool
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
// the name of a file or a directory of the project.
func (p *Python) FirstParty(spec string) bool {
	first, _, _ := strings.Cut(spec, ".")
	return strings.HasPrefix(spec, ".") || p.files.HasName(first)
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
	return r.j.Resolve(importer, spec, r.typescript)
}

// FirstParty reports whether a module is relative or starts with the name
// of a file or a directory of the project.
func (r jsResolver) FirstParty(spec string) bool {
	first, _, _ := strings.Cut(spec, "/")
	return strings.HasPrefix(spec, ".") || r.j.files.HasName(first)
}

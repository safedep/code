package projectgraph

import (
	"cmp"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
)

// scope holds what one file binds at its top level.
type scope struct {
	ix       *fileindex.Index
	profile  *profile
	bindings map[string]fileindex.Import
	// types holds the names that a TypeScript import type binds. Only a
	// declared type or a base class reads them.
	types   map[string]fileindex.Import
	imports []fileindex.Import
	exports map[string][]fileindex.Export
	stars   []star
	// wildcards are the Go dot imports and the Java wildcard imports,
	// which put the names of a package or the members of a class in the
	// scope of the file.
	wildcards  []fileindex.Import
	hasExports bool
}

// star is an export * from of JavaScript or a from m import * of Python.
type star struct {
	spec string
	line int
}

func newScope(ix *fileindex.Index) *scope {
	sc := &scope{
		ix:       ix,
		profile:  profileOf(ix.Language),
		bindings: map[string]fileindex.Import{},
		types:    map[string]fileindex.Import{},
		exports:  map[string][]fileindex.Export{},
	}
	for _, imp := range ix.Imports {
		if imp.Kind == fileindex.ImportTypeOnly {
			if name := bindingName(ix.Language, imp); name != "" {
				sc.types[name] = imp
			}
			continue
		}
		if imp.Kind != fileindex.ImportStatic && imp.Kind != fileindex.ImportDynamicString {
			continue
		}
		if imp.Wildcard && sc.profile.starImports {
			sc.stars = append(sc.stars, star{spec: imp.Module, line: imp.Line})
			continue
		}
		if imp.Wildcard && sc.profile.scopeWildcards {
			sc.wildcards = append(sc.wildcards, imp)
		}
		// A JavaScript import for its side effect binds no name. Its
		// namespace still names the module of a call on require('x').
		sc.imports = append(sc.imports, imp)
		if name := bindingName(ix.Language, imp); name != "" {
			sc.bindings[name] = imp
		}
	}
	for _, ex := range ix.Exports {
		sc.hasExports = true
		if ex.Name == "*" && ex.From != "" {
			sc.stars = append(sc.stars, star{spec: ex.From, line: ex.Line})
			continue
		}
		sc.exports[ex.Name] = append(sc.exports[ex.Name], ex)
	}
	return sc
}

// enclosing returns the key of the innermost definition that holds the line,
// or the module key.
func (sc *scope) enclosing(line int) string {
	key, span := sc.ix.Path, -1
	for _, def := range sc.ix.Definitions {
		if def.StartLine > line || def.EndLine < line {
			continue
		}
		if s := def.EndLine - def.StartLine; span < 0 || s <= span {
			key, span = def.Key, s
		}
	}
	return key
}

// importFor returns the import whose namespace is the longest prefix of the
// target, and the segments of the target after that namespace.
func (sc *scope) importFor(target string) (fileindex.Import, []string, bool) {
	var best fileindex.Import
	found := false
	for _, imp := range sc.imports {
		ns := imp.Namespace
		if ns == "" || !fileindex.HasKeyPrefix(target, ns) || found && len(ns) <= len(best.Namespace) {
			continue
		}
		best, found = imp, true
	}
	if !found {
		return best, nil, false
	}
	var rest []string
	if len(target) > len(best.Namespace) {
		rest = strings.Split(target[len(best.Namespace)+len(fileindex.Separator):], fileindex.Separator)
	}
	return best, rest, true
}

func (sc *scope) exported(name string) bool {
	return len(sc.exports[name]) > 0
}

// bindingName returns the name that an import binds in the file. A Python
// import a.b with no alias binds a.
func bindingName(lang langcode.Code, imp fileindex.Import) string {
	switch {
	case imp.Item != "":
		return lastDotted(cmp.Or(imp.Alias, imp.Item))
	case imp.Alias != "" && imp.Alias != imp.Module:
		return imp.Alias
	case profileOf(lang).importBindsRoot:
		return firstDotted(imp.Module)
	}
	return ""
}

// importedModule returns the module that the name of an import holds, and
// the names that lead from that module to the bound value. A Python import
// a.b binds the package a. An import of an item binds the item.
func importedModule(lang langcode.Code, imp fileindex.Import) (string, []string) {
	switch {
	case bindsRoot(lang, imp):
		return firstDotted(imp.Module), nil
	case imp.Item != "":
		return imp.Module, []string{imp.Item}
	}
	return imp.Module, nil
}

// bindsRoot reports whether a Python import a.b binds the root package a. A
// bound name of a or no bound name both mean that.
func bindsRoot(lang langcode.Code, imp fileindex.Import) bool {
	return profileOf(lang).importBindsRoot && imp.Item == "" &&
		(imp.Alias == "" || imp.Alias == imp.Module || imp.Alias == firstDotted(imp.Module))
}

// boundNamespace returns the external key of the name that an import binds.
func boundNamespace(lang langcode.Code, imp fileindex.Import) string {
	if bindsRoot(lang, imp) {
		return fileindex.Namespace(lang, firstDotted(imp.Module))
	}
	return imp.Namespace
}

func firstDotted(s string) string {
	first, _, _ := strings.Cut(s, ".")
	return first
}

func lastDotted(s string) string {
	return s[strings.LastIndex(s, ".")+1:]
}

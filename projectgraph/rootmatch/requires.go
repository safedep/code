package rootmatch

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	rules "github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

var pythonSeparator = regexp.MustCompile(`[-_.]+`)

// requires reports whether the project depends on a package that the rule
// requires. A project whose manifests declare no dependency of the
// ecosystem of the rule gives no evidence, so the rule applies. A file that
// imports the package shows the dependency too, as a manifest can miss a
// package that the environment gives.
func (f *finder) requires(r rules.Rule) bool {
	if len(r.Requires) == 0 {
		return true
	}
	lang := langcode.Code(r.Language)
	deps := f.dependencies(lang)
	if len(deps) == 0 {
		return true
	}
	imported := f.importedPackages(lang)
	return slices.ContainsFunc(r.Requires, func(name string) bool {
		name = normalizeDependency(lang, name)
		return deps[name] || imported[name] || lang == langcode.Go && slices.ContainsFunc(slices.Collect(maps.Keys(imported)), func(imp string) bool {
			return strings.HasPrefix(imp, name+"/")
		})
	})
}

// importedPackages returns the normalized names of the packages that the
// files of the ecosystem of the language import.
func (f *finder) importedPackages(lang langcode.Code) map[string]bool {
	if pkgs, ok := f.imported[lang]; ok {
		return pkgs
	}
	pkgs := map[string]bool{}
	for _, ix := range f.Indexes {
		if !(rules.Rule{Language: string(lang)}).Applies(ix.Language) {
			continue
		}
		for _, imp := range ix.Imports {
			if name := packageOf(lang, imp.Module); name != "" {
				pkgs[normalizeDependency(lang, name)] = true
			}
		}
	}
	f.imported[lang] = pkgs
	return pkgs
}

// packageOf returns the package of an import as written: the first part of
// a Python module, the name of an npm package with its scope, the first
// segment of a Rust path, or a Go import path, which a module path
// prefixes. A language whose imports name no package gives "".
func packageOf(lang langcode.Code, module string) string {
	if module == "" || strings.HasPrefix(module, ".") || strings.HasPrefix(module, "/") {
		return ""
	}
	switch lang.Family() {
	case langcode.Python:
		first, _, _ := strings.Cut(module, ".")
		return first
	case langcode.Javascript:
		name, _ := modules.SplitPackage(module)
		return name
	case langcode.Rust:
		first, _, _ := strings.Cut(module, "::")
		return first
	case langcode.Go:
		return module
	}
	return ""
}

// dependencies returns the normalized names of the packages that the
// manifests of the project declare for the ecosystem of the language.
func (f *finder) dependencies(lang langcode.Code) map[string]bool {
	if deps, ok := f.deps[lang]; ok {
		return deps
	}
	deps := map[string]bool{}
	if f.Dependencies != nil {
		names, problems := f.Dependencies(lang)
		f.problems = append(f.problems, problems...)
		for _, name := range names {
			if name != "" {
				deps[normalizeDependency(lang, name)] = true
			}
		}
	}
	f.deps[lang] = deps
	return deps
}

// normalizeDependency returns the name of a package as the registry of the
// language compares it. Python compares names in lower case, with -, _
// and . as one separator. A Rust path writes a crate with _ for -.
func normalizeDependency(lang langcode.Code, name string) string {
	switch lang {
	case langcode.Python:
		return pythonSeparator.ReplaceAllString(strings.ToLower(name), "-")
	case langcode.Rust:
		return strings.ReplaceAll(name, "-", "_")
	}
	return name
}

package rootmatch

import (
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	rules "github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

var (
	requirementName = regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)`)
	quoted          = regexp.MustCompile(`"([^"]*)"|'([^']*)'`)
	installRequires = regexp.MustCompile(`(?s)install_requires\s*=\s*\[([^\]]*)\]`)
	pythonSeparator = regexp.MustCompile(`[-_.]+`)
)

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
		return deps[name] || imported[name]
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
// a Python module, or the name of an npm package, with its scope.
func packageOf(lang langcode.Code, module string) string {
	if module == "" || strings.HasPrefix(module, ".") || strings.HasPrefix(module, "/") {
		return ""
	}
	if lang == langcode.Python {
		first, _, _ := strings.Cut(module, ".")
		return first
	}
	name, _ := modules.SplitPackage(module)
	return name
}

// dependencies returns the normalized names of the packages that the
// manifests of the project declare for the ecosystem of the language.
func (f *finder) dependencies(lang langcode.Code) map[string]bool {
	if deps, ok := f.deps[lang]; ok {
		return deps
	}
	deps := map[string]bool{}
	add := func(name string) {
		if name != "" {
			deps[normalizeDependency(lang, name)] = true
		}
	}
	switch lang {
	case langcode.Javascript:
		for _, pj := range f.JS.Packages() {
			for _, name := range pj.DependencyNames() {
				add(name)
			}
		}
	case langcode.Python:
		for _, dir := range f.Files.Dirs() {
			for _, name := range f.pythonDependencies(dir) {
				add(name)
			}
		}
	}
	f.deps[lang] = deps
	return deps
}

// pythonDependencies returns the requirement names of the Python manifests
// of a directory: pyproject.toml, setup.py, Pipfile and the requirements
// files.
func (f *finder) pythonDependencies(dir string) []string {
	var names []string
	if data, ok := f.read(path.Join(dir, rules.PyProject)); ok {
		names = append(names, pyprojectDependencies(data)...)
	}
	if data, ok := f.read(path.Join(dir, rules.SetupPy)); ok {
		for _, list := range installRequires.FindAllSubmatch(data, -1) {
			names = append(names, quotedRequirements(string(list[1]))...)
		}
	}
	if data, ok := f.read(path.Join(dir, "Pipfile")); ok {
		names = append(names, tomlTableKeys(data, "packages", "dev-packages")...)
	}
	for _, pattern := range []string{"requirements*.txt", "requirements/*.txt"} {
		if f.Root == nil {
			break
		}
		files, err := fs.Glob(f.Root, path.Join(dir, pattern))
		if err != nil {
			continue
		}
		for _, file := range files {
			if data, ok := f.read(file); ok {
				names = append(names, requirementLines(data)...)
			}
		}
	}
	return names
}

// pyprojectDependencies returns the names of the dependencies and the
// optional dependencies of [project], and the keys of the dependency tables
// of Poetry.
func pyprojectDependencies(data []byte) []string {
	var names []string
	for _, pair := range modules.ScanTOML(data) {
		switch {
		case poetryDependencies(pair.Table):
			if pair.Key != "python" {
				names = append(names, pair.Key)
			}
		case pair.Table == "project" && pair.Key == "dependencies", pair.Table == "project.optional-dependencies":
			if strings.HasPrefix(pair.Value, "[") {
				names = append(names, quotedRequirements(pair.Value)...)
			}
		}
	}
	return names
}

// poetryDependencies reports whether a table holds Poetry dependencies, as
// tool.poetry.dependencies or tool.poetry.group.dev.dependencies.
func poetryDependencies(table string) bool {
	return table == "tool.poetry.dependencies" || table == "tool.poetry.dev-dependencies" ||
		strings.HasPrefix(table, "tool.poetry.group.") && strings.HasSuffix(table, ".dependencies")
}

// tomlTableKeys returns the keys of the tables of a TOML file.
func tomlTableKeys(data []byte, tables ...string) []string {
	var keys []string
	for _, pair := range modules.ScanTOML(data) {
		if slices.Contains(tables, pair.Table) {
			keys = append(keys, pair.Key)
		}
	}
	return keys
}

// quotedRequirements returns the names of the quoted requirements of a
// text, as flask for "flask>=2.0".
func quotedRequirements(text string) []string {
	var names []string
	for _, m := range quoted.FindAllStringSubmatch(text, -1) {
		if name := requirementName.FindStringSubmatch(m[1] + m[2]); name != nil {
			names = append(names, name[1])
		}
	}
	return names
}

// requirementLines returns the names of the requirements of a requirements
// file. An option line, as -r base.txt or -e ., names no package.
func requirementLines(data []byte) []string {
	var names []string
	for _, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "#")
		if strings.HasPrefix(strings.TrimSpace(line), "-") {
			continue
		}
		if m := requirementName.FindStringSubmatch(line); m != nil {
			names = append(names, m[1])
		}
	}
	return names
}

// normalizeDependency returns the name of a package as the registry of the
// language compares it. Python compares names in lower case, with -, _
// and . as one separator.
func normalizeDependency(lang langcode.Code, name string) string {
	if lang != langcode.Python {
		return name
	}
	return pythonSeparator.ReplaceAllString(strings.ToLower(name), "-")
}

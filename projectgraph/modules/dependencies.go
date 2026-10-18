package modules

import (
	"errors"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
)

var (
	requirementName = regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)`)
	quoted          = regexp.MustCompile(`"([^"]*)"|'([^']*)'`)
	installRequires = regexp.MustCompile(`(?s)install_requires\s*=\s*\[([^\]]*)\]`)
)

// PythonDependencies returns the requirement names of the Python manifests
// of each directory, as written: pyproject.toml, setup.py, Pipfile and the
// requirements files. It returns the manifests that it could not read.
func PythonDependencies(root fs.FS, dirs []string) ([]string, []Problem) {
	var names []string
	var ps problems
	read := func(name string) ([]byte, bool) {
		if root == nil {
			return nil, false
		}
		data, err := fs.ReadFile(root, fsName(name))
		if err == nil {
			return data, true
		}
		if !errors.Is(err, fs.ErrNotExist) {
			ps.add(name, err)
		}
		return nil, false
	}
	for _, dir := range dirs {
		if data, ok := read(path.Join(dir, "pyproject.toml")); ok {
			names = append(names, pyprojectDependencies(data)...)
		}
		if data, ok := read(path.Join(dir, "setup.py")); ok {
			for _, list := range installRequires.FindAllSubmatch(data, -1) {
				names = append(names, quotedRequirements(string(list[1]))...)
			}
		}
		if data, ok := read(path.Join(dir, "Pipfile")); ok {
			names = append(names, tomlTableKeys(data, "packages", "dev-packages")...)
		}
		for _, pattern := range []string{"requirements*.txt", "requirements/*.txt"} {
			if root == nil {
				break
			}
			files, err := fs.Glob(root, path.Join(dir, pattern))
			if err != nil {
				continue
			}
			for _, file := range files {
				if data, ok := read(file); ok {
					names = append(names, requirementLines(data)...)
				}
			}
		}
	}
	return names, ps
}

// pyprojectDependencies returns the names of the dependencies and the
// optional dependencies of [project], and the keys of the dependency tables
// of Poetry.
func pyprojectDependencies(data []byte) []string {
	var names []string
	for _, pair := range ScanTOML(data) {
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
	for _, pair := range ScanTOML(data) {
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

// cargoDependency returns the crate that a pair of a Cargo.toml depends on:
// a key of a dependencies table, as serde = "1" under [dependencies] or
// [target.'cfg(unix)'.dev-dependencies], or the name of a table under one,
// as [dependencies.serde].
func cargoDependency(pair TOMLPair) (string, bool) {
	parts := strings.Split(pair.Table, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		if !strings.HasSuffix(parts[i], "dependencies") {
			continue
		}
		switch len(parts) - i {
		case 1:
			return pair.Key, true
		case 2:
			return parts[i+1], true
		}
		return "", false
	}
	return "", false
}

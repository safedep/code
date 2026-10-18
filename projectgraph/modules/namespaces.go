package modules

import (
	"io/fs"
	"path"
	"slices"
	"strings"
)

// manifestSkipDirs hold the manifests of dependencies, not of the project.
var manifestSkipDirs = []string{"node_modules", "vendor", "testdata"}

// OwnNamespaces returns the namespaces that the manifests under root give
// to the code of the project: the module path of each go.mod, the crate
// name of each Cargo.toml and the PSR-4 prefix of each composer.json, in
// sorted order. An import under one of them is first-party. The walk skips
// hidden directories and the directories of dependencies. It returns the
// manifests that it could not read.
func OwnNamespaces(root fs.FS) ([]string, []Problem) {
	dirs, ps := manifestDirs(root)
	files := NewFiles(dirs)
	golang := NewGolang(files, root, nil)
	cargo := NewCargo(files, root)
	composer := NewComposer(files, root)

	var names []string
	for _, m := range golang.modules {
		names = append(names, m.path)
	}
	for _, c := range cargo.crates {
		names = append(names, c.Name)
	}
	for _, p := range composer.prefixes {
		names = append(names, p.prefix)
	}
	slices.Sort(names)
	ps = append(ps, golang.Problems()...)
	ps = append(ps, cargo.Problems()...)
	ps = append(ps, composer.Problems()...)
	return slices.Compact(names), ps
}

// manifestDirs returns one file path per directory under root that holds
// a manifest, so NewFiles lists the directory.
func manifestDirs(root fs.FS) ([]string, []Problem) {
	var dirs []string
	var ps problems
	if root == nil {
		return nil, nil
	}
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			ps.add(p, err)
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != "." && (strings.HasPrefix(name, ".") || slices.Contains(manifestSkipDirs, name)) {
				return fs.SkipDir
			}
			return nil
		}
		if name == "go.mod" || name == "Cargo.toml" || name == "composer.json" {
			dirs = append(dirs, path.Clean(p))
		}
		return nil
	})
	if err != nil {
		ps.add(".", err)
	}
	return dirs, ps
}

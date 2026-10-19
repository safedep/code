package modules

import (
	"cmp"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Cargo holds the crates of the Cargo.toml files of a project. A crate has
// the directory of its manifest and the name that a path of another crate
// writes, as serde_json for the package serde-json.
type Cargo struct {
	crates       []Crate
	dependencies []string
	problems     problems
}

// Crate is one package of a Cargo.toml. Renames maps the name that the
// crate writes for a dependency to the name of its package, as helper to
// kit for helper = { package = "kit" }.
type Crate struct {
	Dir     string
	Name    string
	Renames map[string]string
}

// NewCargo reads the Cargo.toml of each directory of the file set. The name
// of the lib table wins over the name of the package table.
func NewCargo(files *Files, root fs.FS) *Cargo {
	c := &Cargo{}
	for _, d := range files.Dirs() {
		if root == nil {
			break
		}
		name := path.Join(d, "Cargo.toml")
		data, err := fs.ReadFile(root, fsName(name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			c.problems.add(name, err)
			continue
		}
		pairs := ScanTOML(data)
		if crate := cargoCrate(data); crate != "" {
			c.crates = append(c.crates, Crate{Dir: d, Name: crateName(crate), Renames: cargoRenames(pairs)})
		}
		for _, pair := range pairs {
			if name, ok := cargoDependency(pair); ok && !slices.Contains(c.dependencies, name) {
				c.dependencies = append(c.dependencies, name)
			}
		}
	}
	// The deepest manifest holds a file, so it comes first.
	slices.SortStableFunc(c.crates, func(a, b Crate) int { return cmp.Compare(len(b.Dir), len(a.Dir)) })
	return c
}

// cargoCrate returns the name of the lib table, or else of the package
// table. A workspace manifest has neither.
func cargoCrate(data []byte) string {
	names := map[string]string{}
	for _, pair := range ScanTOML(data) {
		if pair.Key != "name" || pair.Table != "package" && pair.Table != "lib" {
			continue
		}
		if name, ok := TOMLString(pair.Value); ok && name != "" {
			names[pair.Table] = name
		}
	}
	return cmp.Or(names["lib"], names["package"])
}

// cargoRenames returns the dependencies that a Cargo.toml renames with the
// package key, inline as helper = { package = "kit" } or in a
// [dependencies.helper] table. Both names are as a path writes them.
func cargoRenames(pairs []TOMLPair) map[string]string {
	var renames map[string]string
	for _, pair := range pairs {
		name, ok := cargoDependency(pair)
		if !ok {
			continue
		}
		pkg := ""
		if inline, ok := TOMLInlineTable(pair.Value); ok && name == pair.Key {
			for _, p := range inline {
				if p.Key == "package" {
					pkg = p.Value
				}
			}
		} else if pair.Key == "package" && name != pair.Key {
			pkg, _ = TOMLString(pair.Value)
		}
		if pkg == "" {
			continue
		}
		if renames == nil {
			renames = map[string]string{}
		}
		renames[crateName(name)] = crateName(pkg)
	}
	return renames
}

// crateName returns the name that a path writes for a package, as
// serde_json for serde-json.
func crateName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// Problems returns the Cargo.toml files that the reader could not read.
func (c *Cargo) Problems() []Problem { return c.problems }

// Dependencies returns the crates that the manifests depend on, as written.
func (c *Cargo) Dependencies() []string { return c.dependencies }

// Crate returns the crate of the deepest Cargo.toml that holds the file.
func (c *Cargo) Crate(file string) (Crate, bool) {
	for _, crate := range c.crates {
		if crate.Dir == "" || strings.HasPrefix(file, crate.Dir+"/") {
			return crate, true
		}
	}
	return Crate{}, false
}

// Rename returns the package that a file names by a crate name: the
// package of a rename in the Cargo.toml of the file, or else the name.
func (c *Cargo) Rename(file, name string) string {
	if crate, ok := c.Crate(file); ok {
		if pkg, ok := crate.Renames[name]; ok {
			return pkg
		}
	}
	return name
}

// Has reports whether a crate of the project has the name.
func (c *Cargo) Has(name string) bool {
	return slices.ContainsFunc(c.crates, func(crate Crate) bool { return crate.Name == name })
}

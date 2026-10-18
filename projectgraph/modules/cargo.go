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
	crates   []Crate
	problems problems
}

// Crate is one package of a Cargo.toml.
type Crate struct {
	Dir  string
	Name string
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
		if crate := cargoCrate(data); crate != "" {
			c.crates = append(c.crates, Crate{Dir: d, Name: strings.ReplaceAll(crate, "-", "_")})
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

// Problems returns the Cargo.toml files that the reader could not read.
func (c *Cargo) Problems() []Problem { return c.problems }

// Crate returns the crate of the deepest Cargo.toml that holds the file.
func (c *Cargo) Crate(file string) (Crate, bool) {
	for _, crate := range c.crates {
		if crate.Dir == "" || strings.HasPrefix(file, crate.Dir+"/") {
			return crate, true
		}
	}
	return Crate{}, false
}

// Has reports whether a crate of the project has the name.
func (c *Cargo) Has(name string) bool {
	return slices.ContainsFunc(c.crates, func(crate Crate) bool { return crate.Name == name })
}

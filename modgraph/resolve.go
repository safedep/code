package modgraph

import (
	"io/fs"
	"path"
	"strings"
)

// candidate is a module that can hold the package of an import path.
type candidate struct {
	mod *module
	// dir is the directory of the package inside the module, "." for its
	// root.
	dir string
	// provides is true when the source of the module has the package, so
	// the walk reads it.
	provides bool
}

// resolve returns the modules that can hold the package of an import path.
// The go command takes the module of the build list whose path is the
// longest prefix of the import path and that has the package directory. A
// module with no source may have the directory, so it is a candidate, and the
// test goes on to the next prefix. The first module with source that has the
// directory ends the list. A list with no module that has or may have the
// directory is empty.
func (b *builder) resolve(importPath string) []candidate {
	var out []candidate
	for _, p := range b.paths {
		dir, ok := under(importPath, p)
		if !ok {
			continue
		}
		m := b.byPath[p]
		if m.src.FS == nil {
			out = append(out, candidate{mod: m, dir: dir})
			continue
		}
		if hasPackageDir(m.src.FS, dir) {
			return append(out, candidate{mod: m, dir: dir, provides: true})
		}
	}
	return out
}

// prefixed reports whether a module of the build list or the main module has
// a path that is a prefix of the import path. Such a path is never a package
// of the standard library, even with no dot in its first element, as the
// path corp/lib of a replaced module.
func (b *builder) prefixed(importPath string) bool {
	for _, p := range b.paths {
		if _, ok := under(importPath, p); ok {
			return true
		}
	}
	return false
}

// under returns the path of a package below a module path, "." for the
// module root, at a "/" boundary.
func under(importPath, modPath string) (string, bool) {
	switch {
	case importPath == modPath:
		return ".", true
	case strings.HasPrefix(importPath, modPath+"/"):
		return importPath[len(modPath)+1:], true
	}
	return "", false
}

// isStdPath reports whether an import path can name a package of the
// standard library: its first element has no dot.
func isStdPath(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return first != "" && !strings.Contains(first, ".")
}

// hasPackageDir reports whether the source of a module has the directory of
// a package that an importer can build. A directory that the go command
// skips, or that belongs to a module nested in this one, is not a package of
// the module.
func hasPackageDir(fsys fs.FS, dir string) bool {
	if dir != "." {
		for _, part := range strings.Split(dir, "/") {
			if skippedDir(part) {
				return false
			}
		}
		if nestedModule(fsys, dir) {
			return false
		}
	}
	info, err := fs.Stat(fsys, dir)
	return err == nil && info.IsDir()
}

// nestedModule reports whether a directory below the root of a module, or
// one of its parents below the root, holds a go.mod, so it is a module of its
// own.
func nestedModule(fsys fs.FS, dir string) bool {
	for d := dir; d != "."; d = path.Dir(d) {
		if _, err := fs.Stat(fsys, path.Join(d, "go.mod")); err == nil {
			return true
		}
	}
	return false
}

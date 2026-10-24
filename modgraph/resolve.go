package modgraph

import (
	"errors"
	"io/fs"
	"iter"
	"path"
	"slices"
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
// longest prefix of the import path and that has a Go file in the package
// directory. A module with no source may have the file, so it is a candidate,
// and the test goes on to the next prefix. So is a module whose directory
// Build cannot read. The first module with source that has the file ends the
// list. A list with no module that has or may have the file is empty.
func (b *builder) resolve(importPath string) []candidate {
	var out []candidate
	for m, dir := range b.prefixes(importPath) {
		if m.src.FS == nil {
			out = append(out, candidate{mod: m, dir: dir})
			continue
		}
		has, err := hasPackage(m.src.FS, dir)
		switch {
		case err != nil:
			out = append(out, candidate{mod: m, dir: dir, provides: true})
		case has:
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
	for range b.prefixes(importPath) {
		return true
	}
	return false
}

// prefixes yields each module of the build list, and the main module, whose
// path is the import path or a prefix of it at a "/" boundary, longest
// first. It yields the directory of the package in the module with it, "."
// for the module root.
func (b *builder) prefixes(importPath string) iter.Seq2[*module, string] {
	return func(yield func(*module, string) bool) {
		for p := importPath; ; {
			if m := b.byPath[p]; m != nil && m != b.std {
				dir := "."
				if p != importPath {
					dir = importPath[len(p)+1:]
				}
				if !yield(m, dir) {
					return
				}
			}
			i := strings.LastIndexByte(p, '/')
			if i < 0 {
				return
			}
			p = p[:i]
		}
	}
}

// hasPackage reports whether the source of a module holds a package in a
// directory, as the go command decides it: the directory has an entry whose
// name ends in .go and that is a file or a link to a file. A build
// constraint or a name that starts with _ does not matter here. An import can
// name a testdata directory or one whose name starts with _ or a dot, and the
// go command builds it, though ./... skips it. A vendor directory, and a
// directory of a module nested in this one, is not a package of the module.
// It returns an error when it cannot tell.
func hasPackage(fsys fs.FS, dir string) (bool, error) {
	if dir != "." {
		if slices.Contains(strings.Split(dir, "/"), "vendor") || nestedModule(fsys, dir) {
			return false, nil
		}
	}
	info, err := fs.Stat(fsys, dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case !info.IsDir():
		return false, nil
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return false, err
	}
	var firstErr error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if e.Type().IsRegular() {
			return true, nil
		}
		info, err := fs.Stat(fsys, path.Join(dir, e.Name()))
		if err == nil && info.Mode().IsRegular() {
			return true, nil
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return false, firstErr
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

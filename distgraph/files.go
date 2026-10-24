package distgraph

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/safedep/code/projectgraph/modules"
)

// compiledExts are the extensions of a Python extension module.
var compiledExts = map[string]bool{".so": true, ".pyd": true, ".dylib": true}

// pythonFiles returns the Python files of a node, in path order, and its
// extension modules. The project gives each .py file under its root, except
// under a hidden directory, a __pycache__, a node_modules, a site-packages
// or a virtual environment, which holds a pyvenv.cfg. A distribution gives
// the files under each of its top-level names, except its test files,
// which no program imports.
func (b *builder) pythonFiles(d *dist) ([]string, []string, error) {
	var files, compiled []string
	visit := func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			if b.skipDir(d, p) {
				return fs.SkipDir
			}
			return nil
		}
		switch ext := path.Ext(p); {
		case ext == ".py" && (d.meta.Name == "" || !modules.PythonTestPath(p)):
			files = append(files, p)
		case compiledExts[ext]:
			compiled = append(compiled, p)
		}
		return nil
	}
	roots := []string{"."}
	if d.meta.Name != "" {
		roots = nil
		for _, top := range d.meta.TopLevel {
			if _, err := fs.Stat(d.fsys, top+".py"); err == nil {
				files = append(files, top+".py")
			}
			roots = append(roots, top)
		}
		compiled = append(compiled, d.meta.Compiled...)
	}
	for _, root := range roots {
		if root != "." {
			if info, err := fs.Stat(d.fsys, root); err != nil || !info.IsDir() {
				continue
			}
		}
		if err := fs.WalkDir(d.fsys, root, visit); err != nil {
			return nil, nil, err
		}
	}
	slices.Sort(files)
	slices.Sort(compiled)
	return slices.Compact(files), slices.Compact(compiled), nil
}

// skipDir reports whether no file of a directory is code of the node.
func (b *builder) skipDir(d *dist, p string) bool {
	if p == "." {
		return false
	}
	base := path.Base(p)
	switch {
	case strings.HasPrefix(base, ".") || base == "__pycache__" || base == "node_modules":
		return true
	case d.meta.Name != "":
		return false
	case base == "site-packages":
		return true
	}
	_, err := fs.Stat(d.fsys, path.Join(p, "pyvenv.cfg"))
	return err == nil
}

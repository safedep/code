package modules

import (
	"cmp"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Python resolves Python imports against the import roots of a project. A
// module is a file path, or a directory path with a trailing "/" for a
// namespace package, which has no file.
type Python struct {
	files    *Files
	roots    []string
	problems problems
}

// NewPython returns the resolver of a project. The roots are the project
// root, each directory with a pyproject.toml or a setup.py, the src
// directory under each of those, and each entry of sysPath, in that order.
func NewPython(files *Files, root fs.FS, sysPath []string) *Python {
	p := &Python{files: files}
	projects := []string{""}
	for _, d := range files.Dirs() {
		if d == "" {
			continue
		}
		for _, manifest := range []string{"pyproject.toml", "setup.py"} {
			ok, err := exists(root, path.Join(d, manifest))
			if err != nil {
				p.problems.add(path.Join(d, manifest), err)
			}
			if ok {
				projects = append(projects, d)
				break
			}
		}
	}

	roots := slices.Clone(projects)
	for _, d := range projects {
		if src, _ := Join(d, "src"); files.HasDir(src) {
			roots = append(roots, src)
		}
	}
	for _, entry := range sysPath {
		if d, ok := Join(entry); ok {
			roots = append(roots, d)
		}
	}
	p.roots = compactStable(roots)
	return p
}

// Roots returns the import roots in the order of the search.
func (p *Python) Roots() []string { return p.roots }

// Problems returns the manifests that the resolver could not read.
func (p *Python) Problems() []Problem { return p.problems }

// Resolve returns the module that importer names with module, as written:
// yaml.loader, .utils or ..pkg.
func (p *Python) Resolve(importer, module string) (string, bool) {
	if module == "" {
		return "", false
	}
	if strings.HasPrefix(module, ".") {
		return p.relative(importer, module)
	}
	return p.Absolute(importer, module)
}

// Absolute resolves a dotted module name against the roots. The roots that
// hold the importer come first, the deepest first. The directory of the
// importer comes last, because Python puts the directory of a script on
// sys.path. A module file or a regular package in any root wins over a
// namespace package.
func (p *Python) Absolute(importer, module string) (string, bool) {
	rel := strings.ReplaceAll(module, ".", "/")
	roots := p.searchOrder(importer)
	if dir := Dir(importer); !slices.Contains(roots, dir) {
		roots = append(roots, dir)
	}
	for _, r := range roots {
		base, ok := Join(r, rel)
		if !ok {
			continue
		}
		if file, ok := p.file(base); ok {
			return file, true
		}
	}
	for _, r := range roots {
		if base, ok := Join(r, rel); ok && p.files.HasDir(base) {
			return base + "/", true
		}
	}
	return "", false
}

// Submodule returns the module name under a package module.
func (p *Python) Submodule(module, name string) (string, bool) {
	dir, ok := PackageDir(module)
	if !ok {
		return "", false
	}
	base, ok := Join(dir, name)
	if !ok {
		return "", false
	}
	return p.moduleAt(base)
}

// PackageDir returns the directory of a package module: an __init__.py or
// a namespace package.
func PackageDir(module string) (string, bool) {
	switch {
	case strings.HasSuffix(module, "/"):
		return strings.TrimSuffix(module, "/"), true
	case path.Base(module) == "__init__.py":
		return Dir(module), true
	}
	return "", false
}

func (p *Python) relative(importer, module string) (string, bool) {
	rest := strings.TrimLeft(module, ".")
	dots := len(module) - len(rest)
	dir := Dir(importer)
	for range dots - 1 {
		if dir == "" {
			return "", false
		}
		dir = Dir(dir)
	}
	if rest == "" {
		return p.packageAt(dir)
	}
	base, ok := Join(dir, strings.ReplaceAll(rest, ".", "/"))
	if !ok {
		return "", false
	}
	return p.moduleAt(base)
}

func (p *Python) moduleAt(base string) (string, bool) {
	if file, ok := p.file(base); ok {
		return file, true
	}
	if p.files.HasDir(base) {
		return base + "/", true
	}
	return "", false
}

func (p *Python) packageAt(dir string) (string, bool) {
	init, _ := Join(dir, "__init__.py")
	if p.files.Has(init) {
		return init, true
	}
	return dir + "/", true
}

func (p *Python) file(base string) (string, bool) {
	if init := base + "/__init__.py"; p.files.Has(init) {
		return init, true
	}
	if p.files.Has(base + ".py") {
		return base + ".py", true
	}
	return "", false
}

func (p *Python) searchOrder(importer string) []string {
	var own, rest []string
	for _, r := range p.roots {
		if r == "" || strings.HasPrefix(importer, r+"/") {
			own = append(own, r)
		} else {
			rest = append(rest, r)
		}
	}
	slices.SortStableFunc(own, func(a, b string) int {
		return cmp.Compare(len(b), len(a))
	})
	return append(own, rest...)
}

func exists(root fs.FS, name string) (bool, error) {
	if root == nil {
		return false, nil
	}
	_, err := fs.Stat(root, fsName(name))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func compactStable(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := items[:0]
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}

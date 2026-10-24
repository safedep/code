package modgraph

import (
	"fmt"
	"go/version"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	gomodule "golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// workspace is a go.work file and the modules that it uses.
type workspace struct {
	dir   string
	file  *modfile.WorkFile
	mains []mainModule
}

// mainModule is a module that a workspace uses.
type mainModule struct {
	path string
	dir  string
	file *modfile.File
}

// discoverWorkspace finds the sources of the build list of a workspace for
// the main module in opts.Dir.
func discoverWorkspace(opts DiscoverOptions) ([]Source, error) {
	w, err := readWorkspace(opts.Work, opts.LocalRoot)
	if err != nil {
		return nil, err
	}
	main := w.mainAt(opts.Dir)
	if main == nil {
		return nil, fmt.Errorf("modgraph: %s does not use the module in %s", opts.Work, opts.Dir)
	}
	sums := map[gomodule.Version]string{}
	maps.Copy(sums, readSums(filepath.Join(w.dir, "go.work.sum")))
	for _, m := range w.mains {
		maps.Copy(sums, readSums(filepath.Join(m.dir, "go.sum")))
	}
	d := finder{opts: opts, vendorDir: filepath.Join(w.dir, "vendor"), sums: sums}

	var list []gomodule.Version
	if w.vendorMode() {
		d.vendored = readVendorList(filepath.Join(w.dir, "vendor", "modules.txt"))
		for path, v := range d.vendored {
			list = append(list, gomodule.Version{Path: path, Version: v.version})
		}
	} else if list, err = w.buildList(opts); err != nil {
		return nil, err
	}

	var out []Source
	for _, m := range list {
		if w.main(m.Path) != nil {
			continue
		}
		t, err := w.replacement(m)
		if err != nil {
			return nil, err
		}
		out = append(out, d.source(m, t))
	}
	for _, m := range w.mains {
		if m.dir == main.dir {
			continue
		}
		s := Source{Module: Module{Path: m.path}, Origin: Module{Path: filepath.ToSlash(m.dir)}}
		s.FS, s.GoMod = localSource(opts.LocalRoot, m.dir)
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b Source) int { return strings.Compare(a.Module.Path, b.Module.Path) })
	return out, nil
}

// readWorkspace reads a go.work file and the go.mod of each module that it
// uses, inside root only. A module of the workspace whose go.mod does not
// read leaves the build list unknown, so it is an error.
func readWorkspace(name, root string) (*workspace, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("modgraph: read go.work: %w", err)
	}
	f, err := modfile.ParseWork(name, data, nil)
	if err != nil {
		return nil, fmt.Errorf("modgraph: parse go.work: %w", err)
	}
	w := &workspace{dir: filepath.Dir(name), file: f}
	seen := map[string]string{}
	for _, u := range f.Use {
		dir := filepath.FromSlash(u.Path)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(w.dir, dir)
		}
		dir = resolved(dir)
		_, data := localSource(root, dir)
		if data == nil {
			return nil, fmt.Errorf("modgraph: read the go.mod in %s: no go.mod inside the local root", dir)
		}
		mf, err := modfile.Parse(filepath.Join(dir, "go.mod"), data, nil)
		if err != nil {
			return nil, fmt.Errorf("modgraph: parse go.mod: %w", err)
		}
		if mf.Module == nil || mf.Module.Mod.Path == "" {
			return nil, fmt.Errorf("modgraph: the go.mod in %s has no module directive", dir)
		}
		// An exclude directive can move the build list off the highest
		// version, and the build list below does not model it.
		if len(mf.Exclude) > 0 {
			return nil, fmt.Errorf("modgraph: the go.mod in %s has an exclude directive, which a workspace build list here does not model", dir)
		}
		path := mf.Module.Mod.Path
		if prev, ok := seen[path]; ok {
			return nil, fmt.Errorf("modgraph: %s uses %s twice, in %s and %s", name, path, prev, dir)
		}
		seen[path] = dir
		w.mains = append(w.mains, mainModule{path: path, dir: dir, file: mf})
	}
	return w, nil
}

// resolved returns a path with its links resolved, or the clean path when
// it does not resolve.
func resolved(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

func (w *workspace) main(path string) *mainModule {
	for i := range w.mains {
		if w.mains[i].path == path {
			return &w.mains[i]
		}
	}
	return nil
}

func (w *workspace) mainAt(dir string) *mainModule {
	dir = resolved(dir)
	for i := range w.mains {
		if w.mains[i].dir == dir {
			return &w.mains[i]
		}
	}
	return nil
}

// vendorMode reports whether the go command reads the workspace from its
// vendor directory: vendor/modules.txt next to a go.work of go 1.22 or
// later.
func (w *workspace) vendorMode() bool {
	if w.file.Go == nil || version.Compare("go"+w.file.Go.Version, "go1.22") < 0 {
		return false
	}
	_, err := os.Stat(filepath.Join(w.dir, "vendor", "modules.txt"))
	return err == nil
}

// replacement returns the module that the go command builds for m: the
// replace directive of go.work, else the one of a module of the workspace.
// Two modules that replace m with other code make the go command fail.
func (w *workspace) replacement(m gomodule.Version) (target, error) {
	if r := findReplace(w.file.Replace, m); r != nil {
		return target{base: w.dir, mod: r.New}, nil
	}
	var found *target
	for _, mm := range w.mains {
		r := findReplace(mm.file.Replace, m)
		if r == nil {
			continue
		}
		t := target{base: mm.dir, mod: r.New}
		if found != nil && !found.same(t) {
			return target{}, fmt.Errorf("modgraph: the modules of the workspace replace %s with other code", m)
		}
		found = &t
	}
	if found != nil {
		return *found, nil
	}
	return target{mod: m}, nil
}

// buildList returns the build list of the workspace, as minimal version
// selection with module graph pruning gives it: the highest version of
// each path in a graph of the requirements of each module of the
// workspace, of the go.mod of each such requirement, and of each go.mod
// below go 1.17 that a go.mod below go 1.17 requires. A go.mod that the
// graph needs and that the disk lacks leaves the build list unknown, so it
// is an error. A wrong version can hide an edge.
func (w *workspace) buildList(opts DiscoverOptions) ([]gomodule.Version, error) {
	selected := map[string]string{}
	type item struct {
		mod      gomodule.Version
		unpruned bool
	}
	var queue []item
	// node adds a requirement to the graph. A main module of the workspace
	// is always the one that the build uses, at no version.
	node := func(m gomodule.Version) bool {
		if w.main(m.Path) != nil || m.Version == "none" {
			return false
		}
		if v, ok := selected[m.Path]; !ok || semver.Compare(m.Version, v) > 0 {
			selected[m.Path] = m.Version
		}
		return true
	}
	for _, mm := range w.mains {
		for _, r := range mm.file.Require {
			if node(r.Mod) {
				queue = append(queue, item{mod: r.Mod})
			}
		}
	}
	loaded := map[item]bool{}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		if loaded[it] {
			continue
		}
		loaded[it] = true
		t, err := w.replacement(it.mod)
		if err != nil {
			return nil, err
		}
		f, err := w.goModOf(opts, t)
		if err != nil {
			return nil, err
		}
		unpruned := it.unpruned || f.Go == nil || version.Compare("go"+f.Go.Version, "go1.17") < 0
		for _, r := range f.Require {
			if node(r.Mod) && unpruned {
				queue = append(queue, item{mod: r.Mod, unpruned: true})
			}
		}
	}
	out := make([]gomodule.Version, 0, len(selected))
	for path, v := range selected {
		out = append(out, gomodule.Version{Path: path, Version: v})
	}
	return out, nil
}

// goModOf reads the go.mod of the code of a target: a local directory
// inside LocalRoot, or the .mod file of the module cache.
func (w *workspace) goModOf(opts DiscoverOptions, t target) (*modfile.File, error) {
	var data []byte
	if t.local() {
		_, data = localSource(opts.LocalRoot, t.dir())
	} else {
		_, data = cacheSource(opts.ModuleCache, t.mod)
	}
	if data == nil {
		return nil, fmt.Errorf("modgraph: no go.mod of %s is on the disk, so the build list of the workspace is not known", t.mod)
	}
	f, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("modgraph: parse the go.mod of %s: %w", t.mod, err)
	}
	return f, nil
}

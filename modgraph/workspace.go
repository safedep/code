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
		d.vendorMode = true
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
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	dir = resolved(dir)
	for i := range w.mains {
		if w.mains[i].dir == dir {
			return &w.mains[i]
		}
	}
	return nil
}

// vendorMode reports whether the go command reads the workspace from its
// vendor directory: a vendor directory next to go.work, a go directive of
// go 1.14 or later in go.work, and a vendor/modules.txt that go work vendor
// wrote for a workspace.
func (w *workspace) vendorMode() bool {
	if w.file.Go == nil || version.Compare("go"+w.file.Go.Version, "go1.14") < 0 {
		return false
	}
	if info, err := os.Stat(filepath.Join(w.dir, "vendor")); err != nil || !info.IsDir() {
		return false
	}
	return vendorForWorkspace(filepath.Join(w.dir, "vendor", "modules.txt"))
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

// buildList returns the build list of the workspace as readModGraph of the
// go command gives it in workspace mode: the highest version of each path
// in a graph of go.mod requirements. The graph holds the requirements of
// each module of the workspace and of the go.mod of each such requirement.
// A go.mod below go 1.17 adds the go.mod of each module that it requires,
// and so on down the chain. Then, while a module that the build needs
// requires a module below its selected version, the go.mod of the selected
// version joins the graph with each go.mod that it requires. A go.mod that
// the graph needs and that the disk lacks leaves the build list unknown, so
// it is an error. A wrong version can hide an edge.
func (w *workspace) buildList(opts DiscoverOptions) ([]gomodule.Version, error) {
	g := &modGraph{w: w, opts: opts, required: map[gomodule.Version][]gomodule.Version{}, loading: map[loadKey]bool{}, selected: map[string]string{}}
	for _, mm := range w.mains {
		g.enqueue(gomodule.Version{Path: mm.path}, loadWorkspace)
	}
	if err := g.drain(); err != nil {
		return nil, err
	}

	// The reload pass of readModGraph.
	needed := map[string]bool{}
	for _, mm := range w.mains {
		needed[mm.path] = true
	}
	seen := map[gomodule.Version]bool{}
	for {
		next := map[gomodule.Version]bool{}
		for p := range needed {
			m := gomodule.Version{Path: p, Version: g.selectedOf(p)}
			if !seen[m] {
				next[m] = true
				continue
			}
			for _, r := range g.required[m] {
				s := gomodule.Version{Path: r.Path, Version: g.selectedOf(r.Path)}
				if laterVersion(s.Version, r.Version) && !seen[s] {
					next[s] = true
				}
			}
		}
		if len(next) == 0 {
			break
		}
		for m := range next {
			g.enqueue(m, loadWorkspace)
			seen[m] = true
			needed[m.Path] = true
		}
		if err := g.drain(); err != nil {
			return nil, err
		}
	}

	out := make([]gomodule.Version, 0, len(g.selected))
	for path, v := range g.selected {
		out = append(out, gomodule.Version{Path: path, Version: v})
	}
	return out, nil
}

// laterVersion compares two versions as the go command does, where "" is
// the version of a module of the workspace, after each other version.
func laterVersion(a, b string) bool {
	switch {
	case a == b:
		return false
	case a == "":
		return true
	case b == "":
		return false
	}
	return semver.Compare(a, b) > 0
}

// loadMode is the pruning that a go.mod is loaded at, as in readModGraph.
type loadMode int

const (
	// loadPruned loads the requirements of a go.mod and does not follow
	// them, unless the go.mod is below go 1.17.
	loadPruned loadMode = iota
	// loadUnpruned follows each requirement down the chain.
	loadUnpruned
	// loadWorkspace loads a go.mod and each go.mod that it requires.
	loadWorkspace
)

type loadKey struct {
	mod  gomodule.Version
	mode loadMode
}

// modGraph is the graph of go.mod requirements of a workspace.
type modGraph struct {
	w        *workspace
	opts     DiscoverOptions
	required map[gomodule.Version][]gomodule.Version
	unpruned map[gomodule.Version]bool
	loading  map[loadKey]bool
	queue    []loadKey
	selected map[string]string
}

// selectedOf returns the selected version of a path, "" for a module of
// the workspace.
func (g *modGraph) selectedOf(path string) string {
	if g.w.main(path) != nil {
		return ""
	}
	return g.selected[path]
}

func (g *modGraph) enqueue(m gomodule.Version, mode loadMode) {
	if m.Version == "none" || g.loading[loadKey{m, mode}] {
		return
	}
	g.loading[loadKey{m, mode}] = true
	g.queue = append(g.queue, loadKey{m, mode})
}

func (g *modGraph) drain() error {
	for len(g.queue) > 0 {
		k := g.queue[0]
		g.queue = g.queue[1:]
		reqs, unpruned, err := g.load(k.mod)
		if err != nil {
			return err
		}
		next := loadPruned
		if unpruned || k.mode == loadUnpruned {
			next = loadUnpruned
		}
		for _, r := range reqs {
			if k.mode != loadPruned || unpruned {
				g.enqueue(r, next)
			}
		}
	}
	return nil
}

// load reads the requirements of a module version once, adds them to the
// graph, and reports whether its go.mod is below go 1.17. A module of the
// workspace at no version is its own go.mod. A versioned requirement on a
// module of the workspace reads that version, as the go command does.
func (g *modGraph) load(m gomodule.Version) ([]gomodule.Version, bool, error) {
	if reqs, ok := g.required[m]; ok {
		return reqs, g.unpruned[m], nil
	}
	var f *modfile.File
	if mm := g.w.main(m.Path); mm != nil && m.Version == "" {
		f = mm.file
	} else {
		t, err := g.w.replacement(m)
		if err != nil {
			return nil, false, err
		}
		if f, err = g.w.goModOf(g.opts, t); err != nil {
			return nil, false, err
		}
	}
	reqs := make([]gomodule.Version, 0, len(f.Require))
	for _, r := range f.Require {
		reqs = append(reqs, r.Mod)
		if v, ok := g.selected[r.Mod.Path]; g.w.main(r.Mod.Path) == nil && r.Mod.Version != "none" && (!ok || laterVersion(r.Mod.Version, v)) {
			g.selected[r.Mod.Path] = r.Mod.Version
		}
	}
	unpruned := f.Go == nil || version.Compare("go"+f.Go.Version, "go1.17") < 0
	g.required[m] = reqs
	if g.unpruned == nil {
		g.unpruned = map[gomodule.Version]bool{}
	}
	g.unpruned[m] = unpruned
	return reqs, unpruned, nil
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

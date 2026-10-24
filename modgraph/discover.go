package modgraph

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	gomodule "golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// DiscoverOptions says where Discover looks for the source of each module.
type DiscoverOptions struct {
	// Dir is the directory of the main go.mod.
	Dir string
	// Work is the go.work file of a workspace that uses Dir, or "" for a
	// module that the go command builds alone. The caller finds it, as the
	// go command does from GOWORK and the parents of Dir.
	Work string
	// ModuleCache is the module cache of the go command, as GOMODCACHE, or ""
	// for none. The caller finds it, so Discover reads no environment.
	ModuleCache string
	// LocalRoot bounds a replace directive with a local path, and each module
	// of a workspace: Discover reads such a directory only inside it, as the
	// root of the scan. "" reads none.
	LocalRoot string
}

// Discover finds the source of each module of the build list of a main
// module, as the go command builds it: the replacement of a replace
// directive, then the vendor directory when vendor/modules.txt lists the
// module at its version, then the module cache. A module with no source
// gets the go.mod of the module cache, when the cache has it. Discover never
// downloads, and a module that it cannot find has no source and no go.mod,
// so Build makes it open. With Work, the build list is that of the
// workspace, and each other module of the workspace is a local source.
func Discover(opts DiscoverOptions) ([]Source, error) {
	// A relative path would meet the resolved, absolute paths of the
	// workspace and of each local source.
	for _, p := range []*string{&opts.Dir, &opts.Work, &opts.LocalRoot} {
		if *p == "" {
			continue
		}
		abs, err := filepath.Abs(*p)
		if err != nil {
			return nil, fmt.Errorf("modgraph: %w", err)
		}
		*p = abs
	}
	if opts.Work != "" {
		return discoverWorkspace(opts)
	}
	f, err := parseGoMod(filepath.Join(opts.Dir, "go.mod"))
	if err != nil {
		return nil, err
	}
	d := finder{
		opts:      opts,
		vendorDir: filepath.Join(opts.Dir, "vendor"),
		sums:      readSums(filepath.Join(opts.Dir, "go.sum")),
	}
	// The go command reads no vendor directory of a workspace outside
	// workspace mode.
	if modules := filepath.Join(opts.Dir, "vendor", "modules.txt"); !vendorForWorkspace(modules) {
		d.vendored = readVendorList(modules)
	}
	var out []Source
	for _, r := range f.Require {
		t := target{base: opts.Dir, mod: r.Mod}
		if rep := findReplace(f.Replace, r.Mod); rep != nil {
			t.mod = rep.New
		}
		out = append(out, d.source(r.Mod, t))
	}
	return out, nil
}

// parseGoMod reads a go.mod in full. A lax parse drops the replace
// directives, and a replaced module read from the module cache is other
// code.
func parseGoMod(name string) (*modfile.File, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("modgraph: read go.mod: %w", err)
	}
	f, err := modfile.Parse(name, data, nil)
	if err != nil {
		return nil, fmt.Errorf("modgraph: parse go.mod: %w", err)
	}
	return f, nil
}

// target is the module that the go command builds for a module of the
// build list, as a replace directive writes it. A local path is relative to
// base.
type target struct {
	base string
	mod  gomodule.Version
}

func (t target) local() bool { return t.mod.Version == "" }

// dir is the directory of a local target.
func (t target) dir() string {
	dir := filepath.FromSlash(t.mod.Path)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(t.base, dir)
	}
	return filepath.Clean(dir)
}

// same reports whether two targets name the same code.
func (t target) same(o target) bool {
	if t.local() || o.local() {
		return t.local() && o.local() && t.dir() == o.dir()
	}
	return t.mod == o.mod
}

// findReplace returns the replace directive of a module version, else the
// one of each version of its path, else nil.
func findReplace(rs []*modfile.Replace, m gomodule.Version) *modfile.Replace {
	var all *modfile.Replace
	for _, r := range rs {
		switch {
		case r.Old.Path != m.Path:
		case r.Old.Version == m.Version:
			return r
		case r.Old.Version == "":
			all = r
		}
	}
	return all
}

// finder finds the source of each module of one build list.
type finder struct {
	opts      DiscoverOptions
	vendorDir string
	vendored  map[string]vendored
	// vendorMode reads each module that vendor lists from vendor, as the
	// go command does for a workspace. Else vendor serves a module only
	// when it holds the version and the replacement of go.mod.
	vendorMode bool
	sums       map[gomodule.Version]string
}

// source finds the source of the module m, which the go command builds
// from t. In vendor mode the go command reads each module from vendor, a
// local replacement too.
func (d finder) source(m gomodule.Version, t target) Source {
	s := Source{Module: Module{Path: m.Path, Version: m.Version}}
	if t.mod != m {
		s.Origin = Module{Path: t.mod.Path, Version: t.mod.Version}
	}
	switch v, ok := d.vendored[m.Path]; {
	case ok && (d.vendorMode || v.holds(m.Path) == t.mod):
		s.FS = os.DirFS(filepath.Join(d.vendorDir, filepath.FromSlash(m.Path)))
	case t.local():
		s.FS, s.GoMod = localSource(d.opts.LocalRoot, t.dir())
	default:
		s.FS, s.GoMod = cacheSource(d.opts.ModuleCache, t.mod)
		s.Immutable = s.FS != nil
		s.Sum = d.sums[t.mod]
	}
	return s
}

// localSource returns the source of a local directory, inside root only.
// Both paths are resolved through links first, and each read of the source
// resolves its links again, so a link cannot lead the read out of root.
func localSource(root, dir string) (fs.FS, []byte) {
	if root == "" {
		return nil, nil
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, nil
	}
	if dir, err = filepath.EvalSymlinks(dir); err != nil || !within(root, dir) {
		return nil, nil
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, nil
	}
	return rootedFS{dir: dir, root: root}, readOptional(filepath.Join(dir, "go.mod"))
}

var errOutsideRoot = errors.New("a link leads out of the local root")

// rootedFS reads a directory as os.DirFS does, and refuses a name whose
// links resolve outside root.
type rootedFS struct{ dir, root string }

func (r rootedFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(r.dir, filepath.FromSlash(name)))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if !within(r.root, resolved) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: errOutsideRoot}
	}
	return os.Open(resolved)
}

// within reports whether a resolved path is root or below it.
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// cacheSource returns the extracted source of a module version in the
// module cache, and its go.mod from cache/download. The escaping of the go
// command keeps a path or a version from leaving the cache.
func cacheSource(cache string, m gomodule.Version) (fs.FS, []byte) {
	if cache == "" {
		return nil, nil
	}
	escPath, err := gomodule.EscapePath(m.Path)
	if err != nil {
		return nil, nil
	}
	escVersion, err := gomodule.EscapeVersion(m.Version)
	if err != nil {
		return nil, nil
	}
	gomod := readOptional(filepath.Join(cache, "cache", "download", filepath.FromSlash(escPath), "@v", escVersion+".mod"))
	dir := filepath.Join(cache, filepath.FromSlash(escPath)+"@"+escVersion)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, gomod
	}
	return os.DirFS(dir), gomod
}

// readOptional reads a file that may be absent. A file that Discover cannot
// read counts as absent, so Build makes its module open, which keeps each
// edge.
func readOptional(name string) []byte {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	return data
}

// vendored is a module that vendor/modules.txt lists with a package: the
// version that the build list holds, and the target of its replace
// directive, or the zero module.
type vendored struct {
	version     string
	replacement gomodule.Version
}

// holds returns the module that the vendor directory holds for v.
func (v vendored) holds(path string) gomodule.Version {
	if v.replacement.Path != "" {
		return v.replacement
	}
	return gomodule.Version{Path: path, Version: v.version}
}

// readVendorList reads vendor/modules.txt as the go command does. A module
// enters the list only from a "# path version" line that package lines
// follow. A line with "=> target" records the replacement of that version,
// or of each version when the line has no version. A missing file lists
// none.
func readVendorList(name string) map[string]vendored {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	out := map[string]vendored{}
	replaced := map[gomodule.Version]gomodule.Version{}
	var mod gomodule.Version
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "# ") {
			f := strings.Fields(line)
			mod = gomodule.Version{}
			switch {
			case len(f) < 3:
				continue
			case semver.IsValid(f[2]):
				mod, f = gomodule.Version{Path: f[1], Version: f[2]}, f[3:]
			case f[2] == "=>":
				mod, f = gomodule.Version{Path: f[1]}, f[2:]
			default:
				continue
			}
			switch {
			case len(f) == 2 && f[0] == "=>":
				replaced[mod] = gomodule.Version{Path: f[1]}
			case len(f) == 3 && f[0] == "=>" && semver.IsValid(f[2]):
				replaced[mod] = gomodule.Version{Path: f[1], Version: f[2]}
			}
			continue
		}
		f := strings.Fields(line)
		if mod.Version == "" || len(f) != 1 || strings.HasPrefix(line, "## ") || gomodule.CheckImportPath(f[0]) != nil {
			continue
		}
		if v, ok := out[mod.Path]; !ok || semver.Compare(v.version, mod.Version) < 0 {
			out[mod.Path] = vendored{version: mod.Version}
		}
	}
	for path, v := range out {
		if r, ok := replaced[gomodule.Version{Path: path, Version: v.version}]; ok {
			v.replacement = r
		} else if r, ok := replaced[gomodule.Version{Path: path}]; ok {
			v.replacement = r
		}
		out[path] = v
	}
	return out
}

// vendorForWorkspace reports whether the first line of vendor/modules.txt
// marks it as the vendor directory of a workspace, as go work vendor
// writes it.
func vendorForWorkspace(name string) bool {
	data, err := os.ReadFile(name)
	if err != nil {
		return false
	}
	line, _, _ := strings.Cut(string(data), "\n")
	annotations, ok := strings.CutPrefix(line, "## ")
	if !ok {
		return false
	}
	for entry := range strings.SplitSeq(annotations, ";") {
		if strings.TrimSpace(entry) == "workspace" {
			return true
		}
	}
	return false
}

// readSums reads the h1 hash of each module version in go.sum. A go.mod
// hash line is not the hash of the source, so it does not count.
func readSums(name string) map[gomodule.Version]string {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	out := map[gomodule.Version]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 3 || strings.HasSuffix(fields[1], "/go.mod") {
			continue
		}
		out[gomodule.Version{Path: fields[0], Version: fields[1]}] = fields[2]
	}
	return out
}

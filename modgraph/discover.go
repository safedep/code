package modgraph

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	gomodule "golang.org/x/mod/module"
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
		vendored:  readVendorList(filepath.Join(opts.Dir, "vendor", "modules.txt")),
		sums:      readSums(filepath.Join(opts.Dir, "go.sum")),
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
	sums      map[gomodule.Version]string
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
	case ok && v.holds == t.mod:
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

// vendored is a module that vendor/modules.txt lists: the version that
// the build list holds, and the module that the vendor directory holds for
// it, the module itself or the target of its replace.
type vendored struct {
	version string
	holds   gomodule.Version
}

// readVendorList reads the modules that vendor/modules.txt lists. A missing
// file lists none.
func readVendorList(name string) map[string]vendored {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	out := map[string]vendored{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		// A module line is "# path version", or
		// "# path [version] => target [version]".
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 || fields[0] != "#" {
			continue
		}
		path, rest := fields[1], fields[2:]
		i := slices.Index(rest, "=>")
		if i < 0 {
			out[path] = vendored{version: rest[0], holds: gomodule.Version{Path: path, Version: rest[0]}}
			continue
		}
		v := vendored{}
		if i == 1 {
			v.version = rest[0]
		}
		switch target := rest[i+1:]; len(target) {
		case 1:
			v.holds = gomodule.Version{Path: target[0]}
		case 2:
			v.holds = gomodule.Version{Path: target[0], Version: target[1]}
		default:
			continue
		}
		out[path] = v
	}
	return out
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

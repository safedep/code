package modgraph

import (
	"bufio"
	"bytes"
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
	// ModuleCache is the module cache of the go command, as GOMODCACHE, or ""
	// for none. The caller finds it, so Discover reads no environment.
	ModuleCache string
	// LocalRoot bounds a replace directive with a local path: Discover reads
	// a replacement only inside it, as the root of the scan. "" reads none.
	LocalRoot string
}

// Discover finds the source of each module of the build list of a main
// module, as the go command builds it: the replacement of a replace
// directive, then the vendor directory when vendor/modules.txt lists the
// module at its version, then the module cache. A module with no source
// gets the go.mod of the module cache, when the cache has it. Discover never
// downloads, and a module that it cannot find has no source and no go.mod,
// so Build makes it open.
func Discover(opts DiscoverOptions) ([]Source, error) {
	data, err := os.ReadFile(filepath.Join(opts.Dir, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("modgraph: read go.mod: %w", err)
	}
	// A lax parse drops the replace directives, and a replaced module read
	// from the module cache is other code. So go.mod must parse in full.
	f, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("modgraph: parse go.mod: %w", err)
	}
	vendored := readVendorList(filepath.Join(opts.Dir, "vendor", "modules.txt"))
	sums := readSums(filepath.Join(opts.Dir, "go.sum"))
	var out []Source
	for _, r := range f.Require {
		s := Source{Module: Module{Path: r.Mod.Path, Version: r.Mod.Version}}
		src := replacement(f, r.Mod)
		switch {
		case src.Version == "":
			s.FS, s.GoMod = localSource(opts, src.Path)
		case vendored[r.Mod.Path] == src:
			s.FS = os.DirFS(filepath.Join(opts.Dir, "vendor", filepath.FromSlash(r.Mod.Path)))
		default:
			s.FS, s.GoMod = cacheSource(opts.ModuleCache, src)
			s.Immutable = s.FS != nil
			s.Sum = sums[src]
		}
		out = append(out, s)
	}
	return out, nil
}

// replacement returns the module that the go command builds for a module of
// the build list: the target of the replace directive of that version, else
// of a replace directive of every version, else the module itself.
func replacement(f *modfile.File, m gomodule.Version) gomodule.Version {
	var all *modfile.Replace
	for _, r := range f.Replace {
		switch {
		case r.Old.Path != m.Path:
		case r.Old.Version == m.Version:
			return r.New
		case r.Old.Version == "":
			all = r
		}
	}
	if all != nil {
		return all.New
	}
	return m
}

// localSource returns the source of a replacement with a local path, inside
// LocalRoot only.
func localSource(opts DiscoverOptions, p string) (fs.FS, []byte) {
	if opts.LocalRoot == "" {
		return nil, nil
	}
	dir := filepath.FromSlash(p)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(opts.Dir, dir)
	}
	root, err := filepath.Abs(opts.LocalRoot)
	if err != nil {
		return nil, nil
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, nil
	}
	if rel, err := filepath.Rel(root, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, nil
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, nil
	}
	return os.DirFS(dir), readOptional(filepath.Join(dir, "go.mod"))
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

// readVendorList reads the modules that vendor/modules.txt lists, each with
// the module that the vendor directory holds for it: the module itself, or
// the target of its replace. A missing file lists none.
func readVendorList(name string) map[string]gomodule.Version {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	out := map[string]gomodule.Version{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		// A module line is "# path version", or
		// "# path [version] => target [version]".
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 || fields[0] != "#" {
			continue
		}
		path, rest := fields[1], fields[2:]
		if i := slices.Index(rest, "=>"); i >= 0 {
			target := rest[i+1:]
			switch len(target) {
			case 1:
				out[path] = gomodule.Version{Path: target[0]}
			case 2:
				out[path] = gomodule.Version{Path: target[0], Version: target[1]}
			}
			continue
		}
		out[path] = gomodule.Version{Path: path, Version: rest[0]}
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

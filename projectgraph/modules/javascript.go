package modules

import (
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/safedep/code/internal/pathglob"
)

var (
	jsExtensions = []string{".js", ".mjs", ".cjs", ".jsx", ".json"}
	tsExtensions = []string{".ts", ".tsx", ".mts", ".cts", ".d.ts", ".js", ".mjs", ".cjs", ".jsx", ".json"}
	tsRewrites   = map[string][]string{".js": {".ts", ".tsx"}, ".jsx": {".tsx"}, ".mjs": {".mts"}, ".cjs": {".cts"}}
)

// JavaScript resolves JavaScript and TypeScript imports: relative paths, the
// packages of the project by package.json, and tsconfig.json paths and
// baseUrl for a TypeScript importer.
type JavaScript struct {
	files     *Files
	manifests map[string]*PackageJSON
	packages  map[string]*PackageJSON
	projects  []*PackageJSON
	tsconfigs map[string]*tsconfig
	problems  problems
}

// NewJavaScript reads the package.json and tsconfig.json of each directory
// of the file set. The project packages are the root package and the
// workspaces of its workspaces field or of pnpm-workspace.yaml. A root with
// no workspaces makes every package.json a project package.
func NewJavaScript(files *Files, root fs.FS) *JavaScript {
	j := &JavaScript{
		files:     files,
		manifests: map[string]*PackageJSON{},
		packages:  map[string]*PackageJSON{},
		tsconfigs: map[string]*tsconfig{},
	}
	for _, d := range files.Dirs() {
		j.readPackageJSON(root, d)
		j.readTSConfig(root, d)
	}

	globs := j.workspaceGlobs(root)
	for _, d := range files.Dirs() {
		pj := j.manifests[d]
		if pj == nil || d != "" && globs != nil && !inWorkspace(globs, d) {
			continue
		}
		j.projects = append(j.projects, pj)
		if _, dup := j.packages[pj.Name]; pj.Name != "" && !dup {
			j.packages[pj.Name] = pj
		}
	}
	return j
}

// Packages returns the package.json of each project package, by directory.
func (j *JavaScript) Packages() []*PackageJSON { return j.projects }

// Problems returns the manifests that the resolver could not read.
func (j *JavaScript) Problems() []Problem { return j.problems }

// Resolve returns the files that importer names with spec in the form,
// and how the resolver found them: relative, tsconfig-paths,
// tsconfig-baseurl, exports-import and the other conditions, exports, main,
// index or subpath. One import names one file, unless the exports map of
// its package has no condition of the form: then every target of the map
// is a file.
func (j *JavaScript) Resolve(importer, spec string, typescript bool, form Form) ([]string, string, bool) {
	if isRelative(spec) {
		base, ok := Join(Dir(importer), spec)
		if !ok {
			return nil, "", false
		}
		if file, ok := j.File(base, typescript); ok {
			return []string{file}, "relative", true
		}
		return nil, "", false
	}
	if spec == "" || strings.HasPrefix(spec, "/") {
		return nil, "", false
	}
	if typescript {
		if cfg := j.tsconfigFor(importer); cfg != nil {
			if file, via, ok := j.viaTSConfig(cfg, spec); ok {
				return []string{file}, via, true
			}
		}
	}
	name, sub := SplitPackage(spec)
	if pj := j.packages[name]; pj != nil {
		return j.Entries(pj, sub, typescript, form)
	}
	return nil, "", false
}

// Package returns the files of a subpath of the package at the root of
// the file set, as "." or "./fp", for the form, and how the resolver found
// them. A root with no package.json gives its index file and its files by
// path. The root of a unit of dependency code is the directory of the
// package.
func (j *JavaScript) Package(sub string, typescript bool, form Form) ([]string, string, bool) {
	pj := j.manifests[""]
	if pj == nil {
		pj = &PackageJSON{}
	}
	return j.Entries(pj, sub, typescript, form)
}

// Subpaths returns the subpaths that the package at the root of the file
// set exports, sorted: the keys of its exports field with no pattern. A
// package with no exports field exports "." and each file by its path
// with no extension, as "./lib/template".
func (j *JavaScript) Subpaths() []string {
	if pj := j.manifests[""]; pj != nil && len(pj.Exports) > 0 && string(pj.Exports) != "null" {
		return pj.exportSubpaths()
	}
	set := map[string]bool{".": true}
	for p := range j.files.files {
		if ext := path.Ext(p); slices.Contains(tsExtensions, ext) && ext != ".json" && !strings.HasSuffix(p, ".d.ts") {
			set["./"+strings.TrimSuffix(p, ext)] = true
		}
	}
	return sortedKeys(set)
}

// Entries returns the files of a subpath of a package, as "." or
// "./utils", for the form.
func (j *JavaScript) Entries(pj *PackageJSON, sub string, typescript bool, form Form) ([]string, string, bool) {
	if len(pj.Exports) > 0 && string(pj.Exports) != "null" {
		targets, cond, ok := exportTargets(pj.Exports, sub, form)
		if !ok {
			return nil, "", false
		}
		var files []string
		for _, target := range targets {
			if file, ok := j.relativeFile(pj.Dir, target, typescript); ok && !slices.Contains(files, file) {
				files = append(files, file)
			}
		}
		if len(files) == 0 {
			return nil, "", false
		}
		return files, strings.TrimSuffix("exports-"+cond, "-"), true
	}
	entries := []struct{ name, via string }{{sub, "subpath"}}
	if sub == "." {
		entries = []struct{ name, via string }{{pj.Main, "main"}, {"index", "index"}}
	}
	for _, e := range entries {
		if e.name == "" {
			continue
		}
		if file, ok := j.relativeFile(pj.Dir, e.name, typescript); ok {
			return []string{file}, e.via, true
		}
	}
	return nil, "", false
}

func (j *JavaScript) relativeFile(dir, name string, typescript bool) (string, bool) {
	base, ok := Join(dir, name)
	if !ok {
		return "", false
	}
	return j.File(base, typescript)
}

// File returns the file that a path without its extension names: the path
// itself, the TypeScript source of a .js path, the path with an extension,
// the main of its package.json, or its index file.
func (j *JavaScript) File(base string, typescript bool) (string, bool) {
	if file, ok := j.withExtension(base, typescript); ok {
		return file, true
	}
	if pj := j.manifests[base]; pj != nil && pj.Main != "" {
		if main, ok := Join(base, pj.Main); ok {
			if file, ok := j.withExtension(main, typescript); ok {
				return file, true
			}
			if file, ok := j.index(main, typescript); ok {
				return file, true
			}
		}
	}
	return j.index(base, typescript)
}

func (j *JavaScript) withExtension(base string, typescript bool) (string, bool) {
	if j.files.Has(base) {
		return base, true
	}
	exts := jsExtensions
	if typescript {
		exts = tsExtensions
		ext := path.Ext(base)
		for _, to := range tsRewrites[ext] {
			if p := strings.TrimSuffix(base, ext) + to; j.files.Has(p) {
				return p, true
			}
		}
	}
	for _, ext := range exts {
		if j.files.Has(base + ext) {
			return base + ext, true
		}
	}
	return "", false
}

func (j *JavaScript) index(dir string, typescript bool) (string, bool) {
	exts := jsExtensions
	if typescript {
		exts = tsExtensions
	}
	for _, ext := range exts {
		if p := path.Join(dir, "index"+ext); j.files.Has(p) {
			return p, true
		}
	}
	return "", false
}

func (j *JavaScript) tsconfigFor(importer string) *tsconfig {
	for d := Dir(importer); ; d = Dir(d) {
		if cfg := j.tsconfigs[d]; cfg != nil {
			return cfg
		}
		if d == "" {
			return nil
		}
	}
}

func (j *JavaScript) viaTSConfig(cfg *tsconfig, spec string) (string, string, bool) {
	if pattern, star, ok := matchPaths(cfg.paths, spec); ok {
		for _, sub := range cfg.paths[pattern] {
			if file, ok := j.relativeFile(cfg.pathsDir(), strings.Replace(sub, "*", star, 1), true); ok {
				return file, "tsconfig-paths", true
			}
		}
	}
	if cfg.hasBaseURL {
		if file, ok := j.relativeFile(cfg.baseURL, spec, true); ok {
			return file, "tsconfig-baseurl", true
		}
	}
	return "", "", false
}

// matchPaths returns the paths pattern that matches spec, the exact one or
// the one with the longest prefix before its "*", and the text that the
// "*" matches.
func matchPaths(paths map[string][]string, spec string) (string, string, bool) {
	if _, ok := paths[spec]; ok {
		return spec, "", true
	}
	best, bestPrefix, star := "", -1, ""
	for pattern := range paths {
		prefix, suffix, ok := strings.Cut(pattern, "*")
		if !ok || len(spec) < len(prefix)+len(suffix) ||
			!strings.HasPrefix(spec, prefix) || !strings.HasSuffix(spec, suffix) {
			continue
		}
		if len(prefix) < bestPrefix || len(prefix) == bestPrefix && pattern > best {
			continue
		}
		best, bestPrefix, star = pattern, len(prefix), spec[len(prefix):len(spec)-len(suffix)]
	}
	return best, star, bestPrefix >= 0
}

func (j *JavaScript) readPackageJSON(root fs.FS, dir string) {
	name := path.Join(dir, "package.json")
	data, ok, err := readFile(root, name)
	if err != nil {
		j.problems.add(name, err)
	}
	if !ok {
		return
	}
	pj, err := parsePackageJSON(dir, data)
	if err != nil {
		j.problems.add(name, err)
		return
	}
	j.manifests[dir] = pj
}

func (j *JavaScript) readTSConfig(root fs.FS, dir string) {
	name := path.Join(dir, "tsconfig.json")
	found, err := exists(root, name)
	if err != nil {
		j.problems.add(name, err)
	}
	if !found {
		return
	}
	cfg, err := loadTSConfig(root, name, 0)
	if err != nil {
		j.problems.add(name, err)
		return
	}
	j.tsconfigs[dir] = cfg
}

// workspaceGlobs returns the workspace globs of the root, or nil when the
// root declares no workspaces.
func (j *JavaScript) workspaceGlobs(root fs.FS) []string {
	var globs []string
	if pj := j.manifests[""]; pj != nil {
		globs = pj.workspaceGlobs()
	}
	data, ok, err := readFile(root, "pnpm-workspace.yaml")
	if err != nil {
		j.problems.add("pnpm-workspace.yaml", err)
	}
	if ok {
		pnpm, err := parsePNPMWorkspace(data)
		if err != nil {
			j.problems.add("pnpm-workspace.yaml", err)
		}
		globs = append(globs, pnpm...)
	}
	return globs
}

func inWorkspace(globs []string, dir string) bool {
	in := false
	for _, g := range globs {
		if neg, ok := strings.CutPrefix(g, "!"); ok {
			if pathglob.Match(neg, dir) {
				in = false
			}
		} else if pathglob.Match(g, dir) {
			in = true
		}
	}
	return in
}

func isRelative(spec string) bool {
	return spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")
}

// SplitPackage splits a bare import into its package name and the subpath,
// as "@scope/name" and "./utils".
func SplitPackage(spec string) (string, string) {
	n := 2
	if strings.HasPrefix(spec, "@") {
		n = 3
	}
	parts := strings.SplitN(spec, "/", n)
	if len(parts) < n {
		return spec, "."
	}
	return strings.Join(parts[:n-1], "/"), "./" + parts[n-1]
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func sortedValues(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	values := make([]string, 0, len(m))
	for _, k := range keys {
		values = append(values, m[k])
	}
	return values
}

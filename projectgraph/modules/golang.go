package modules

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

var errNoModule = errors.New("go.mod has no module directive")

// StdModule is the module path of the standard library in GOROOT/src. The
// import path of each of its packages is the directory, as net/http, with
// no module prefix.
const StdModule = "std"

// IsStdPath reports whether an import path can name a package of the
// standard library: its first element has no dot.
func IsStdPath(spec string) bool {
	first, _, _ := strings.Cut(spec, "/")
	return first != "" && !strings.Contains(first, ".")
}

// goIgnoredDirs hold Go code that is not a package of the module. vendor
// holds the code of a dependency, which stays external, and the go command
// skips testdata.
var goIgnoredDirs = []string{"vendor", "testdata"}

// Golang resolves Go import paths against the modules of a project. A
// module is a directory with a go.mod. A package is a directory with Go
// files, and its import path is the path of the nearest module and the
// directory below it.
type Golang struct {
	modules      []goModule
	byPath       map[string]string
	dirs         map[string]bool
	dependencies []string
	problems     problems
}

type goModule struct{ dir, path string }

// NewGolang reads the go.mod of each directory of the file set, and gives an
// import path to each directory of dirs that holds Go files.
func NewGolang(files *Files, root fs.FS, dirs []string) *Golang {
	g := &Golang{byPath: map[string]string{}, dirs: make(map[string]bool, len(dirs))}
	for _, d := range dirs {
		g.dirs[d] = d != ""
	}
	for _, d := range files.Dirs() {
		if root == nil {
			break
		}
		name := path.Join(d, "go.mod")
		data, err := fs.ReadFile(root, fsName(name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			g.problems.add(name, err)
			continue
		}
		if p := modfile.ModulePath(data); p != "" {
			g.modules = append(g.modules, goModule{dir: d, path: p})
		} else {
			g.problems.add(name, errNoModule)
		}
		if f, err := modfile.ParseLax(name, data, nil); err == nil {
			for _, r := range f.Require {
				if !slices.Contains(g.dependencies, r.Mod.Path) {
					g.dependencies = append(g.dependencies, r.Mod.Path)
				}
			}
		}
	}
	// The deepest module holds a directory, so it comes first.
	slices.SortFunc(g.modules, func(a, b goModule) int { return len(b.dir) - len(a.dir) })
	for _, d := range dirs {
		if p, ok := g.ImportPath(d); ok {
			if _, dup := g.byPath[p]; !dup {
				g.byPath[p] = d
			}
		}
	}
	return g
}

// Problems returns the go.mod files that the resolver could not read.
func (g *Golang) Problems() []Problem { return g.problems }

// Dependencies returns the module paths that the go.mod files require.
func (g *Golang) Dependencies() []string { return g.dependencies }

// Resolve returns the directory of the package that an import path names.
func (g *Golang) Resolve(spec string) (string, bool) {
	d, ok := g.byPath[spec]
	return d, ok
}

// FirstParty reports whether an import path can name a package of the
// project: it is under the path of a module of the project. A project with
// no go.mod has no module path, so an import path that ends with a package
// directory of the project can name it.
func (g *Golang) FirstParty(spec string) bool {
	if len(g.modules) > 0 {
		return slices.ContainsFunc(g.modules, func(m goModule) bool {
			return underPath(spec, m.path) || m.path == StdModule && IsStdPath(spec)
		})
	}
	// Each suffix of the path after a slash is a lookup, because a scan of
	// the directories at each import grows with the square of the project.
	for suffix := spec; ; {
		if g.dirs[suffix] {
			return true
		}
		_, rest, ok := strings.Cut(suffix, "/")
		if !ok {
			return false
		}
		suffix = rest
	}
}

// ImportPath returns the import path of a directory, by the go.mod of the
// nearest module.
func (g *Golang) ImportPath(dir string) (string, bool) {
	for _, m := range g.modules {
		rel, ok := below(dir, m.dir)
		if !ok {
			continue
		}
		if slices.ContainsFunc(strings.Split(rel, "/"), func(part string) bool { return slices.Contains(goIgnoredDirs, part) }) {
			return "", false
		}
		if m.path == StdModule {
			return rel, rel != ""
		}
		return path.Join(m.path, rel), true
	}
	return "", false
}

// below returns the path of dir below base, "" for base itself.
func below(dir, base string) (string, bool) {
	switch {
	case base == "":
		return dir, true
	case dir == base:
		return "", true
	case strings.HasPrefix(dir, base+"/"):
		return dir[len(base)+1:], true
	}
	return "", false
}

func underPath(spec, prefix string) bool {
	return spec == prefix || strings.HasPrefix(spec, prefix+"/")
}

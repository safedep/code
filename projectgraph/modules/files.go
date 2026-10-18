// Package modules maps an import, as the source writes it, to a file of the
// project. It reads the file set and the manifests of the project, and no
// source file.
package modules

import (
	"path"
	"slices"
	"strings"
)

// Files is the set of first-party files of a project and their directories.
// Paths are relative to the project root, with "/". The root directory is "".
type Files struct {
	files map[string]bool
	dirs  map[string]bool
	names map[string]bool
}

// NewFiles returns the set of the paths.
func NewFiles(paths []string) *Files {
	f := &Files{files: make(map[string]bool, len(paths)), dirs: map[string]bool{"": true}, names: map[string]bool{}}
	for _, p := range paths {
		f.files[p] = true
		base := path.Base(p)
		f.names[strings.TrimSuffix(base, path.Ext(base))] = true
		for d := Dir(p); !f.dirs[d]; d = Dir(d) {
			f.dirs[d] = true
			f.names[path.Base(d)] = true
		}
	}
	return f
}

// HasName reports whether a file with no extension or a directory of the set
// has the name, at any depth.
func (f *Files) HasName(name string) bool { return f.names[name] }

// Has reports whether the path is a file of the set.
func (f *Files) Has(p string) bool { return f.files[p] }

// HasDir reports whether a file of the set is under the directory.
func (f *Files) HasDir(d string) bool { return f.dirs[d] }

// Dirs returns every directory of the set, sorted.
func (f *Files) Dirs() []string {
	dirs := make([]string, 0, len(f.dirs))
	for d := range f.dirs {
		dirs = append(dirs, d)
	}
	slices.Sort(dirs)
	return dirs
}

// Dir returns the directory of a path, "" for a path at the root.
func Dir(p string) string {
	d := path.Dir(p)
	if d == "." || d == "/" {
		return ""
	}
	return d
}

// Join joins the parts and cleans the result. It returns "" for the root
// and false for a path that leaves the root.
func Join(parts ...string) (string, bool) {
	p := path.Join(parts...)
	switch {
	case p == "." || p == "":
		return "", true
	case p == ".." || len(p) > 2 && p[:3] == "../" || path.IsAbs(p):
		return "", false
	}
	return p, true
}

// Problem is a manifest that the resolver could not read or parse. The
// resolver goes on without it, and the graph keeps the problem.
type Problem struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// problems collects the problems of one resolver.
type problems []Problem

func (p *problems) add(name string, err error) {
	*p = append(*p, Problem{Path: name, Text: err.Error()})
}

// fsName returns the fs.FS name of a project path.
func fsName(p string) string {
	if p == "" {
		return "."
	}
	return p
}

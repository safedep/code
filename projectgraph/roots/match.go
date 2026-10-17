// Package roots finds the roots of a project graph: the definitions and the
// module top levels that the root rules mark. It reads the indexes, the
// manifests of the project and, for a file_contains rule, the file itself.
package roots

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	rules "github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
)

// Root is a key that a rule marks. Site is the call that registers the
// root, as app.get('/x', handler). It is zero for a root that the key
// itself marks.
type Root struct {
	Key  string
	Kind rules.Kind
	Rule string
	Site Site
}

// Site is a place in a file. Lines and columns are 1-based.
type Site struct {
	Path         string
	Line, Column int
}

// Project is what the matcher reads.
type Project struct {
	// Indexes are in path order.
	Indexes []*fileindex.Index
	IsTest  func(path string) bool
	// Root holds the manifests and the files. A nil Root reads nothing.
	Root   fs.FS
	Files  *modules.Files
	Python *modules.Python
	JS     *modules.JavaScript
	// Defined reports whether a key names a first-party node of the graph.
	Defined func(key string) bool
	// Exports returns the keys of the names that a file exports.
	Exports func(path string) []string
	// Export returns the key of a name that a file exports.
	Export func(path, name string) (string, bool)
	// Resolve returns the keys that the per-file targets of a site in the
	// caller resolve to, first-party or external.
	Resolve func(ix *fileindex.Index, caller string, targets []string) []string
	// ResolveName returns the key of a dotted name as a file writes it, as
	// app.route. A name under a first-party definition that the link step
	// does not know, as cli.command for a function cli, is its key under
	// the definition.
	ResolveName func(path, name string) (string, bool)
}

// Find returns the roots that the rules mark, sorted, and the manifests and
// files that it could not read.
func Find(p Project, rs []rules.Rule) ([]Root, []modules.Problem) {
	f := &finder{Project: p, languages: map[string]langcode.Code{}, seen: map[Root]bool{}, defs: map[string]*fileindex.Definition{}, calls: map[*fileindex.Call][]string{}}
	for _, ix := range p.Indexes {
		f.languages[ix.Path] = ix.Language
		for i := range ix.Definitions {
			f.defs[ix.Definitions[i].Key] = &ix.Definitions[i]
		}
	}
	for _, r := range rs {
		f.match(r)
	}
	slices.SortFunc(f.roots, func(a, b Root) int {
		return cmp.Or(cmp.Compare(a.Key, b.Key), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Rule, b.Rule))
	})
	return f.roots, f.problems
}

type finder struct {
	Project
	languages map[string]langcode.Code
	defs      map[string]*fileindex.Definition
	// calls holds the keys that each call resolves to.
	calls    map[*fileindex.Call][]string
	roots    []Root
	seen     map[Root]bool
	problems []modules.Problem
}

func (f *finder) add(r rules.Rule, key string) {
	f.addAt(r, key, Site{})
}

// addAt marks a root at the site that registers it. A key that the rule
// marks more than once keeps its first site.
func (f *finder) addAt(r rules.Rule, key string, site Site) {
	root := Root{Key: key, Kind: r.Kind, Rule: r.ID}
	if f.seen[root] || !f.Defined(key) {
		return
	}
	f.seen[root] = true
	root.Site = site
	f.roots = append(f.roots, root)
}

// addFile marks the module top level of a file whose language the rule
// takes.
func (f *finder) addFile(r rules.Rule, file string) {
	if lang, ok := f.languages[file]; ok && r.Applies(lang) {
		f.add(r, file)
	}
}

func (f *finder) match(r rules.Rule) {
	m := r.Match
	switch {
	case m.Definition != nil:
		f.matchDefinition(r)
	case m.FileContains != "":
		f.matchFileContains(r)
	case len(m.Path) > 0:
		f.matchPath(r)
	case len(m.Decorator) > 0 || len(m.Annotation) > 0:
		f.matchDecorator(r)
	case m.Call != nil && m.Call.Arg == nil:
		f.matchDecoratorCall(r)
	case m.Call != nil:
		f.matchCall(r)
	case m.Export != nil:
		f.matchExport(r)
	case m.TestFile:
		f.matchTestFiles(r)
	case m.Manifest != nil:
		f.matchManifest(r)
	}
}

func (f *finder) applying(r rules.Rule) []*fileindex.Index {
	var out []*fileindex.Index
	for _, ix := range f.Indexes {
		if r.Applies(ix.Language) {
			out = append(out, ix)
		}
	}
	return out
}

// matchDefinition marks a definition by its name, the name of its class,
// the static mark and the package clause of its file. A match in a package
// marks only a top-level function, as func main in package main of Go.
func (f *finder) matchDefinition(r rules.Rule) {
	m := r.Match.Definition
	for _, ix := range f.applying(r) {
		if m.InPackage != "" && ix.Package != m.InPackage {
			continue
		}
		for _, def := range ix.Definitions {
			switch {
			case def.Name != m.Name:
			case m.Parent != "" && fileindex.Last(def.Parent) != m.Parent:
			case m.InPackage != "" && (def.Kind != fileindex.DefFunction || len(fileindex.Segments(def.Key)) != 1):
			case m.Static && !def.Static:
			default:
				f.add(r, def.Key)
			}
		}
	}
}

func (f *finder) matchFileContains(r rules.Rule) {
	needle := []byte(r.Match.FileContains)
	for _, ix := range f.applying(r) {
		data, ok := f.read(ix.Path)
		if ok && bytes.Contains(data, needle) {
			f.add(r, ix.Path)
		}
	}
}

// matchTestFiles marks the module top level and every function, method and
// constructor of a test file.
func (f *finder) matchTestFiles(r rules.Rule) {
	for _, ix := range f.Indexes {
		if !f.IsTest(ix.Path) {
			continue
		}
		f.add(r, ix.Path)
		for _, def := range ix.Definitions {
			if def.Kind != fileindex.DefClass {
				f.add(r, def.Key)
			}
		}
	}
}

func (f *finder) matchManifest(r rules.Rule) {
	m := r.Match.Manifest
	if m.File == rules.PackageJSON {
		f.matchPackageJSON(r)
		return
	}
	for _, dir := range f.Files.Dirs() {
		data, ok := f.read(path.Join(dir, m.File))
		if !ok {
			continue
		}
		switch m.File {
		case rules.PyProject:
			for _, entry := range pyprojectEntries(data, m.Fields) {
				f.addPythonEntry(r, dir, entry)
			}
		case rules.SetupPy:
			for _, entry := range setupEntries(data) {
				f.addPythonEntry(r, dir, entry)
			}
		case rules.Procfile:
			for _, cmd := range procfileCommands(data) {
				f.addCommand(r, dir, cmd)
			}
		case rules.Dockerfile:
			for _, cmd := range dockerfileCommands(data) {
				f.addCommand(r, dir, cmd)
			}
		}
	}
}

// matchPackageJSON marks the module top level of each entry file. The main,
// module and exports fields name the entry of a library, and the code of
// another package calls what it exports. So the exports of such an entry
// are roots too. A bin entry is a program, and only its top level runs.
func (f *finder) matchPackageJSON(r rules.Rule) {
	for _, pj := range f.JS.Packages() {
		var programs, libraries []string
		for _, field := range r.Match.Manifest.Fields {
			switch field {
			case "main":
				libraries = append(libraries, pj.Main)
			case "module":
				libraries = append(libraries, pj.Module)
			case "bin":
				programs = append(programs, pj.BinFiles()...)
			case "exports":
				libraries = append(libraries, pj.ExportFiles()...)
			}
		}
		for _, file := range f.entryFiles(pj.Dir, programs) {
			f.addFile(r, file)
		}
		for _, file := range f.entryFiles(pj.Dir, libraries) {
			f.addFile(r, file)
			if lang, ok := f.languages[file]; ok && r.Applies(lang) {
				for _, key := range f.Exports(file) {
					f.add(r, key)
				}
			}
		}
	}
}

// entryFiles returns the project files that the entries of a package.json
// in dir name.
func (f *finder) entryFiles(dir string, entries []string) []string {
	var files []string
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		if base, ok := modules.Join(dir, entry); ok {
			if file, ok := f.JS.File(base, true); ok {
				files = append(files, file)
			}
		}
	}
	return files
}

// addPythonEntry marks the target of a module:function entry point. A
// function that the index does not define leaves the module top level.
func (f *finder) addPythonEntry(r rules.Rule, dir, entry string) {
	fields := strings.Fields(entry)
	if len(fields) == 0 {
		return
	}
	module, attr, _ := strings.Cut(fields[0], ":")
	file, ok := f.Python.Absolute(path.Join(dir, rules.PyProject), module)
	if !ok || strings.HasSuffix(file, "/") {
		return
	}
	if lang, ok := f.languages[file]; !ok || !r.Applies(lang) {
		return
	}
	if attr != "" {
		if key := fileindex.Join(file, strings.Split(attr, ".")...); f.Defined(key) {
			f.add(r, key)
			return
		}
	}
	f.add(r, file)
}

// addCommand marks each file of the project that a command names, as
// node server.js, python -m app.main or gunicorn app.wsgi:app.
func (f *finder) addCommand(r rules.Rule, dir string, args []string) {
	for i, arg := range args {
		switch {
		case arg == "-m" && i+1 < len(args):
			f.addPythonEntry(r, dir, args[i+1])
		case strings.Contains(arg, ":") && !strings.Contains(arg, "/"):
			f.addPythonEntry(r, dir, arg)
		default:
			if file, ok := f.commandFile(dir, arg); ok {
				f.addFile(r, file)
			}
		}
	}
}

// commandFile finds the project file of a command argument. An absolute
// path in a container, as /app/src/server.js, loses its leading parts until
// a file of the project matches.
func (f *finder) commandFile(dir, arg string) (string, bool) {
	rel := strings.TrimPrefix(arg, "/")
	for rel != "" {
		if file, ok := modules.Join(dir, rel); ok && file != "" && f.Files.Has(file) {
			return file, true
		}
		if !strings.HasPrefix(arg, "/") {
			return "", false
		}
		_, rest, ok := strings.Cut(rel, "/")
		if !ok {
			return "", false
		}
		rel = rest
	}
	return "", false
}

func (f *finder) read(name string) ([]byte, bool) {
	if f.Root == nil {
		return nil, false
	}
	data, err := fs.ReadFile(f.Root, name)
	if err == nil {
		return data, true
	}
	if !errors.Is(err, fs.ErrNotExist) {
		f.problems = append(f.problems, modules.Problem{Path: name, Text: err.Error()})
	}
	return nil, false
}

var (
	tomlTable  = regexp.MustCompile(`^\[\[?\s*([^\]]+?)\s*\]\]?`)
	tomlPair   = regexp.MustCompile(`^("[^"]*"|'[^']*'|[A-Za-z0-9_.-]+)\s*=\s*(.*)$`)
	tomlString = regexp.MustCompile(`^("[^"]*"|'[^']*')`)
	tomlInline = regexp.MustCompile(`("[^"]*"|'[^']*'|[A-Za-z0-9_.-]+)\s*=\s*("[^"]*"|'[^']*')`)
	setupList  = regexp.MustCompile(`console_scripts['"]?\s*[:=]?\s*\[([^\]]*)\]`)
	setupEntry = regexp.MustCompile(`['"]\s*[^'"=\s]+\s*=\s*([^'"\s]+)\s*['"]`)
)

// pyprojectEntries returns the string values of the tables that the fields
// name, as project.scripts, and of their sub-tables, as
// project.entry-points.console_scripts. It reads plain tables and inline
// tables, which is how a pyproject.toml writes entry points.
func pyprojectEntries(data []byte, fields []string) []string {
	var entries []string
	wanted := func(table string) bool {
		return slices.ContainsFunc(fields, func(f string) bool {
			return table == f || strings.HasPrefix(table, f+".")
		})
	}
	table := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if m := tomlTable.FindStringSubmatch(line); m != nil {
			table = unquoteDotted(m[1])
			continue
		}
		m := tomlPair.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key, value := unquote(m[1]), strings.TrimSpace(m[2])
		if strings.HasPrefix(value, "{") {
			if !wanted(joinTable(table, key)) {
				continue
			}
			for _, pair := range tomlInline.FindAllStringSubmatch(value, -1) {
				entries = append(entries, unquote(pair[2]))
			}
			continue
		}
		if str := tomlString.FindString(value); wanted(table) && str != "" {
			entries = append(entries, unquote(str))
		}
	}
	return entries
}

// setupEntries returns the targets of the console_scripts entry points of a
// setup.py, as pkg.cli:main.
func setupEntries(data []byte) []string {
	var entries []string
	for _, list := range setupList.FindAllSubmatch(data, -1) {
		for _, m := range setupEntry.FindAllSubmatch(list[1], -1) {
			entries = append(entries, string(m[1]))
		}
	}
	return entries
}

// procfileCommands returns the command of each process of a Procfile.
func procfileCommands(data []byte) [][]string {
	var cmds [][]string
	for _, line := range strings.Split(string(data), "\n") {
		if _, cmd, ok := strings.Cut(line, ":"); ok && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			cmds = append(cmds, strings.Fields(cmd))
		}
	}
	return cmds
}

// dockerfileCommands returns the arguments of each CMD and ENTRYPOINT, in
// the exec form or the shell form.
func dockerfileCommands(data []byte) [][]string {
	var cmds [][]string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if op := strings.ToUpper(fields[0]); op != "CMD" && op != "ENTRYPOINT" {
			continue
		}
		rest := strings.TrimSpace(line[len(fields[0])+strings.Index(line, fields[0]):])
		var exec []string
		if json.Unmarshal([]byte(rest), &exec) == nil {
			cmds = append(cmds, exec)
			continue
		}
		cmds = append(cmds, strings.Fields(rest))
	}
	return cmds
}

func joinTable(table, key string) string {
	if table == "" {
		return key
	}
	return table + "." + key
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// unquoteDotted removes the quotes of the parts of a table name, as
// project.entry-points."flake8.extension".
func unquoteDotted(s string) string {
	return strings.NewReplacer(`"`, "", `'`, "").Replace(s)
}

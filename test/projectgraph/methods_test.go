//go:build projectgraph

package projectgraph

import (
	"bufio"
	"context"
	"encoding/json"
	iofs "io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	codefs "github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/plugin/indexer"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// methodCorpusEnv names the directory that scripts/fetch_method_corpus.sh
// fills with the projects and the dependencies of the method measure.
const methodCorpusEnv = "PROJECTGRAPH_METHOD_CORPUS"

// methodLanguagesEnv names the languages that the index step reads, as
// python,javascript. Empty reads every language. A Python project bundles
// the JavaScript of its web pages, and a minified file of it is truncated,
// so a run of python alone shows the Python causes.
const methodLanguagesEnv = "PROJECTGRAPH_METHOD_LANGUAGES"

// methodsDir holds the list of the projects and the advisories of their
// dependencies, from the Go vulndb and from RustSec through OSV, and the
// PyPI advisories with the symbols that scripts/pypi_advisories.py holds.
var methodsDir = filepath.Join("public", "methods")

// methodProject is one line of corpus.txt.
type methodProject struct {
	language, name, source string
	deps                   []string
}

// advisory is one advisory of a package: the events of its affected
// ranges, and the symbols of Go or the functions of Rust that it names. A
// PyPI advisory has the Go shape, with a module path and its symbols, and
// Source says that a person wrote the symbols from the advisory text,
// because OSV lists none for PyPI. scripts/pypi_advisories.py holds them.
type advisory struct {
	ID     string `json:"id"`
	Source string `json:"source,omitempty"`
	Ranges []struct {
		Introduced string `json:"introduced"`
		Fixed      string `json:"fixed"`
	} `json:"ranges"`
	Imports []struct {
		Path    string   `json:"path"`
		Symbols []string `json:"symbols"`
	} `json:"imports"`
	Functions []string `json:"functions"`
}

// affects reports whether the ranges of the advisory hold the version.
func (a advisory) affects(version string) bool {
	v := "v" + strings.TrimPrefix(version, "v")
	inside := false
	for _, e := range a.Ranges {
		switch {
		case e.Introduced == "0", e.Introduced != "" && semver.Compare("v"+e.Introduced, v) <= 0:
			inside = true
		case e.Fixed != "" && semver.Compare("v"+e.Fixed, v) <= 0:
			inside = false
		}
	}
	return inside
}

// methodTarget is a target of the measure and the kind of its symbol.
type methodTarget struct {
	key  string
	kind string
}

// symbolKinds name the kind of a symbol: a function, a method of a type,
// or another name, as a type that RustSec lists.
const (
	kindFunction = "function"
	kindMethod   = "method"
	kindOther    = "other"
)

func readMethodCorpus(t *testing.T) ([]methodProject, map[string]map[string][]advisory) {
	f, err := os.Open(filepath.Join(methodsDir, "corpus.txt"))
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	var projects []methodProject
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		require.GreaterOrEqual(t, len(fields), 3, scanner.Text())
		projects = append(projects, methodProject{language: fields[0], name: fields[1], source: fields[2], deps: fields[3:]})
	}
	require.NoError(t, scanner.Err())

	data, err := os.ReadFile(filepath.Join(methodsDir, "advisories.json"))
	require.NoError(t, err)
	var advisories map[string]map[string][]advisory
	require.NoError(t, json.Unmarshal(data, &advisories))
	return projects, advisories
}

// methodTargets returns the symbols that the advisories of a dependency
// name at its version, each once. An advisory with no symbol list covers
// the whole package, which the measure cannot list, and a Rust name with
// type arguments is no key.
func methodTargets(language, dep string, advisories map[string]map[string][]advisory) []methodTarget {
	name, version, _ := strings.Cut(dep, "@")
	seen := map[string]bool{}
	var out []methodTarget
	add := func(key, kind string) {
		if !seen[key] {
			seen[key] = true
			out = append(out, methodTarget{key: key, kind: kind})
		}
	}
	if eco, lang := ecosystemOf(language); eco != "cargo" {
		for _, a := range advisories[eco][name] {
			if !a.affects(version) {
				continue
			}
			for _, imp := range a.Imports {
				for _, sym := range imp.Symbols {
					kind := kindFunction
					if strings.Contains(sym, ".") {
						kind = kindMethod
					}
					add(fileindex.Join(fileindex.Namespace(lang, imp.Path), strings.Split(sym, ".")...), kind)
				}
			}
		}
		return out
	}
	for _, a := range advisories["cargo"][name] {
		if !a.affects(version) {
			continue
		}
		for _, fn := range a.Functions {
			if strings.ContainsAny(fn, "<> ") {
				continue
			}
			segs := strings.Split(fn, "::")
			kind := kindFunction
			switch {
			case len(segs) > 2 && startsUpper(segs[len(segs)-2]):
				kind = kindMethod
			case startsUpper(segs[len(segs)-1]):
				kind = kindOther
			}
			add(strings.Join(segs, fileindex.Separator), kind)
		}
	}
	return out
}

func startsUpper(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r)
}

// ecosystemOf returns the ecosystem of the packages of a language of the
// corpus, as the keys of advisories.json and the units write it, and the
// language code of the import namespaces of its keys.
func ecosystemOf(language string) (string, langcode.Code) {
	switch language {
	case "go":
		return "go", langcode.Go
	case "python":
		return "pypi", langcode.Python
	}
	return "cargo", langcode.Rust
}

// methodDir returns the directory of a project or a dependency in the
// corpus, and its unit. A PyPI dependency is an unpacked wheel, so its
// directory has the layout of site-packages, and the unit is rooted there.
func methodDir(t *testing.T, corpus, language, item string) (string, string) {
	name, version, _ := strings.Cut(item, "@")
	eco, _ := ecosystemOf(language)
	switch eco {
	case "go":
		escaped, err := module.EscapePath(name)
		require.NoError(t, err)
		return filepath.Join(corpus, "gomod", filepath.FromSlash(escaped)+"@"+version), "go:" + item
	case "pypi":
		return filepath.Join(corpus, "pypi", name+"-"+version), "pypi:" + item
	}
	return filepath.Join(corpus, "crates", name+"-"+version), "cargo:" + item
}

// nonTestRoots returns the keys of the roots of the graph that are not
// test roots, as the search of a query without tests starts from them.
func nonTestRoots(g *projectgraph.Graph) []string {
	var keys []string
	for _, r := range g.Roots {
		if r.Kind != projectgraph.RootTest {
			keys = append(keys, r.Key)
		}
	}
	return keys
}

// methodCounts holds the outcomes of the targets by language and kind.
type methodCounts map[[2]string]map[string]int

func (c methodCounts) add(language, kind, outcome string) {
	k := [2]string{language, kind}
	if c[k] == nil {
		c[k] = map[string]int{}
	}
	c[k][outcome]++
}

// rows writes one line per language and kind with the count of each
// outcome, and the share of the method targets whose cause is an unknown
// receiver.
func (c methodCounts) rows() []string {
	keys := slices.SortedFunc(maps.Keys(c), func(a, b [2]string) int {
		return strings.Compare(a[0]+"/"+a[1], b[0]+"/"+b[1])
	})
	var rows []string
	for _, k := range keys {
		counts := c[k]
		total := 0
		for _, n := range counts {
			total += n
		}
		parts := []string{"language=" + k[0], "kind=" + k[1], "targets=" + strconv.Itoa(total)}
		for _, o := range slices.Sorted(maps.Keys(counts)) {
			parts = append(parts, o+"="+strconv.Itoa(counts[o]))
		}
		rows = append(rows, strings.Join(parts, " "))
		if k[1] == kindMethod {
			rows = append(rows, "  unknown-receiver="+ratio{hit: counts["uncertain/"+string(fileindex.UnresolvedUnknownReceiver)], all: total}.String())
		}
	}
	return rows
}

// TestMethodTargets measures how often a target of a vulnerable function
// or method of a dependency of a real project ends uncertain. Each target
// is a symbol of an advisory of a dependency at its pinned version, and
// the code of each such dependency is a unit of the link step.
func TestMethodTargets(t *testing.T) {
	corpus := os.Getenv(methodCorpusEnv)
	if corpus == "" {
		t.Skipf("%s is not set: run scripts/fetch_method_corpus.sh", methodCorpusEnv)
	}
	languages, err := lang.AllLanguages()
	require.NoError(t, err)
	if names := os.Getenv(methodLanguagesEnv); names != "" {
		languages = slices.DeleteFunc(languages, func(l core.Language) bool {
			return !slices.Contains(strings.Split(names, ","), string(l.Meta().Code))
		})
	}
	projects, advisories := readMethodCorpus(t)

	counts := methodCounts{}
	var lines []string
	for _, p := range projects {
		dir, _ := methodDir(t, corpus, p.language, p.source)
		cfg := codefs.LocalFileSystemConfig{AppDirectories: []string{dir}, ImportUnits: map[string]string{}, ExcludePatterns: codefs.DefaultSkipPatterns(dir)}
		units := map[string]iofs.FS{}
		var targets []methodTarget
		for _, dep := range p.deps {
			depDir, unit := methodDir(t, corpus, p.language, dep)
			cfg.ImportDirectories = append(cfg.ImportDirectories, depDir)
			cfg.ImportUnits[depDir] = unit
			cfg.ExcludePatterns = append(cfg.ExcludePatterns, codefs.SkipPattern(depDir))
			units[unit] = os.DirFS(depDir)
			targets = append(targets, methodTargets(p.language, dep, advisories)...)
		}
		fsys, err := codefs.NewLocalFileSystem(cfg)
		require.NoError(t, err)
		indexed := indexProject(t, fsys, indexer.Options{Languages: languages, Imports: true})
		g := linkProject(t, dir, indexed, projectgraph.Options{TestPaths: fileindex.DefaultTestPaths, Units: units})

		keys := make([]reach.Target, 0, len(targets))
		for _, mt := range targets {
			target, err := reach.TargetByKey(mt.key)
			require.NoError(t, err, mt.key)
			keys = append(keys, target)
		}
		results, err := reach.Reach(context.Background(), g, reach.Query{Targets: keys})
		require.NoError(t, err)
		methods := receiverReasons(t, g, nonTestRoots(g))
		project := methodCounts{}
		for i, r := range results {
			o := outcome(r, methods)
			counts.add(p.language, targets[i].kind, o)
			project.add(p.language, targets[i].kind, o)
		}
		lines = append(lines, "project="+p.name+" files="+strconv.Itoa(len(g.Files))+" truncated="+strconv.Itoa(len(g.Truncated))+" problems="+strconv.Itoa(len(g.Problems)))
		for _, row := range project.rows() {
			lines = append(lines, "  "+row)
		}
	}
	rows := append([]string{"set=methods projects=" + strconv.Itoa(len(projects))}, counts.rows()...)
	rows = append(rows, lines...)
	t.Log("\n" + strings.Join(rows, "\n"))
	if out := os.Getenv("PROJECTGRAPH_METHOD_REPORT"); out != "" {
		require.NoError(t, os.WriteFile(out, []byte(strings.Join(rows, "\n")+"\n"), 0o644))
	}
}

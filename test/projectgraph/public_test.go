//go:build projectgraph

package projectgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	codefs "github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/plugin/fileindex"
	"github.com/safedep/code/projectgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// swarmJSEnv names a directory with the SWARM-JS snippets, as
// benchmarks/javascript/swarm_js of a SWARM-CG checkout. SWARM-CG has no
// license, so the repository does not hold a copy.
const swarmJSEnv = "PROJECTGRAPH_SWARM_JS"

// publicSet is a set of cases in the PyCG layout. Each directory with a
// callgraph.json is one project, and callgraph.json maps each caller name to
// its callee names.
type publicSet struct {
	name     string
	language core.LanguageCode
	dir      string
}

func publicSets() []publicSet {
	sets := []publicSet{{name: "pycg-micro", language: core.LanguageCodePython, dir: filepath.Join("public", "pycg-micro", "snippets")}}
	if dir := os.Getenv(swarmJSEnv); dir != "" {
		sets = append(sets, publicSet{name: "swarm-js", language: core.LanguageCodeJavascript, dir: dir})
	}
	return sets
}

// callPair is one caller and callee, as callgraph.json names them.
type callPair struct{ from, to string }

// calleeClass splits the recall by where the callee lives.
type calleeClass string

const (
	calleeProject  calleeClass = "project"
	calleeExternal calleeClass = "external"
	calleeBuiltin  calleeClass = "builtin"
)

// ratio is a share of pairs, as the matched pairs of all pairs.
type ratio struct{ hit, all int }

func (r *ratio) add(hit bool) {
	r.all++
	if hit {
		r.hit++
	}
}

func (r ratio) String() string {
	if r.all == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.3f (%d/%d)", float64(r.hit)/float64(r.all), r.hit, r.all)
}

// score counts the pairs of a set. The category of a case is the first
// directory of its name, as classes or imports. A pair of the graph can have edges of
// more than one kind, and it counts once for each kind. A case is exact when
// the pairs of its graph are the pairs of its callgraph.json.
type score struct {
	files      int
	exact      ratio
	precision  ratio
	confident  ratio
	recall     ratio
	unresolved ratio
	byKind     map[projectgraph.EdgeKind]*ratio
	byClass    map[calleeClass]*ratio
	byCategory map[string]*ratio
}

func newScore() *score {
	return &score{byKind: map[projectgraph.EdgeKind]*ratio{}, byClass: map[calleeClass]*ratio{}, byCategory: map[string]*ratio{}}
}

func at[K comparable](m map[K]*ratio, k K) *ratio {
	if m[k] == nil {
		m[k] = &ratio{}
	}
	return m[k]
}

func TestPublicSets(t *testing.T) {
	ctx := context.Background()
	var rows []string
	for _, set := range publicSets() {
		t.Run(set.name, func(t *testing.T) {
			language, err := lang.GetLanguage(string(set.language))
			require.NoError(t, err)
			dirs := caseDirs(t, set.dir)
			require.NotEmpty(t, dirs)

			total := newScore()
			for _, dir := range dirs {
				rel, err := filepath.Rel(set.dir, dir)
				require.NoError(t, err)
				name := filepath.ToSlash(rel)
				scoreCase(ctx, t, name, dir, language, total)
			}
			rows = append(rows, total.rows(set)...)
		})
	}
	t.Log("\n" + strings.Join(rows, "\n"))
	if out := os.Getenv("PROJECTGRAPH_PUBLIC_REPORT"); out != "" {
		require.NoError(t, os.WriteFile(out, []byte(strings.Join(rows, "\n")+"\n"), 0o644))
	}
}

func caseDirs(t *testing.T, root string) []string {
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "callgraph.json" {
			dirs = append(dirs, filepath.Dir(p))
		}
		return nil
	})
	require.NoError(t, err)
	slices.Sort(dirs)
	return dirs
}

func scoreCase(ctx context.Context, t *testing.T, name, dir string, language core.Language, total *score) {
	expected := readCallGraph(t, filepath.Join(dir, "callgraph.json"))

	fsys, err := codefs.NewLocalFileSystem(codefs.LocalFileSystemConfig{AppDirectories: []string{dir}})
	require.NoError(t, err)
	indexed, err := (&fileindex.Indexer{}).Run(ctx, fsys, fileindex.Options{Languages: []core.Language{language}})
	require.NoError(t, err, name)
	assert.Empty(t, indexed.Report.Failed, name)
	g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{Root: os.DirFS(dir), Files: indexed.Files})
	require.NoError(t, err, name)

	got := graphPairs(g, importedUnits(indexed.Indexes))
	if d := os.Getenv("PROJECTGRAPH_PUBLIC_DUMP"); d == name || d == "all" {
		dump(t, g, got, expected)
	}

	total.files += len(indexed.Indexes)
	extra := 0
	for pair, kinds := range got {
		hit := expected[pair]
		if !hit {
			extra++
		}
		total.precision.add(hit)
		if slices.ContainsFunc(kinds, func(k projectgraph.EdgeKind) bool { return k.Confidence() != projectgraph.ConfidenceLow }) {
			total.confident.add(hit)
		}
		for _, k := range kinds {
			at(total.byKind, k).add(hit)
		}
	}
	var modules []string
	for file := range g.Files {
		if m := callgraphName(file); m != "" {
			modules = append(modules, m)
		}
	}
	missed := 0
	for pair := range expected {
		_, hit := got[pair]
		if !hit {
			missed++
		}
		total.recall.add(hit)
		at(total.byClass, classify(pair.to, modules)).add(hit)
		at(total.byCategory, strings.Split(name, "/")[0]).add(hit)
	}
	total.exact.add(extra == 0 && missed == 0)

	sites, unresolved := callSites(g, indexed.Indexes)
	total.unresolved.hit += unresolved
	total.unresolved.all += sites
}

func readCallGraph(t *testing.T, file string) map[callPair]bool {
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	var cg map[string][]string
	require.NoError(t, json.Unmarshal(data, &cg))
	pairs := map[callPair]bool{}
	for from, tos := range cg {
		for _, to := range tos {
			pairs[callPair{from: from, to: to}] = true
		}
	}
	return pairs
}

// classify tells a builtin, an external and a project callee apart. A
// builtin starts with a tag, as <builtin>.len or <global>.eval. A project
// callee lives in a module of the case.
func classify(callee string, modules []string) calleeClass {
	if strings.HasPrefix(callee, "<") {
		return calleeBuiltin
	}
	if slices.ContainsFunc(modules, func(m string) bool { return callee == m || strings.HasPrefix(callee, m+".") }) {
		return calleeProject
	}
	return calleeExternal
}

// graphPairs maps the call edges of g to the names of callgraph.json. An
// import edge and an export edge are not calls, so they make no pair. An
// edge to a re-exported name goes on through the export hops to the
// definition. A call of a class names the constructor that the link step
// hangs off the class, and a class with none makes no pair, as PyCG does.
func graphPairs(g *projectgraph.Graph, imported map[string]bool) map[callPair][]projectgraph.EdgeKind {
	hops := map[string][]string{}
	for _, e := range g.Edges {
		if e.Kind == projectgraph.EdgeExport || isConstructorHop(e) {
			hops[e.From] = append(hops[e.From], e.To)
		}
	}
	pairs := map[callPair][]projectgraph.EdgeKind{}
	for _, e := range g.Edges {
		if e.Kind == projectgraph.EdgeImport || e.Kind == projectgraph.EdgeExport || isConstructorHop(e) {
			continue
		}
		for _, to := range follow(e.To, hops, map[string]bool{}) {
			if n, ok := g.Nodes[to]; ok && n.Kind == projectgraph.NodeClass {
				continue
			}
			pair := callPair{from: callgraphName(e.From), to: calleeName(g, imported, to)}
			if !slices.Contains(pairs[pair], e.Kind) {
				pairs[pair] = append(pairs[pair], e.Kind)
			}
		}
	}
	return pairs
}

func isConstructorHop(e projectgraph.Edge) bool {
	return strings.HasSuffix(e.Resolver, ".constructor")
}

func follow(key string, hops map[string][]string, seen map[string]bool) []string {
	next, ok := hops[key]
	if !ok || seen[key] {
		return []string{key}
	}
	seen[key] = true
	var out []string
	for _, to := range next {
		out = append(out, follow(to, hops, seen)...)
	}
	return out
}

// calleeName names an external key that no file imports as a builtin, as
// <builtin>.len or <builtin>.console.log.
func calleeName(g *projectgraph.Graph, imported map[string]bool, key string) string {
	if g.IsExternal(key) && !imported[fidx.Unit(key)] {
		return "<builtin>." + callgraphName(key)
	}
	return callgraphName(key)
}

func importedUnits(indexes []*fidx.Index) map[string]bool {
	units := map[string]bool{}
	for _, ix := range indexes {
		for _, imp := range ix.Imports {
			units[fidx.Unit(imp.Namespace)] = true
		}
	}
	return units
}

// packageFiles name the file that stands for its directory, by extension.
var packageFiles = map[string]string{".py": "__init__", ".js": "index", ".mjs": "index", ".cjs": "index"}

// callgraphName writes a key as PyCG and SWARM-JS do: the module path with
// dots and no extension, a package file as its directory, then the
// segments.
func callgraphName(key string) string {
	unit := fidx.Unit(key)
	segments := fidx.Segments(key)
	ext := path.Ext(unit)
	if pkg, ok := packageFiles[ext]; ok {
		unit = strings.TrimSuffix(unit, ext)
		if path.Base(unit) == pkg {
			unit = path.Dir(unit)
		}
	}
	parts := append(strings.Split(unit, "/"), segments...)
	return strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), ".")
}

// callSites counts the distinct call sites of the indexes and the sites
// that the graph leaves unresolved.
func callSites(g *projectgraph.Graph, indexes []*fidx.Index) (int, int) {
	type site struct {
		path         string
		line, column int
	}
	all := map[site]bool{}
	for _, ix := range indexes {
		for _, c := range ix.Calls {
			if c.Kind != fidx.CallImport {
				all[site{ix.Path, c.Line, c.Column}] = true
			}
		}
		for _, u := range ix.Unresolved {
			all[site{ix.Path, u.Line, u.Column}] = true
		}
	}
	unresolved := map[site]bool{}
	for _, u := range g.Unresolved {
		unresolved[site{u.Site.Path, u.Site.Line, u.Site.Column}] = true
	}
	return len(all), len(unresolved)
}

func (s *score) rows(set publicSet) []string {
	rows := []string{
		fmt.Sprintf("set=%s language=%s cases=%d files=%d", set.name, set.language, s.exact.all, s.files),
		fmt.Sprintf("  exact-cases=%s", s.exact),
		fmt.Sprintf("  precision=%s", s.precision),
		fmt.Sprintf("  precision-high-medium=%s", s.confident),
		fmt.Sprintf("  recall=%s", s.recall),
		fmt.Sprintf("  unresolved-rate=%s", s.unresolved),
	}
	for _, k := range slices.Sorted(maps.Keys(s.byKind)) {
		rows = append(rows, fmt.Sprintf("  kind=%s precision=%s", k, s.byKind[k]))
	}
	for _, c := range []calleeClass{calleeProject, calleeExternal, calleeBuiltin} {
		rows = append(rows, fmt.Sprintf("  callee=%s recall=%s", c, at(s.byClass, c)))
	}
	for _, c := range slices.Sorted(maps.Keys(s.byCategory)) {
		rows = append(rows, fmt.Sprintf("  category=%s recall=%s", c, s.byCategory[c]))
	}
	return rows
}

func dump(t *testing.T, g *projectgraph.Graph, got map[callPair][]projectgraph.EdgeKind, expected map[callPair]bool) {
	for _, e := range g.Edges {
		t.Logf("edge %s -> %s %s %s %s", e.From, e.To, e.Kind, e.Resolver, site(e.Site))
	}
	for pair, kinds := range got {
		if !expected[pair] {
			t.Logf("extra %s -> %s %v", pair.from, pair.to, kinds)
		}
	}
	for pair := range expected {
		if _, ok := got[pair]; !ok {
			t.Logf("missed %s -> %s", pair.from, pair.to)
		}
	}
}

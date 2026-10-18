// Package projectgraph runs the hand-written cases end to end: the source
// files of each case go through the indexer, the link step, the merge of an
// optional overlay.json and the reach query, and the result must match the
// expect.yaml of the case. The summary of each unit of dependency code of a
// case must match its golden file under summaries/.
package projectgraph

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	iofs "io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex/store/memstore"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/internal/testutil"
	"github.com/safedep/code/plugin/indexer"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/modules"
	"github.com/safedep/code/projectgraph/overlay"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/safedep/code/projectgraph/summary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var (
	update = flag.Bool("update", false, "write the golden summaries of the cases")
	// dumpDir names a directory that receives the graph and the reach results
	// of each case, so a refactor proves that it changes no output.
	dumpDir = flag.String("dump", "", "write the graph and the reach results of each case under this directory")
)

// expectation is the expect.yaml of a case. A site is path:line. Roots,
// unresolved sites and the fields of a result are the whole set. Edges are a
// subset of the edges of the graph, and no edge of the graph goes from a
// key of NoEdges to its to key. Units names the package unit of each
// directory of dependency code, relative to the project. ImportDirs are the
// import directories that hold them, and they default to the directories of
// Units.
type expectation struct {
	Source       string            `yaml:"source"`
	Description  string            `yaml:"description"`
	Units        map[string]string `yaml:"units,omitempty"`
	ImportDirs   []string          `yaml:"import_dirs,omitempty"`
	TestPaths    []string          `yaml:"test_paths,omitempty"`
	MaxFileBytes int               `yaml:"max_file_bytes,omitempty"`
	Failed       []string          `yaml:"failed,omitempty"`
	Problems     []modules.Problem `yaml:"problems,omitempty"`
	Roots        []root            `yaml:"roots"`
	Edges        []edge            `yaml:"edges"`
	NoEdges      []noEdge          `yaml:"no_edges,omitempty"`
	Unresolved   []unresolved      `yaml:"unresolved"`
	Truncated    []truncation      `yaml:"truncated,omitempty"`
	Dropped      []dropped         `yaml:"dropped,omitempty"`
	Results      []result          `yaml:"results"`
}

// dropped is an edge of the overlay.json of a case that the merge drops.
type dropped struct {
	Site   string `yaml:"site"`
	Callee string `yaml:"callee"`
	Reason string `yaml:"reason"`
}

type truncation struct {
	Kind string `yaml:"kind"`
	Key  string `yaml:"key,omitempty"`
	Site string `yaml:"site"`
}

type noEdge struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type root struct {
	Key  string `yaml:"key"`
	Kind string `yaml:"kind"`
	Rule string `yaml:"rule"`
	Site string `yaml:"site,omitempty"`
}

type edge struct {
	From       string `yaml:"from"`
	To         string `yaml:"to"`
	Kind       string `yaml:"kind"`
	Confidence string `yaml:"confidence"`
	Site       string `yaml:"site"`
}

type unresolved struct {
	Caller string `yaml:"caller"`
	Site   string `yaml:"site"`
	Reason string `yaml:"reason"`
	Method string `yaml:"method,omitempty"`
}

type result struct {
	Target        string        `yaml:"target"`
	IncludeTests  bool          `yaml:"include_tests,omitempty"`
	Status        string        `yaml:"status"`
	Path          []hop         `yaml:"path,omitempty"`
	TestPath      []hop         `yaml:"test_path,omitempty"`
	CallSites     []callSite    `yaml:"call_sites,omitempty"`
	Uncertainties []uncertainty `yaml:"uncertainties,omitempty"`
	SetAside      []uncertainty `yaml:"set_aside,omitempty"`
}

// hop has a resolver only for an llm edge, so a case checks the model and
// the prompt of the hop.
type hop struct {
	Key        string `yaml:"key"`
	Site       string `yaml:"site,omitempty"`
	Kind       string `yaml:"kind,omitempty"`
	Confidence string `yaml:"confidence,omitempty"`
	Resolver   string `yaml:"resolver,omitempty"`
}

type callSite struct {
	Caller  string `yaml:"caller"`
	Site    string `yaml:"site"`
	Reached bool   `yaml:"reached"`
}

type uncertainty struct {
	Kind string `yaml:"kind"`
	Site string `yaml:"site,omitempty"`
}

func TestCases(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("cases", "*", "*"))
	require.NoError(t, err)
	require.NotEmpty(t, dirs)

	languages := caseLanguages(t)
	for _, dir := range dirs {
		name := filepath.ToSlash(strings.TrimPrefix(dir, "cases"+string(filepath.Separator)))
		t.Run(name, func(t *testing.T) {
			runCase(t, dir, languages)
		})
	}
}

func readExpectation(t testing.TB, dir string) expectation {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "expect.yaml"))
	require.NoError(t, err)
	var want expectation
	require.NoError(t, yaml.Unmarshal(data, &want))
	require.NotEmpty(t, want.Source, "expect.yaml names the source of its labels")
	return want
}

func runCase(t *testing.T, dir string, languages []core.Language) {
	want := readExpectation(t, dir)

	project := filepath.Join(dir, "project")
	cfg, roots := fileSystemConfig(project, want)
	fsys, err := fs.NewLocalFileSystem(cfg)
	require.NoError(t, err)
	ctx := context.Background()
	indexed := indexProject(t, fsys, indexer.Options{Languages: languages, Store: memstore.New(), MaxFileBytes: want.MaxFileBytes, Imports: len(want.Units) > 0})
	var failed []string
	for _, f := range indexed.Report.Failed {
		failed = append(failed, f.File)
	}
	require.Equal(t, want.Failed, failed, "failed files")

	g := linkProject(t, project, indexed, projectgraph.Options{TestPaths: want.TestPaths, Units: roots})
	assert.Equal(t, want.Problems, g.Problems, "problems")
	assert.Equal(t, want.Dropped, mergeOverlay(t, filepath.Join(dir, "overlay.json"), g), "dropped overlay edges")

	assert.Equal(t, want.Roots, rootsOf(g, want.Roots), "roots")
	assert.Subset(t, edgesOf(g), want.Edges, "edges")
	for _, e := range g.Edges {
		assert.NotContains(t, want.NoEdges, noEdge{From: e.From, To: e.To}, "edge at %s", site(e.Site))
	}
	assert.Equal(t, want.Unresolved, unresolvedOf(g), "unresolved sites")
	assert.Equal(t, want.Truncated, truncationsOf(g), "truncations")

	var results []reach.Result
	for _, w := range want.Results {
		got, err := reach.Reach(ctx, g, reach.Query{Targets: []string{w.Target}, IncludeTests: w.IncludeTests})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, w, resultOf(w, got[0]), "result for %s", w.Target)
		results = append(results, got...)
	}
	dumpCase(t, dir, g, results)
	for _, unit := range slices.Sorted(maps.Values(want.Units)) {
		checkSummary(t, filepath.Join(dir, "summaries"), unit, indexed, roots)
	}
}

// fileSystemConfig returns the file system of a case, and the root of the
// files of each unit: the import directory that holds the unit.
func fileSystemConfig(project string, want expectation) (fs.LocalFileSystemConfig, map[string]iofs.FS) {
	cfg := fs.LocalFileSystemConfig{AppDirectories: []string{project}, ImportUnits: map[string]string{}}
	importDirs := want.ImportDirs
	if len(importDirs) == 0 {
		importDirs = slices.Sorted(maps.Keys(want.Units))
	}
	for _, d := range importDirs {
		cfg.ImportDirectories = append(cfg.ImportDirectories, filepath.Join(project, d))
	}
	roots := map[string]iofs.FS{}
	for d, unit := range want.Units {
		cfg.ImportUnits[filepath.Join(project, d)] = unit
		for _, imp := range importDirs {
			if d == imp || strings.HasPrefix(d, imp+"/") {
				roots[unit] = os.DirFS(filepath.Join(project, imp))
			}
		}
	}
	return cfg, roots
}

// dumpCase writes the graph and the reach results of a case under the
// directory of the flag -dump.
func dumpCase(t *testing.T, dir string, g *projectgraph.Graph, results []reach.Result) {
	t.Helper()
	if *dumpDir == "" {
		return
	}
	out := filepath.Join(*dumpDir, filepath.ToSlash(strings.TrimPrefix(dir, "cases"+string(filepath.Separator))))
	require.NoError(t, os.MkdirAll(out, 0o755))
	for name, v := range map[string]any{"graph.json": g, "results.json": results} {
		data, err := json.MarshalIndent(v, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(out, name), append(data, '\n'), 0o644))
	}
}

// checkSummary compares the summary of one unit with its golden file. The
// flag -update writes the golden file.
func checkSummary(t *testing.T, dir, unit string, indexed *indexer.Result, roots map[string]iofs.FS) {
	t.Helper()
	indexes, files := indexed.Unit(unit)
	s, err := summary.Summarize(context.Background(), indexes, projectgraph.Options{Files: files, Units: map[string]iofs.FS{unit: roots[unit]}})
	require.NoError(t, err)
	testutil.AssertGolden(t, filepath.Join(dir, strings.NewReplacer(":", "_", "/", "_", "@", "_").Replace(unit)+".json"), *update, s)
}

// mergeOverlay merges the overlay at path, when the case has one, and returns
// the edges that the merge drops.
func mergeOverlay(t *testing.T, path string, g *projectgraph.Graph) []dropped {
	t.Helper()
	f, err := overlay.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)
	report, err := overlay.Merge(g, f)
	require.NoError(t, err)
	var out []dropped
	for _, d := range report.Diagnostics {
		out = append(out, dropped{Site: site(d.Edge.Site), Callee: d.Edge.Callee, Reason: string(d.Reason)})
	}
	return out
}

func site(s projectgraph.Site) string {
	if s.Line == 0 {
		return s.Path
	}
	return fmt.Sprintf("%s:%d", s.Path, s.Line)
}

// rootsOf returns the roots of the graph. A root has its site only when the
// expectation names a site for it.
func rootsOf(g *projectgraph.Graph, want []root) []root {
	sited := map[root]bool{}
	for _, w := range want {
		if w.Site != "" {
			sited[root{Key: w.Key, Kind: w.Kind, Rule: w.Rule}] = true
		}
	}
	out := []root{}
	for _, r := range g.Roots {
		got := root{Key: r.Key, Kind: string(r.Kind), Rule: r.Rule}
		if sited[got] {
			got.Site = site(r.Site)
		}
		out = append(out, got)
	}
	return out
}

func edgesOf(g *projectgraph.Graph) []edge {
	out := make([]edge, 0, len(g.Edges))
	for _, e := range g.Edges {
		out = append(out, edge{From: e.From, To: e.To, Kind: string(e.Kind), Confidence: string(e.Confidence), Site: site(e.Site)})
	}
	return out
}

func unresolvedOf(g *projectgraph.Graph) []unresolved {
	out := []unresolved{}
	for _, u := range g.Unresolved {
		out = append(out, unresolved{Caller: u.Caller, Site: site(u.Site), Reason: string(u.Reason), Method: u.Method})
	}
	return out
}

func truncationsOf(g *projectgraph.Graph) []truncation {
	var out []truncation
	for _, t := range g.Truncated {
		out = append(out, truncation{Kind: string(t.Kind), Key: t.Key, Site: site(t.Site)})
	}
	return out
}

func resultOf(w result, r reach.Result) result {
	got := result{
		Target:        r.Target,
		IncludeTests:  w.IncludeTests,
		Status:        string(r.Status),
		Path:          hopsOf(r.Path),
		TestPath:      hopsOf(r.TestPath),
		Uncertainties: uncertaintiesOf(r.Uncertainties),
		SetAside:      uncertaintiesOf(r.SetAside),
	}
	for _, c := range r.CallSites {
		got.CallSites = append(got.CallSites, callSite{Caller: c.Caller, Site: site(c.Site), Reached: c.Reached})
	}
	return got
}

func hopsOf(hops []reach.Hop) []hop {
	var out []hop
	for _, h := range hops {
		got := hop{Key: h.Key, Site: site(h.Site), Kind: string(h.EdgeKind), Confidence: string(h.Confidence)}
		if h.EdgeKind == projectgraph.EdgeLLM {
			got.Resolver = h.Resolver
		}
		out = append(out, got)
	}
	return out
}

func uncertaintiesOf(us []reach.Uncertainty) []uncertainty {
	var out []uncertainty
	for _, u := range us {
		out = append(out, uncertainty{Kind: string(u.Kind), Site: site(u.Site)})
	}
	return out
}

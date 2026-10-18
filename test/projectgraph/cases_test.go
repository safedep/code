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
	"github.com/safedep/code/projectgraph/summary/summarytest"
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
// subset of the edges of the graph, or the whole set with ExactEdges, and
// no edge of the graph goes from a key of NoEdges to its to key. Units
// names the package unit of each
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
	ExactEdges   bool              `yaml:"exact_edges,omitempty"`
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

// result is one reach query of a case and its result. Sources are the
// sources of the query, as keys, and SourcesOnly its mode. Error is the
// text of the error that the query must give, in place of a result.
type result struct {
	Target        string        `yaml:"target"`
	Sources       []string      `yaml:"sources,omitempty"`
	SourcesOnly   bool          `yaml:"sources_only,omitempty"`
	IncludeTests  bool          `yaml:"include_tests,omitempty"`
	Error         string        `yaml:"error,omitempty"`
	Status        string        `yaml:"status,omitempty"`
	Path          []hop         `yaml:"path,omitempty"`
	TestPath      []hop         `yaml:"test_path,omitempty"`
	CallSites     []callSite    `yaml:"call_sites,omitempty"`
	Uncertainties []uncertainty `yaml:"uncertainties,omitempty"`
	SetAside      []uncertainty `yaml:"set_aside,omitempty"`
}

// hop has a resolver only for an llm edge, so a case checks the model and
// the prompt of the hop, and a root only when the case names one on the
// first hop, so a case checks who chose the start of the path.
type hop struct {
	Key        string `yaml:"key"`
	Root       string `yaml:"root,omitempty"`
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

// caseGraph indexes and links the project of a case. It returns the
// expectation, the result of the index step, the graph and the root of
// each unit.
func caseGraph(t testing.TB, dir string, languages []core.Language) (expectation, *indexer.Result, *projectgraph.Graph, map[string]iofs.FS) {
	t.Helper()
	want := readExpectation(t, dir)
	project := filepath.Join(dir, "project")
	cfg, roots := fileSystemConfig(project, want)
	fsys, err := fs.NewLocalFileSystem(cfg)
	require.NoError(t, err)
	indexed := indexProject(t, fsys, indexer.Options{Languages: languages, Store: memstore.New(), MaxFileBytes: want.MaxFileBytes, Imports: len(want.Units) > 0})
	g := linkProject(t, project, indexed, projectgraph.Options{TestPaths: want.TestPaths, Units: roots})
	return want, indexed, g, roots
}

func runCase(t *testing.T, dir string, languages []core.Language) {
	want, indexed, g, roots := caseGraph(t, dir, languages)
	var failed []string
	for _, f := range indexed.Report.Failed {
		failed = append(failed, f.File)
	}
	require.Equal(t, want.Failed, failed, "failed files")
	assert.Equal(t, want.Problems, g.Problems, "problems")
	assert.Equal(t, want.Dropped, mergeOverlay(t, filepath.Join(dir, "overlay.json"), g), "dropped overlay edges")

	assert.Equal(t, want.Roots, rootsOf(g, want.Roots), "roots")
	if want.ExactEdges {
		assert.ElementsMatch(t, want.Edges, edgesOf(g), "edges")
	} else {
		assert.Subset(t, edgesOf(g), want.Edges, "edges")
	}
	for _, e := range g.Edges {
		assert.NotContains(t, want.NoEdges, noEdge{From: e.From, To: e.To}, "edge at %s", site(e.Site))
	}
	assert.Equal(t, want.Unresolved, unresolvedOf(g), "unresolved sites")
	assert.Equal(t, want.Truncated, truncationsOf(g), "truncations")

	results := caseResults(t, g, want)
	for i, w := range want.Results {
		if w.Error != "" {
			assert.ErrorContains(t, results[i].err, w.Error, "query for %s", w.Target)
			continue
		}
		require.NoError(t, results[i].err, "query for %s", w.Target)
		assert.Equal(t, w, resultOf(w, results[i].Result), "result for %s", w.Target)
	}
	dumpCase(t, dir, g, results)
	for _, unit := range slices.Sorted(maps.Values(want.Units)) {
		checkSummary(t, filepath.Join(dir, "summaries"), unit, indexed, roots)
	}
}

// caseResult is the result of one query of a case, or its error. The
// JSON form is the form of the result, so the dump and the hash of a case
// hold the results alone.
type caseResult struct {
	reach.Result
	err error
}

// caseResults runs the reach query of each result of the expectation, one
// target per query, and returns the results in the order of the
// expectation.
func caseResults(t testing.TB, g *projectgraph.Graph, want expectation) []caseResult {
	t.Helper()
	var results []caseResult
	for _, w := range want.Results {
		sources, err := reach.SourcesByKey(w.Sources...)
		require.NoError(t, err)
		q := reach.Query{Targets: targetsOf(t, w.Target), Sources: sources, IncludeTests: w.IncludeTests}
		if w.SourcesOnly {
			q.SourceMode = reach.SourcesOnly
		}
		got, err := reach.Reach(context.Background(), g, q)
		if err != nil {
			results = append(results, caseResult{err: err})
			continue
		}
		require.Len(t, got, 1)
		results = append(results, caseResult{Result: got[0]})
	}
	return results
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
func dumpCase(t *testing.T, dir string, g *projectgraph.Graph, results []caseResult) {
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
	r, err := summary.Build(context.Background(), indexes, projectgraph.Options{Files: files, Units: map[string]iofs.FS{unit: roots[unit]}})
	require.NoError(t, err)
	testutil.AssertGolden(t, filepath.Join(dir, strings.NewReplacer(":", "_", "/", "_", "@", "_").Replace(unit)+".json"), *update, r.Summary)
	summarytest.AssertStatusMatchesReach(t, r)
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
		Target:        r.Target.Key(),
		Sources:       w.Sources,
		SourcesOnly:   w.SourcesOnly,
		IncludeTests:  w.IncludeTests,
		Status:        string(r.Status),
		Path:          hopsOf(w.Path, r.Path),
		TestPath:      hopsOf(w.TestPath, r.TestPath),
		Uncertainties: uncertaintiesOf(r.Uncertainties),
		SetAside:      uncertaintiesOf(r.SetAside),
	}
	for _, c := range r.CallSites {
		got.CallSites = append(got.CallSites, callSite{Caller: c.Caller, Site: site(c.Site), Reached: c.Reached})
	}
	return got
}

// hopsOf returns the hops of a path. The first hop has its root kind
// only when the expectation names one.
func hopsOf(want []hop, hops []reach.Hop) []hop {
	var out []hop
	for i, h := range hops {
		got := hop{Key: h.Key, Site: site(h.Site), Kind: string(h.EdgeKind), Confidence: string(h.Confidence)}
		if h.EdgeKind == projectgraph.EdgeLLM {
			got.Resolver = h.Resolver
		}
		if i == 0 && len(want) > 0 && want[0].Root != "" {
			got.Root = string(h.Root)
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

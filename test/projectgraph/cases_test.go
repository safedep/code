// Package projectgraph runs the hand-written cases end to end: the source
// files of each case go through the indexer, the link step and the reach
// query, and the result must match the expect.yaml of the case.
package projectgraph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/plugin/fileindex"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/safedep/code/projectgraph/store/memstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// expectation is the expect.yaml of a case. A site is path:line. Roots,
// unresolved sites and the fields of a result are the whole set. Edges are a
// subset of the edges of the graph, and no edge of the graph goes from a
// key of NoEdges to its to key.
type expectation struct {
	Source       string       `yaml:"source"`
	Description  string       `yaml:"description"`
	TestPaths    []string     `yaml:"test_paths,omitempty"`
	MaxFileBytes int          `yaml:"max_file_bytes,omitempty"`
	Failed       []string     `yaml:"failed,omitempty"`
	Roots        []root       `yaml:"roots"`
	Edges        []edge       `yaml:"edges"`
	NoEdges      []noEdge     `yaml:"no_edges,omitempty"`
	Unresolved   []unresolved `yaml:"unresolved"`
	Truncated    []truncation `yaml:"truncated,omitempty"`
	Results      []result     `yaml:"results"`
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

type hop struct {
	Key        string `yaml:"key"`
	Site       string `yaml:"site,omitempty"`
	Kind       string `yaml:"kind,omitempty"`
	Confidence string `yaml:"confidence,omitempty"`
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

var languageCodes = []core.LanguageCode{core.LanguageCodePython, core.LanguageCodeJavascript, core.LanguageCodeTypescript, core.LanguageCodeGo, core.LanguageCodeJava}

func TestCases(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("cases", "*", "*"))
	require.NoError(t, err)
	require.NotEmpty(t, dirs)

	languages := make([]core.Language, 0, len(languageCodes))
	for _, code := range languageCodes {
		language, err := lang.GetLanguage(string(code))
		require.NoError(t, err)
		languages = append(languages, language)
	}

	for _, dir := range dirs {
		name := filepath.ToSlash(strings.TrimPrefix(dir, "cases"+string(filepath.Separator)))
		t.Run(name, func(t *testing.T) {
			runCase(t, dir, languages)
		})
	}
}

func runCase(t *testing.T, dir string, languages []core.Language) {
	data, err := os.ReadFile(filepath.Join(dir, "expect.yaml"))
	require.NoError(t, err)
	var want expectation
	require.NoError(t, yaml.Unmarshal(data, &want))
	require.NotEmpty(t, want.Source, "expect.yaml names the source of its labels")

	project := filepath.Join(dir, "project")
	fsys, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{AppDirectories: []string{project}})
	require.NoError(t, err)
	ctx := context.Background()
	indexed, err := (&fileindex.Indexer{}).Run(ctx, fsys, fileindex.Options{Languages: languages, Store: memstore.New(), MaxFileBytes: want.MaxFileBytes})
	require.NoError(t, err)
	var failed []string
	for _, f := range indexed.Report.Failed {
		failed = append(failed, f.File)
	}
	require.Equal(t, want.Failed, failed, "failed files")

	g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{
		Root:      os.DirFS(project),
		Files:     indexed.Files,
		TestPaths: want.TestPaths,
	})
	require.NoError(t, err)
	assert.Empty(t, g.Problems)

	assert.Equal(t, want.Roots, rootsOf(g, want.Roots), "roots")
	assert.Subset(t, edgesOf(g), want.Edges, "edges")
	for _, e := range g.Edges {
		assert.NotContains(t, want.NoEdges, noEdge{From: e.From, To: e.To}, "edge at %s", site(e.Site))
	}
	assert.Equal(t, want.Unresolved, unresolvedOf(g), "unresolved sites")
	assert.Equal(t, want.Truncated, truncationsOf(g), "truncations")

	for _, w := range want.Results {
		results, err := reach.Reach(ctx, g, reach.Query{Targets: []string{w.Target}, IncludeTests: w.IncludeTests})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, w, resultOf(w, results[0]), "result for %s", w.Target)
	}
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
		out = append(out, hop{Key: h.Key, Site: site(h.Site), Kind: string(h.EdgeKind), Confidence: string(h.Confidence)})
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

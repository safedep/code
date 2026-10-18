package overlay

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dispatchSite = projectgraph.Site{Path: "main.py", Line: 5, Column: 12}

// testGraph is a graph where main.py calls dispatch, and dispatch calls
// w.run() on a receiver that the link step did not resolve. Nothing reaches
// parsers.py//parse, which calls yaml//load.
func testGraph() *projectgraph.Graph {
	nodes := map[string]*projectgraph.Node{}
	for _, n := range []projectgraph.Node{
		{Key: "main.py", Kind: projectgraph.NodeModule, Path: "main.py"},
		{Key: "main.py//dispatch", Kind: projectgraph.NodeFunction, Path: "main.py", Line: 4},
		{Key: "parsers.py", Kind: projectgraph.NodeModule, Path: "parsers.py"},
		{Key: "parsers.py//parse", Kind: projectgraph.NodeFunction, Path: "parsers.py", Line: 5},
		{Key: "yaml//load", Kind: projectgraph.NodeExternal},
	} {
		nodes[n.Key] = &n
	}
	edge := func(from, to string, site projectgraph.Site) projectgraph.Edge {
		return projectgraph.Edge{From: from, To: to, Site: site, Kind: projectgraph.EdgeDirect, Confidence: projectgraph.ConfidenceHigh, Resolver: "python.local"}
	}
	return &projectgraph.Graph{
		Nodes: nodes,
		Edges: []projectgraph.Edge{
			edge("main.py", "main.py//dispatch", projectgraph.Site{Path: "main.py", Line: 9, Column: 5}),
			edge("parsers.py//parse", "yaml//load", projectgraph.Site{Path: "parsers.py", Line: 6, Column: 12}),
		},
		Unresolved: []projectgraph.UnresolvedSite{{
			Caller: "main.py//dispatch", Site: dispatchSite, Text: "w.run", Receiver: "w", Method: "run",
			Reason: fileindex.UnresolvedUnknownReceiver,
		}},
		Roots: []projectgraph.Root{{Key: "main.py", Kind: projectgraph.RootMain, Rule: "python.main-guard"}},
		Files: map[string]fileindex.FileStatus{"main.py": fileindex.FileIndexed, "parsers.py": fileindex.FileIndexed},
	}
}

func llmEdge(callee string) Edge {
	return Edge{
		Caller: "main.py//dispatch", Site: dispatchSite, Callee: callee,
		Kind: projectgraph.EdgeLLM, Confidence: projectgraph.ConfidenceLow,
		Model: "m1", PromptVersion: "v2", InputHash: "sha256:00", Output: callee,
	}
}

func merge(t *testing.T, g *projectgraph.Graph, edges ...Edge) Report {
	t.Helper()
	report, err := Merge(g, &File{Version: Version, Edges: edges})
	require.NoError(t, err)
	return report
}

func TestMergeAddsAnEdge(t *testing.T) {
	for _, callee := range []string{"parsers.py//parse", "yaml//load"} {
		t.Run(callee, func(t *testing.T) {
			g := testGraph()
			unresolved := slices.Clone(g.Unresolved)

			report := merge(t, g, llmEdge(callee))

			assert.Equal(t, Report{Added: 1}, report)
			assert.Contains(t, g.Edges, projectgraph.Edge{
				From: "main.py//dispatch", To: callee, Site: dispatchSite,
				Kind: projectgraph.EdgeLLM, Confidence: projectgraph.ConfidenceLow, Resolver: "model m1, prompt v2",
			})
			assert.Equal(t, 3, g.Stats.Edges)
			assert.Equal(t, unresolved, g.Unresolved, "the unresolved site stays")
		})
	}
}

func TestMergeCountsAnEdgeOnceForItsSlot(t *testing.T) {
	g := testGraph()
	assert.Equal(t, Report{Added: 1}, merge(t, g, llmEdge("parsers.py//parse"), llmEdge("parsers.py//parse")))
	assert.Len(t, g.Edges, 3)
}

func TestMergeDropsAnEdge(t *testing.T) {
	otherCaller := llmEdge("parsers.py//parse")
	otherCaller.Caller = "main.py"
	otherLine := llmEdge("parsers.py//parse")
	otherLine.Site.Line = 9

	tests := []struct {
		name   string
		edge   Edge
		reason Reason
	}{
		{"undefined first-party callee", llmEdge("parsers.py//load"), UndefinedCallee},
		{"external callee not in the graph", llmEdge("requests//get"), UndefinedCallee},
		{"site of a resolved call", otherLine, NoUnresolvedSite},
		{"site under another caller", otherCaller, NoUnresolvedSite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := testGraph()
			want := testGraph()

			report := merge(t, g, tt.edge)

			assert.Equal(t, Report{Diagnostics: []Diagnostic{{Index: 0, Edge: tt.edge, Reason: tt.reason}}}, report)
			assert.Equal(t, want, g, "a dropped edge leaves the graph as it was")
		})
	}
}

func TestReadRejectsABadFile(t *testing.T) {
	file := func(version, kind, confidence string) string {
		return fmt.Sprintf(`{"version": %q, "edges": [{"caller": "a.py//f", "site": {"path": "a.py", "line": 3}, "callee": "a.py//g",
			"kind": %q, "confidence": %q, "model": "m1", "prompt_version": "v2", "input_hash": "h", "output": "g"}]}`, version, kind, confidence)
	}

	f, err := Read(strings.NewReader(file(Version, "llm", "low") + "\n"))
	require.NoError(t, err)
	assert.Equal(t, "model m1, prompt v2", f.Edges[0].Resolver())
	_, err = Read(strings.NewReader(file(Version, "name", "low")))
	require.NoError(t, err, "any kind of low confidence passes")

	tests := []struct {
		name, data, err string
	}{
		{"newer version", file("2", "llm", "low"), `overlay version "2" is not "1"`},
		{"no version", `{"edges": []}`, `overlay version "" is not "1"`},
		{"other kind", file(Version, "direct", "low"), `overlay edge 0: kind "direct" has the confidence high, want low`},
		{"other confidence", file(Version, "llm", "high"), `overlay edge 0: confidence "high" is not low`},
		{"no model", `{"version": "1", "edges": [{"caller": "a.py//f", "site": {"path": "a.py", "line": 3}, "callee": "a.py//g", "kind": "llm", "confidence": "low"}]}`, "no model"},
		{"no site", `{"version": "1", "edges": [{"caller": "a.py//f", "callee": "a.py//g", "kind": "llm", "confidence": "low"}]}`, "no site"},
		{"unknown field", `{"version": "1", "edges": [], "candidates": []}`, "unknown field"},
		{"not JSON", `version: 1`, "failed to decode"},
		{"second object", file(Version, "llm", "low") + file(Version, "llm", "low"), "data follows the overlay object"},
		{"trailing text", file(Version, "llm", "low") + " and more", "data follows the overlay object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Read(strings.NewReader(tt.data))
			assert.ErrorContains(t, err, tt.err)
		})
	}
}

func TestMergeRejectsABadVersion(t *testing.T) {
	g := testGraph()
	_, err := Merge(g, &File{Version: "0", Edges: []Edge{llmEdge("parsers.py//parse")}})
	assert.ErrorContains(t, err, `overlay version "0" is not "1"`)
	assert.Equal(t, testGraph(), g)
}

func reachOne(t *testing.T, g *projectgraph.Graph, target string) reach.Result {
	t.Helper()
	results, err := reach.Reach(context.Background(), g, reach.Query{Targets: []string{target}})
	require.NoError(t, err)
	require.Len(t, results, 1)
	return results[0]
}

func TestMergeTurnsNotFoundIntoFound(t *testing.T) {
	g := testGraph()
	before := reachOne(t, g, "yaml//load")
	require.Equal(t, reach.NotFound, before.Status, "the record of w.run concerns only targets named run")

	merge(t, g, llmEdge("parsers.py//parse"))

	after := reachOne(t, g, "yaml//load")
	assert.Equal(t, reach.Found, after.Status)
	assert.Equal(t, []reach.Hop{
		{Key: "main.py"},
		{Key: "main.py//dispatch", Site: projectgraph.Site{Path: "main.py", Line: 9, Column: 5}, EdgeKind: projectgraph.EdgeDirect, Confidence: projectgraph.ConfidenceHigh, Resolver: "python.local"},
		{Key: "parsers.py//parse", Site: dispatchSite, EdgeKind: projectgraph.EdgeLLM, Confidence: projectgraph.ConfidenceLow, Resolver: "model m1, prompt v2"},
		{Key: "yaml//load", Site: projectgraph.Site{Path: "parsers.py", Line: 6, Column: 12}, EdgeKind: projectgraph.EdgeDirect, Confidence: projectgraph.ConfidenceHigh, Resolver: "python.local"},
	}, after.Path)
}

func TestMergeKeepsAnUncertainTarget(t *testing.T) {
	g := testGraph()
	before := reachOne(t, g, "subprocess//run")
	require.Equal(t, reach.Uncertain, before.Status)

	merge(t, g, llmEdge("parsers.py//parse"))

	after := reachOne(t, g, "subprocess//run")
	assert.Equal(t, reach.Uncertain, after.Status, "the unresolved site stays, so the target is not proved dead")
	assert.Equal(t, before.Uncertainties, after.Uncertainties)
}

func TestDiagnosticString(t *testing.T) {
	d := Diagnostic{Index: 2, Edge: llmEdge("parsers.py//load"), Reason: UndefinedCallee}
	assert.Equal(t, "overlay edge 2 at main.py:5 from main.py//dispatch to parsers.py//load: undefined-callee", d.String())
}

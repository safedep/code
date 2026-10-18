package projectgraph

import (
	"context"
	"slices"
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphAddEdgeKeepsTheBestEdgeOfASlot(t *testing.T) {
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: "a.py", Language: langcode.Python,
		Definitions: []fileindex.Definition{def(fileindex.DefFunction, "a.py//run", 2), def(fileindex.DefFunction, "a.py//go", 5)},
		Calls:       []fileindex.Call{{Caller: "a.py//run", Line: 3, Column: 5, Callee: "go", Targets: []string{"a.py//go"}, Kind: fileindex.CallCall}},
		Unresolved:  []fileindex.Unresolved{{Caller: "a.py//run", Line: 4, Column: 5, Text: "x.load", Receiver: "x", Method: "load", Reason: fileindex.UnresolvedUnknownReceiver}},
	}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{})
	require.NoError(t, err)
	unresolved := slices.Clone(g.Unresolved)

	direct := Site{Path: "a.py", Line: 3, Column: 5}
	changed, err := g.AddEdge(Edge{From: "a.py//run", To: "a.py//go", Site: direct, Kind: EdgeLLM, Resolver: "llm"})
	require.NoError(t, err)
	assert.False(t, changed, "a low edge does not replace the high edge of the slot")

	site := Site{Path: "a.py", Line: 4, Column: 5}
	changed, err = g.AddEdge(Edge{From: "a.py//run", To: "yaml//load", Site: site, Kind: EdgeLLM, Resolver: "llm"})
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, NodeExternal, g.Nodes["yaml//load"].Kind)
	changed, err = g.AddEdge(Edge{From: "a.py//run", To: "yaml//load", Site: site, Kind: EdgeDirect, Resolver: "python.external"})
	require.NoError(t, err)
	assert.True(t, changed, "a high edge replaces the low edge of the slot")

	assert.Equal(t, []string{"a.py//run -> a.py//go direct python.local", "a.py//run -> yaml//load direct python.external"}, edgeStrings(g))
	assert.True(t, slices.IsSortedFunc(g.Edges, compareSlots))
	assert.Equal(t, len(g.Edges), g.Stats.Edges)
	assert.Equal(t, unresolved, g.Unresolved, "an edge never removes an unresolved site")

	_, err = g.AddEdge(Edge{From: "a.py//missing", To: "a.py//go", Site: site, Kind: EdgeLLM})
	assert.ErrorContains(t, err, "no such node")
	_, err = g.AddEdge(Edge{From: "a.py//run", To: "a.py//missing", Site: site, Kind: EdgeLLM})
	assert.ErrorContains(t, err, "no such node", "a first-party callee needs a node")
}

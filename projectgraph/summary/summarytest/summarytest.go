// Package summarytest checks a summary against the reach query of its
// unit graph.
package summarytest

import (
	"cmp"
	"context"
	"slices"
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/safedep/code/projectgraph/summary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusOf asks the unit graph from one symbol for the status of every
// target, as a reach query of the project would with the symbol as its
// one root. The depth cap is far above the depth of the graph, because a
// summary has none.
func statusOf(t testing.TB, g *projectgraph.Graph, symbol string, targets []reach.Target) map[string]reach.Status {
	t.Helper()
	from := *g
	from.Roots = []projectgraph.Root{{Key: symbol, Kind: projectgraph.RootMain, Rule: "test"}}
	from.Stats.RootFallback = false
	results, err := reach.Reach(context.Background(), &from, reach.Query{Targets: targets, MaxDepth: 1 << 20})
	require.NoError(t, err)
	out := map[string]reach.Status{}
	for _, r := range results {
		out[r.Target.Key()] = r.Status
	}
	return out
}

// AssertStatusMatchesReach checks that the summary answers found, not
// found or uncertain for every symbol and every target as a reach query
// of the unit graph from the symbol does. The targets are the ones of the
// summary, a wildcard under the module of the first one, a key that no
// node has, and a key and a wildcard that only the method rule concerns.
func AssertStatusMatchesReach(t testing.TB, r *summary.Result) {
	t.Helper()
	s := r.Summary
	require.NotEmpty(t, s.Symbols)
	keys := append(slices.Clone(s.Targets), "nope//missing", "fs//"+fileindex.Last(cmp.Or(firstMethod(s), "x")), "fs//*")
	if len(s.Targets) > 0 {
		keys = append(keys, fileindex.Unit(s.Targets[0])+"//*")
	}
	targets, err := reach.TargetsByKey(keys...)
	require.NoError(t, err)
	for _, sym := range s.Symbols {
		want := statusOf(t, r.Graph(), sym.Key, targets)
		for _, target := range targets {
			assert.Equal(t, want[target.Key()], s.Status(sym.Key, target), "%s -> %s", sym.Key, target)
		}
	}
}

// firstMethod returns the method of the first uncertainty with one, so the
// check has a target that the method rule concerns and the graph does not
// hold.
func firstMethod(s *summary.Summary) string {
	for _, u := range s.Uncertainties {
		if u.Method != "" {
			return u.Method
		}
	}
	return ""
}

package reach

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// graphOf builds a graph from lines of a small text form:
//
//	root <key> [kind]
//	edge <from> <to> [kind] [line]
//	unresolved <caller> <line> <reason> [method] [prefix]
//	entry <external key or -> <unit>
//	flag <key> <kind> <line>
//	file <path> <status>
//	truncated <kind> [key]
//	fallback
//
// A key whose unit ends in .py or .js is first party, and any other key is
// external. A first-party key under tests/ is a test node.
func graphOf(t *testing.T, spec string) *projectgraph.Graph {
	t.Helper()
	g := &projectgraph.Graph{Nodes: map[string]*projectgraph.Node{}, Files: map[string]fileindex.FileStatus{}}
	node := func(key string) *projectgraph.Node {
		if n := g.Nodes[key]; n != nil {
			return n
		}
		n := &projectgraph.Node{Key: key, Kind: projectgraph.NodeExternal}
		if unit := fileindex.Unit(key); strings.HasSuffix(unit, ".py") || strings.HasSuffix(unit, ".js") {
			n.Kind, n.Path, n.Line, n.Test = projectgraph.NodeFunction, unit, 1, strings.HasPrefix(unit, "tests/")
			if _, ok := g.Files[unit]; !ok {
				g.Files[unit] = fileindex.FileIndexed
			}
		}
		g.Nodes[key] = n
		return n
	}
	arg := func(f []string, i int, def string) string {
		if i < len(f) {
			return f[i]
		}
		return def
	}
	for _, line := range strings.Split(spec, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "root":
			n := node(f[1])
			g.Roots = append(g.Roots, projectgraph.Root{Key: f[1], Kind: projectgraph.RootKind(arg(f, 2, "main")), Rule: "test.rule", Site: projectgraph.Site{Path: n.Path, Line: n.Line}})
		case "edge":
			from, to := node(f[1]), node(f[2])
			kind := projectgraph.EdgeKind(arg(f, 3, string(projectgraph.EdgeDirect)))
			l, err := strconv.Atoi(arg(f, 4, "1"))
			require.NoError(t, err)
			g.Edges = append(g.Edges, projectgraph.Edge{
				From: from.Key, To: to.Key, Site: projectgraph.Site{Path: from.Path, Line: l},
				Kind: kind, Confidence: kind.Confidence(), Resolver: "test",
			})
		case "unresolved":
			n := node(f[1])
			l, err := strconv.Atoi(f[2])
			require.NoError(t, err)
			g.Unresolved = append(g.Unresolved, projectgraph.UnresolvedSite{
				Caller: f[1], Site: projectgraph.Site{Path: n.Path, Line: l}, Text: "call",
				Reason: fileindex.UnresolvedReason(f[3]), Method: strings.Trim(arg(f, 4, ""), "-"), ImportPrefix: arg(f, 5, ""),
			})
		case "entry":
			caller := strings.Trim(f[1], "-")
			if caller != "" {
				node(caller)
			}
			g.Unresolved = append(g.Unresolved, projectgraph.UnresolvedSite{
				Caller: caller, Site: projectgraph.Site{Path: f[2]}, Text: f[1], Reason: fileindex.UnresolvedNoDefinition, Unit: f[2],
			})
		case "flag":
			n := node(f[1])
			l, err := strconv.Atoi(f[3])
			require.NoError(t, err)
			n.Flags = append(n.Flags, fileindex.Flag{Kind: fileindex.FlagKind(f[2]), Line: l})
		case "file":
			g.Files[f[1]] = fileindex.FileStatus(f[2])
		case "truncated":
			g.Truncated = append(g.Truncated, projectgraph.Truncation{Kind: projectgraph.TruncationKind(f[1]), Key: arg(f, 2, "")})
		case "fallback":
			g.Stats.RootFallback = true
		default:
			t.Fatalf("unknown line %q", line)
		}
	}
	return g
}

func reachOne(t *testing.T, g *projectgraph.Graph, q Query) Result {
	t.Helper()
	results, err := Reach(context.Background(), g, q)
	require.NoError(t, err)
	require.Len(t, results, 1)
	return results[0]
}

func pathKeys(hops []Hop) []string {
	keys := make([]string, 0, len(hops))
	for _, h := range hops {
		keys = append(keys, h.Key)
	}
	return keys
}

func kinds(us []Uncertainty) []UncertaintyKind {
	out := []UncertaintyKind{}
	for _, u := range us {
		out = append(out, u.Kind)
	}
	return out
}

const basic = `
root main.py
edge main.py handlers.py//parse direct 3
edge handlers.py//parse yaml//load direct 12
edge handlers.py//dead yaml//load direct 40
`

func TestReachFound(t *testing.T) {
	r := reachOne(t, graphOf(t, basic), Query{Targets: []string{"yaml//load"}})
	assert.Equal(t, Found, r.Status)
	assert.Equal(t, []Hop{
		{Key: "main.py"},
		{Key: "handlers.py//parse", Site: projectgraph.Site{Path: "main.py", Line: 3}, EdgeKind: projectgraph.EdgeDirect, Confidence: projectgraph.ConfidenceHigh, Resolver: "test"},
		{Key: "yaml//load", Site: projectgraph.Site{Path: "handlers.py", Line: 12}, EdgeKind: projectgraph.EdgeDirect, Confidence: projectgraph.ConfidenceHigh, Resolver: "test"},
	}, r.Path)
	assert.Equal(t, []CallSite{
		{Caller: "handlers.py//parse", Site: projectgraph.Site{Path: "handlers.py", Line: 12}, Reached: true},
		{Caller: "handlers.py//dead", Site: projectgraph.Site{Path: "handlers.py", Line: 40}},
	}, r.CallSites)
	assert.Empty(t, r.Uncertainties)
	assert.Len(t, r.Searched.Roots, 1)
}

func TestReachNotFound(t *testing.T) {
	g := graphOf(t, basic+"\nedge other.py//f os//system direct 2\n")
	results, err := Reach(context.Background(), g, Query{Targets: []string{"os//system", "lodash//template"}})
	require.NoError(t, err)

	dead := results[0]
	assert.Equal(t, NotFound, dead.Status)
	assert.Empty(t, dead.Uncertainties)
	assert.Equal(t, []CallSite{{Caller: "other.py//f", Site: projectgraph.Site{Path: "other.py", Line: 2}}}, dead.CallSites)
	assert.Equal(t, 3, dead.Searched.Nodes)

	absent := results[1]
	assert.Equal(t, NoSuchTarget, absent.Status)
	assert.Empty(t, absent.CallSites)
	assert.NotNil(t, absent.CallSites)
}

func TestReachUncertainties(t *testing.T) {
	tests := []struct {
		name     string
		extra    string
		target   string
		status   Status
		kinds    []UncertaintyKind
		setAside int
	}{
		{name: "a record of the same method", extra: "unresolved handlers.py//parse 5 unknown-receiver load", target: "yaml//dump", status: NotFound, kinds: []UncertaintyKind{}},
		{name: "a record of the target method", extra: "unresolved handlers.py//parse 5 unknown-receiver dump", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{UnresolvedCall}},
		{name: "a record of the method of an unreached definition that reaches the target", extra: "unresolved handlers.py//parse 5 unknown-receiver f", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{UnresolvedCall}},
		{name: "a record of the method of an unreached definition that does not reach the target", extra: "unresolved handlers.py//parse 5 unknown-receiver f", target: "yaml//safe_load", status: NoSuchTarget, kinds: []UncertaintyKind{}},
		{name: "a record of the method of an external key that leads to the target", extra: "unresolved handlers.py//parse 5 unknown-receiver run\nedge kit//run npm:kit@1.0.0//index.js//run package\nedge npm:kit@1.0.0//index.js//run yaml//dump", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{UnresolvedCall}},
		{name: "a computed name", extra: "unresolved handlers.py//parse 5 computed-name", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{UnresolvedCall}},
		{name: "a computed name with a method", extra: "unresolved handlers.py//parse 5 computed-name getattr", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{UnresolvedCall}},
		{name: "a computed import by a named function", extra: "unresolved handlers.py//parse 5 dynamic-import import_module", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{UnresolvedCall}},
		{name: "a record on an unreached node", extra: "unresolved handlers.py//dead 41 computed-name", target: "yaml//dump", status: NotFound, kinds: []UncertaintyKind{}},
		{name: "a dynamic flag", extra: "flag handlers.py//parse eval 6", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{DynamicFlag}},
		{name: "a flag on an unreached node", extra: "flag handlers.py//dead eval 41", target: "yaml//dump", status: NotFound, kinds: []UncertaintyKind{}},
		{
			name:   "a computed import with another prefix",
			extra:  "unresolved handlers.py//parse 7 dynamic-import - plugins//\nflag handlers.py//parse computed-import 7",
			target: "yaml//dump", status: NotFound, kinds: []UncertaintyKind{}, setAside: 2,
		},
		{
			name:   "a computed import with the prefix of the target",
			extra:  "unresolved handlers.py//parse 7 dynamic-import - ya\nflag handlers.py//parse computed-import 7",
			target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{DynamicFlag, UnresolvedCall},
		},
		{name: "a missing file", extra: "file handlers.py missing", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{Budget}},
		{name: "a truncated file", extra: "file handlers.py truncated", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{Budget}},
		{name: "a failed file", extra: "file handlers.py failed", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{Budget}},
		{name: "a failed file that no search reaches", extra: "file server.py failed", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{Budget}},
		{name: "a missing file that no node names", extra: "file lib/gen.py missing", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{Budget}},
		{name: "a parse error", extra: "flag handlers.py//parse parse-error 9", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{Budget}},
		{name: "the edge limit", extra: "truncated edges", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{Budget}},
		{name: "a re-export limit on a reached node", extra: "truncated export-hops handlers.py//parse", target: "yaml//dump", status: Uncertain, kinds: []UncertaintyKind{Budget}},
		{name: "a re-export limit on an unreached node", extra: "truncated export-hops handlers.py//dead", target: "yaml//dump", status: NotFound, kinds: []UncertaintyKind{}},
		{name: "the root fallback alone", extra: "fallback", target: "yaml//dump", status: NotFound, kinds: []UncertaintyKind{RootFallback}},
		{name: "the root fallback on a path", extra: "fallback", target: "yaml//load", status: Found, kinds: []UncertaintyKind{RootFallback}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graphOf(t, basic+"edge other.py//f yaml//dump\n"+tt.extra)
			r := reachOne(t, g, Query{Targets: []string{tt.target}})
			assert.Equal(t, tt.status, r.Status)
			assert.Equal(t, tt.kinds, kinds(r.Uncertainties))
			assert.Len(t, r.SetAside, tt.setAside)
			for _, u := range r.Uncertainties {
				if u.Kind != RootFallback && u.Key != "" {
					assert.NotEmpty(t, u.Site.Path, "an uncertainty names its site")
				}
			}
		})
	}
}

func TestReachMethodClosureSkipsTests(t *testing.T) {
	g := graphOf(t, basic+`
root tests/test_app.py test
edge tests/test_app.py//f yaml//dump
unresolved handlers.py//parse 5 unknown-receiver f
`)
	r := reachOne(t, g, Query{Targets: []string{"yaml//dump"}})
	assert.Equal(t, NotFound, r.Status)
	r = reachOne(t, g, Query{Targets: []string{"yaml//dump"}, IncludeTests: true})
	assert.Equal(t, Uncertain, r.Status)
}

func TestReachCapsUncertaintiesPerKind(t *testing.T) {
	spec := basic
	for i := range 5 {
		spec += fmt.Sprintf("unresolved handlers.py//parse %d computed-name\nflag handlers.py//parse eval %d\n", 20+i, 20+i)
	}
	r := reachOne(t, graphOf(t, spec), Query{Targets: []string{"yaml//dump"}})
	assert.Equal(t, []UncertaintyKind{DynamicFlag, DynamicFlag, DynamicFlag, UnresolvedCall, UnresolvedCall, UnresolvedCall}, kinds(r.Uncertainties))
	assert.Equal(t, 10, r.Searched.Uncertainties)
}

func TestReachTestRoots(t *testing.T) {
	g := graphOf(t, `
root main.py
root tests/test_app.py test
edge tests/test_app.py handlers.py//parse direct 2
edge handlers.py//parse yaml//load direct 12
`)
	r := reachOne(t, g, Query{Targets: []string{"yaml//load"}})
	assert.Equal(t, NotFound, r.Status)
	assert.Empty(t, r.Path)
	assert.Equal(t, []string{"tests/test_app.py", "handlers.py//parse", "yaml//load"}, pathKeys(r.TestPath))
	assert.Len(t, r.Searched.Roots, 1)

	r = reachOne(t, g, Query{Targets: []string{"yaml//load"}, IncludeTests: true})
	assert.Equal(t, Found, r.Status)
	assert.Empty(t, r.TestPath)
	assert.Len(t, r.Searched.Roots, 2)
}

func TestReachWildcard(t *testing.T) {
	g := graphOf(t, `
root main.py
edge main.py a.py//f direct 1
edge a.py//f yaml//load direct 2
edge main.py yaml//safe_load direct 3
edge main.py yamlish//x direct 4
unresolved main.py 5 unknown-receiver dump
`)
	r := reachOne(t, g, Query{Targets: []string{"yaml//*"}})
	assert.Equal(t, Found, r.Status)
	assert.Equal(t, []string{"main.py", "yaml//safe_load"}, pathKeys(r.Path))
	assert.Len(t, r.CallSites, 2)

	r = reachOne(t, g, Query{Targets: []string{"lodash//*"}})
	assert.Equal(t, Uncertain, r.Status, "a method record can name any key under a wildcard")
	assert.Empty(t, r.CallSites)
}

func TestReachDepth(t *testing.T) {
	g := graphOf(t, `
root a.py
edge a.py b.py//f
edge b.py//f c.py//f
edge c.py//f d.py//f
edge d.py//f yaml//load
`)
	r := reachOne(t, g, Query{Targets: []string{"yaml//load"}, MaxDepth: 3})
	assert.Equal(t, Uncertain, r.Status)
	assert.True(t, r.Searched.DepthCut)
	assert.Equal(t, []UncertaintyKind{Budget}, kinds(r.Uncertainties))

	r = reachOne(t, g, Query{Targets: []string{"yaml//load"}, MaxDepth: 4})
	assert.Equal(t, Found, r.Status)

	r = reachOne(t, g, Query{Targets: []string{"yaml//load"}})
	assert.Equal(t, Found, r.Status, "depth 0 selects 64")
}

func TestReachTieRule(t *testing.T) {
	g := graphOf(t, `
root main.py
edge main.py a.py//guess field 1
edge main.py b.py//known direct 2
edge a.py//guess yaml//load direct 3
edge b.py//known yaml//load direct 4
`)
	r := reachOne(t, g, Query{Targets: []string{"yaml//load"}})
	assert.Equal(t, []string{"main.py", "b.py//known", "yaml//load"}, pathKeys(r.Path))

	g = graphOf(t, `
root main.py
edge main.py a.py//guess field 1
edge a.py//guess yaml//load direct 3
edge main.py b.py//one direct 2
edge b.py//one c.py//two direct 2
edge c.py//two yaml//load direct 4
`)
	r = reachOne(t, g, Query{Targets: []string{"yaml//load"}})
	assert.Equal(t, []string{"main.py", "a.py//guess", "yaml//load"}, pathKeys(r.Path), "a shorter path wins over a surer one")
	assert.Equal(t, projectgraph.ConfidenceLow, r.Path[1].Confidence)
}

func TestReachQuery(t *testing.T) {
	g := graphOf(t, basic)
	results, err := Reach(context.Background(), g, Query{Targets: []string{"lodash//x", "yaml//load"}})
	require.NoError(t, err)
	assert.Equal(t, "lodash//x", results[0].Target)
	assert.Equal(t, "yaml//load", results[1].Target)

	_, err = Reach(context.Background(), g, Query{Targets: []string{""}})
	assert.Error(t, err)
	_, err = Reach(context.Background(), g, Query{Targets: []string{"x"}, MaxDepth: -1})
	assert.Error(t, err)
}

func TestReachDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err := Reach(ctx, graphOf(t, basic), Query{Targets: []string{"yaml//load"}})
	require.NoError(t, err)
	assert.Equal(t, Uncertain, results[0].Status)
	assert.True(t, results[0].Searched.Stopped)
	assert.Equal(t, []UncertaintyKind{Budget}, kinds(results[0].Uncertainties))
}

// TestReachBuiltGraph runs the acceptance case of the design on a graph that
// projectgraph.Build links: main.py calls handlers.parse, which calls
// yaml.load.
func TestReachBuiltGraph(t *testing.T) {
	indexes := []*fileindex.Index{
		{
			Version: fileindex.Version, Path: "main.py", Language: langcode.Python, MainGuard: true,
			Imports: []fileindex.Import{{Line: 1, Module: "handlers", Alias: "handlers", Kind: fileindex.ImportStatic, Namespace: "handlers"}},
			Calls:   []fileindex.Call{{Caller: "main.py", Line: 3, Column: 1, Callee: "handlers.parse", Targets: []string{"handlers//parse"}, Receiver: "handlers", Method: "parse", Kind: fileindex.CallCall}},
		},
		{
			Version: fileindex.Version, Path: "handlers.py", Language: langcode.Python,
			Imports:     []fileindex.Import{{Line: 1, Module: "yaml", Alias: "yaml", Kind: fileindex.ImportStatic, Namespace: "yaml"}},
			Definitions: []fileindex.Definition{{Key: "handlers.py//parse", Kind: fileindex.DefFunction, Name: "parse", StartLine: 11, EndLine: 12, Exported: true}},
			Calls:       []fileindex.Call{{Caller: "handlers.py//parse", Line: 12, Column: 5, Callee: "yaml.load", Targets: []string{"yaml//load"}, Receiver: "yaml", Method: "load", Kind: fileindex.CallCall}},
		},
	}
	g, err := projectgraph.Build(context.Background(), indexes, projectgraph.Options{})
	require.NoError(t, err)

	r := reachOne(t, g, Query{Targets: []string{"yaml//load"}})
	require.Equal(t, Found, r.Status)
	var sites []string
	for _, h := range r.Path[1:] {
		sites = append(sites, fmt.Sprintf("%s:%d", h.Site.Path, h.Site.Line))
	}
	assert.Equal(t, []string{"main.py:3", "handlers.py:12"}, sites)
	assert.Equal(t, "yaml//load", r.Path[len(r.Path)-1].Key)
	assert.Equal(t, projectgraph.RootMain, r.Searched.Roots[0].Kind)
}

// largeGraph returns a graph of n first-party nodes in layers of fan-out
// edges, with one external target per 500 nodes.
func largeGraph(n int) (*projectgraph.Graph, []string) {
	g := &projectgraph.Graph{Nodes: make(map[string]*projectgraph.Node, n+n/500), Files: map[string]fileindex.FileStatus{}}
	key := func(i int) string { return fmt.Sprintf("src/m%d.py//f%d", i/100, i) }
	for i := range n {
		k := key(i)
		g.Nodes[k] = &projectgraph.Node{Key: k, Kind: projectgraph.NodeFunction, Path: fileindex.Unit(k)}
		g.Files[fileindex.Unit(k)] = fileindex.FileIndexed
	}
	edge := func(from, to string, line int) {
		g.Edges = append(g.Edges, projectgraph.Edge{
			From: from, To: to, Site: projectgraph.Site{Path: fileindex.Unit(from), Line: line},
			Kind: projectgraph.EdgeDirect, Confidence: projectgraph.ConfidenceHigh,
		})
	}
	for i := 1; i < n; i++ {
		edge(key((i-1)/3), key(i), 1)
		edge(key(i/2), key(i), 2)
	}
	var targets []string
	for i := 0; i < n; i += 500 {
		t := fmt.Sprintf("pkg%d//call", i)
		g.Nodes[t] = &projectgraph.Node{Key: t, Kind: projectgraph.NodeExternal}
		edge(key(n-1-i), t, 3)
		targets = append(targets, t)
		g.Unresolved = append(g.Unresolved, projectgraph.UnresolvedSite{Caller: key(i), Text: "x.y", Method: "y", Reason: fileindex.UnresolvedUnknownReceiver})
	}
	g.Roots = []projectgraph.Root{{Key: key(0), Kind: projectgraph.RootMain}}
	return g, append(targets, "missing//target")
}

func TestReachLargeGraph(t *testing.T) {
	g, targets := largeGraph(100_000)
	require.Len(t, targets, 201)

	start := time.Now()
	results, err := Reach(context.Background(), g, Query{Targets: targets})
	elapsed := time.Since(start)
	require.NoError(t, err)

	for _, r := range results[:200] {
		assert.Equal(t, Found, r.Status, r.Target)
	}
	assert.Equal(t, NoSuchTarget, results[200].Status)
	assert.Equal(t, 100_200, results[200].Searched.Nodes)
	if !raceEnabled {
		assert.Less(t, elapsed, time.Second)
	}
	t.Logf("reach of %d targets over %d nodes and %d edges: %s", len(targets), len(g.Nodes), len(g.Edges), elapsed)
}

const dependencies = `
root main.py//run
edge main.py//run lodash//template direct 3
edge lodash//template npm:lodash@4.17.21//t.js//template package
edge npm:lodash@4.17.21//t.js//template child_process//exec direct 2
edge npm:lodash@4.17.21//t.js//template npm:lodash@4.17.21//u.js//helper direct 3
edge npm:other@1.0.0//o.js//f fs//rm direct 2
`

func TestReachDependencyCode(t *testing.T) {
	tests := []struct {
		name   string
		extra  string
		target string
		status Status
		kinds  []UncertaintyKind
	}{
		{name: "a path through a package", target: "child_process//exec", status: Found, kinds: []UncertaintyKind{}},
		{name: "a failed file of a unit that the search enters", extra: "file npm:lodash@4.17.21//big.js failed", target: "fs//rm", status: Uncertain, kinds: []UncertaintyKind{Budget}},
		{name: "a failed file of a unit that the search does not enter", extra: "file npm:other@1.0.0//big.js failed", target: "fs//rm", status: NotFound, kinds: []UncertaintyKind{}},
		{
			name:   "an entry into a unit whose code reaches the target",
			extra:  "edge main.py//run kit//load//close direct 4\nentry kit//load//close npm:kit@1.0.0\nedge npm:kit@1.0.0//k.js//close fs//unlink",
			target: "fs//unlink", status: Uncertain, kinds: []UncertaintyKind{UnresolvedCall},
		},
		{
			name:   "an entry into a unit whose code does not reach the target",
			extra:  "edge main.py//run kit//load//close direct 4\nentry kit//load//close npm:kit@1.0.0\nedge npm:kit@1.0.0//k.js//close fs//unlink",
			target: "fs//rm", status: NotFound, kinds: []UncertaintyKind{},
		},
		{
			name: "an entry into a unit whose code calls a unit that reaches the target",
			extra: "edge main.py//run kit//load//close direct 4\nentry kit//load//close npm:kit@1.0.0\nedge npm:kit@1.0.0//k.js//close other//f\n" +
				"edge other//f npm:other@1.0.0//o.js//f package",
			target: "fs//rm", status: Uncertain, kinds: []UncertaintyKind{UnresolvedCall},
		},
		{
			name:   "a unit that no import names, whose code reaches the target",
			extra:  "entry - npm:kit@1.0.0\nedge npm:kit@1.0.0//k.js//close fs//unlink",
			target: "fs//unlink", status: Uncertain, kinds: []UncertaintyKind{UnresolvedCall},
		},
		{
			name:   "a unit that no import names, whose code does not reach the target",
			extra:  "entry - npm:kit@1.0.0\nedge npm:kit@1.0.0//k.js//close fs//unlink",
			target: "fs//rename", status: NoSuchTarget, kinds: []UncertaintyKind{},
		},
		{
			name:   "an entry into a unit with a failed file",
			extra:  "edge main.py//run kit//load//close direct 4\nentry kit//load//close npm:kit@1.0.0\nfile npm:kit@1.0.0//big.js failed",
			target: "fs//rm", status: Uncertain, kinds: []UncertaintyKind{Budget},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := reachOne(t, graphOf(t, dependencies+tt.extra), Query{Targets: []string{tt.target}})
			assert.Equal(t, tt.status, r.Status)
			assert.Equal(t, tt.kinds, kinds(r.Uncertainties))
		})
	}
}

func TestExplore(t *testing.T) {
	g := graphOf(t, `
edge kit//run npm:kit@1.0.0//k.js//run package
edge npm:kit@1.0.0//k.js//run fs//rm direct 2
edge npm:kit@1.0.0//k.js//run npm:kit@1.0.0//k.js//helper direct 3
edge npm:kit@1.0.0//k.js//helper child_process//exec direct 6
unresolved npm:kit@1.0.0//k.js//helper 7 unknown-receiver send
unresolved npm:kit@1.0.0//k.js//helper 8 computed-name
edge npm:kit@1.0.0//k.js//dead os//exit direct 9
`)
	e, err := Explore(context.Background(), g, []string{"kit//run"}, 0)
	require.NoError(t, err)
	require.Len(t, e.Reached, 2)
	assert.Equal(t, "child_process//exec", e.Reached[0].Target)
	assert.Equal(t, []string{"kit//run", "npm:kit@1.0.0//k.js//run", "npm:kit@1.0.0//k.js//helper", "child_process//exec"}, pathKeys(e.Reached[0].Path))
	assert.Equal(t, "fs//rm", e.Reached[1].Target)
	assert.Equal(t, []UncertaintyKind{UnresolvedCall, UnresolvedCall}, kinds(e.Uncertainties))
	assert.Empty(t, e.Uncertainties[0].Method, "a record that concerns every target comes first")
	assert.Equal(t, "send", e.Uncertainties[1].Method)

	_, err = Explore(context.Background(), g, []string{"kit//nope"}, 0)
	assert.ErrorContains(t, err, "no node kit//nope")
	_, err = Explore(context.Background(), g, []string{"kit//run"}, -1)
	assert.Error(t, err)
}

func TestPathStringNamesAWeakHop(t *testing.T) {
	hops := []Hop{
		{Key: "main.py"},
		{Key: "app.py//Parser//parse", Site: projectgraph.Site{Path: "main.py", Line: 3}, EdgeKind: projectgraph.EdgeName, Confidence: projectgraph.ConfidenceLow},
		{Key: "yaml//load", Site: projectgraph.Site{Path: "app.py", Line: 6}, EdgeKind: projectgraph.EdgeDirect, Confidence: projectgraph.ConfidenceHigh},
	}
	assert.Equal(t, "main.py:3 (name) -> app.py:6 -> yaml//load", PathString(hops))

	hops[1].EdgeKind, hops[1].Resolver = projectgraph.EdgeLLM, "model m1, prompt v2"
	assert.Equal(t, "main.py:3 (llm, model m1, prompt v2) -> app.py:6 -> yaml//load", PathString(hops))
}

func TestResultString(t *testing.T) {
	found := Result{Target: "yaml//load", Status: Found, Path: []Hop{{Key: "main.py"}, {Key: "yaml//load", Site: projectgraph.Site{Path: "main.py", Line: 3}, EdgeKind: projectgraph.EdgeDirect, Confidence: projectgraph.ConfidenceHigh}}}
	assert.Equal(t, "yaml//load: found\n  main.py:3 -> yaml//load", found.String())

	missing := Result{
		Target: "yaml//dump", Status: Uncertain,
		CallSites:     []CallSite{{Caller: "dead.py//f", Site: projectgraph.Site{Path: "dead.py", Line: 2}}},
		Uncertainties: []Uncertainty{{Kind: Budget, Site: projectgraph.Site{Path: "big.py"}, Text: "the file is missing"}},
		Searched:      Searched{Roots: []projectgraph.Root{{Key: "main.py"}}, Nodes: 4, Edges: 3},
	}
	assert.Equal(t, "yaml//dump: uncertain\n  budget: big.py the file is missing\n  call at dead.py:2 in dead.py//f, which no root reaches\n  searched 1 roots, 4 nodes, 3 edges", missing.String())
	assert.Contains(t, Result{Target: "Thread//sleep", Status: NoSuchTarget}.String(), "no node has the key")

	fallback := Result{Target: "yaml//dump", Status: NotFound, Uncertainties: []Uncertainty{{Kind: RootFallback, Text: "roots: every exported symbol"}}}
	assert.Equal(t, "yaml//dump: not-found\n  root-fallback: roots: every exported symbol\n  searched 0 roots, 0 nodes, 0 edges", fallback.String(), "an uncertainty with no site has one space")
}

// exploreAllGraphs are the graphs that compare ExploreAll with Explore: a
// package with a cycle, records of each scope, a failed file of a unit and
// a unit that no import names.
var exploreAllGraphs = map[string]string{
	"cycle and scopes": `
edge kit//run npm:kit@1.0.0//k.js//run package
edge npm:kit@1.0.0//k.js//run fs//rm direct 2
edge npm:kit@1.0.0//k.js//run npm:kit@1.0.0//k.js//helper direct 3
edge npm:kit@1.0.0//k.js//helper npm:kit@1.0.0//k.js//run direct 4
edge npm:kit@1.0.0//k.js//helper child_process//exec direct 6
unresolved npm:kit@1.0.0//k.js//helper 7 unknown-receiver send
unresolved npm:kit@1.0.0//k.js//helper 8 computed-name
edge kit//dead npm:kit@1.0.0//k.js//dead package
edge npm:kit@1.0.0//k.js//dead os//exit direct 9
unresolved npm:kit@1.0.0//k.js//dead 10 unknown-receiver exit
edge kit//quiet npm:kit@1.0.0//k.js//quiet package
flag npm:kit@1.0.0//k.js//quiet eval 11
file npm:kit@1.0.0//big.js failed
entry - npm:other@1.0.0
edge npm:other@1.0.0//o.js//f fs//unlink direct 2
`,
	"project with units": dependencies + "edge main.py//run kit//load//close direct 4\nentry kit//load//close npm:kit@1.0.0\nedge npm:kit@1.0.0//k.js//close fs//unlink\n",
}

func TestExploreAllMatchesExplore(t *testing.T) {
	for name, text := range exploreAllGraphs {
		t.Run(name, func(t *testing.T) {
			g := graphOf(t, text)
			x := NewExplorer(g)
			var from []string
			for key, n := range g.Nodes {
				if n.Kind == projectgraph.NodeExternal {
					from = append(from, key)
				}
			}
			slices.Sort(from)
			sets, err := x.ExploreAll(context.Background(), from)
			require.NoError(t, err)
			require.Len(t, sets.Sources, len(from))

			for i, key := range from {
				e, err := x.Explore(context.Background(), []string{key}, 0)
				require.NoError(t, err)
				var reached []string
				for _, r := range e.Reached {
					reached = append(reached, r.Target)
				}
				if reached == nil {
					reached = []string{}
				}
				assert.Equal(t, reached, sets.Sources[i].Reached, "reached from %s", key)

				want := slices.Clone(e.Uncertainties)
				var got []Uncertainty
				for _, idx := range sets.Sources[i].Uncertainties {
					got = append(got, sets.Uncertainties[idx].Uncertainty)
				}
				assert.ElementsMatch(t, want, got, "uncertainties from %s", key)
			}
		})
	}
}

func TestExploreAllScopesTheTargets(t *testing.T) {
	g := graphOf(t, exploreAllGraphs["cycle and scopes"])
	sets, err := NewExplorer(g).ExploreAll(context.Background(), []string{"kit//dead", "kit//quiet"})
	require.NoError(t, err)

	dead := sets.Sources[0]
	assert.Equal(t, []string{"os//exit"}, dead.Reached)
	var methods []string
	for _, idx := range dead.Uncertainties {
		u := sets.Uncertainties[idx]
		switch {
		case u.Method != "":
			methods = append(methods, u.Method)
			assert.Equal(t, []string{"os//exit"}, u.Targets, "the external node with the last segment %s", u.Method)
		case u.Unit != "":
			assert.Equal(t, []string{"fs//unlink"}, u.Targets, "the code of the unit reaches unlink")
		}
	}
	assert.Equal(t, []string{"exit"}, methods)

	quiet := sets.Sources[1]
	assert.Empty(t, quiet.Reached)
	var kinds []UncertaintyKind
	for _, idx := range quiet.Uncertainties {
		kinds = append(kinds, sets.Uncertainties[idx].Kind)
	}
	assert.Equal(t, []UncertaintyKind{UnresolvedCall, Budget, DynamicFlag}, kinds, "the dark unit, the failed file of the unit and the flag")

	_, err = NewExplorer(g).ExploreAll(context.Background(), []string{"kit//nope"})
	assert.ErrorContains(t, err, "no node kit//nope")
}

func TestExploreAllSharesTheSetOfAChain(t *testing.T) {
	g := graphOf(t, `
edge a//x npm:a@1.0.0//a.js//x package
edge npm:a@1.0.0//a.js//x npm:a@1.0.0//a.js//y direct 2
edge npm:a@1.0.0//a.js//y npm:a@1.0.0//a.js//z direct 3
edge npm:a@1.0.0//a.js//z fs//rm direct 4
`)
	a := NewExplorer(g).a
	c := a.condense()
	sets := c.sets(a, a.table())
	x, z := a.id["npm:a@1.0.0//a.js//x"], a.id["npm:a@1.0.0//a.js//z"]
	assert.Same(t, &sets[c.comp[x]][0], &sets[c.comp[z]][0], "x adds nothing to what z reaches, so it shares the set of z")
	assert.NotSame(t, &sets[c.comp[z]][0], &sets[c.comp[a.id["fs//rm"]]][0], "z adds fs//rm")
}

func TestCondenseGroupsACycle(t *testing.T) {
	g := graphOf(t, `
edge main.py main.py//a direct 1
edge main.py//a main.py//b direct 2
edge main.py//b main.py//a direct 3
edge main.py//b fs//rm direct 4
`)
	a := NewExplorer(g).a
	c := a.condense()
	assert.Equal(t, c.comp[a.id["main.py//a"]], c.comp[a.id["main.py//b"]])
	assert.NotEqual(t, c.comp[a.id["main.py"]], c.comp[a.id["main.py//a"]])
	assert.Len(t, c.order, 3)
	assert.Equal(t, c.comp[a.id["fs//rm"]], c.order[0], "a component comes after every component that it reaches")
	assert.Equal(t, c.comp[a.id["main.py"]], c.order[2])
}

func TestExplorerPath(t *testing.T) {
	g := graphOf(t, exploreAllGraphs["cycle and scopes"])
	x := NewExplorer(g)
	hops, found, err := x.Path(context.Background(), "kit//run", "child_process//exec", 0)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, []string{"kit//run", "npm:kit@1.0.0//k.js//run", "npm:kit@1.0.0//k.js//helper", "child_process//exec"}, pathKeys(hops))

	_, found, err = x.Path(context.Background(), "kit//run", "os//exit", 0)
	require.NoError(t, err)
	assert.False(t, found, "dead is not reached from run")
	_, found, err = x.Path(context.Background(), "kit//run", "fs//nope", 0)
	require.NoError(t, err)
	assert.False(t, found, "a key of no node")
	_, _, err = x.Path(context.Background(), "kit//nope", "fs//rm", 0)
	assert.ErrorContains(t, err, "no node kit//nope")
	_, _, err = x.Path(context.Background(), "kit//run", "fs//rm", -1)
	assert.Error(t, err)
}

func TestBitset(t *testing.T) {
	b := newBitset(130)
	b.set(0)
	b.set(129)
	assert.True(t, b.has(0) && b.has(129))
	assert.False(t, b.has(64))
	o := newBitset(130)
	o.set(64)
	assert.False(t, o.subset(b))
	b.or(o)
	assert.True(t, o.subset(b))
	assert.Equal(t, 3, b.count())
	assert.Len(t, b.bytes(), 24)
}

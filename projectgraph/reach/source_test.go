package reach

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/safedep/code/projectgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sourcesOf(t testing.TB, keys ...string) []Source {
	t.Helper()
	sources, err := SourcesByKey(keys...)
	require.NoError(t, err)
	return sources
}

func TestSourceByKey(t *testing.T) {
	for _, tt := range []struct {
		key  string
		want string
	}{
		{key: "app/server.py//main"},
		{key: "app/server.py"},
		{key: "src/api.ts//Router//handle"},
		{key: "", want: "the key is empty"},
		{key: "//main", want: "an empty segment"},
		{key: "app.py//", want: "an empty segment"},
		{key: "app.py//*", want: "has a wildcard"},
	} {
		t.Run(tt.key, func(t *testing.T) {
			source, err := SourceByKey(tt.key)
			if tt.want != "" {
				assert.ErrorContains(t, err, tt.want)
				assert.True(t, source.IsZero())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.key, source.Key())
			assert.Equal(t, tt.key, source.String())
			assert.False(t, source.IsZero())
		})
	}
}

func TestSourceInFile(t *testing.T) {
	source, err := SourceInFile("app/server.py", "main")
	require.NoError(t, err)
	assert.Equal(t, "app/server.py//main", source.Key())

	source, err = SourceInFile("src/api.ts", "Router", "handle")
	require.NoError(t, err)
	assert.Equal(t, "src/api.ts//Router//handle", source.Key())

	source, err = SourceInFile("app/server.py")
	require.NoError(t, err)
	assert.Equal(t, "app/server.py", source.Key(), "the module top level")

	_, err = SourceInFile("", "main")
	assert.ErrorContains(t, err, "no path")
	_, err = SourceInFile("app/server.py", "")
	assert.ErrorContains(t, err, "an empty segment")
}

func TestSourceJSONIsTheKey(t *testing.T) {
	data, err := json.Marshal(sourcesOf(t, "app.py//main"))
	require.NoError(t, err)
	assert.Equal(t, `["app.py//main"]`, string(data))

	var sources []Source
	require.NoError(t, json.Unmarshal(data, &sources))
	assert.Equal(t, sourcesOf(t, "app.py//main"), sources)
	assert.ErrorContains(t, json.Unmarshal([]byte(`["app.py//*"]`), &sources), "has a wildcard")
}

// sourced has a root that reaches json//load, a dead function that
// reaches yaml//load, a test root that reaches os//system, and a unit.
const sourced = `
root main.py
root tests/test_x.py test
edge main.py handlers.py//parse direct 3
edge handlers.py//parse json//load direct 8
edge handlers.py//load_old yaml//load direct 13
edge tests/test_x.py os//system direct 2
edge handlers.py//parse npm:kit@1.0.0//k.js//run package 9
`

func TestReachFromASource(t *testing.T) {
	g := graphOf(t, sourced)
	r := reachOne(t, g, Query{Targets: targetsOf(t, "yaml//load")})
	assert.Equal(t, NotFound, r.Status, "no rule root reaches load_old")

	r = reachOne(t, g, Query{Targets: targetsOf(t, "yaml//load"), Sources: sourcesOf(t, "handlers.py//load_old")})
	assert.Equal(t, Found, r.Status)
	assert.Equal(t, []string{"handlers.py//load_old", "yaml//load"}, pathKeys(r.Path))
	assert.Equal(t, RootQuery, r.Path[0].Root)
	assert.Equal(t, "handlers.py//load_old (query) -> handlers.py:13 -> yaml//load", PathString(r.Path))
	assert.Equal(t, []projectgraph.Root{
		{Key: "handlers.py//load_old", Kind: RootQuery, Rule: "query", Site: projectgraph.Site{Path: "handlers.py", Line: 1}},
		{Key: "main.py", Kind: projectgraph.RootMain, Rule: "test.rule", Site: projectgraph.Site{Path: "main.py", Line: 1}},
	}, r.Searched.Roots, "the sources come first, then the roots of the graph")

	r = reachOne(t, g, Query{Targets: targetsOf(t, "json//load"), Sources: sourcesOf(t, "handlers.py//load_old")})
	assert.Equal(t, Found, r.Status, "the roots of the graph stay in the search")
	assert.Equal(t, projectgraph.RootMain, r.Path[0].Root, "a path from a rule root names its kind")
	assert.Equal(t, "main.py:3 -> handlers.py:8 -> json//load", PathString(r.Path), "a rule root does not show in the string")
}

func TestReachFromSourcesOnly(t *testing.T) {
	g := graphOf(t, sourced)
	q := Query{Targets: targetsOf(t, "json//load", "yaml//load", "os//system"), Sources: sourcesOf(t, "handlers.py//load_old"), SourceMode: SourcesOnly}
	results, err := Reach(context.Background(), g, q)
	require.NoError(t, err)
	assert.Equal(t, NotFound, results[0].Status, "only a rule root reaches json//load")
	assert.Equal(t, Found, results[1].Status)
	assert.Equal(t, NotFound, results[2].Status)
	assert.Empty(t, results[2].TestPath, "the test roots are not in the search")
	assert.Len(t, results[0].Searched.Roots, 1)
	assert.Equal(t, RootQuery, results[0].Searched.Roots[0].Kind)

	q.SourceMode = SourcesAndRoots
	results, err = Reach(context.Background(), g, q)
	require.NoError(t, err)
	assert.Equal(t, NotFound, results[2].Status)
	assert.Equal(t, []string{"tests/test_x.py", "os//system"}, pathKeys(results[2].TestPath), "a test path comes with the roots of the graph")
	assert.Equal(t, projectgraph.RootTest, results[2].TestPath[0].Root)
}

func TestReachSourcesOnlyHasNoRootFallback(t *testing.T) {
	g := graphOf(t, sourced+"\nfallback\n")
	r := reachOne(t, g, Query{Targets: targetsOf(t, "yaml//load"), Sources: sourcesOf(t, "handlers.py//load_old")})
	assert.Equal(t, []UncertaintyKind{RootFallback}, kinds(r.Uncertainties), "the fallback roots stay in the search")

	r = reachOne(t, g, Query{Targets: targetsOf(t, "yaml//load"), Sources: sourcesOf(t, "handlers.py//load_old"), SourceMode: SourcesOnly})
	assert.Equal(t, Found, r.Status)
	assert.Empty(t, r.Uncertainties)
	assert.Equal(t, 0, r.Searched.Uncertainties)
}

func TestReachRejectsAnUnknownSource(t *testing.T) {
	g := graphOf(t, sourced)
	for _, tt := range []struct {
		name    string
		sources []Source
		want    []string
	}{
		{name: "no node", sources: sourcesOf(t, "handlers.py//load_odl"), want: []string{"no node has the key handlers.py//load_odl"}},
		{name: "every bad source", sources: sourcesOf(t, "a.py//x", "handlers.py//load_old", "b.py//y"), want: []string{"a.py//x", "b.py//y"}},
		{name: "an external node", sources: sourcesOf(t, "yaml//load"), want: []string{"yaml//load is an external symbol"}},
		{name: "dependency code", sources: sourcesOf(t, "npm:kit@1.0.0//k.js//run"), want: []string{"is in dependency code"}},
		{name: "a zero source", sources: []Source{{}}, want: []string{"a zero source"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Reach(context.Background(), g, Query{Targets: targetsOf(t, "yaml//load"), Sources: tt.sources})
			require.Error(t, err)
			for _, want := range tt.want {
				assert.ErrorContains(t, err, want)
			}
			if tt.name != "a zero source" {
				assert.ErrorIs(t, err, ErrUnknownSource)
			}
		})
	}
}

func TestReachRejectsAnUndefinedSourceMode(t *testing.T) {
	g := graphOf(t, sourced)
	_, err := Reach(context.Background(), g, Query{Targets: targetsOf(t, "yaml//load"), SourceMode: SourcesOnly + 1})
	assert.ErrorContains(t, err, "source mode 2 is not SourcesAndRoots or SourcesOnly")
}

func TestReachRejectsSourcesOnlyWithNoSource(t *testing.T) {
	g := graphOf(t, sourced)
	_, err := Reach(context.Background(), g, Query{Targets: targetsOf(t, "yaml//load"), SourceMode: SourcesOnly})
	assert.ErrorContains(t, err, "source mode SourcesOnly needs at least one source")
}

// loaded has the Go package app of two files, whose b.go holds os.Remove
// as a value, and the package lib, whose c.go holds os.Chdir. run calls
// the field hook of a value, and dispatch calls Handle on a value whose
// type the link step does not know. T.Handle of lib calls the field hook
// too. The Python module other.py holds os.system as a value.
const loaded = `
file app/a.go indexed go
file app/b.go indexed go
file lib/c.go indexed go
file other.py indexed python
root app/a.go//main
edge app/a.go//main app/a.go//run direct 3
edge app/b.go os//Remove reference 3
edge lib/c.go os//Chdir reference 2
edge other.py os//system reference 1
unresolved app/a.go//run 5 unknown-receiver hook
unresolved app/a.go//dispatch 7 unknown-receiver Handle
unresolved lib/c.go//T//Handle 4 unknown-receiver hook
edge other.py//f os//getcwd direct 2
`

func TestReachFromASourceEntersItsLoadGroup(t *testing.T) {
	g := graphOf(t, loaded)
	for _, tt := range []struct {
		name, source, target string
		want                 Status
		path                 []string
	}{
		{name: "a file of the package of the source", source: "app/a.go//run", target: "os//Remove", want: Found, path: []string{"app/a.go//run", "app/b.go", "os//Remove"}},
		{name: "a package that the span of a method enters", source: "app/a.go//dispatch", target: "os//Chdir", want: Uncertain, path: []string{}},
		{name: "a file of no package of the search", source: "app/a.go//run", target: "os//system", want: NotFound, path: []string{}},
		{name: "the file of the source", source: "other.py//f", target: "os//system", want: Found, path: []string{"other.py//f", "other.py", "os//system"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := reachOne(t, g, Query{Targets: targetsOf(t, tt.target), Sources: sourcesOf(t, tt.source), SourceMode: SourcesOnly})
			assert.Equal(t, tt.want, r.Status)
			assert.Equal(t, tt.path, pathKeys(r.Path))
		})
	}

	r := reachOne(t, g, Query{Targets: targetsOf(t, "os//Remove")})
	assert.Equal(t, NotFound, r.Status, "a search from the roots of kind main enters the load code through the roots and the import edges")
}

// TestReachScopesAValueCallOnlyFromTheRoots checks that a call of a
// function value concerns no target, and an ambiguous call only what its
// method name concerns, in a search from the roots that the rules marked.
// A search with a source and the sets of ExploreAll may not reach the code
// that stores the value, so there each record concerns every target.
func TestReachScopesAValueCallOnlyFromTheRoots(t *testing.T) {
	const spec = `
root app/a.go//main
edge app/a.go//main app/a.go//serve direct 3
edge app/a.go//H//Handle os//Getwd direct 12
edge app/a.go//dead os//Remove direct 16
`
	for _, tt := range []struct{ name, record string }{
		{name: "a call of a function value", record: "unresolved app/a.go//serve 8 function-value"},
		{name: "an ambiguous call of a method name", record: "unresolved app/a.go//serve 8 ambiguous Handle"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := graphOf(t, spec+tt.record)
			remove := targetsOf(t, "os//Remove")
			assert.Equal(t, NotFound, reachOne(t, g, Query{Targets: remove}).Status, "from the roots")
			for _, mode := range []SourceMode{SourcesAndRoots, SourcesOnly} {
				r := reachOne(t, g, Query{Targets: remove, Sources: sourcesOf(t, "app/a.go//serve"), SourceMode: mode})
				assert.Equal(t, Uncertain, r.Status, "source mode %d", mode)
			}
			sets, err := NewExplorer(g).ExploreAll(context.Background(), []string{"app/a.go//serve"})
			require.NoError(t, err)
			require.Len(t, sets.Sources[0].Uncertainties, 1)
			assert.True(t, sets.Uncertainties[sets.Sources[0].Uncertainties[0]].Concerns(remove[0]), "a summary")
		})
	}
}

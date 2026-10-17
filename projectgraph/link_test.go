package projectgraph

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden graphs")

// caseOptions is the options.json of a case.
type caseOptions struct {
	SysPath []string              `json:"sys_path"`
	Files   map[string]FileStatus `json:"files"`
}

// loadCase reads the indexes of a case under testdata, its options, and the
// files under its fs directory as the project root.
func loadCase(t *testing.T, name string) ([]*fileindex.Index, Options) {
	t.Helper()
	dir := filepath.Join("testdata", name)

	data, err := os.ReadFile(filepath.Join(dir, "indexes.json"))
	require.NoError(t, err)
	var indexes []*fileindex.Index
	require.NoError(t, json.Unmarshal(data, &indexes))

	var co caseOptions
	if data, err := os.ReadFile(filepath.Join(dir, "options.json")); err == nil {
		require.NoError(t, json.Unmarshal(data, &co))
	} else {
		require.ErrorIs(t, err, fs.ErrNotExist)
	}

	root := fstest.MapFS{}
	fsDir := filepath.Join(dir, "fs")
	if _, err := os.Stat(fsDir); err == nil {
		require.NoError(t, filepath.WalkDir(fsDir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(fsDir, p)
			if err != nil {
				return err
			}
			root[filepath.ToSlash(rel)] = &fstest.MapFile{Data: data}
			return nil
		}))
	}
	return indexes, Options{Root: root, SysPath: co.SysPath, Files: co.Files}
}

func buildCase(t *testing.T, name string) *Graph {
	t.Helper()
	indexes, opts := loadCase(t, name)
	g, err := Build(context.Background(), indexes, opts)
	require.NoError(t, err)
	return g
}

// checkGolden compares the JSON form of the graph with the golden file of
// the case. The -update flag rewrites the file.
func checkGolden(t *testing.T, name string, g *Graph) {
	t.Helper()
	got, err := json.MarshalIndent(g, "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')

	golden := filepath.Join("testdata", name, "graph.golden.json")
	if *update {
		require.NoError(t, os.WriteFile(golden, got, 0o644))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

func edgeStrings(g *Graph) []string {
	out := make([]string, 0, len(g.Edges))
	for _, e := range g.Edges {
		out = append(out, fmt.Sprintf("%s -> %s %s %s", e.From, e.To, e.Kind, e.Resolver))
	}
	return out
}

func unresolvedStrings(g *Graph) []string {
	out := make([]string, 0, len(g.Unresolved))
	for _, u := range g.Unresolved {
		out = append(out, fmt.Sprintf("%s:%d %s %s %s", u.Site.Path, u.Site.Line, u.Caller, u.Reason, u.Method))
	}
	return out
}

type linkCase struct {
	name       string
	edges      []string
	unresolved []string
	check      func(t *testing.T, g *Graph)
}

func runLinkCases(t *testing.T, cases []linkCase) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := buildCase(t, tc.name)
			edges := edgeStrings(g)
			for _, e := range tc.edges {
				assert.Contains(t, edges, e)
			}
			assert.Equal(t, tc.unresolved, nonNil(unresolvedStrings(g)))
			if tc.check != nil {
				tc.check(t, g)
			}
			checkGolden(t, tc.name, g)
		})
	}
}

func nonNil(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func TestBuildPython(t *testing.T) {
	runLinkCases(t, []linkCase{
		{
			name: "python/sibling-import",
			edges: []string{
				"main.py -> handlers.py import python.import.absolute",
				"main.py -> handlers.py//parse direct python.import",
				"handlers.py//parse -> yaml//load direct python.external",
			},
		},
		{
			name: "python/package-init",
			edges: []string{
				"main.py -> pkg/__init__.py import python.import.absolute",
				"main.py -> pkg/__init__.py//load direct python.import",
				"pkg/__init__.py//load -> pkg/loader.py//load export python.reexport",
				"pkg/__init__.py -> pkg/loader.py import python.import.relative",
				"pkg/loader.py//load -> yaml//load direct python.external",
			},
			check: func(t *testing.T, g *Graph) {
				assert.Equal(t, NodeExport, g.Nodes["pkg/__init__.py//load"].Kind)
			},
		},
		{
			name: "python/relative-imports",
			edges: []string{
				"pkg/sub/a.py -> pkg/sub/b.py import python.import.relative",
				"pkg/sub/a.py -> pkg/c.py import python.import.relative",
				"pkg/sub/a.py//run -> pkg/sub/b.py//f direct python.import",
				"pkg/sub/a.py//run -> pkg/c.py//g direct python.import",
			},
		},
		{
			name: "python/src-layout",
			edges: []string{
				"services/api/src/api/cli.py -> services/api/src/api/core.py import python.import.absolute",
				"services/api/src/api/cli.py -> services/api/src/api/core.py//run direct python.import",
			},
		},
		{
			name: "python/sys-path",
			edges: []string{
				"app/main.py -> lib/shared.py import python.import.absolute",
				"app/main.py -> lib/shared.py//go direct python.import",
			},
		},
		{
			name: "python/all-hides",
			edges: []string{
				"main.py -> pkg/__init__.py//f direct python.import",
				"pkg/__init__.py//f -> pkg/a.py//f export python.reexport",
			},
			unresolved: []string{"main.py:3 main.py no-definition g"},
		},
		{
			name: "python/member-call",
			edges: []string{
				"main.py -> parser.py//Parser direct python.import",
				"parser.py//Parser -> parser.py//Parser//__init__ direct python.constructor",
				"main.py -> parser.py//Parser//parse member python.member",
				"main.py -> io//IOBase//close member python.member",
				"parser.py//Parser//parse -> base.py//Base//helper member python.member",
			},
		},
		{
			name: "python/external",
			edges: []string{
				"main.py -> requests//get direct python.external",
				"main.py -> yaml//load direct python.external",
				"main.py -> print direct python.external",
			},
			unresolved: []string{"main.py:6 main.py//load_plugin dynamic-import "},
			check: func(t *testing.T, g *Graph) {
				assert.Equal(t, "plugins//", g.Unresolved[0].ImportPrefix)
				assert.True(t, g.IsExternal("yaml//load"))
				assert.False(t, g.IsExternal("main.py//load_plugin"))
				assert.Empty(t, g.Nodes["main.py"].Flags, "the flag of a function stays off the module")
			},
		},
		{
			name: "python/assignment",
			edges: []string{
				"main.py -> main.py//helper assignment python.assignment",
				"main.py -> yaml//load assignment python.assignment",
			},
		},
		{
			name: "python/reexport-cycle",
			edges: []string{
				"a/__init__.py -> b/__init__.py import python.import.absolute",
				"b/__init__.py -> a/__init__.py import python.import.absolute",
			},
			unresolved: []string{"main.py:2 main.py no-definition x"},
			check: func(t *testing.T, g *Graph) {
				assert.False(t, slices.ContainsFunc(g.Edges, func(e Edge) bool { return e.Kind == EdgeExport }))
			},
		},
		{
			name: "python/missing-file",
			edges: []string{
				"main.py -> handlers.py import python.import.absolute",
				"main.py -> handlers.py direct python.import",
			},
			check: func(t *testing.T, g *Graph) {
				assert.Equal(t, FileMissing, g.Files["handlers.py"])
				assert.Equal(t, FileCached, g.Files["main.py"])
			},
		},
		{
			name: "python/wildcard",
			edges: []string{
				"main.py -> util.py import python.import.absolute",
				"main.py -> util.py//helper wildcard python.wildcard",
				"main.py -> _private direct python.external",
			},
		},
	})
}

// chain returns n Python packages p0 to p(n-1), where each package
// re-exports x from the next and the last defines x, and a main.py that
// calls p0.x.
func chain(n int) []*fileindex.Index {
	main := &fileindex.Index{
		Version: fileindex.Version, Path: "main.py", Language: langcode.Python,
		Imports: []fileindex.Import{{Line: 1, Module: "p0", Item: "x", Alias: "x", Kind: fileindex.ImportStatic, Namespace: "p0//x"}},
		Calls:   []fileindex.Call{{Caller: "main.py", Line: 2, Column: 1, Callee: "x", Targets: []string{"p0//x"}, Kind: fileindex.CallCall}},
	}
	indexes := []*fileindex.Index{main}
	for i := range n {
		path := fmt.Sprintf("p%d/__init__.py", i)
		ix := &fileindex.Index{Version: fileindex.Version, Path: path, Language: langcode.Python}
		if i == n-1 {
			ix.Definitions = []fileindex.Definition{{Key: path + "//x", Kind: fileindex.DefFunction, Name: "x", StartLine: 1, EndLine: 2, Exported: true}}
		} else {
			next := fmt.Sprintf("p%d", i+1)
			ix.Imports = []fileindex.Import{{Line: 1, Module: next, Item: "x", Alias: "x", Kind: fileindex.ImportStatic, Namespace: next + "//x"}}
		}
		indexes = append(indexes, ix)
	}
	return indexes
}

func TestBuildExportHopLimit(t *testing.T) {
	tests := []struct {
		name      string
		packages  int
		truncated bool
	}{
		{name: "ten hops resolve", packages: 11},
		{name: "eleven hops stop", packages: 12, truncated: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := Build(context.Background(), chain(tt.packages), Options{})
			require.NoError(t, err)
			if !tt.truncated {
				assert.Empty(t, g.Truncated)
				assert.Empty(t, g.Unresolved)
				assert.Contains(t, edgeStrings(g), "main.py -> p0/__init__.py//x direct python.import")
				return
			}
			require.Len(t, g.Truncated, 1)
			assert.Equal(t, TruncationExportHops, g.Truncated[0].Kind)
			assert.Equal(t, []string{"main.py:2 main.py limit x"}, unresolvedStrings(g))
		})
	}
}

func TestBuildEdgeLimit(t *testing.T) {
	indexes, opts := loadCase(t, "python/package-init")
	opts.maxEdges = 3
	g, err := Build(context.Background(), indexes, opts)
	require.NoError(t, err)
	assert.Len(t, g.Edges, 3)
	assert.Equal(t, []Truncation{{Kind: TruncationEdges}}, g.Truncated)
}

func TestBuildIsDeterministic(t *testing.T) {
	indexes, opts := loadCase(t, "python/member-call")
	first, err := Build(context.Background(), indexes, opts)
	require.NoError(t, err)

	slices.Reverse(indexes)
	second, err := Build(context.Background(), indexes, opts)
	require.NoError(t, err)

	a, err := json.Marshal(first)
	require.NoError(t, err)
	b, err := json.Marshal(second)
	require.NoError(t, err)
	assert.Equal(t, string(a), string(b))
}

func TestBuildRejectsBadInput(t *testing.T) {
	ix := &fileindex.Index{Version: fileindex.Version, Path: "a.py", Language: langcode.Python}
	_, err := Build(context.Background(), []*fileindex.Index{ix, ix}, Options{})
	assert.ErrorContains(t, err, "two indexes for the path a.py")

	old := *ix
	old.Version = "0"
	_, err = Build(context.Background(), []*fileindex.Index{&old}, Options{})
	assert.ErrorContains(t, err, `has version "0"`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Build(ctx, []*fileindex.Index{ix}, Options{})
	assert.ErrorIs(t, err, context.Canceled)
}

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
	"github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden graphs")

// caseOptions is the options.json of a case.
type caseOptions struct {
	SysPath   []string                        `json:"sys_path"`
	Files     map[string]fileindex.FileStatus `json:"files"`
	TestPaths []string                        `json:"test_paths"`
}

// loadCase reads the indexes of a case under testdata, its options, its
// extra root rules, and the files under its fs directory as the project
// root.
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
	var rules []roots.Rule
	if data, err := os.ReadFile(filepath.Join(dir, "rules.yaml")); err == nil {
		rules, err = roots.Parse(data)
		require.NoError(t, err)
	} else {
		require.ErrorIs(t, err, fs.ErrNotExist)
	}
	return indexes, Options{Root: root, SysPath: co.SysPath, Files: co.Files, TestPaths: co.TestPaths, Rules: rules}
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
				assert.Equal(t, fileindex.FileMissing, g.Files["handlers.py"])
				assert.Equal(t, fileindex.FileCached, g.Files["main.py"])
			},
		},
		{
			name: "python/class-body",
			edges: []string{
				"config.py -> yaml//load direct python.external",
				"config.py -> yaml//dump direct python.external",
				"config.py//Config -> config.py//Config//__init__ direct python.constructor",
				"config.py//Config//__init__ -> yaml//safe_dump direct python.external",
				"config.py//build -> yaml//safe_load direct python.external",
			},
			unresolved: []string{"config.py:6 config.py computed-name "},
			check: func(t *testing.T, g *Graph) {
				assert.Equal(t, []fileindex.Flag{{Kind: fileindex.FlagReflection, Line: 6}, {Kind: fileindex.FlagEval, Line: 7}}, g.Nodes["config.py"].Flags)
				assert.Empty(t, g.Nodes["config.py//Config"].Flags)
			},
		},
		{
			name: "python/local-receiver",
			unresolved: []string{
				"main.py:6 main.py//read unknown-receiver read",
				"main.py:10 main.py//load_plugin dynamic-import import_module",
				"main.py:11 main.py//load_plugin unknown-receiver run",
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

func TestBuildUnindexedFile(t *testing.T) {
	for _, status := range []fileindex.FileStatus{fileindex.FileMissing, fileindex.FileFailed} {
		t.Run(string(status), func(t *testing.T) {
			indexes, opts := loadCase(t, "python/missing-file")
			opts.Files = map[string]fileindex.FileStatus{"main.py": fileindex.FileIndexed, "handlers.py": status}
			g, err := Build(context.Background(), indexes, opts)
			require.NoError(t, err)

			assert.Equal(t, status, g.Files["handlers.py"])
			require.Contains(t, g.Nodes, "handlers.py")
			assert.Equal(t, NodeModule, g.Nodes["handlers.py"].Kind)
			assert.Contains(t, edgeStrings(g), "main.py -> handlers.py direct python.import")
		})
	}
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

func TestBuildJavaScript(t *testing.T) {
	runLinkCases(t, []linkCase{
		{
			name: "javascript/esm-named-import",
			edges: []string{
				"src/main.js -> src/parser.js import javascript.import.relative",
				"src/main.js -> src/parser.js//parse direct javascript.import",
				"src/main.js -> src/util.js//fmt direct javascript.import",
				"src/parser.js//parse -> js-yaml//load direct javascript.external",
			},
			unresolved: []string{"src/main.js:5 src/main.js dynamic-import "},
			check: func(t *testing.T, g *Graph) {
				assert.Equal(t, "src/locale/", g.Unresolved[0].ImportPrefix)
			},
		},
		{
			name: "javascript/unknown-receiver",
			edges: []string{
				"main.js//handle -> console//log direct javascript.external",
			},
			unresolved: []string{
				"main.js:2 main.js//loadPlugin dynamic-import require",
				"main.js:3 main.js//loadPlugin unknown-receiver run",
				"main.js:8 main.js//handle unknown-receiver json",
			},
			check: func(t *testing.T, g *Graph) {
				assert.NotContains(t, g.Nodes, "plugin//run")
				assert.NotContains(t, g.Nodes, "res//json")
			},
		},
		{
			name: "javascript/commonjs-require",
			edges: []string{
				"main.js -> lib/index.js import javascript.import.relative",
				"main.js -> lib/index.js//run direct javascript.import",
				"main.js -> helper.js//helper direct javascript.import",
			},
		},
		{
			name: "javascript/default-import",
			edges: []string{
				"main.js -> log.js//log direct javascript.import",
				"main.js -> make.js//make direct javascript.import",
			},
		},
		{
			name: "javascript/barrel-chain",
			edges: []string{
				"src/main.js -> src/api/index.js//parse direct javascript.import",
				"src/api/index.js//parse -> src/core/index.js//parse export javascript.reexport",
				"src/core/index.js//parse -> src/core/parse.js//doParse export javascript.reexport",
				"src/api/index.js -> src/core/index.js import javascript.import.relative",
				"src/core/index.js -> src/core/parse.js import javascript.import.relative",
			},
		},
		{
			name: "javascript/export-star",
			edges: []string{
				"main.js -> all.js//a direct javascript.import",
				"all.js//a -> a.js//a export javascript.reexport-star",
				"all.js//b -> b.js//b export javascript.reexport-star",
				"all.js//ns -> c.js export javascript.reexport",
				"main.js -> c.js//c direct javascript.import",
			},
		},
		{
			name: "javascript/workspaces",
			edges: []string{
				"packages/app/src/main.js -> packages/core/lib/index.js import javascript.import.main",
				"packages/app/src/main.js -> packages/core/lib/index.js//start direct javascript.import",
				"packages/app/src/main.js -> packages/core/util.js import javascript.import.subpath",
				"packages/app/src/main.js -> packages/core/util.js//tidy direct javascript.import",
				"packages/app/src/main.js -> lodash//template direct javascript.external",
			},
		},
		{
			name: "javascript/exports-conditions",
			edges: []string{
				"app/main.mjs -> lib/esm/index.mjs import javascript.import.exports-import",
				"app/main.mjs -> lib/esm/index.mjs//x direct javascript.import",
				"app/main.mjs -> lib/src/feature/y.js import javascript.import.exports-default",
				"app/main.mjs -> lib/src/feature/y.js//y direct javascript.import",
			},
		},
	})
}

func TestBuildTypeScript(t *testing.T) {
	runLinkCases(t, []linkCase{
		{
			name: "typescript/tsconfig-paths",
			edges: []string{
				"src/main.ts -> src/app/server.ts import typescript.import.tsconfig-paths",
				"src/main.ts -> src/app/server.ts//serve direct typescript.import",
				"src/main.ts -> src/config.ts import typescript.import.tsconfig-baseurl",
				"src/main.ts -> src/config.ts//cfg direct typescript.import",
			},
		},
		{
			name: "typescript/js-specifier",
			edges: []string{
				"src/main.ts -> src/util.ts import typescript.import.relative",
				"src/main.ts -> src/util.ts//f direct typescript.import",
			},
		},
		{
			name: "typescript/type-only",
			edges: []string{
				"src/main.ts -> src/decl.d.ts import typescript.import.relative",
				"src/main.ts -> src/decl.d.ts//decl direct typescript.import",
			},
			check: func(t *testing.T, g *Graph) {
				for _, e := range g.Edges {
					assert.NotEqual(t, "src/types.ts", e.To, "a type-only import makes no edge")
					assert.NotEqual(t, "src/decl.d.ts", e.From, "a declaration file makes no call edge")
				}
			},
		},
	})
}

func rootStrings(g *Graph) []string {
	out := make([]string, 0, len(g.Roots))
	for _, r := range g.Roots {
		out = append(out, fmt.Sprintf("%s %s %s", r.Key, r.Kind, r.Rule))
	}
	return out
}

func TestBuildRoots(t *testing.T) {
	tests := []struct {
		name     string
		roots    []string
		fallback bool
	}{
		{
			name:  "roots/package-json-bin",
			roots: []string{"bin/cli.js manifest node.package-json"},
		},
		{
			name: "roots/library-surface",
			roots: []string{
				"lib/api.py library-surface library-surface",
				"lib/api.py//public library-surface library-surface",
				"lib/test_api.py test test.files",
				"lib/test_api.py//test_public test test.files",
			},
			fallback: true,
		},
		{
			name: "roots/test-file",
			roots: []string{
				"tests/test_app.py test test.files",
				"tests/test_app.py//TestApp//test_run test test.files",
				"tests/test_app.py//helper test test.files",
			},
			fallback: true,
		},
		{
			name:  "roots/main-guard",
			roots: []string{"app.py main python.main-guard"},
		},
		{
			name: "roots/pyproject-scripts",
			roots: []string{
				"src/app/cli.py//main manifest python.project-scripts",
				"src/app/legacy.py manifest python.project-scripts",
				"src/app/plugin.py//Plugin//load manifest python.project-scripts",
				"src/app/tool.py//run manifest python.project-scripts",
			},
		},
		{
			name: "roots/setup-py",
			roots: []string{
				"tool/admin.py//run manifest python.setup-py",
				"tool/main.py//cli manifest python.setup-py",
			},
		},
		{
			name: "roots/procfile-dockerfile",
			roots: []string{
				"app/cli.py manifest dockerfile",
				"app/wsgi.py//app manifest procfile",
				"web/server.js manifest dockerfile",
				"worker.py manifest procfile",
			},
		},
		{
			name: "roots/extra-rules",
			roots: []string{
				"app/users/route.ts framework routes",
				"src/handler.js//Handler//handle framework handlers",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := buildCase(t, tt.name)
			assert.Equal(t, tt.roots, rootStrings(g))
			assert.Equal(t, tt.fallback, g.Stats.RootFallback)
			assert.Empty(t, g.Problems)
			checkGolden(t, tt.name, g)
		})
	}
}

func TestBuildMarksTestFiles(t *testing.T) {
	g := buildCase(t, "roots/test-file")
	assert.True(t, g.Nodes["tests/test_app.py//helper"].Test)
}

func TestBuildRejectsBadOptions(t *testing.T) {
	_, err := Build(context.Background(), nil, Options{TestPaths: []string{"a/["}})
	assert.ErrorContains(t, err, "test path")

	_, err = Build(context.Background(), nil, Options{Rules: []roots.Rule{{ID: "x", Language: "any", Kind: "route"}}})
	assert.ErrorContains(t, err, `root rule "x": unknown kind "route"`)
}

func TestBuildReportsProblems(t *testing.T) {
	ix := &fileindex.Index{Version: fileindex.Version, Path: "a.js", Language: langcode.Javascript}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{Root: fstest.MapFS{"package.json": {Data: []byte("{")}}})
	require.NoError(t, err)
	require.Len(t, g.Problems, 1)
	assert.Equal(t, "package.json", g.Problems[0].Path)
}

func TestBuildImportBindings(t *testing.T) {
	indexes := []*fileindex.Index{
		{
			Version: fileindex.Version, Path: "main.js", Language: langcode.Javascript,
			Imports: []fileindex.Import{
				{Line: 1, Module: "./polyfill", Wildcard: true, Kind: fileindex.ImportStatic, Namespace: ".//polyfill"},
				{Line: 2, Module: "./api", Item: "f", Alias: "f", Kind: fileindex.ImportStatic, Namespace: ".//api//f"},
			},
			Calls: []fileindex.Call{{Caller: "main.js", Line: 3, Column: 1, Callee: "f", Targets: []string{".//api//f"}, Kind: fileindex.CallCall}},
		},
		{Version: fileindex.Version, Path: "polyfill.js", Language: langcode.Javascript, Exports: []fileindex.Export{{Name: "f", Target: "polyfill.js//f", Line: 1}},
			Definitions: []fileindex.Definition{{Key: "polyfill.js//f", Kind: fileindex.DefFunction, Name: "f", StartLine: 1, EndLine: 1, Exported: true}}},
		{
			Version: fileindex.Version, Path: "api.js", Language: langcode.Javascript,
			Imports: []fileindex.Import{{Line: 1, Module: "./polyfill", Wildcard: true, Kind: fileindex.ImportStatic, Namespace: ".//polyfill"}},
		},
		{
			Version: fileindex.Version, Path: "app.py", Language: langcode.Python,
			Imports: []fileindex.Import{{Line: 1, Module: "pkg", Item: "os", Alias: "os", Kind: fileindex.ImportStatic, Namespace: "pkg//os"}},
			Calls:   []fileindex.Call{{Caller: "app.py", Line: 2, Column: 1, Callee: "os.getcwd", Targets: []string{"pkg//os//getcwd"}, Receiver: "os", Method: "getcwd", Kind: fileindex.CallCall}},
		},
		{
			Version: fileindex.Version, Path: "pkg/__init__.py", Language: langcode.Python,
			Imports: []fileindex.Import{{Line: 1, Module: "os.path", Alias: "os", Kind: fileindex.ImportStatic, Namespace: "os//path"}},
		},
	}
	g, err := Build(context.Background(), indexes, Options{})
	require.NoError(t, err)
	edges := edgeStrings(g)
	assert.Contains(t, edges, "main.js -> polyfill.js import javascript.import.relative", "a side-effect import runs the module")
	assert.Contains(t, edges, "api.js -> polyfill.js import javascript.import.relative")
	assert.Equal(t, []string{"main.js:3 main.js no-definition f"}, unresolvedStrings(g), "a side-effect import gives on no name")
	assert.Contains(t, edges, "app.py -> os//getcwd direct python.external", "import os.path binds os")
	assert.Contains(t, edges, "pkg/__init__.py//os -> os export python.reexport")
}

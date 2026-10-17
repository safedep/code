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
				"main.py -> pkg/__init__.py//g direct python.import",
				"pkg/__init__.py//g -> pkg/a.py//g export python.reexport",
			},
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
			name:  "python/local-receiver",
			edges: []string{"main.py//read -> main.py//read name python.name"},
			unresolved: []string{
				"main.py:6 main.py//read unknown-receiver read",
				"main.py:10 main.py//load_plugin dynamic-import import_module",
				"main.py:11 main.py//load_plugin unknown-receiver run",
			},
		},
		{
			name: "python/decorators",
			edges: []string{
				"app.py -> app.py//logged direct python.decorator",
				"app.py -> dataclasses//dataclass direct python.decorator",
				"app.py//logged -> functools//wraps direct python.external",
				"app.py//helper -> app.py//logged//inner reference python.decorator-closure",
				"app.py//Point//norm -> app.py//logged//inner reference python.decorator-closure",
			},
			unresolved: []string{"app.py:8 app.py//logged//inner unknown-receiver fn"},
			check: func(t *testing.T, g *Graph) {
				assert.False(t, slices.ContainsFunc(g.Edges, func(e Edge) bool {
					return e.To == "functools//wraps" && e.Resolver == "python.decorator"
				}), "a called decorator has its call edge only")
				assert.NotContains(t, g.Nodes, "staticmethod")
			},
		},
		{
			name: "python/wildcard",
			edges: []string{
				"main.py -> util.py import python.import.absolute",
				"main.py -> util.py//helper wildcard python.wildcard",
				"main.py -> util.py//_private name python.name",
			},
			unresolved: []string{"main.py:3 main.py unknown-receiver _private"},
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
			name: "roots/package-json-main",
			roots: []string{
				"cli.js manifest node.package-json",
				"fmt.js//format manifest node.package-json",
				"index.js manifest node.package-json",
				"index.js//parse manifest node.package-json",
				"index.js//readConfig manifest node.package-json",
			},
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
				"src/app/cli.py manifest python.project-scripts",
				"src/app/cli.py//main manifest python.project-scripts",
				"src/app/legacy.py manifest python.project-scripts",
				"src/app/plugin.py manifest python.project-scripts",
				"src/app/plugin.py//Plugin//load manifest python.project-scripts",
				"src/app/tool.py manifest python.project-scripts",
				"src/app/tool.py//run manifest python.project-scripts",
			},
		},
		{
			name: "roots/setup-py",
			roots: []string{
				"tool/admin.py manifest python.setup-py",
				"tool/admin.py//run manifest python.setup-py",
				"tool/main.py manifest python.setup-py",
				"tool/main.py//cli manifest python.setup-py",
			},
		},
		{
			name: "roots/procfile-dockerfile",
			roots: []string{
				"app/cli.py manifest dockerfile",
				"app/wsgi.py manifest procfile",
				"app/wsgi.py//app manifest procfile",
				"web/server.js manifest dockerfile",
				"worker.py manifest procfile",
			},
		},
		{
			name: "roots/extra-rules",
			roots: []string{
				"app/users/route.ts framework node.next.route",
				"app/users/route.ts framework routes",
				"src/handler.js//Handler//handle framework handlers",
				"src/handler.js//handle framework serverless",
				"views.py//index framework flask",
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

func TestBuildImportShapes(t *testing.T) {
	py := func(path string, imports []fileindex.Import, calls ...fileindex.Call) *fileindex.Index {
		return &fileindex.Index{Version: fileindex.Version, Path: path, Language: langcode.Python, Imports: imports, Calls: calls}
	}
	def := func(ix *fileindex.Index, key string) *fileindex.Index {
		ix.Definitions = append(ix.Definitions, fileindex.Definition{Key: key, Kind: fileindex.DefFunction, Name: fileindex.Last(key), StartLine: 1, EndLine: 2, Exported: true})
		return ix
	}
	call := func(caller, callee string, target string) fileindex.Call {
		return fileindex.Call{Caller: caller, Line: 3, Column: 1, Callee: callee, Targets: []string{target}, Kind: fileindex.CallCall}
	}
	bare := func(caller, callee string) fileindex.Call {
		return fileindex.Call{Caller: caller, Line: 3, Column: 1, Callee: callee, Method: callee, Kind: fileindex.CallCall}
	}

	tests := []struct {
		name       string
		root       fstest.MapFS
		indexes    []*fileindex.Index
		edges      []string
		unresolved []string
	}{
		{
			name: "a dotted import binds its first package",
			indexes: []*fileindex.Index{
				py("main.py", []fileindex.Import{{Line: 1, Module: "pkg.loader", Alias: "pkg", Kind: fileindex.ImportStatic, Namespace: "pkg"}},
					call("main.py", "pkg.loader.go", "pkg//loader//go")),
				py("pkg/__init__.py", nil),
				def(py("pkg/loader.py", nil), "pkg/loader.py//go"),
			},
			edges: []string{"main.py -> pkg/loader.py//go direct python.import"},
		},
		{
			name: "a bare name can come from each external star",
			indexes: []*fileindex.Index{
				py("main.py", []fileindex.Import{
					{Line: 1, Module: "yaml", Wildcard: true, Kind: fileindex.ImportStatic, Namespace: "yaml"},
					{Line: 2, Module: "json", Wildcard: true, Kind: fileindex.ImportStatic, Namespace: "json"},
				}, bare("main.py", "load")),
			},
			edges: []string{
				"main.py -> yaml//load wildcard python.wildcard-external",
				"main.py -> json//load wildcard python.wildcard-external",
			},
		},
		{
			name: "a module that names a first-party directory is a site that can reach any target",
			indexes: []*fileindex.Index{
				py("main.py", []fileindex.Import{
					{Line: 1, Module: "conf.loader", Item: "go", Alias: "go", Kind: fileindex.ImportStatic, Namespace: "conf//loader//go"},
					{Line: 2, Module: "yaml", Alias: "yaml", Kind: fileindex.ImportStatic, Namespace: "yaml"},
				}),
				py("conf/__init__.py", nil),
			},
			unresolved: []string{"main.py:1 main.py no-definition "},
		},
		{
			name: "a language with no module resolver makes every import a site that can reach any target",
			indexes: []*fileindex.Index{{
				Version: fileindex.Version, Path: "main.swift", Language: langcode.Code("swift"),
				Imports: []fileindex.Import{{Line: 3, Module: "app/util", Kind: fileindex.ImportStatic, Namespace: "app//util"}},
			}},
			unresolved: []string{"main.swift:3 main.swift no-definition "},
		},
		{
			name: "a Go import under the module path that names no package can reach any target",
			root: fstest.MapFS{"go.mod": {Data: []byte("module example.com/app\n")}},
			indexes: []*fileindex.Index{{
				Version: fileindex.Version, Path: "main.go", Language: langcode.Go, Package: "main",
				Imports: []fileindex.Import{
					{Line: 3, Module: "example.com/app/util", Alias: "util", Kind: fileindex.ImportStatic, Namespace: "example.com//app//util"},
					{Line: 4, Module: "fmt", Alias: "fmt", Kind: fileindex.ImportStatic, Namespace: "fmt"},
				},
			}},
			unresolved: []string{"main.go:3 main.go no-definition "},
		},
		{
			name: "a Go import runs each file of the package that has an init",
			root: fstest.MapFS{"go.mod": {Data: []byte("module example.com/app\n")}},
			indexes: []*fileindex.Index{
				{
					Version: fileindex.Version, Path: "main.go", Language: langcode.Go, Package: "main",
					Imports: []fileindex.Import{{Line: 3, Module: "example.com/app/util", Alias: "util", Kind: fileindex.ImportStatic, Namespace: "example.com//app//util"}},
					Calls:   []fileindex.Call{call("main.go", "util.Load", "example.com//app//util//Load")},
				},
				def(&fileindex.Index{Version: fileindex.Version, Path: "util/a.go", Language: langcode.Go, Package: "util"}, "util/a.go//init"),
				def(&fileindex.Index{Version: fileindex.Version, Path: "util/b.go", Language: langcode.Go, Package: "util"}, "util/b.go//Load"),
			},
			edges: []string{
				"main.go -> util/a.go import go.import.module",
				"util/a.go -> util/a.go//init direct go.init",
				"main.go -> util/b.go//Load direct go.import",
			},
		},
		{
			name: "a side-effect require names the module of a member call",
			indexes: []*fileindex.Index{
				{
					Version: fileindex.Version, Path: "main.js", Language: langcode.Javascript,
					Imports: []fileindex.Import{{Line: 3, Module: "./parse", Wildcard: true, Kind: fileindex.ImportStatic, Namespace: ".//parse"}},
					Calls:   []fileindex.Call{call("main.js", "require('./parse').parse", ".//parse//parse")},
				},
				{
					Version: fileindex.Version, Path: "parse.js", Language: langcode.Javascript,
					Definitions: []fileindex.Definition{{Key: "parse.js//parse", Kind: fileindex.DefFunction, Name: "parse", StartLine: 1, EndLine: 1}},
					Exports:     []fileindex.Export{{Name: "parse", Target: "parse.js//parse", Line: 2}},
				},
			},
			edges: []string{"main.js -> parse.js//parse direct javascript.import"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := Build(context.Background(), tt.indexes, Options{Root: tt.root})
			require.NoError(t, err)
			edges := edgeStrings(g)
			for _, e := range tt.edges {
				assert.Contains(t, edges, e)
			}
			assert.Equal(t, tt.unresolved, nonNil(unresolvedStrings(g)))
		})
	}
}

func TestBuildNameEdges(t *testing.T) {
	fn := func(key string) fileindex.Definition {
		return fileindex.Definition{Key: key, Kind: fileindex.DefFunction, Name: fileindex.Last(key), StartLine: 1, EndLine: 1}
	}
	caller := &fileindex.Index{
		Version: fileindex.Version, Path: "main.py", Language: langcode.Python,
		Calls: []fileindex.Call{
			{Caller: "main.py", Line: 2, Column: 1, Callee: "cb", Method: "cb", Kind: fileindex.CallCall},
			{Caller: "main.py", Line: 3, Column: 1, Callee: "print", Targets: []string{"print"}, Method: "print", Kind: fileindex.CallCall},
		},
		Unresolved: []fileindex.Unresolved{
			{Caller: "main.py", Line: 4, Column: 1, Text: "x.run", Receiver: "x", Method: "run", Reason: fileindex.UnresolvedUnknownReceiver},
			{Caller: "main.py", Line: 5, Column: 1, Text: "y.get", Receiver: "y", Method: "get", Reason: fileindex.UnresolvedUnknownReceiver},
			{Caller: "main.py", Line: 6, Column: 1, Text: "getattr(z, n)", Reason: fileindex.UnresolvedComputedName},
		},
	}
	lib := &fileindex.Index{Version: fileindex.Version, Path: "lib.py", Language: langcode.Python,
		Definitions: []fileindex.Definition{fn("lib.py//cb"), fn("lib.py//run")}}
	tests := &fileindex.Index{Version: fileindex.Version, Path: "tests/test_lib.py", Language: langcode.Python,
		Definitions: []fileindex.Definition{fn("tests/test_lib.py//run")}}
	js := &fileindex.Index{Version: fileindex.Version, Path: "web.js", Language: langcode.Javascript,
		Definitions: []fileindex.Definition{fn("web.js//run")}}
	many := &fileindex.Index{Version: fileindex.Version, Path: "many.py", Language: langcode.Python}
	for i := range maxNameCandidates + 1 {
		many.Definitions = append(many.Definitions, fn(fmt.Sprintf("many.py//C%d//get", i)))
	}

	g, err := Build(context.Background(), []*fileindex.Index{caller, lib, tests, js, many}, Options{TestPaths: []string{"tests"}})
	require.NoError(t, err)

	var names []string
	for _, e := range g.Edges {
		if e.Kind == EdgeName {
			assert.Equal(t, ConfidenceLow, e.Confidence)
			names = append(names, fmt.Sprintf("%s -> %s %s", e.From, e.To, e.Resolver))
		}
	}
	assert.Equal(t, []string{
		"main.py -> lib.py//cb python.name",
		"main.py -> lib.py//run python.name",
	}, names, "a test file and another language hold no candidate")
	assert.Equal(t, []string{
		"main.py:2 main.py unknown-receiver cb",
		"main.py:4 main.py unknown-receiver run",
		"main.py:5 main.py ambiguous ",
		"main.py:6 main.py computed-name ",
	}, unresolvedStrings(g), "a bare name with no binding is a record, and too many candidates make the site ambiguous")
	assert.Contains(t, g.Nodes, "print", "a builtin stays external")
	assert.NotContains(t, g.Nodes, "cb")
}

func TestBuildReferenceEdges(t *testing.T) {
	main := &fileindex.Index{
		Version: fileindex.Version, Path: "main.py", Language: langcode.Python,
		Imports: []fileindex.Import{
			{Line: 1, Module: "yaml", Alias: "yaml", Kind: fileindex.ImportStatic, Namespace: "yaml"},
			{Line: 2, Module: "tasks", Item: "process", Alias: "process", Kind: fileindex.ImportStatic, Namespace: "tasks//process"},
			{Line: 3, Module: "tasks", Item: "gone", Alias: "gone", Kind: fileindex.ImportStatic, Namespace: "tasks//gone"},
		},
		References: []fileindex.Reference{
			{Caller: "main.py", Line: 5, Column: 5, Text: "yaml.load", Targets: []string{"yaml//load"}},
			{Caller: "main.py", Line: 6, Column: 5, Text: "process", Targets: []string{"tasks//process"}},
			{Caller: "main.py", Line: 7, Column: 5, Text: "gone", Targets: []string{"tasks//gone"}},
		},
	}
	tasks := &fileindex.Index{
		Version: fileindex.Version, Path: "tasks.py", Language: langcode.Python,
		Definitions: []fileindex.Definition{{Key: "tasks.py//process", Kind: fileindex.DefFunction, Name: "process", StartLine: 1, EndLine: 2, Exported: true}},
	}
	g, err := Build(context.Background(), []*fileindex.Index{main, tasks}, Options{})
	require.NoError(t, err)

	var refs []string
	for _, e := range g.Edges {
		if e.Kind == EdgeReference {
			assert.Equal(t, ConfidenceLow, e.Confidence)
			refs = append(refs, fmt.Sprintf("%s -> %s %s", e.From, e.To, e.Resolver))
		}
	}
	assert.Equal(t, []string{
		"main.py -> yaml//load python.reference",
		"main.py -> tasks.py//process python.reference",
	}, refs)
	assert.Empty(t, g.Unresolved, "a reference that does not resolve is no call")
}

func TestBuildSuperCalls(t *testing.T) {
	class := func(path, name string, bases []string, methods ...string) ([]fileindex.Definition, fileindex.Class) {
		key := path + fileindex.Separator + name
		defs := []fileindex.Definition{{Key: key, Kind: fileindex.DefClass, Name: name, StartLine: 1, EndLine: 9}}
		c := fileindex.Class{Key: key, Bases: bases}
		for _, m := range methods {
			kind := fileindex.DefMethod
			if m == "constructor" || m == "__init__" {
				kind = fileindex.DefConstructor
			}
			defs = append(defs, fileindex.Definition{Key: fileindex.Join(key, m), Kind: kind, Name: m, StartLine: 2, EndLine: 3, Parent: key})
			c.Methods = append(c.Methods, fileindex.Join(key, m))
		}
		return defs, c
	}
	index := func(path string, lang langcode.Code, imports []fileindex.Import, calls []fileindex.Call, classes ...func() ([]fileindex.Definition, fileindex.Class)) *fileindex.Index {
		ix := &fileindex.Index{Version: fileindex.Version, Path: path, Language: lang, Imports: imports, Calls: calls}
		for _, c := range classes {
			defs, cl := c()
			ix.Definitions = append(ix.Definitions, defs...)
			ix.Classes = append(ix.Classes, cl)
		}
		return ix
	}
	call := func(caller, target string) fileindex.Call {
		return fileindex.Call{Caller: caller, Line: 3, Column: 9, Callee: target, Targets: []string{target}, Method: fileindex.Last(target), Kind: fileindex.CallCall}
	}

	py := index("a.py", langcode.Python,
		[]fileindex.Import{{Line: 1, Module: "lib", Item: "Remote", Alias: "Remote", Kind: fileindex.ImportStatic, Namespace: "lib//Remote"}},
		[]fileindex.Call{
			call("a.py//Strict//load", "super//load"),
			call("a.py//Plain//__init__", "super//__init__"),
			call("a.py//Err//__init__", "super//__init__"),
			call("a.py//Client//send", "super//send"),
		},
		func() ([]fileindex.Definition, fileindex.Class) { return class("a.py", "Base", nil, "load") },
		func() ([]fileindex.Definition, fileindex.Class) {
			return class("a.py", "Strict", []string{"Base"}, "load")
		},
		func() ([]fileindex.Definition, fileindex.Class) { return class("a.py", "Plain", nil, "__init__") },
		func() ([]fileindex.Definition, fileindex.Class) {
			return class("a.py", "Err", []string{"Exception"}, "__init__")
		},
		func() ([]fileindex.Definition, fileindex.Class) {
			return class("a.py", "Client", []string{"Remote"}, "send")
		},
	)
	js := index("a.js", langcode.Javascript, nil,
		[]fileindex.Call{
			call("a.js//Strict//constructor", "super"),
			call("a.js//Strict//load", "super//load"),
			call("a.js//Lone//load", "super//load"),
		},
		func() ([]fileindex.Definition, fileindex.Class) { return class("a.js", "Base", nil, "load") },
		func() ([]fileindex.Definition, fileindex.Class) {
			return class("a.js", "Strict", []string{"Base"}, "constructor", "load")
		},
		func() ([]fileindex.Definition, fileindex.Class) { return class("a.js", "Lone", nil, "load") },
	)
	js.Unresolved = []fileindex.Unresolved{{Caller: "a.js//Strict//load", Line: 3, Column: 9, Text: "super.load", Receiver: "super", Method: "load", Reason: fileindex.UnresolvedUnknownReceiver}}

	g, err := Build(context.Background(), []*fileindex.Index{py, js}, Options{})
	require.NoError(t, err)
	edges := edgeStrings(g)
	for _, e := range []string{
		"a.py//Strict//load -> a.py//Base//load member python.super",
		"a.py//Plain//__init__ -> object//__init__ direct python.super",
		"a.py//Err//__init__ -> Exception//__init__ direct python.super",
		"a.py//Client//send -> lib//Remote//send direct python.super",
		"a.js//Strict//constructor -> a.js//Base direct javascript.super",
		"a.js//Strict//load -> a.js//Base//load member javascript.super",
	} {
		assert.Contains(t, edges, e)
	}
	assert.Equal(t, []string{"a.js:3 a.js//Lone//load no-definition load"}, unresolvedStrings(g), "a class with no base has no super method")
}

func TestBuildDecoratorFactoryClosures(t *testing.T) {
	fn := func(key string, start, end int, decorators ...string) fileindex.Definition {
		return fileindex.Definition{Key: key, Kind: fileindex.DefFunction, Name: fileindex.Last(key), StartLine: start, EndLine: end, Decorators: decorators}
	}
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: "a.py", Language: langcode.Python,
		Imports: []fileindex.Import{{Line: 1, Module: "click", Alias: "click", Kind: fileindex.ImportStatic, Namespace: "click"}},
		Definitions: []fileindex.Definition{
			fn("a.py//retry", 3, 9),
			fn("a.py//retry//decorate", 4, 8),
			fn("a.py//retry//decorate//wrapper", 5, 7),
			fn("a.py//job", 12, 13, "@retry", "@click.command"),
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{})
	require.NoError(t, err)

	var closures []string
	for _, e := range g.Edges {
		if e.Resolver == "python.decorator-closure" {
			closures = append(closures, e.From+" -> "+e.To)
		}
	}
	assert.Equal(t, []string{
		"a.py//job -> a.py//retry//decorate",
		"a.py//job -> a.py//retry//decorate//wrapper",
	}, closures, "an external decorator gets no closure edge")
}

func TestBuildModuleVariables(t *testing.T) {
	variable := func(key string, values ...string) fileindex.Definition {
		return fileindex.Definition{Key: key, Kind: fileindex.DefVariable, Name: fileindex.Last(key), StartLine: 9, EndLine: 9, Exported: true, Values: values}
	}
	app := &fileindex.Index{
		Version: fileindex.Version, Path: "app.py", Language: langcode.Python,
		Imports: []fileindex.Import{{Line: 1, Module: "flask", Item: "Flask", Alias: "Flask", Kind: fileindex.ImportStatic, Namespace: "flask//Flask"}},
		Definitions: []fileindex.Definition{
			{Key: "app.py//Parser", Kind: fileindex.DefClass, Name: "Parser", StartLine: 3, EndLine: 5, Exported: true},
			{Key: "app.py//Parser//parse", Kind: fileindex.DefMethod, Name: "parse", StartLine: 4, EndLine: 5, Exported: true, Parent: "app.py//Parser"},
			variable("app.py//parser", "app.py//Parser"),
			variable("app.py//app", "flask//Flask"),
			variable("app.py//either", "app.py//Parser", "flask//Flask"),
			variable("app.py//unknown"),
		},
		Classes: []fileindex.Class{{Key: "app.py//Parser", Methods: []string{"app.py//Parser//parse"}}},
		Calls:   []fileindex.Call{{Caller: "app.py", Line: 12, Column: 1, Callee: "unknown", Targets: []string{"app.py//unknown"}, Method: "unknown", Kind: fileindex.CallCall}},
	}
	call := func(line int, callee, target, method string) fileindex.Call {
		return fileindex.Call{Caller: "main.py", Line: line, Column: 1, Callee: callee, Targets: []string{target}, Receiver: firstDotted(callee), Method: method, Kind: fileindex.CallCall}
	}
	main := &fileindex.Index{
		Version: fileindex.Version, Path: "main.py", Language: langcode.Python,
		Imports: []fileindex.Import{{Line: 1, Module: "app", Alias: "app", Kind: fileindex.ImportStatic, Namespace: "app"}},
		Calls: []fileindex.Call{
			call(2, "app.parser.parse", "app//parser//parse", "parse"),
			call(3, "app.app.run", "app//app//run", "run"),
			call(4, "app.either.get", "app//either//get", "get"),
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{app, main}, Options{})
	require.NoError(t, err)

	edges := edgeStrings(g)
	assert.Contains(t, edges, "main.py -> app.py//Parser//parse member python.member")
	assert.Contains(t, edges, "main.py -> flask//Flask//run direct python.external")
	assert.Equal(t, []string{
		"app.py:12 app.py unknown-receiver unknown",
		"main.py:4 main.py unknown-receiver get",
	}, unresolvedStrings(g), "a variable with no value or two values has no known value")
	assert.NotContains(t, edges, "app.py -> app.py//unknown direct python.local")
	assert.Equal(t, NodeVariable, g.Nodes["app.py//parser"].Kind)
}

func TestFirstPartyMatchesAPackagePrefix(t *testing.T) {
	java := &namespaces{sep: ".", units: map[string]*unit{"": {}, "com.acme": {}, "com.acme.web": {}}}
	for name, want := range map[string]bool{
		"com.acme":            true,
		"com.acme.Service":    true,
		"com.acme.web.Routes": true,
		"com.acmex.Service":   false,
		"com":                 false,
		"Service":             false,
		"":                    false,
	} {
		assert.Equal(t, want, java.firstParty(name), name)
	}
	php := &namespaces{sep: `\`, units: map[string]*unit{`App\Http`: {}}}
	assert.True(t, php.firstParty(`\App\Http\Kernel`))
	assert.False(t, php.firstParty(`App\Console`))
}

func TestBuildWalksIntoTheDefaultValueOfAModule(t *testing.T) {
	job := &fileindex.Index{
		Version: fileindex.Version, Path: "job.js", Language: langcode.Javascript,
		Exports:     []fileindex.Export{{Name: "default", Target: "job.js//default", Line: 1}, {Name: "helper", Target: "job.js//helper", Line: 5}},
		Definitions: []fileindex.Definition{{Key: "job.js//default", Kind: fileindex.DefClass, Name: "default", StartLine: 1, EndLine: 3}, {Key: "job.js//default//run", Kind: fileindex.DefMethod, Name: "run", StartLine: 2, EndLine: 2, Parent: "job.js//default"}, {Key: "job.js//helper", Kind: fileindex.DefFunction, Name: "helper", StartLine: 5, EndLine: 5}},
		Classes:     []fileindex.Class{{Key: "job.js//default", Methods: []string{"job.js//default//run"}}},
	}
	main := &fileindex.Index{
		Version: fileindex.Version, Path: "main.js", Language: langcode.Javascript,
		Imports: []fileindex.Import{{Line: 1, Module: "./job", Alias: "Job", Kind: fileindex.ImportStatic, Namespace: ".//job"}},
		Calls: []fileindex.Call{
			{Caller: "main.js", Line: 2, Column: 1, Callee: "new Job().run", Targets: []string{".//job//run"}, Receiver: "new Job()", Method: "run", Kind: fileindex.CallCall},
			{Caller: "main.js", Line: 3, Column: 1, Callee: "Job.helper", Targets: []string{".//job//helper"}, Receiver: "Job", Method: "helper", Kind: fileindex.CallCall},
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{job, main}, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"main.js -> job.js//default//run member javascript.member"}, edgesOfKind(g, EdgeMember),
		"a member that the module does not name is a member of its default class")
	assert.Contains(t, edgeStrings(g), "main.js -> job.js//helper direct javascript.import", "a name of the module wins")
}

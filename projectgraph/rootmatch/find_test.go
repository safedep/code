package rootmatch

import (
	"context"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/safedep/code/fileindex"
	rules "github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testProject is a project of five files whose definitions, exports and
// calls the matchers read. The callbacks resolve a name to the key under
// the file that binds it, and a decorator of app.py to a key under flask.
func testProject(root fstest.MapFS) Project {
	def := func(path, name string, kind fileindex.DefKind, decorators ...string) fileindex.Definition {
		key := path
		if name != "" {
			key = fileindex.Join(path, name)
		}
		return fileindex.Definition{Key: key, Kind: kind, Name: strings.TrimPrefix(fileindex.Last(key), path), Decorators: decorators, Exported: true}
	}
	indexes := []*fileindex.Index{
		{Path: "app.py", Language: langcode.Python, MainGuard: true, Definitions: []fileindex.Definition{
			def("app.py", "", fileindex.DefModule),
			def("app.py", "main", fileindex.DefFunction, "@click.command()"),
			def("app.py", "index", fileindex.DefFunction, "@app.route"),
			def("app.py", "View", fileindex.DefClass),
			{Key: "app.py//View//get", Kind: fileindex.DefMethod, Name: "get", Parent: "app.py//View"},
		}, Calls: []fileindex.Call{
			{Caller: "app.py", Line: 20, Column: 1, Callee: "View.as_view", Targets: []string{"app.py//View//as_view"}, Kind: fileindex.CallCall},
		}},
		{Path: "cmd/main.go", Language: langcode.Go, Package: "main", Definitions: []fileindex.Definition{
			def("cmd/main.go", "", fileindex.DefModule),
			def("cmd/main.go", "main", fileindex.DefFunction),
			{Key: "cmd/main.go//T//main", Kind: fileindex.DefMethod, Name: "main", Parent: "cmd/main.go//T"},
		}},
		{Path: "server.js", Language: langcode.Javascript, Definitions: []fileindex.Definition{
			def("server.js", "", fileindex.DefModule),
			def("server.js", "handler", fileindex.DefFunction),
		}, Calls: []fileindex.Call{
			{Caller: "server.js", Line: 5, Column: 1, Callee: "app.get", Targets: []string{"express//Router//get"}, Kind: fileindex.CallCall, Args: []fileindex.Arg{{Shape: fileindex.ArgString}, {Shape: fileindex.ArgName, Targets: []string{"server.js//handler"}}}},
		}, Exports: []fileindex.Export{{Name: "handler", Target: "server.js//handler"}}},
		{Path: "pages/index.js", Language: langcode.Javascript, Definitions: []fileindex.Definition{
			def("pages/index.js", "", fileindex.DefModule),
			def("pages/index.js", "Page", fileindex.DefFunction),
		}, Exports: []fileindex.Export{{Name: "default", Target: "pages/index.js//Page"}}},
		{Path: "tests/test_app.py", Language: langcode.Python, Definitions: []fileindex.Definition{
			def("tests/test_app.py", "", fileindex.DefModule),
			def("tests/test_app.py", "test_main", fileindex.DefFunction),
			def("tests/test_app.py", "Suite", fileindex.DefClass),
		}},
	}
	defined := map[string]bool{}
	exports := map[string][]string{}
	var paths []string
	for _, ix := range indexes {
		paths = append(paths, ix.Path)
		for _, d := range ix.Definitions {
			defined[d.Key] = true
		}
		for _, e := range ix.Exports {
			exports[ix.Path] = append(exports[ix.Path], e.Target)
		}
	}
	files := modules.NewFiles(paths)
	return Project{
		Indexes: indexes,
		IsTest:  func(p string) bool { return strings.HasPrefix(p, "tests/") },
		Root:    root,
		Files:   files,
		State: map[langcode.Code]any{
			langcode.Python:     modules.NewPython(files, root, nil),
			langcode.Javascript: modules.NewJavaScript(files, root),
			langcode.PHP:        modules.NewComposer(files, root),
		},
		Defined: func(key string) bool { return defined[key] },
		Exports: func(p string) []string { return exports[p] },
		Export: func(p, name string) (string, bool) {
			for _, key := range exports[p] {
				if fileindex.Last(key) == name {
					return key, true
				}
			}
			return "", false
		},
		Resolve: func(_ *fileindex.Index, _ string, targets []string) []string { return targets },
		ResolveName: func(p, name string) (string, bool) {
			if p == "app.py" && name == "app.route" {
				return "flask//Flask//route", true
			}
			return "", false
		},
	}
}

func find(t *testing.T, p Project, rs ...rules.Rule) []Root {
	t.Helper()
	for _, r := range rs {
		require.NoError(t, r.Validate(), r.ID)
	}
	roots, problems, err := Find(context.Background(), p, rs)
	require.NoError(t, err)
	assert.Empty(t, problems)
	return roots
}

func keys(roots []Root) []string {
	var out []string
	for _, r := range roots {
		out = append(out, r.Key)
	}
	return out
}

func TestFindMatchers(t *testing.T) {
	p := testProject(fstest.MapFS{})
	arg := 1
	tests := []struct {
		name string
		rule rules.Rule
		want []string
	}{
		{"definition in a package", rules.Rule{ID: "r", Language: "go", Kind: rules.Main, Match: rules.Match{Definition: &rules.DefinitionMatch{Name: "main", InPackage: "main"}}}, []string{"cmd/main.go//main"}},
		{"definition under a parent", rules.Rule{ID: "r", Language: "go", Kind: rules.Main, Match: rules.Match{Definition: &rules.DefinitionMatch{Name: "main", Parent: "T"}}}, []string{"cmd/main.go//T//main"}},
		{"definition of another language", rules.Rule{ID: "r", Language: "python", Kind: rules.Main, Match: rules.Match{Definition: &rules.DefinitionMatch{Name: "main", InPackage: "main"}}}, nil},
		{"main guard", rules.Rule{ID: "r", Language: "python", Kind: rules.Main, Match: rules.Match{MainGuard: true}}, []string{"app.py"}},
		{"path and its exports", rules.Rule{ID: "r", Language: "javascript", Kind: rules.Framework, Match: rules.Match{Path: []string{"pages/*.js"}}}, []string{"pages/index.js", "pages/index.js//Page"}},
		{"path of kind main keeps the top level", rules.Rule{ID: "r", Language: "javascript", Kind: rules.Main, Match: rules.Match{Path: []string{"pages/*.js"}}}, []string{"pages/index.js"}},
		{"decorator", rules.Rule{ID: "r", Language: "python", Kind: rules.Framework, Match: rules.Match{Decorator: []string{"@click.command"}}}, []string{"app.py//main"}},
		{"decorator by call", rules.Rule{ID: "r", Language: "python", Kind: rules.Framework, Match: rules.Match{Call: &rules.CallMatch{Callee: []string{"flask//Flask//route"}}}}, []string{"app.py//index"}},
		{"call argument", rules.Rule{ID: "r", Language: "javascript", Kind: rules.Framework, Match: rules.Match{Call: &rules.CallMatch{Callee: []string{"express//Router//get"}, Arg: &arg, Shape: rules.Shapes{fileindex.ArgName}}}}, []string{"server.js//handler"}},
		{"call argument of another shape", rules.Rule{ID: "r", Language: "javascript", Kind: rules.Framework, Match: rules.Match{Call: &rules.CallMatch{Callee: []string{"express//Router//get"}, Arg: &arg, Shape: rules.Shapes{fileindex.ArgFunction}}}}, nil},
		{"call receiver", rules.Rule{ID: "r", Language: "python", Kind: rules.Framework, Match: rules.Match{Call: &rules.CallMatch{CalleeSuffix: []string{"//as_view"}, Root: rules.RootReceiver}}}, []string{"app.py//View"}},
		{"export", rules.Rule{ID: "r", Language: "javascript", Kind: rules.Framework, Match: rules.Match{Export: &rules.ExportMatch{Names: []string{"handler"}, Path: []string{"server.js"}}}}, []string{"server.js//handler"}},
		{"export in another path", rules.Rule{ID: "r", Language: "javascript", Kind: rules.Framework, Match: rules.Match{Export: &rules.ExportMatch{Names: []string{"handler"}, Path: []string{"lambda.js"}}}}, nil},
		{"test files", rules.Rule{ID: "r", Language: "any", Kind: rules.Test, Match: rules.Match{TestFile: true}}, []string{"tests/test_app.py", "tests/test_app.py//test_main"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, keys(find(t, p, tt.rule)))
		})
	}
}

func TestFindCallArgumentKeepsTheSite(t *testing.T) {
	arg := -1
	roots := find(t, testProject(fstest.MapFS{}), rules.Rule{ID: "r", Language: "javascript", Kind: rules.Framework, Match: rules.Match{Call: &rules.CallMatch{Callee: []string{"express//Router//get"}, Arg: &arg}}})
	assert.Equal(t, []Root{{Key: "server.js//handler", Kind: rules.Framework, Rule: "r", Site: Site{Path: "server.js", Line: 5, Column: 1}}}, roots)
}

func TestFindManifests(t *testing.T) {
	root := fstest.MapFS{
		"package.json":   {Data: []byte(`{"main": "server.js", "bin": {"srv": "bin/srv.js"}}`)},
		"pyproject.toml": {Data: []byte("[project.scripts]\napp = \"app:main\"\n")},
		"setup.py":       {Data: []byte(`setup(entry_points={"console_scripts": ["view = app:View.get"]})`)},
		"Procfile":       {Data: []byte("web: python app.py\nworker: node server.js\n")},
		"Dockerfile":     {Data: []byte("FROM node\nCMD [\"node\", \"/srv/pages/index.js\"]\n")},
		"composer.json":  {Data: []byte(`{"bin": ["app.py"]}`)},
	}
	p := testProject(root)
	tests := []struct {
		name string
		rule rules.Rule
		want []string
	}{
		{"package.json main and its exports", rules.Rule{ID: "r", Language: "javascript", Kind: rules.Manifest, Match: rules.Match{Manifest: &rules.ManifestMatch{File: rules.PackageJSON, Fields: []string{"main"}}}}, []string{"server.js", "server.js//handler"}},
		{"package.json bin names no file of the project", rules.Rule{ID: "r", Language: "javascript", Kind: rules.Manifest, Match: rules.Match{Manifest: &rules.ManifestMatch{File: rules.PackageJSON, Fields: []string{"bin"}}}}, nil},
		{"pyproject script and its function", rules.Rule{ID: "r", Language: "python", Kind: rules.Manifest, Match: rules.Match{Manifest: &rules.ManifestMatch{File: rules.PyProject, Fields: []string{"project.scripts"}}}}, []string{"app.py", "app.py//main"}},
		{"setup.py entry point of a method", rules.Rule{ID: "r", Language: "python", Kind: rules.Manifest, Match: rules.Match{Manifest: &rules.ManifestMatch{File: rules.SetupPy, Fields: []string{"console_scripts"}}}}, []string{"app.py", "app.py//View//get"}},
		{"Procfile commands of the language", rules.Rule{ID: "r", Language: "python", Kind: rules.Manifest, Match: rules.Match{Manifest: &rules.ManifestMatch{File: rules.Procfile}}}, []string{"app.py"}},
		{"Dockerfile command with a container path", rules.Rule{ID: "r", Language: "javascript", Kind: rules.Manifest, Match: rules.Match{Manifest: &rules.ManifestMatch{File: rules.Dockerfile}}}, []string{"pages/index.js"}},
		{"composer bin", rules.Rule{ID: "r", Language: "python", Kind: rules.Manifest, Match: rules.Match{Manifest: &rules.ManifestMatch{File: rules.Composer, Fields: []string{"bin"}}}}, []string{"app.py"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, keys(find(t, p, tt.rule)))
		})
	}
}

func TestFindReportsAnUnreadableManifest(t *testing.T) {
	p := testProject(fstest.MapFS{"Procfile": {Mode: 0o000 | 1<<31}})
	_, problems, err := Find(context.Background(), p, []rules.Rule{{ID: "r", Language: "python", Kind: rules.Manifest, Match: rules.Match{Manifest: &rules.ManifestMatch{File: rules.Procfile}}}})
	require.NoError(t, err)
	require.Len(t, problems, 1)
	assert.Equal(t, "Procfile", problems[0].Path)
}

func TestFindStopsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := Find(ctx, testProject(fstest.MapFS{}), rules.Default())
	assert.ErrorIs(t, err, context.Canceled)
}

func TestFindSortsAndKeepsTheFirstSite(t *testing.T) {
	p := testProject(fstest.MapFS{})
	guard := rules.Rule{ID: "b", Language: "python", Kind: rules.Main, Match: rules.Match{MainGuard: true}}
	decorator := rules.Rule{ID: "a", Language: "python", Kind: rules.Framework, Match: rules.Match{Decorator: []string{"@click.command"}}}
	roots := find(t, p, guard, decorator, guard)
	assert.True(t, slices.IsSortedFunc(roots, func(a, b Root) int { return strings.Compare(a.Key, b.Key) }))
	assert.Equal(t, []string{"app.py", "app.py//main"}, keys(roots), "a rule that runs twice marks a key once")
}

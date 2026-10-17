package projectgraph

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/projectgraph/modules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// importer is a JavaScript file of the project that imports names of a
// package and calls each one on its own line.
func importer(module string, names ...string) *fileindex.Index {
	ix := &fileindex.Index{Version: fileindex.Version, Path: "main.js", Language: langcode.Javascript}
	for i, name := range names {
		ns := fileindex.Join(fileindex.Namespace(langcode.Javascript, module), name)
		ix.Imports = append(ix.Imports, fileindex.Import{Line: 1, Module: module, Item: name, Alias: name, Kind: fileindex.ImportStatic, Namespace: ns})
		ix.Calls = append(ix.Calls, fileindex.Call{Caller: "main.js", Line: i + 2, Column: 1, Callee: name, Targets: []string{ns}, Kind: fileindex.CallCall})
	}
	return ix
}

// unitFile is a JavaScript file of a unit that defines and exports the
// functions.
func unitFile(unit, path string, names ...string) *fileindex.Index {
	p := fileindex.UnitPath(unit, path)
	ix := &fileindex.Index{Version: fileindex.Version, Path: p, Language: langcode.Javascript, Unit: unit}
	for i, name := range names {
		key := fileindex.Join(p, name)
		ix.Definitions = append(ix.Definitions, def(fileindex.DefFunction, key, i+1))
		ix.Exports = append(ix.Exports, fileindex.Export{Name: name, Target: key, Line: i + 1})
	}
	return ix
}

// typescript makes a JavaScript index a TypeScript one, with the path of a
// TypeScript file.
func typescript(ix *fileindex.Index) *fileindex.Index {
	old := ix.Path
	if !strings.HasSuffix(old, ".d.ts") {
		ix.Path = strings.TrimSuffix(old, ".js") + ".ts"
	}
	ix.Language = langcode.Typescript
	re := strings.NewReplacer(old, ix.Path)
	for i := range ix.Calls {
		ix.Calls[i].Caller = re.Replace(ix.Calls[i].Caller)
	}
	for i := range ix.Definitions {
		ix.Definitions[i].Key = re.Replace(ix.Definitions[i].Key)
	}
	for i := range ix.Exports {
		ix.Exports[i].Target = re.Replace(ix.Exports[i].Target)
	}
	return ix
}

func manifest(text string) fs.FS {
	return fstest.MapFS{"package.json": {Data: []byte(text)}}
}

func TestBuildLinksAnExternalKeyIntoAUnit(t *testing.T) {
	tests := []struct {
		name       string
		indexes    []*fileindex.Index
		units      map[string]fs.FS
		files      map[string]fileindex.FileStatus
		edges      []string
		unresolved []string
	}{
		{
			name:    "the main of package.json gives the definition",
			indexes: []*fileindex.Index{importer("lodash", "template"), unitFile("npm:lodash@4.17.21", "lib/main.js", "template")},
			units:   map[string]fs.FS{"npm:lodash@4.17.21": manifest(`{"name": "lodash", "main": "lib/main.js"}`)},
			edges: []string{
				"lodash//template -> npm:lodash@4.17.21//lib/main.js//template package javascript.package.main",
				"main.js -> npm:lodash@4.17.21//lib/main.js import javascript.import.package",
				"main.js -> lodash//template direct javascript.external",
			},
		},
		{
			name:    "the exports field picks the file of a subpath",
			indexes: []*fileindex.Index{importer("lodash/fp", "map"), unitFile("npm:lodash@4.17.21", "fp/index.mjs", "map")},
			units:   map[string]fs.FS{"npm:lodash@4.17.21": manifest(`{"name": "lodash", "exports": {"./fp": {"import": "./fp/index.mjs"}}}`)},
			edges: []string{
				"lodash//fp//map -> npm:lodash@4.17.21//fp/index.mjs//map package javascript.package.exports-import",
				"main.js -> npm:lodash@4.17.21//fp/index.mjs import javascript.import.package",
				"main.js -> lodash//fp//map direct javascript.external",
			},
		},
		{
			name: "each installed version of a package gets an edge",
			indexes: []*fileindex.Index{
				importer("lodash", "template"),
				unitFile("npm:lodash@3.10.1", "index.js", "template"),
				unitFile("npm:lodash@4.17.21", "index.js", "template"),
			},
			edges: []string{
				"lodash//template -> npm:lodash@3.10.1//index.js//template package javascript.package.index",
				"lodash//template -> npm:lodash@4.17.21//index.js//template package javascript.package.index",
				"main.js -> npm:lodash@3.10.1//index.js import javascript.import.package",
				"main.js -> npm:lodash@4.17.21//index.js import javascript.import.package",
				"main.js -> lodash//template direct javascript.external",
			},
		},
		{
			name: "a TypeScript importer gets the JavaScript of a package and not its declarations",
			indexes: []*fileindex.Index{
				typescript(importer("lodash", "template")),
				typescript(unitFile("npm:lodash@4.17.21", "index.d.ts", "template")),
				unitFile("npm:lodash@4.17.21", "index.js", "template"),
			},
			edges: []string{
				"lodash//template -> npm:lodash@4.17.21//index.js//template package typescript.package.index",
				"main.ts -> npm:lodash@4.17.21//index.js import typescript.import.package",
				"main.ts -> lodash//template direct typescript.external",
			},
		},
		{
			name:    "a unit of another name gives nothing",
			indexes: []*fileindex.Index{importer("lodash", "template"), unitFile("npm:underscore@1.13.6", "index.js", "template")},
			edges:   []string{"main.js -> lodash//template direct javascript.external"},
		},
		{
			name:       "a name that the package does not define enters the unit at no known definition",
			indexes:    []*fileindex.Index{importer("lodash", "chain"), unitFile("npm:lodash@4.17.21", "index.js", "template")},
			edges:      []string{"main.js -> npm:lodash@4.17.21//index.js import javascript.import.package", "main.js -> lodash//chain direct javascript.external"},
			unresolved: []string{"npm:lodash@4.17.21:0 lodash//chain no-definition "},
		},
		{
			name:       "a subpath that the package does not have is a record at the import",
			indexes:    []*fileindex.Index{importer("lodash/nope", "x"), unitFile("npm:lodash@4.17.21", "index.js", "template")},
			edges:      []string{"main.js -> lodash//nope//x direct javascript.external"},
			unresolved: []string{"main.js:1 main.js no-definition ", "npm:lodash@4.17.21:0 lodash//nope//x no-definition "},
		},
		{
			name:    "a missing file of the unit is the end of the package edge",
			indexes: []*fileindex.Index{importer("lodash", "template")},
			files:   map[string]fileindex.FileStatus{"npm:lodash@4.17.21//index.js": fileindex.FileMissing},
			edges: []string{
				"lodash//template -> npm:lodash@4.17.21//index.js package javascript.package.index",
				"main.js -> npm:lodash@4.17.21//index.js import javascript.import.package",
				"main.js -> lodash//template direct javascript.external",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := Build(context.Background(), tt.indexes, Options{Units: tt.units, Files: tt.files})
			require.NoError(t, err)
			assert.Equal(t, tt.edges, edgeStrings(g))
			assert.Equal(t, tt.unresolved, nonNil(unresolvedStrings(g)))
			for _, u := range g.Unresolved {
				assert.Equal(t, "npm:lodash@4.17.21", u.Unit, "a record of an entry into a unit names the unit")
			}
			for _, r := range g.Roots {
				assert.Empty(t, fileindex.PackageOf(r.Key), "dependency code holds no root")
			}
		})
	}
}

func TestBuildLinksAUnitOfPythonAndOfGo(t *testing.T) {
	py := &fileindex.Index{
		Version: fileindex.Version, Path: "app.py", Language: langcode.Python,
		Imports: []fileindex.Import{{Line: 1, Module: "requests", Kind: fileindex.ImportStatic, Namespace: "requests"}},
		Calls:   []fileindex.Call{{Caller: "app.py", Line: 2, Column: 1, Callee: "requests.get", Targets: []string{"requests//get"}, Receiver: "requests", Method: "get", Kind: fileindex.CallCall}},
	}
	const reqs = "pypi:requests@2.31.0"
	init := &fileindex.Index{
		Version: fileindex.Version, Path: reqs + "//requests/__init__.py", Language: langcode.Python, Unit: reqs,
		Imports: []fileindex.Import{{Line: 1, Module: ".api", Item: "get", Alias: "get", Kind: fileindex.ImportStatic, Namespace: "api//get"}},
	}
	api := &fileindex.Index{
		Version: fileindex.Version, Path: reqs + "//requests/api.py", Language: langcode.Python, Unit: reqs,
		Definitions: []fileindex.Definition{def(fileindex.DefFunction, reqs+"//requests/api.py//get", 1)},
	}

	gomain := &fileindex.Index{
		Version: fileindex.Version, Path: "main.go", Language: langcode.Go, Package: "main",
		Imports: []fileindex.Import{{Line: 1, Module: "github.com/acme/kit/v2/sub", Alias: "sub", Kind: fileindex.ImportStatic, Namespace: "github.com//acme//kit//v2//sub"}},
		Calls:   []fileindex.Call{{Caller: "main.go", Line: 2, Column: 1, Callee: "sub.Run", Targets: []string{"github.com//acme//kit//v2//sub//Run"}, Receiver: "sub", Method: "Run", Kind: fileindex.CallCall}},
	}
	gofile := func(unit string) *fileindex.Index {
		return &fileindex.Index{
			Version: fileindex.Version, Path: unit + "//sub/run.go", Language: langcode.Go, Package: "sub", Unit: unit,
			Definitions: []fileindex.Definition{def(fileindex.DefFunction, unit+"//sub/run.go//Run", 1)},
		}
	}
	g, err := Build(context.Background(), []*fileindex.Index{py, init, api, gomain, gofile("go:github.com/acme/kit@v1.0.0"), gofile("go:github.com/acme/kit/v2@v2.0.0")}, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"github.com//acme//kit//v2//sub//Run -> go:github.com/acme/kit/v2@v2.0.0//sub/run.go//Run package go.package.module",
		"requests//get -> pypi:requests@2.31.0//requests/__init__.py//get package python.package.absolute",
	}, edgesOfKind(g, EdgePackage), "the package tree of Python and the longest module path of Go give the definition")
	assert.Contains(t, edgeStrings(g), "app.py -> pypi:requests@2.31.0//requests/__init__.py import python.import.package")
}

func TestBuildChecksTheUnitsOfTheIndexes(t *testing.T) {
	bad := unitFile("npm:lodash@4.17.21", "index.js")
	bad.Unit = ""
	_, err := Build(context.Background(), []*fileindex.Index{bad}, Options{})
	assert.ErrorContains(t, err, `has the unit "", and its path names the unit "npm:lodash@4.17.21"`)

	java := &fileindex.Index{Version: fileindex.Version, Path: "maven:org.acme:kit@1.0//Kit.java", Language: langcode.Java, Unit: "maven:org.acme:kit@1.0"}
	g, err := Build(context.Background(), []*fileindex.Index{java, importer("kit", "run")}, Options{})
	require.NoError(t, err)
	assert.NotContains(t, g.Nodes, java.Path)
	assert.Equal(t, []modules.Problem{{Path: "maven:org.acme:kit@1.0", Text: "the link step does not link the java code of a dependency"}}, g.Problems)
}

func TestBuildResolvesTheImportsOfAUnitInsideTheUnit(t *testing.T) {
	const unit = "npm:kit@1.0.0"
	main := unitFile(unit, "index.js")
	main.Imports = []fileindex.Import{{Line: 1, Module: "./util", Item: "helper", Alias: "helper", Kind: fileindex.ImportStatic, Namespace: ".//util//helper"}}
	main.Calls = []fileindex.Call{{Caller: main.Path, Line: 2, Column: 1, Callee: "helper", Targets: []string{".//util//helper"}, Kind: fileindex.CallCall}}
	util := unitFile(unit, "util.js", "helper")
	project := unitFile("", "util.js", "helper")
	g, err := Build(context.Background(), []*fileindex.Index{main, util, project}, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"npm:kit@1.0.0//index.js -> npm:kit@1.0.0//util.js import javascript.import.relative",
		"npm:kit@1.0.0//index.js -> npm:kit@1.0.0//util.js//helper direct javascript.import",
	}, edgeStrings(g), "a relative import of a unit names a file of the unit, not the file of the project with that path")
	assert.Equal(t, []Root{{Key: "util.js", Kind: RootLibrarySurface, Rule: string(RootLibrarySurface), Site: Site{Path: "util.js", Line: 1}}}, g.Roots,
		"only the files of the project give roots")
}

func TestBuildSurfaceLinksEachExportedSymbol(t *testing.T) {
	const unit = "npm:kit@1.0.0"
	g, err := Build(context.Background(), []*fileindex.Index{unitFile(unit, "index.js", "run", "stop"), unitFile(unit, "lib/extra.js", "more")}, Options{Surface: true})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"kit//index//run -> npm:kit@1.0.0//index.js//run package javascript.package.subpath",
		"kit//index//stop -> npm:kit@1.0.0//index.js//stop package javascript.package.subpath",
		"kit//lib//extra//more -> npm:kit@1.0.0//lib/extra.js//more package javascript.package.subpath",
		"kit//run -> npm:kit@1.0.0//index.js//run package javascript.package.index",
		"kit//stop -> npm:kit@1.0.0//index.js//stop package javascript.package.index",
	}, edgesOfKind(g, EdgePackage), "a package with no exports field exports its main and each file by path")
}

func TestBuildLinksAModuleOfAPythonNamespacePackage(t *testing.T) {
	app := &fileindex.Index{
		Version: fileindex.Version, Path: "app.py", Language: langcode.Python,
		Imports: []fileindex.Import{{Line: 1, Module: "google.protobuf", Item: "parse", Alias: "parse", Kind: fileindex.ImportStatic, Namespace: "google//protobuf//parse"}},
		Calls:   []fileindex.Call{{Caller: "app.py", Line: 2, Column: 1, Callee: "parse", Targets: []string{"google//protobuf//parse"}, Kind: fileindex.CallCall}},
	}
	pb := &fileindex.Index{
		Version: fileindex.Version, Path: "pypi:protobuf@4.25.0//google/protobuf/__init__.py", Language: langcode.Python, Unit: "pypi:protobuf@4.25.0",
		Definitions: []fileindex.Definition{def(fileindex.DefFunction, "pypi:protobuf@4.25.0//google/protobuf/__init__.py//parse", 1)},
	}
	auth := &fileindex.Index{Version: fileindex.Version, Path: "pypi:google-auth@2.0.0//google/auth/__init__.py", Language: langcode.Python, Unit: "pypi:google-auth@2.0.0"}
	g, err := Build(context.Background(), []*fileindex.Index{app, pb, auth}, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"google//protobuf//parse -> pypi:protobuf@4.25.0//google/protobuf/__init__.py//parse package python.package.absolute",
	}, edgesOfKind(g, EdgePackage))
	assert.Empty(t, g.Unresolved, "a distribution that shares the namespace package and lacks the module does not miss it")
}

package projectgraph

import (
	"context"
	"fmt"
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/roots"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFrameworkRoots(t *testing.T) {
	route := def(fileindex.DefFunction, "app.py//route", 8)
	route.Decorators = []string{"@app.get"}
	items := def(fileindex.DefFunction, "app.py//items", 12)
	items.Decorators = []string{"@api.get"}
	py := &fileindex.Index{
		Version: fileindex.Version, Path: "app.py", Language: langcode.Python,
		Imports: []fileindex.Import{
			{Line: 1, Module: "flask", Item: "Flask", Alias: "Flask", Kind: fileindex.ImportStatic, Namespace: "flask//Flask"},
			{Line: 2, Module: "fastapi", Item: "FastAPI", Alias: "FastAPI", Kind: fileindex.ImportStatic, Namespace: "fastapi//FastAPI"},
		},
		Definitions: []fileindex.Definition{
			{Key: "app.py//app", Kind: fileindex.DefVariable, Name: "app", StartLine: 4, EndLine: 4, Values: []string{"flask//Flask"}},
			{Key: "app.py//api", Kind: fileindex.DefVariable, Name: "api", StartLine: 5, EndLine: 5, Values: []string{"fastapi//FastAPI"}},
			route, items,
		},
	}
	handler := def(fileindex.DefFunction, "server.js//handler", 3)
	js := &fileindex.Index{
		Version: fileindex.Version, Path: "server.js", Language: langcode.Javascript,
		Imports:     []fileindex.Import{{Line: 1, Module: "express", Alias: "express", Kind: fileindex.ImportStatic, Namespace: "express"}},
		Definitions: []fileindex.Definition{handler, def(fileindex.DefFunction, "server.js//other", 4)},
		Calls: []fileindex.Call{
			{Caller: "server.js", Line: 6, Column: 1, Callee: "app.get", Targets: []string{"express//get"}, Receiver: "app", Method: "get", Kind: fileindex.CallCall,
				Args: []fileindex.Arg{{Shape: fileindex.ArgString}, {Shape: fileindex.ArgName, Targets: []string{"server.js//handler"}}}},
			{Caller: "server.js", Line: 7, Column: 1, Callee: "app.listen", Targets: []string{"express//listen"}, Receiver: "app", Method: "listen", Kind: fileindex.CallCall,
				Args: []fileindex.Arg{{Shape: fileindex.ArgName, Targets: []string{"server.js//other"}}}},
			{Caller: "server.js", Line: 8, Column: 1, Callee: "app.get", Targets: []string{"express//get"}, Receiver: "app", Method: "get", Kind: fileindex.CallCall,
				Args: []fileindex.Arg{{Shape: fileindex.ArgComputed, Targets: []string{"server.js//other"}}}},
			{Caller: "server.js", Line: 9, Column: 1, Callee: "app.get", Targets: []string{"express//get"}, Receiver: "app", Method: "get", Kind: fileindex.CallCall},
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{py, js}, Options{})
	require.NoError(t, err)

	got := map[string]string{}
	for _, r := range g.Roots {
		assert.Equal(t, roots.Framework, r.Kind)
		got[r.Key] = r.Rule + " " + site(r.Site)
	}
	assert.Equal(t, map[string]string{
		"app.py//route":      "python.flask.route app.py:8",
		"app.py//items":      "python.fastapi.route app.py:12",
		"server.js//handler": "node.express.route server.js:6",
	}, got, "the receiver tells Flask from FastAPI, and a callee, a shape or an argument that does not match marks nothing")
}

func site(s Site) string {
	return fmt.Sprintf("%s:%d", s.Path, s.Line)
}

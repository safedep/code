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
		Definitions: []fileindex.Definition{handler, def(fileindex.DefFunction, "server.js//other", 4), def(fileindex.DefFunction, "server.js//other//<anonymous:11>", 11)},
		Calls: []fileindex.Call{
			{Caller: "server.js", Line: 6, Column: 1, Callee: "app.get", Targets: []string{"express//get"}, Receiver: "app", Method: "get", Kind: fileindex.CallCall,
				Args: []fileindex.Arg{{Shape: fileindex.ArgString}, {Shape: fileindex.ArgName, Targets: []string{"server.js//handler"}}}},
			{Caller: "server.js", Line: 7, Column: 1, Callee: "app.listen", Targets: []string{"express//listen"}, Receiver: "app", Method: "listen", Kind: fileindex.CallCall,
				Args: []fileindex.Arg{{Shape: fileindex.ArgName, Targets: []string{"server.js//other"}}}},
			{Caller: "server.js", Line: 8, Column: 1, Callee: "app.get", Targets: []string{"express//get"}, Receiver: "app", Method: "get", Kind: fileindex.CallCall,
				Args: []fileindex.Arg{{Shape: fileindex.ArgComputed, Targets: []string{"server.js//other"}}}},
			{Caller: "server.js", Line: 9, Column: 1, Callee: "app.get", Targets: []string{"express//get"}, Receiver: "app", Method: "get", Kind: fileindex.CallCall},
			{Caller: "server.js//other", Line: 10, Column: 1, Callee: "app.get", Targets: []string{"express//get"}, Receiver: "app", Method: "get", Kind: fileindex.CallCall,
				Args: []fileindex.Arg{{Shape: fileindex.ArgFunction}}},
			{Caller: "server.js//other", Line: 11, Column: 1, Callee: "app.get", Targets: []string{"express//get"}, Receiver: "app", Method: "get", Kind: fileindex.CallCall,
				Args: []fileindex.Arg{{Shape: fileindex.ArgFunction, Targets: []string{"server.js//other//<anonymous:11>"}}}},
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
		"app.py//route":                    "python.flask.route app.py:8",
		"app.py//items":                    "python.fastapi.route app.py:12",
		"server.js//handler":               "node.express.route server.js:6",
		"server.js//other//<anonymous:11>": "node.express.route server.js:11",
	}, got, "the receiver tells Flask from FastAPI, a callee, a shape or an argument that does not match marks nothing, and the scope of a call is no root")
	assert.Contains(t, edgesOfKind(g, EdgeDirect), "server.js//other -> server.js//other//<anonymous:11> direct javascript.argument-function",
		"the scope that passes a function literal to a call runs the literal")
}

func site(s Site) string {
	return fmt.Sprintf("%s:%d", s.Path, s.Line)
}

func TestBuildJavaClassInit(t *testing.T) {
	init := def(fileindex.DefMethod, "A.java//A//<clinit>", 2)
	init.Static = true
	ctor := def(fileindex.DefConstructor, "A.java//A//<init>", 3)
	ctor.Parent = "A.java//A"
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: "A.java", Language: langcode.Java, Package: "app",
		Definitions: []fileindex.Definition{def(fileindex.DefClass, "A.java//A", 1), init, ctor, def(fileindex.DefMethod, "A.java//A//run", 4)},
		Classes:     []fileindex.Class{{Key: "A.java//A", Methods: []string{"A.java//A//<clinit>", "A.java//A//<init>", "A.java//A//run"}}},
	}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"A.java//A -> A.java//A//<clinit> direct java.class-init",
		"A.java//A -> A.java//A//<init> direct java.constructor",
		"A.java//A//<init> -> A.java//A//<clinit> direct java.class-init",
		"A.java//A//run -> A.java//A//<clinit> direct java.class-init",
	}, edgesOfKind(g, EdgeDirect), "a static call reaches the static initializer and not the constructor")
}

func TestBuildReceiverRoots(t *testing.T) {
	views := &fileindex.Index{
		Version: fileindex.Version, Path: "views.py", Language: langcode.Python,
		Imports:     []fileindex.Import{{Line: 1, Module: "django.views", Item: "View", Alias: "View", Kind: fileindex.ImportStatic, Namespace: "django//views//View"}},
		Definitions: []fileindex.Definition{def(fileindex.DefClass, "views.py//Report", 3), def(fileindex.DefMethod, "views.py//Report//get", 4), def(fileindex.DefClass, "views.py//Plain", 6)},
		Classes:     []fileindex.Class{{Key: "views.py//Report", Bases: []string{"View"}, Methods: []string{"views.py//Report//get"}}, {Key: "views.py//Plain"}},
	}
	urls := &fileindex.Index{
		Version: fileindex.Version, Path: "urls.py", Language: langcode.Python,
		Imports: []fileindex.Import{{Line: 1, Module: "views", Alias: "views", Kind: fileindex.ImportStatic, Namespace: "views"}},
		Calls: []fileindex.Call{
			{Caller: "urls.py", Line: 3, Column: 10, Callee: "views.Report.as_view", Targets: []string{"views//Report//as_view"}, Receiver: "views.Report", Method: "as_view", Kind: fileindex.CallCall},
			{Caller: "urls.py", Line: 4, Column: 10, Callee: "views.Plain.as_view", Targets: []string{"views//Plain//as_view"}, Receiver: "views.Plain", Method: "as_view", Kind: fileindex.CallCall},
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{views, urls}, Options{})
	require.NoError(t, err)
	var got []string
	for _, r := range g.Roots {
		if r.Kind == roots.Framework {
			got = append(got, r.Key+" "+r.Rule+" "+site(r.Site))
		}
	}
	assert.Equal(t, []string{"views.py//Report python.django.view urls.py:3"}, got, "the as_view of a class with no Django base is no Django view")
}

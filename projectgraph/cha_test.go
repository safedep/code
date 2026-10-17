package projectgraph

import (
	"context"
	"fmt"
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func def(kind fileindex.DefKind, key string, line int) fileindex.Definition {
	d := fileindex.Definition{Key: key, Kind: kind, Name: fileindex.Last(key), StartLine: line, EndLine: line}
	if kind == fileindex.DefMethod {
		d.Parent = fileindex.Parent(key)
	}
	return d
}

func edgesOfKind(g *Graph, kinds ...EdgeKind) []string {
	var out []string
	for _, e := range g.Edges {
		for _, k := range kinds {
			if e.Kind == k {
				out = append(out, fmt.Sprintf("%s -> %s %s %s", e.From, e.To, e.Kind, e.Resolver))
			}
		}
	}
	return out
}

// pythonClasses is a diamond: B(M, A), A(Base), C(A). The C3 MRO of B is
// B, M, A, Base.
func pythonClasses() *fileindex.Index {
	ix := &fileindex.Index{Version: fileindex.Version, Path: "app.py", Language: langcode.Python}
	for i, key := range []string{"app.py//Base", "app.py//A", "app.py//M", "app.py//B", "app.py//C"} {
		ix.Definitions = append(ix.Definitions, def(fileindex.DefClass, key, 10*i+1))
	}
	for i, key := range []string{"app.py//Base//run", "app.py//Base//step", "app.py//A//step", "app.py//M//step", "app.py//C//other"} {
		ix.Definitions = append(ix.Definitions, def(fileindex.DefMethod, key, 10*i+2))
	}
	ix.Classes = []fileindex.Class{
		{Key: "app.py//Base", Methods: []string{"app.py//Base//run", "app.py//Base//step"}},
		{Key: "app.py//A", Bases: []string{"Base"}, Methods: []string{"app.py//A//step"}},
		{Key: "app.py//M", Methods: []string{"app.py//M//step"}},
		{Key: "app.py//B", Bases: []string{"M", "A"}},
		{Key: "app.py//C", Bases: []string{"A"}, Methods: []string{"app.py//C//other"}},
	}
	return ix
}

func TestBuildCHA(t *testing.T) {
	ix := pythonClasses()
	ix.Calls = []fileindex.Call{
		{Caller: "app.py//Base//run", Line: 3, Column: 9, Callee: "self.step", Targets: []string{"app.py//Base//step"}, Receiver: "self", Method: "step", Kind: fileindex.CallCall},
		{Caller: "app.py//Base//run", Line: 4, Column: 9, Callee: "self.missing", Targets: []string{"app.py//Base//self//missing"}, Receiver: "self", Method: "missing", Kind: fileindex.CallCall},
		{Caller: "app.py//C//other", Line: 43, Column: 9, Callee: "self.run", Targets: []string{"app.py//C//self//run"}, Receiver: "self", Method: "run", Kind: fileindex.CallCall},
		{Caller: "app.py", Line: 60, Column: 1, Callee: "A().step", Targets: []string{"app.py//A//step"}, Receiver: "A()", Method: "step", Kind: fileindex.CallCall},
		{Caller: "app.py", Line: 61, Column: 1, Callee: "Base.step", Targets: []string{"app.py//Base//step"}, Receiver: "Base", Method: "step", Kind: fileindex.CallCall},
	}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		"app.py -> app.py//A//step member python.member",
		"app.py -> app.py//Base//step member python.member",
		"app.py//Base//run -> app.py//Base//step member python.member",
		"app.py//Base//run -> app.py//A//step cha python.cha",
		"app.py//Base//run -> app.py//M//step cha python.cha",
		"app.py//C//other -> app.py//Base//run member python.member",
	}, edgesOfKind(g, EdgeMember, EdgeCHA), "a call on self reaches the override of each subclass by its MRO, and an exact or a class receiver gets no cha edge")
	for _, e := range g.Edges {
		if e.Kind == EdgeCHA {
			assert.Equal(t, ConfidenceMedium, e.Confidence)
		}
	}
	assert.Equal(t, []string{"app.py:4 app.py//Base//run no-definition missing"}, unresolvedStrings(g),
		"a narrowed call that no class of the hierarchy defines stays a record")
	assert.Empty(t, edgesOfKind(g, EdgeName, EdgeField), "a narrowed site gets no name edge")
}

func TestMRO(t *testing.T) {
	ix := &fileindex.Index{Version: fileindex.Version, Path: "m.py", Language: langcode.Python}
	bases := map[string][]string{
		"O": nil, "A": {"O"}, "B": {"O"}, "C": {"O"}, "D": {"O"}, "E": {"O"},
		"K1": {"A", "B", "C"}, "K2": {"D", "B", "E"}, "K3": {"D", "A"}, "Z": {"K1", "K2", "K3"},
		"X": {"A", "B"}, "Y": {"B", "A"}, "Bad": {"X", "Y"},
	}
	for name, bs := range bases {
		key := fileindex.Join("m.py", name)
		ix.Definitions = append(ix.Definitions, def(fileindex.DefClass, key, 1))
		ix.Classes = append(ix.Classes, fileindex.Class{Key: key, Bases: bs})
	}
	b, err := newBuilder([]*fileindex.Index{ix}, Options{})
	require.NoError(t, err)
	require.NoError(t, b.link(context.Background()))

	names := func(class string) []string {
		var out []string
		for _, k := range b.mro(fileindex.Join("m.py", class)) {
			out = append(out, fileindex.Last(k))
		}
		return out
	}
	assert.Equal(t, []string{"Z", "K1", "K2", "K3", "D", "A", "B", "C", "E", "O"}, names("Z"))
	assert.Equal(t, []string{"Bad", "X", "A", "O", "B", "Y"}, names("Bad"), "bases with no C3 order fall back to depth first")
}

func TestBuildDeclaredTypes(t *testing.T) {
	store := &fileindex.Index{
		Version: fileindex.Version, Path: "src/store.ts", Language: langcode.Typescript,
		Exports: []fileindex.Export{{Name: "Store", Target: "src/store.ts//Store", Line: 1}, {Name: "Base", Target: "src/store.ts//Base", Line: 2}},
		Definitions: []fileindex.Definition{
			def(fileindex.DefInterface, "src/store.ts//Store", 1),
			def(fileindex.DefClass, "src/store.ts//Base", 2),
			def(fileindex.DefMethod, "src/store.ts//Base//load", 3),
		},
		Classes: []fileindex.Class{{Key: "src/store.ts//Store"}, {Key: "src/store.ts//Base", Methods: []string{"src/store.ts//Base//load"}}},
	}
	run := def(fileindex.DefFunction, "src/main.ts//run", 20)
	run.EndLine = 25
	run.Params = []fileindex.Typed{{Name: "base", Type: "Base"}, {Name: "res", Type: "Response"}}
	main := &fileindex.Index{
		Version: fileindex.Version, Path: "src/main.ts", Language: langcode.Typescript,
		Imports: []fileindex.Import{
			{Line: 1, Module: "./store", Item: "Store", Alias: "Store", Kind: fileindex.ImportTypeOnly, Namespace: ".//store//Store"},
			{Line: 2, Module: "./store", Item: "Base", Alias: "Base", Kind: fileindex.ImportStatic, Namespace: ".//store//Base"},
			{Line: 3, Module: "express", Item: "Response", Alias: "Response", Kind: fileindex.ImportStatic, Namespace: "express//Response"},
		},
		Definitions: []fileindex.Definition{
			def(fileindex.DefClass, "src/main.ts//App", 5),
			def(fileindex.DefMethod, "src/main.ts//App//get", 6),
			def(fileindex.DefMethod, "src/main.ts//App//read", 7),
			def(fileindex.DefClass, "src/main.ts//Sub", 10),
			def(fileindex.DefMethod, "src/main.ts//Sub//load", 11),
			run,
		},
		Classes: []fileindex.Class{
			{Key: "src/main.ts//App", Bases: []string{"Store"}, Methods: []string{"src/main.ts//App//get", "src/main.ts//App//read"},
				Fields: []fileindex.Typed{{Name: "store", Type: "Store"}}},
			{Key: "src/main.ts//Sub", Bases: []string{"Base"}, Methods: []string{"src/main.ts//Sub//load"}},
		},
		Calls: []fileindex.Call{
			{Caller: "src/main.ts//App//read", Line: 8, Column: 5, Callee: "this.store.get", Targets: []string{"src/main.ts//App//this//store//get"}, Receiver: "this.store", Method: "get", Kind: fileindex.CallCall},
			{Caller: "src/main.ts//run", Line: 21, Column: 3, Callee: "base.load", Targets: []string{"base//load"}, Receiver: "base", Method: "load", Kind: fileindex.CallCall},
			{Caller: "src/main.ts//run", Line: 22, Column: 3, Callee: "res.json", Targets: []string{"res//json"}, Receiver: "res", Method: "json", Kind: fileindex.CallCall},
		},
		Unresolved: []fileindex.Unresolved{
			{Caller: "src/main.ts//run", Line: 21, Column: 3, Text: "base.load", Receiver: "base", Method: "load", Reason: fileindex.UnresolvedUnknownReceiver},
			{Caller: "src/main.ts//run", Line: 22, Column: 3, Text: "res.json", Receiver: "res", Method: "json", Reason: fileindex.UnresolvedUnknownReceiver},
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{store, main}, Options{})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		"src/main.ts//App//read -> src/main.ts//App//get cha typescript.cha",
		"src/main.ts//run -> src/store.ts//Base//load member typescript.declared-type",
		"src/main.ts//run -> src/main.ts//Sub//load cha typescript.cha",
	}, edgesOfKind(g, EdgeMember, EdgeCHA, EdgeField), "a field of an interface type and a parameter of a class type narrow the receiver")
	assert.Equal(t, NodeInterface, g.Nodes["src/store.ts//Store"].Kind)
	assert.Equal(t, []string{
		"src/main.ts:8 src/main.ts//App//read no-definition get",
		"src/main.ts:22 src/main.ts//run unknown-receiver json",
	}, unresolvedStrings(g), "an interface has no code for the method, and a type of a dependency narrows nothing")
}

func TestBuildFieldEdges(t *testing.T) {
	lib := &fileindex.Index{
		Version: fileindex.Version, Path: "lib.js", Language: langcode.Javascript,
		Exports: []fileindex.Export{{Name: "parse", Target: "lib.js//parse", Line: 9}},
		Definitions: []fileindex.Definition{
			def(fileindex.DefClass, "lib.js//Yaml", 1),
			def(fileindex.DefMethod, "lib.js//Yaml//parse", 2),
			def(fileindex.DefMethod, "lib.js//json//parse", 4),
			def(fileindex.DefFunction, "lib.js//parse", 6),
			def(fileindex.DefFunction, "lib.js//helper//parse", 7),
			def(fileindex.DefClass, "lib.js//parse2", 8),
		},
		Classes: []fileindex.Class{{Key: "lib.js//Yaml", Methods: []string{"lib.js//Yaml//parse"}}},
	}
	lib.Definitions[3].Exported = true
	many := &fileindex.Index{Version: fileindex.Version, Path: "many.js", Language: langcode.Javascript}
	for i := range 25 {
		many.Definitions = append(many.Definitions, def(fileindex.DefMethod, fmt.Sprintf("many.js//o%d//get", i), i+1))
	}
	main := &fileindex.Index{
		Version: fileindex.Version, Path: "main.js", Language: langcode.Javascript,
		Unresolved: []fileindex.Unresolved{
			{Caller: "main.js", Line: 1, Column: 1, Text: "p.parse", Receiver: "p", Method: "parse", Reason: fileindex.UnresolvedUnknownReceiver},
			{Caller: "main.js", Line: 2, Column: 1, Text: "parse", Method: "parse", Reason: fileindex.UnresolvedUnknownReceiver},
			{Caller: "main.js", Line: 3, Column: 1, Text: "o.get", Receiver: "o", Method: "get", Reason: fileindex.UnresolvedUnknownReceiver},
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{lib, many, main}, Options{})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		"main.js -> lib.js//Yaml//parse field javascript.field",
		"main.js -> lib.js//json//parse field javascript.field",
		"main.js -> lib.js//parse field javascript.field",
		"main.js -> lib.js//Yaml//parse name javascript.name",
		"main.js -> lib.js//json//parse name javascript.name",
		"main.js -> lib.js//parse name javascript.name",
		"main.js -> lib.js//helper//parse name javascript.name",
	}, edgesOfKind(g, EdgeField, EdgeName), "a member call reaches the methods and the exported functions, and a bare call every function of the name")
	assert.Equal(t, []string{
		"main.js:1 main.js unknown-receiver parse",
		"main.js:2 main.js unknown-receiver parse",
		"main.js:3 main.js ambiguous ",
	}, unresolvedStrings(g), "25 candidates make the site ambiguous")
}

func TestBuildKeepsTheBestEdgeOfASite(t *testing.T) {
	b, err := newBuilder(nil, Options{})
	require.NoError(t, err)
	site := Site{Path: "a.js", Line: 1, Column: 1}
	b.addEdge(Edge{From: "a", To: "b", Site: site, Kind: EdgeField})
	b.addEdge(Edge{From: "a", To: "b", Site: site, Kind: EdgeMember})
	b.addEdge(Edge{From: "a", To: "b", Site: site, Kind: EdgeCHA})
	b.addEdge(Edge{From: "a", To: "c", Site: site, Kind: EdgeCHA})
	g := b.finish()

	var got []string
	for _, e := range g.Edges {
		got = append(got, fmt.Sprintf("%s -> %s %s %s", e.From, e.To, e.Kind, e.Confidence))
	}
	assert.Equal(t, []string{"a -> b member high", "a -> c cha medium"}, got)
}

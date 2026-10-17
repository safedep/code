package projectgraph

import (
	"context"
	"fmt"
	"testing"
	"time"

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

func TestBuildGoInterfaceMethodSets(t *testing.T) {
	method := func(key string, arity int) fileindex.Definition {
		d := def(fileindex.DefMethod, key, 1)
		d.Arity = arity
		return d
	}
	run := fileindex.Call{Caller: "a.go//start", Line: 20, Column: 2, Callee: "r.Run", Targets: []string{"a.go//start//r//Run"}, Receiver: "r", Method: "Run", Kind: fileindex.CallCall}
	start := def(fileindex.DefFunction, "a.go//start", 19)
	start.Params = []fileindex.Typed{{Name: "r", Type: "Runner"}}
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: "a.go", Language: langcode.Go, Package: "a",
		Definitions: []fileindex.Definition{
			def(fileindex.DefInterface, "a.go//Stopper", 1), def(fileindex.DefInterface, "a.go//Runner", 2),
			def(fileindex.DefClass, "a.go//Full", 3), method("a.go//Full//Run", 0), method("a.go//Full//Stop", 1),
			def(fileindex.DefClass, "a.go//Half", 4), method("a.go//Half//Run", 0),
			def(fileindex.DefClass, "a.go//Wrong", 5), method("a.go//Wrong//Run", 0), method("a.go//Wrong//Stop", 0),
			def(fileindex.DefClass, "a.go//Embeds", 6), method("a.go//Embeds//Run", 0),
			def(fileindex.DefClass, "a.go//Remote", 7), method("a.go//Remote//Run", 0),
			start,
		},
		Imports: []fileindex.Import{{Line: 1, Module: "example.com/ext", Alias: "ext", Kind: fileindex.ImportStatic, Namespace: "example.com//ext"}},
		Classes: []fileindex.Class{
			{Key: "a.go//Stopper", Abstract: []fileindex.Signature{{Name: "Stop", Arity: 1}}},
			{Key: "a.go//Runner", Bases: []string{"Stopper", "ext.Closer"}, Abstract: []fileindex.Signature{{Name: "Run", Arity: 0}}},
			{Key: "a.go//Full"}, {Key: "a.go//Half"}, {Key: "a.go//Wrong"},
			{Key: "a.go//Embeds", Bases: []string{"Full"}},
			{Key: "a.go//Remote", Bases: []string{"ext.Base"}},
		},
		Calls: []fileindex.Call{run},
	}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"a.go//start -> a.go//Embeds//Run cha go.cha",
		"a.go//start -> a.go//Full//Run cha go.cha",
		"a.go//start -> a.go//Remote//Run cha go.cha",
	}, edgesOfKind(g, EdgeCHA), "a type implements Runner when its methods, its embedded types or a dependency hold Run and Stop by arity")
}

func TestBuildExternalBaseGivesOnlyTheLastName(t *testing.T) {
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: "w.py", Language: langcode.Python,
		Imports: []fileindex.Import{{Line: 1, Module: "threading", Alias: "threading", Kind: fileindex.ImportStatic, Namespace: "threading"}},
		Definitions: []fileindex.Definition{
			def(fileindex.DefClass, "w.py//W", 3), def(fileindex.DefMethod, "w.py//W//go", 4),
			def(fileindex.DefClass, "w.py//P", 8), def(fileindex.DefMethod, "w.py//P//parse", 9),
		},
		Classes: []fileindex.Class{{Key: "w.py//W", Bases: []string{"threading.Thread"}, Methods: []string{"w.py//W//go"}}, {Key: "w.py//P", Methods: []string{"w.py//P//parse"}}},
		Calls: []fileindex.Call{
			{Caller: "w.py//W//go", Line: 5, Column: 9, Callee: "self.start", Targets: []string{"w.py//W//self//start"}, Receiver: "self", Method: "start", Kind: fileindex.CallCall},
			{Caller: "w.py//W//go", Line: 6, Column: 9, Callee: "self.parser.parse", Targets: []string{"w.py//W//self//parser//parse"}, Receiver: "self.parser", Method: "parse", Kind: fileindex.CallCall},
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"w.py//W//go -> threading//Thread//start member python.member"}, edgesOfKind(g, EdgeMember))
	assert.Equal(t, []string{"w.py//W//go -> w.py//P//parse name python.name"}, edgesOfKind(g, EdgeName))
	assert.Equal(t, []string{"w.py:6 w.py//W//go unknown-receiver parse"}, unresolvedStrings(g),
		"an attribute of an instance of a class with a dependency base has no known value")
}

func TestBuildStructuralTypesKeepFieldEdges(t *testing.T) {
	start := def(fileindex.DefFunction, "a.ts//start", 10)
	start.EndLine = 12
	start.Params = []fileindex.Typed{{Name: "r", Type: "Runner"}, {Name: "b", Type: "Base"}, {Name: "c", Type: "Closed"}}
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: "a.ts", Language: langcode.Typescript,
		Definitions: []fileindex.Definition{
			def(fileindex.DefInterface, "a.ts//Runner", 1),
			def(fileindex.DefClass, "a.ts//Base", 2),
			def(fileindex.DefClass, "a.ts//Closed", 3), def(fileindex.DefMethod, "a.ts//Closed//stop", 3),
			def(fileindex.DefClass, "a.ts//Plain", 4), def(fileindex.DefMethod, "a.ts//Plain//run", 4), def(fileindex.DefMethod, "a.ts//Plain//stop", 4),
			def(fileindex.DefMethod, "a.ts//obj//halt", 5),
			start,
		},
		Classes: []fileindex.Class{{Key: "a.ts//Runner"}, {Key: "a.ts//Base"}, {Key: "a.ts//Closed", Methods: []string{"a.ts//Closed//stop"}},
			{Key: "a.ts//Plain", Methods: []string{"a.ts//Plain//run", "a.ts//Plain//stop"}}},
		Calls: []fileindex.Call{
			{Caller: "a.ts//start", Line: 11, Column: 3, Callee: "r.run", Targets: []string{"r//run"}, Receiver: "r", Method: "run", Kind: fileindex.CallCall},
			{Caller: "a.ts//start", Line: 12, Column: 3, Callee: "b.halt", Targets: []string{"b//halt"}, Receiver: "b", Method: "halt", Kind: fileindex.CallCall},
			{Caller: "a.ts//start", Line: 13, Column: 3, Callee: "c.stop", Targets: []string{"c//stop"}, Receiver: "c", Method: "stop", Kind: fileindex.CallCall},
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"a.ts//start -> a.ts//Plain//run field typescript.field",
		"a.ts//start -> a.ts//obj//halt field typescript.field",
		"a.ts//start -> a.ts//Closed//stop member typescript.declared-type",
	}, edgesOfKind(g, EdgeMember, EdgeCHA, EdgeField),
		"an interface and a method that no class of the hierarchy defines keep the field edges, and a class with the method narrows the site")
}

func TestBuildOpenClassesTakeCHAEdges(t *testing.T) {
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: "a.js", Language: langcode.Javascript,
		Definitions: []fileindex.Definition{
			def(fileindex.DefFunction, "a.js//mix", 1),
			def(fileindex.DefClass, "a.js//mix//<anonymous:1>", 1), def(fileindex.DefMethod, "a.js//mix//<anonymous:1>//load", 2),
			def(fileindex.DefClass, "a.js//Err", 4), def(fileindex.DefMethod, "a.js//Err//load", 4),
			def(fileindex.DefClass, "a.js//App", 5), def(fileindex.DefMethod, "a.js//App//start", 6),
		},
		Classes: []fileindex.Class{
			{Key: "a.js//mix//<anonymous:1>", Bases: []string{"B"}, Methods: []string{"a.js//mix//<anonymous:1>//load"}},
			{Key: "a.js//Err", Bases: []string{"Error"}, Methods: []string{"a.js//Err//load"}},
			{Key: "a.js//App", Bases: []string{"mix(Object)"}, Methods: []string{"a.js//App//start"}},
		},
		Calls: []fileindex.Call{{Caller: "a.js//App//start", Line: 7, Column: 5, Callee: "this.load", Targets: []string{"a.js//App//this//load"}, Receiver: "this", Method: "load", Kind: fileindex.CallCall}},
	}
	const mix = "npm:mix@1.0.0//m.js//mix//<anonymous:1>"
	dep := &fileindex.Index{
		Version: fileindex.Version, Path: "npm:mix@1.0.0//m.js", Language: langcode.Javascript, Unit: "npm:mix@1.0.0",
		Definitions: []fileindex.Definition{def(fileindex.DefFunction, "npm:mix@1.0.0//m.js//mix", 1), def(fileindex.DefClass, mix, 1), def(fileindex.DefMethod, mix+"//load", 2)},
		Classes:     []fileindex.Class{{Key: mix, Bases: []string{"B"}, Methods: []string{mix + "//load"}}},
	}
	g, err := Build(context.Background(), []*fileindex.Index{ix, dep}, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"a.js//App//start -> a.js//mix//<anonymous:1>//load cha javascript.cha"}, edgesOfKind(g, EdgeCHA),
		"a class whose base is a parameter can extend any class of its own unit, and a builtin base names a class")
}

func TestBuildOverridesOfADependencyBase(t *testing.T) {
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: "w.py", Language: langcode.Python,
		Imports: []fileindex.Import{{Line: 1, Module: "threading", Alias: "threading", Kind: fileindex.ImportStatic, Namespace: "threading"}},
		Definitions: []fileindex.Definition{
			def(fileindex.DefClass, "w.py//Base", 2), def(fileindex.DefMethod, "w.py//Base//step", 3),
			def(fileindex.DefClass, "w.py//W", 5), def(fileindex.DefConstructor, "w.py//W//__init__", 6),
			def(fileindex.DefMethod, "w.py//W//run", 7), def(fileindex.DefMethod, "w.py//W//step", 8),
			def(fileindex.DefClass, "w.py//Plain", 10), def(fileindex.DefMethod, "w.py//Plain//go", 11),
		},
		Classes: []fileindex.Class{
			{Key: "w.py//Base", Methods: []string{"w.py//Base//step"}},
			{Key: "w.py//W", Bases: []string{"Base", "threading.Thread"}, Methods: []string{"w.py//W//__init__", "w.py//W//run", "w.py//W//step"}},
			{Key: "w.py//Plain", Methods: []string{"w.py//Plain//go"}},
		},
	}
	ix.Definitions[3].Parent = "w.py//W"
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"w.py//W -> w.py//W//run reference python.override",
		"w.py//W -> w.py//W//step reference python.override",
	}, edgesOfKind(g, EdgeReference), "the dependency can call each method of an instance, and the constructor runs with the class")
}

func TestBuildFieldCandidatesComeFromTheUnitOfTheCaller(t *testing.T) {
	unresolved := func(caller string) []fileindex.Unresolved {
		return []fileindex.Unresolved{{Caller: caller, Line: 1, Column: 1, Text: "o.get", Receiver: "o", Method: "get", Reason: fileindex.UnresolvedUnknownReceiver}}
	}
	dep := &fileindex.Index{Version: fileindex.Version, Path: "npm:lib@1.0.0//index.js", Language: langcode.Javascript, Unit: "npm:lib@1.0.0"}
	for i := range 25 {
		dep.Definitions = append(dep.Definitions, def(fileindex.DefMethod, fmt.Sprintf("npm:lib@1.0.0//index.js//o%d//get", i), i+1))
	}
	other := &fileindex.Index{
		Version: fileindex.Version, Path: "npm:other@2.0.0//index.js", Language: langcode.Javascript, Unit: "npm:other@2.0.0",
		Definitions: []fileindex.Definition{def(fileindex.DefMethod, "npm:other@2.0.0//index.js//box//get", 1)},
		Unresolved:  unresolved("npm:other@2.0.0//index.js"),
	}
	own := &fileindex.Index{
		Version: fileindex.Version, Path: "store.js", Language: langcode.Javascript,
		Definitions: []fileindex.Definition{def(fileindex.DefMethod, "store.js//cache//get", 1)},
	}
	main := &fileindex.Index{Version: fileindex.Version, Path: "main.js", Language: langcode.Javascript, Unresolved: unresolved("main.js")}
	g, err := Build(context.Background(), []*fileindex.Index{dep, other, own, main}, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"main.js -> store.js//cache//get field javascript.field",
		"npm:other@2.0.0//index.js -> npm:other@2.0.0//index.js//box//get field javascript.field",
	}, edgesOfKind(g, EdgeField),
		"a project call takes no candidate of a dependency, and a dependency call takes candidates only from its own unit")
	assert.Equal(t, []string{
		"main.js:1 main.js unknown-receiver get",
		"npm:other@2.0.0//index.js:1 npm:other@2.0.0//index.js unknown-receiver get",
	}, unresolvedStrings(g))
}

func TestBuildJavaCallOnThisOfADependencyBase(t *testing.T) {
	ix := &fileindex.Index{
		Version: fileindex.Version, Path: "P.java", Language: langcode.Java, Package: "app",
		Definitions: []fileindex.Definition{def(fileindex.DefClass, "P.java//P", 1), def(fileindex.DefMethod, "P.java//P//run", 2)},
		Classes:     []fileindex.Class{{Key: "P.java//P", Bases: []string{"Thread"}, Methods: []string{"P.java//P//run"}}},
		Calls: []fileindex.Call{
			{Caller: "P.java//P//run", Line: 3, Column: 9, Callee: "ProcessBuilder", Targets: []string{"ProcessBuilder"}, Kind: fileindex.CallNew},
			{Caller: "P.java//P//run", Line: 4, Column: 9, Callee: "interrupt", Targets: []string{"interrupt"}, Method: "interrupt", Kind: fileindex.CallCall},
		},
	}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{})
	require.NoError(t, err)
	assert.Contains(t, edgeStrings(g), "P.java//P//run -> java//lang//Thread//interrupt member java.declared-type", "a call with no receiver runs on this")
	assert.NotContains(t, edgeStrings(g), "P.java//P//run -> Thread//ProcessBuilder member java.declared-type", "a new names a class, not a method of the base")
}

func TestMROOfALongChainIsFast(t *testing.T) {
	const n = 6000
	indexes := make([]*fileindex.Index, 0, n)
	for i := range n {
		p := fmt.Sprintf("c%d.py", i)
		class := fileindex.Join(p, "C")
		ix := &fileindex.Index{
			Version:     fileindex.Version,
			Path:        p,
			Language:    langcode.Python,
			Definitions: []fileindex.Definition{{Key: class, Kind: fileindex.DefClass, Name: "C", StartLine: 1, EndLine: 2, Exported: true}},
			Classes:     []fileindex.Class{{Key: class}},
		}
		if i > 0 {
			base := fmt.Sprintf("c%d", i-1)
			ix.Imports = []fileindex.Import{{Line: 1, Module: base, Item: "C", Alias: "B", Kind: fileindex.ImportStatic, Namespace: base + "//C"}}
			ix.Classes[0].Bases = []string{"B"}
		}
		indexes = append(indexes, ix)
	}
	start := time.Now()
	g, err := Build(context.Background(), indexes, Options{})
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 20*time.Second)
	assert.Len(t, g.Nodes, n*2)
}

package fileindex

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
)

func TestFileIndexCSharp(t *testing.T) {
	source := strings.Join([]string{
		"using System;",
		"namespace Shop.Web",
		"{",
		"    [ApiController]",
		"    public partial class Api : ControllerBase, IHandler",
		"    {",
		"        private readonly IStore _store;",
		"        public Repo Repo { get; set; }",
		"        private static readonly Log Log = Log.Create();",
		"        public Api(IStore store) : base(store) { _store = store; }",
		"        static Api() { Boot(); }",
		"        [HttpGet(\"/a\")]",
		"        public Item Get(string id) { Cart c = Make(); var d = new Order(); return _store.Find(id); }",
		"        public void Run() { list.ForEach(Handle); }",
		"        void Handle(string s) => typeof(Api).GetMethod(s);",
		"    }",
		"    public interface IHandler { void Handle(string s); }",
		"    public static class Ext { public static void Shout(this string s, int n) {} }",
		"}",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeCSharp, "Api.cs", source)

	assert.Equal(t, "Shop.Web", ix.Package)
	t.Run("a class has its bases, its fields and its attributes", func(t *testing.T) {
		api := classOf(t, ix, "Api.cs//Api")
		assert.Equal(t, []string{"ControllerBase", "IHandler"}, api.Bases)
		assert.Equal(t, []fidx.Typed{{Name: "_store", Type: "IStore"}, {Name: "Repo", Type: "Repo"}, {Name: "Log", Type: "Log"}}, api.Fields)
		assert.Equal(t, []string{"@ApiController"}, definitionOf(t, ix, "Api.cs//Api").Decorators)
	})
	t.Run("a method has its parameters, its locals, its result and its attributes", func(t *testing.T) {
		get := definitionOf(t, ix, "Api.cs//Api//Get")
		assert.Equal(t, []string{"@HttpGet"}, get.Decorators)
		assert.Empty(t, get.Params, "a predefined type is no class")
		assert.Equal(t, []fidx.Typed{{Name: "c", Type: "Cart"}, {Name: "d", Type: "Order"}}, get.Locals)
		assert.Equal(t, "Item", get.Returns)
		assert.Equal(t, []fidx.Typed{{Name: "store", Type: "IStore"}}, definitionOf(t, ix, "Api.cs//Api//Api").Params)
	})
	t.Run("the static members are the static initializer", func(t *testing.T) {
		init := definitionOf(t, ix, "Api.cs//Api//<clinit>")
		assert.Equal(t, fidx.DefMethod, init.Kind)
		assert.True(t, init.Static)
		callers := map[string]string{}
		for _, call := range ix.Calls {
			callers[call.Callee] = call.Caller
		}
		assert.Equal(t, "Api.cs//Api//<clinit>", callers["Log.Create"])
		assert.Equal(t, "Api.cs//Api//<clinit>", callers["Boot"])
	})
	t.Run("an extension method has its receiver as the parameter this", func(t *testing.T) {
		shout := definitionOf(t, ix, "Api.cs//Ext//Shout")
		assert.True(t, shout.Static)
		assert.Equal(t, []fidx.Typed{{Name: "this", Type: "string"}}, shout.Params)
	})
	t.Run("an interface method with no body is no definition", func(t *testing.T) {
		assert.Equal(t, fidx.DefInterface, definitionOf(t, ix, "Api.cs//IHandler").Kind)
		for _, def := range ix.Definitions {
			assert.NotEqual(t, "Api.cs//IHandler//Handle", def.Key)
		}
	})
	t.Run("a method group is a reference and reflection is a flag", func(t *testing.T) {
		var targets [][]string
		for _, ref := range ix.References {
			targets = append(targets, ref.Targets)
		}
		assert.Contains(t, targets, []string{"Api.cs//Api//Handle"})
		assert.Contains(t, ix.Flags, fidx.Flag{Kind: fidx.FlagReflection, Line: 15})
	})
}

func TestFileIndexCSharpTopLevelStatements(t *testing.T) {
	source := "using System;\nConsole.WriteLine(Run());\nint Run() => Helper.Go();\n"
	ix := indexSource(t, core.LanguageCodeCSharp, "Program.cs", source)

	main := definitionOf(t, ix, "Program.cs//<Main>$")
	assert.True(t, main.Static)
	callers := map[string]string{}
	for _, call := range ix.Calls {
		callers[call.Callee] = call.Caller
	}
	assert.Equal(t, "Program.cs//<Main>$", callers["Console.WriteLine"])
	assert.Equal(t, "Program.cs//Run", callers["Helper.Go"], "a local function keeps its calls")
}

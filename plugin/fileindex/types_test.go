package fileindex

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func definitionOf(t *testing.T, ix *fidx.Index, key string) fidx.Definition {
	t.Helper()
	for _, def := range ix.Definitions {
		if def.Key == key {
			return def
		}
	}
	require.Failf(t, "no definition", "key %s", key)
	return fidx.Definition{}
}

func classOf(t *testing.T, ix *fidx.Index, key string) fidx.Class {
	t.Helper()
	for _, c := range ix.Classes {
		if c.Key == key {
			return c
		}
	}
	require.Failf(t, "no class", "key %s", key)
	return fidx.Class{}
}

func TestFileIndexDeclaredTypes(t *testing.T) {
	source := strings.Join([]string{
		"import type { Store } from './store';",
		"export interface Local extends Store, ns.Other { put(): void }",
		"interface Hidden {}",
		"export class Impl implements Local {",
		"  private parser: Parser;",
		"  count = 0;",
		"  static shared: Parser;",
		"  maybe?: Parser | undefined;",
		"  constructor(private readonly store: Store, other: Base) {}",
		"  get(k: string, b?: Base, c: Base<T> | null = null, { d }: Opts, ...r: Base[]): string { return k; }",
		"  put() {}",
		"}",
		"const run = (s: ns.Store) => s.get('b');",
		"function both(a: A | B) {}",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeTypescript, "a.ts", source)

	t.Run("a parameter that names a type has the type", func(t *testing.T) {
		assert.Equal(t, []fidx.Typed{{Name: "b", Type: "Base"}, {Name: "c", Type: "Base"}},
			definitionOf(t, ix, "a.ts//Impl//get").Params)
		assert.Equal(t, []fidx.Typed{{Name: "store", Type: "Store"}, {Name: "other", Type: "Base"}},
			definitionOf(t, ix, "a.ts//Impl//constructor").Params)
		assert.Equal(t, []fidx.Typed{{Name: "s", Type: "ns.Store"}}, definitionOf(t, ix, "a.ts//run").Params)
	})
	t.Run("a union of two types names no type", func(t *testing.T) {
		assert.Empty(t, definitionOf(t, ix, "a.ts//both").Params)
	})
	t.Run("the fields of a class have their types", func(t *testing.T) {
		assert.Equal(t, []fidx.Typed{{Name: "parser", Type: "Parser"}, {Name: "maybe", Type: "Parser"}, {Name: "store", Type: "Store"}},
			classOf(t, ix, "a.ts//Impl").Fields)
	})
	t.Run("an interface is a definition and a class with its bases", func(t *testing.T) {
		local := definitionOf(t, ix, "a.ts//Local")
		assert.Equal(t, fidx.DefInterface, local.Kind)
		assert.True(t, local.Exported)
		assert.False(t, definitionOf(t, ix, "a.ts//Hidden").Exported)
		assert.Equal(t, []string{"Store", "ns.Other"}, classOf(t, ix, "a.ts//Local").Bases)
		assert.Contains(t, ix.Exports, fidx.Export{Name: "Local", Target: "a.ts//Local", Line: 2})
	})
}

func TestFileIndexDeclaredTypesOnlyInTypeScript(t *testing.T) {
	ix := indexSource(t, core.LanguageCodePython, "a.py", "def run(p: Parser):\n    p.parse()\n")
	assert.Empty(t, definitionOf(t, ix, "a.py//run").Params)
}

func TestFileIndexPropertyFunctions(t *testing.T) {
	source := strings.Join([]string{
		"const yaml = require('js-yaml');",
		"const api = { run(t) { yaml.a(); }, go: function () { yaml.b(); } };",
		"api.later = function () { yaml.c(); };",
		"exports.load = () => yaml.d();",
		"function outer() { const o = { inner: function () { yaml.e(); } }; }",
		"class K { handle = () => { yaml.f(); }; }",
		"module.exports.extra = function () { yaml.g(); };",
		"K.prototype.later = function () { yaml.h(); };",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeJavascript, "a.js", source)

	callers := map[string]string{}
	for _, call := range ix.Calls {
		callers[call.Callee] = call.Caller
	}
	assert.Equal(t, map[string]string{
		"yaml.a": "a.js//run",
		"yaml.b": "a.js//api//go",
		"yaml.c": "a.js//api//later",
		"yaml.d": "a.js//load",
		"yaml.e": "a.js//outer//o//inner",
		"yaml.f": "a.js//K//handle",
		"yaml.g": "a.js//extra",
		"yaml.h": "a.js//K//later",
	}, callers)

	for _, key := range []string{"a.js//api//go", "a.js//api//later", "a.js//load", "a.js//outer//o//inner"} {
		def := definitionOf(t, ix, key)
		assert.Equal(t, fidx.DefMethod, def.Kind, key)
		assert.Empty(t, def.Parent, key)
	}
	assert.Equal(t, "a.js//K", definitionOf(t, ix, "a.js//K//handle").Parent)
	assert.Equal(t, "a.js//K", definitionOf(t, ix, "a.js//K//later").Parent)
	assert.True(t, definitionOf(t, ix, "a.js//load").Exported)
	assert.True(t, definitionOf(t, ix, "a.js//extra").Exported)
	assert.False(t, definitionOf(t, ix, "a.js//api//later").Exported)
}

func TestFileIndexArgumentFunctions(t *testing.T) {
	source := strings.Join([]string{
		"const yaml = require('js-yaml');",
		"app.get('/a', (req, res) => yaml.a());",
		"p.then(() => yaml.b(), function () { yaml.c(); });",
		"function outer() { run(() => { db.q(() => yaml.d()); }); }",
		"class K { static { register(() => yaml.e()); } }",
		"(() => yaml.f())();",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeJavascript, "a.js", source)

	callers := map[string]string{}
	for _, call := range ix.Calls {
		callers[call.Callee] = call.Caller
	}
	assert.Equal(t, "a.js//<anonymous:2>", callers["yaml.a"])
	assert.Equal(t, "a.js//<anonymous:3>", callers["yaml.b"])
	assert.Equal(t, "a.js//<anonymous:3#2>", callers["yaml.c"])
	assert.Equal(t, "a.js//outer//<anonymous:4>//<anonymous:4>", callers["yaml.d"])
	assert.Equal(t, "a.js//K//<anonymous:5>", callers["yaml.e"])
	assert.Equal(t, "a.js", callers["yaml.f"], "the body of an immediately invoked function runs in the caller")

	def := definitionOf(t, ix, "a.js//K//<anonymous:5>")
	assert.Equal(t, fidx.DefFunction, def.Kind)
	assert.Empty(t, def.Parent, "a function of a static block is no method")
	assert.False(t, def.Exported)
	assert.Equal(t, []fidx.Arg{{Shape: fidx.ArgString}, {Shape: fidx.ArgFunction, Targets: []string{"a.js//<anonymous:2>"}}}, argsOf(ix, "app.get"))
}

func TestFileIndexModuleExportsValue(t *testing.T) {
	fn := indexSource(t, core.LanguageCodeJavascript, "f.js", "module.exports = function (t) { yaml.a(t); };\n")
	assert.Equal(t, fidx.DefFunction, definitionOf(t, fn, "f.js//default").Kind)
	assert.True(t, definitionOf(t, fn, "f.js//default").Exported)
	assert.Equal(t, []fidx.Export{{Name: "default", Target: "f.js//default", Line: 1}}, fn.Exports)
	assert.Equal(t, "f.js//default", fn.Calls[0].Caller)

	class := indexSource(t, core.LanguageCodeJavascript, "c.js", "module.exports = class extends Base { m() { yaml.b(); } };\n")
	assert.Equal(t, fidx.DefClass, definitionOf(t, class, "c.js//default").Kind)
	assert.Equal(t, []fidx.Export{{Name: "default", Target: "c.js//default", Line: 1}}, class.Exports)
	assert.Equal(t, fidx.Class{Key: "c.js//default", Bases: []string{"Base"}, Methods: []string{"c.js//default//m"}}, classOf(t, class, "c.js//default"))
}

func TestFileIndexClassExpressions(t *testing.T) {
	source := strings.Join([]string{
		"const Loading = (B) => class extends B { load() {} };",
		"const Named = class extends Base { run() {} };",
		"exports.Kept = class { keep() {} };",
	}, "\n") + "\n"
	for _, code := range []core.LanguageCode{core.LanguageCodeJavascript, core.LanguageCodeTypescript} {
		ix := indexSource(t, code, "a.js", source)
		assert.Equal(t, []string{"B"}, classOf(t, ix, "a.js//Loading//<anonymous:1>").Bases, code)
		assert.Equal(t, []string{"a.js//Loading//<anonymous:1>//load"}, classOf(t, ix, "a.js//Loading//<anonymous:1>").Methods, code)
		assert.Equal(t, []string{"Base"}, classOf(t, ix, "a.js//Named").Bases, code)
		assert.Equal(t, fidx.DefClass, definitionOf(t, ix, "a.js//Named").Kind, code)
		assert.True(t, definitionOf(t, ix, "a.js//Kept").Exported, code)
	}
}

func TestFileIndexArrowInAnObjectOrAnArgument(t *testing.T) {
	source := "const o = { m: () => yaml.x() };\nconst h = handle(async (req, res) => yaml.y());\n"
	ix := indexSource(t, core.LanguageCodeJavascript, "a.js", source)
	callers := map[string]string{}
	for _, call := range ix.Calls {
		callers[call.Callee] = call.Caller
	}
	assert.Equal(t, "a.js//o//m", callers["yaml.x"])
	assert.Equal(t, "a.js//<anonymous:2>", callers["yaml.y"])
	assert.Equal(t, fidx.DefVariable, definitionOf(t, ix, "a.js//h").Kind)
}

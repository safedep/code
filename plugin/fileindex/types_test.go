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
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeJavascript, "a.js", source)

	callers := map[string]string{}
	for _, call := range ix.Calls {
		callers[call.Callee] = call.Caller
	}
	assert.Equal(t, map[string]string{
		"yaml.a": "a.js//run",
		"yaml.b": "a.js//go",
		"yaml.c": "a.js//later",
		"yaml.d": "a.js//load",
		"yaml.e": "a.js//outer//inner",
		"yaml.f": "a.js//K//handle",
		"yaml.g": "a.js//extra",
	}, callers)

	for _, key := range []string{"a.js//go", "a.js//later", "a.js//load", "a.js//outer//inner"} {
		def := definitionOf(t, ix, key)
		assert.Equal(t, fidx.DefMethod, def.Kind, key)
		assert.Empty(t, def.Parent, key)
	}
	assert.Equal(t, "a.js//K", definitionOf(t, ix, "a.js//K//handle").Parent)
	assert.True(t, definitionOf(t, ix, "a.js//load").Exported)
	assert.True(t, definitionOf(t, ix, "a.js//extra").Exported)
	assert.False(t, definitionOf(t, ix, "a.js//later").Exported)
}

package indexer

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
)

func TestFileIndexKotlin(t *testing.T) {
	source := strings.Join([]string{
		"package com.shop.web",
		"import org.springframework.beans.factory.annotation.Autowired",
		"val client = HttpClient.newHttpClient()",
		"@RestController",
		"class Api(private val store: Store, name: String) : Base(), Handler {",
		"  @Autowired lateinit var repo: Repo",
		"  @GetMapping(\"/a\") fun get(id: String): Item { val s: Store = make(); return s.find(id) }",
		"  fun ref() { list.forEach(::helper); list.forEach(Util::go) }",
		"  companion object { val log = Log.get(); fun create(): Api = Api(Store(), \"x\") }",
		"}",
		"interface Handler { fun handle(x: Int); fun close() {} }",
		"object Registry { fun add() { Class.forName(\"x\") } }",
		"fun String.shout(times: Int): String = this",
		"fun main(args: Array<String>) {}",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeKotlin, "Api.kt", source)

	assert.Equal(t, "com.shop.web", ix.Package)
	t.Run("a class has its bases, its fields and its annotations", func(t *testing.T) {
		api := classOf(t, ix, "Api.kt//Api")
		assert.Equal(t, []string{"Base", "Handler"}, api.Bases)
		assert.Equal(t, []fileindex.Typed{{Name: "store", Type: "Store"}, {Name: "repo", Type: "Repo"}, {Name: "Companion", Type: "Api"}}, api.Fields)
		assert.Equal(t, []string{"@RestController"}, definitionOf(t, ix, "Api.kt//Api").Decorators)
		assert.Equal(t, []fileindex.Typed{{Name: "INSTANCE", Type: "Registry"}}, classOf(t, ix, "Api.kt//Registry").Fields)
	})
	t.Run("an interface is an interface and its abstract method is no definition", func(t *testing.T) {
		assert.Equal(t, fileindex.DefInterface, definitionOf(t, ix, "Api.kt//Handler").Kind)
		for _, def := range ix.Definitions {
			assert.NotEqual(t, "handle", def.Name)
		}
		assert.Equal(t, "Api.kt//Handler", definitionOf(t, ix, "Api.kt//Handler//close").Parent)
	})
	t.Run("a method has its parameters, its locals and its result", func(t *testing.T) {
		get := definitionOf(t, ix, "Api.kt//Api//get")
		assert.Equal(t, []string{"@GetMapping"}, get.Decorators)
		assert.Equal(t, []fileindex.Typed{{Name: "id", Type: "String"}}, get.Params)
		assert.Equal(t, []fileindex.Typed{{Name: "s", Type: "Store"}}, get.Locals)
		assert.Equal(t, "Item", get.Returns)
		assert.False(t, get.Static)
	})
	t.Run("a top-level function and a companion member are static", func(t *testing.T) {
		assert.True(t, definitionOf(t, ix, "Api.kt//main").Static)
		create := definitionOf(t, ix, "Api.kt//Api//create")
		assert.True(t, create.Static)
		assert.Equal(t, "Api.kt//Api", create.Parent)
		assert.False(t, definitionOf(t, ix, "Api.kt//Registry//add").Static, "a member of an object runs on its instance")
	})
	t.Run("the initializers of a companion are its static initializer", func(t *testing.T) {
		init := definitionOf(t, ix, "Api.kt//Api//<clinit>")
		assert.Equal(t, fileindex.DefMethod, init.Kind)
		assert.True(t, init.Static)
		callers := map[string]string{}
		for _, call := range ix.Calls {
			callers[call.Callee] = call.Caller
		}
		assert.Equal(t, "Api.kt//Api//<clinit>", callers["Log.get"])
		assert.Equal(t, "Api.kt", callers["HttpClient.newHttpClient"])
	})
	t.Run("an extension has its receiver as the parameter this", func(t *testing.T) {
		shout := definitionOf(t, ix, "Api.kt//shout")
		assert.Equal(t, []fileindex.Typed{{Name: "this", Type: "String"}, {Name: "times", Type: "Int"}}, shout.Params)
	})
	t.Run("a top-level property is a variable with no value", func(t *testing.T) {
		client := definitionOf(t, ix, "Api.kt//client")
		assert.Equal(t, fileindex.DefVariable, client.Kind)
		assert.Empty(t, client.Values)
	})
	t.Run("a callable reference is a reference", func(t *testing.T) {
		var targets [][]string
		for _, ref := range ix.References {
			targets = append(targets, ref.Targets)
		}
		assert.Equal(t, [][]string{{"helper"}, {"Util//go"}}, targets)
	})
	t.Run("injection and reflection set flags", func(t *testing.T) {
		assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagInjection, Line: 6})
		assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagReflection, Line: 12})
	})
}

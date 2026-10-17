package fileindex

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
)

func TestFileIndexJava(t *testing.T) {
	source := strings.Join([]string{
		"package com.shop.web;",
		"import org.springframework.beans.factory.annotation.Autowired;",
		"@RestController",
		"public class Api extends Base implements Handler, com.x.Other<String> {",
		"  @Autowired private Store store;",
		"  private int count;",
		"  public Api(Store store) { super(store); }",
		"  @GetMapping(\"/a\")",
		"  public static void main(String[] args) { Store s = make(); var t = new Cart(); for (Item i : items()) {} }",
		"  abstract void hook();",
		"  void ref() { list.forEach(this::hook2); list.forEach(Util::go); }",
		"  void hook2() { Class.forName(\"x\"); }",
		"}",
		"interface Handler extends Base2 { default void d() { run(); } }",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeJava, "Api.java", source)

	assert.Equal(t, "com.shop.web", ix.Package)
	t.Run("a class has its bases, its fields and its annotations", func(t *testing.T) {
		api := classOf(t, ix, "Api.java//Api")
		assert.Equal(t, []string{"Base", "Handler", "com.x.Other"}, api.Bases)
		assert.Equal(t, []fidx.Typed{{Name: "store", Type: "Store"}}, api.Fields)
		assert.Equal(t, []string{"@RestController"}, definitionOf(t, ix, "Api.java//Api").Decorators)
		assert.Equal(t, []string{"Base2"}, classOf(t, ix, "Api.java//Handler").Bases)
	})
	t.Run("a constructor is <init> and a static method is static", func(t *testing.T) {
		ctor := definitionOf(t, ix, "Api.java//Api//<init>")
		assert.Equal(t, fidx.DefConstructor, ctor.Kind)
		assert.Equal(t, []fidx.Typed{{Name: "store", Type: "Store"}}, ctor.Params)
		main := definitionOf(t, ix, "Api.java//Api//main")
		assert.True(t, main.Static)
		assert.Equal(t, []string{"@GetMapping"}, main.Decorators)
		assert.Equal(t, []fidx.Typed{{Name: "s", Type: "Store"}, {Name: "t", Type: "Cart"}, {Name: "i", Type: "Item"}}, main.Locals)
	})
	t.Run("an abstract method is no definition and a default method is a method", func(t *testing.T) {
		for _, def := range ix.Definitions {
			assert.NotEqual(t, "hook", def.Name)
		}
		assert.Equal(t, "Api.java//Handler", definitionOf(t, ix, "Api.java//Handler//d").Parent)
	})
	t.Run("a method reference is a reference", func(t *testing.T) {
		var targets [][]string
		for _, ref := range ix.References {
			targets = append(targets, ref.Targets)
		}
		assert.Equal(t, [][]string{{"Api.java//Api//hook2"}, {"Util//go"}}, targets)
	})
	t.Run("injection and reflection set flags", func(t *testing.T) {
		assert.Contains(t, ix.Flags, fidx.Flag{Kind: fidx.FlagInjection, Line: 5})
		assert.Contains(t, ix.Flags, fidx.Flag{Kind: fidx.FlagReflection, Line: 12})
	})
}

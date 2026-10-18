package indexer

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Equal(t, []fileindex.Typed{{Name: "store", Type: "Store"}}, api.Fields)
		assert.Equal(t, []string{"@RestController"}, definitionOf(t, ix, "Api.java//Api").Decorators)
		assert.Equal(t, []string{"Base2"}, classOf(t, ix, "Api.java//Handler").Bases)
	})
	t.Run("a constructor is <init> and a static method is static", func(t *testing.T) {
		ctor := definitionOf(t, ix, "Api.java//Api//<init>")
		assert.Equal(t, fileindex.DefConstructor, ctor.Kind)
		assert.Equal(t, []fileindex.Typed{{Name: "store", Type: "Store"}}, ctor.Params)
		main := definitionOf(t, ix, "Api.java//Api//main")
		assert.True(t, main.Static)
		assert.Equal(t, []string{"@GetMapping"}, main.Decorators)
		assert.Equal(t, []fileindex.Typed{{Name: "s", Type: "Store"}, {Name: "t", Type: "Cart"}, {Name: "i", Type: "Item"}}, main.Locals)
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
		assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagInjection, Line: 5})
		assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagReflection, Line: 12})
	})
}

func TestFileIndexJavaStaticInitializer(t *testing.T) {
	source := strings.Join([]string{
		"class A {",
		"  static { Service s = new Service(); s.boot(); }",
		"  static final Log LOG = Log.get();",
		"  static final int N = 1;",
		"  Store store = Store.open();",
		"  static void run() {}",
		"}",
		"class B { static final int M = 2; }",
		"enum Op { PLUS, MINUS; }",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeJava, "A.java", source)

	callers := map[string]string{}
	for _, call := range ix.Calls {
		callers[call.Callee] = call.Caller
	}
	assert.Equal(t, "A.java//A//<clinit>", callers["s.boot"])
	assert.Equal(t, "A.java//A//<clinit>", callers["Log.get"])
	assert.Equal(t, "A.java//A", callers["Store.open"], "an instance field runs with the instance")
	assert.Equal(t, "A.java//Op//<clinit>", callers["Op"], "the constants of an enum are instances of it")

	init := definitionOf(t, ix, "A.java//A//<clinit>")
	assert.Equal(t, fileindex.DefMethod, init.Kind)
	assert.True(t, init.Static)
	assert.Equal(t, []fileindex.Typed{{Name: "s", Type: "Service"}}, init.Locals)
	assert.Contains(t, classOf(t, ix, "A.java//A").Methods, "A.java//A//<clinit>")
	for _, def := range ix.Definitions {
		assert.NotEqual(t, "A.java//B//<clinit>", def.Key, "a static field with no call runs no code")
	}
}

func TestFileIndexJavaStaticFieldReference(t *testing.T) {
	source := strings.Join([]string{
		"class App {",
		"  static final Runnable R = pick(App::run);",
		"  static void run() {}",
		"}",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeJava, "App.java", source)

	require.Len(t, ix.References, 1)
	assert.Equal(t, "App.java//App//<clinit>", ix.References[0].Caller, "a method reference in a static initializer runs when the class loads")
	assert.Equal(t, []string{"App.java//App//run"}, ix.References[0].Targets)
	require.Len(t, ix.Calls, 1)
	assert.Equal(t, ix.Calls[0].Caller, ix.References[0].Caller, "the call and the reference of one initializer have one caller")
}

func TestFileIndexJavaRecordsAndEnumBodies(t *testing.T) {
	source := strings.Join([]string{
		"record Point(Store store, int x) implements Shape {",
		"  Point { check(x); }",
		"  double norm() { return store.size(); }",
		"}",
		"enum Op {",
		"  PLUS { int apply(int a) { return a; } },",
		"  MINUS;",
		"  abstract int apply(int a);",
		"}",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeJava, "P.java", source)

	point := classOf(t, ix, "P.java//Point")
	assert.Equal(t, []string{"Shape"}, point.Bases)
	assert.Equal(t, []fileindex.Typed{{Name: "store", Type: "Store"}}, point.Fields, "a component of a record is a field")
	assert.Equal(t, fileindex.DefConstructor, definitionOf(t, ix, "P.java//Point//<init>").Kind)
	assert.Equal(t, "P.java//Point", definitionOf(t, ix, "P.java//Point//norm").Parent)

	plus := classOf(t, ix, "P.java//Op//PLUS")
	assert.Equal(t, []string{"Op"}, plus.Bases)
	assert.Equal(t, []string{"P.java//Op//PLUS//apply"}, plus.Methods)
	for _, def := range ix.Definitions {
		assert.NotEqual(t, "P.java//Op//MINUS", def.Key, "a constant with no body is no class")
	}
}

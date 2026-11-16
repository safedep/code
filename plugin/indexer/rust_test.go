package indexer

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
)

func TestFileIndexRust(t *testing.T) {
	source := strings.Join([]string{
		"use crate::store::Store;",
		"pub struct Job { store: Box<dyn Store> }",
		"impl Job {",
		"    pub fn new() -> Self { Job { store: make() } }",
		"    #[instrument]",
		"    fn run<S: Store>(&self, s: &S) -> Result<(), Error> { let n: Note = note(); read()?; Ok(()) }",
		"}",
		"impl Drop for Job { fn drop(&mut self) {} }",
		"impl Shout for String { fn shout(&self) {} }",
		"macro_rules! quit { () => { std::process::exit(1) }; }",
		"mod inner { pub fn f() {} }",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeRust, "job.rs", source)

	t.Run("the impl blocks of a type are one class with the traits and the fields of the type", func(t *testing.T) {
		job := classOf(t, ix, "job.rs//Job")
		assert.Equal(t, []string{"Drop"}, job.Bases)
		assert.Equal(t, []fileindex.Typed{{Name: "store", Type: "Store"}}, job.Fields)
		assert.True(t, definitionOf(t, ix, "job.rs//Job").Exported)
	})
	t.Run("a function of an impl block has its static mark, its parameters and its result", func(t *testing.T) {
		build := definitionOf(t, ix, "job.rs//Job//new")
		assert.True(t, build.Static)
		assert.True(t, build.Exported)
		assert.Equal(t, "Job", build.Returns)
		run := definitionOf(t, ix, "job.rs//Job//run")
		assert.False(t, run.Static)
		assert.False(t, run.Exported)
		assert.Equal(t, []fileindex.Typed{{Name: "s", Type: "Store"}}, run.Params)
		assert.Equal(t, []fileindex.Typed{{Name: "n", Type: "Note"}}, run.Locals)
		assert.Equal(t, []string{"@instrument"}, run.Decorators)
		assert.True(t, definitionOf(t, ix, "job.rs//Job//drop").Exported)
	})
	t.Run("an impl of a trait for a type of another crate takes the receiver as this", func(t *testing.T) {
		assert.Equal(t, []fileindex.Typed{{Name: "this", Type: "String"}}, definitionOf(t, ix, "job.rs//String//shout").Params)
	})
	t.Run("a macro is a function and an inline module holds static methods", func(t *testing.T) {
		assert.Equal(t, fileindex.DefFunction, definitionOf(t, ix, "job.rs//quit").Kind)
		f := definitionOf(t, ix, "job.rs//inner//f")
		assert.Equal(t, fileindex.DefMethod, f.Kind)
		assert.True(t, f.Static)
	})
	t.Run("a let type and the error type of a ? are references", func(t *testing.T) {
		var targets []string
		for _, ref := range ix.References {
			targets = append(targets, ref.Targets...)
		}
		assert.Equal(t, []string{"Error", "Note"}, targets)
	})
}

func TestFileIndexRustTry(t *testing.T) {
	source := strings.Join([]string{
		"fn run() -> Result<(), Error> {",
		"    let t = dep::load()?;",
		"    t.save();",
		"    dep::load()?.check()?.save();",
		"    dep::find().inspect(|t| {});",
		"    Ok(())",
		"}",
		"fn read() -> std::io::Result<Job> { open() }",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeRust, "main.rs", source)

	targets := map[string][]string{}
	for _, call := range ix.Calls {
		targets[call.Callee] = call.Targets
	}
	tests := []struct {
		callee string
		want   []string
	}{
		{"t.save", []string{"dep//load//?//save"}},
		{"dep::load()?.check", []string{"dep//load//?//check"}},
		{"dep::load()?.check()?.save", []string{"dep//load//?//check//?//save"}},
		{"dep::find().inspect", []string{"dep//find//inspect"}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, targets[tt.callee], tt.callee)
	}

	read := definitionOf(t, ix, "main.rs//read")
	assert.Equal(t, "Job", read.Returns)
	assert.Equal(t, "Result", read.Wraps)
}

func TestFileIndexRustTypeAlias(t *testing.T) {
	source := strings.Join([]string{
		"use image::ImageBuffer;",
		"pub type Image<P> = ImageBuffer<P, Vec<u8>>;",
		"mod inner { type Store = crate::store::Store; }",
		"#[cfg(unix)] type Imp = unix::Thing;",
		"#[cfg(windows)] type Imp = windows::Thing;",
		"#[cfg(feature = \"native\")] pub struct Client;",
		"#[cfg(not(feature = \"native\"))] pub type Client = fallback::Client;",
		"#[cfg(unix)] type Same = io::Thing;",
		"#[cfg(windows)] type Same = io::Thing;",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeRust, "defs.rs", source)

	for key, alias := range map[string]string{"defs.rs//Imp": "", "defs.rs//Client": "", "defs.rs//Same": "io::Thing"} {
		def := definitionOf(t, ix, key)
		assert.Equal(t, fileindex.DefAlias, def.Kind, key)
		assert.Equal(t, alias, def.Alias, key)
	}

	image := definitionOf(t, ix, "defs.rs//Image")
	assert.Equal(t, fileindex.DefAlias, image.Kind)
	assert.Equal(t, "ImageBuffer", image.Alias)
	assert.True(t, image.Exported)
	store := definitionOf(t, ix, "defs.rs//inner//Store")
	assert.Equal(t, fileindex.DefAlias, store.Kind)
	assert.Equal(t, "crate::store::Store", store.Alias)
	assert.False(t, store.Exported)
}

func TestFileIndexRustDeclaredTypes(t *testing.T) {
	source := strings.Join([]string{
		"fn plain() { let t: Store = make(); }",
		"fn sibling() { { use dep::Thing as Store; } let t: Store = make(); }",
		"fn alias() { type Store = u8; let t: Store = make(); }",
		"fn after() { let t: Store = make(); use dep::Thing as Store; }",
		"fn outer() { struct Store; { let t: Store = make(); } }",
		"fn listed() { use dep::{store::{self}, Other}; let t: store::Store = make(); }",
		"fn globbed() { use dep::*; let t: Store = make(); }",
		"fn crated() { extern crate other as dep; let t: dep::Store = make(); }",
		"fn generic<S>(s: S) { let t: S = s; }",
		"fn bounded<S: Store>(s: S) {}",
		"struct Holder<S>(S);",
		"impl<S> Holder<S> { fn go(&self, s: S) { let t: Self = make(); } }",
		"mod up { use super::*; fn run() { let t: Store = make(); } }",
		"mod outer { mod down { use super::*; fn run() { let t: Store = make(); } } }",
		"mod own { fn run() { let t: self::Store = make(); } }",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeRust, "main.rs", source)

	store := []fileindex.Typed{{Name: "t", Type: "Store"}}
	tests := []struct {
		key    string
		locals []fileindex.Typed
		params []fileindex.Typed
	}{
		{key: "main.rs//plain", locals: store},
		{key: "main.rs//sibling", locals: store},
		{key: "main.rs//alias"},
		{key: "main.rs//after", locals: []fileindex.Typed{{Name: "t", Type: "dep::Thing"}}},
		{key: "main.rs//outer"},
		{key: "main.rs//listed", locals: []fileindex.Typed{{Name: "t", Type: "dep::store::Store"}}},
		{key: "main.rs//globbed"},
		{key: "main.rs//crated"},
		{key: "main.rs//generic"},
		{key: "main.rs//bounded", params: []fileindex.Typed{{Name: "s", Type: "Store"}}},
		{key: "main.rs//Holder//go", locals: []fileindex.Typed{{Name: "t", Type: "Holder"}}},
		{key: "main.rs//up//run", locals: store},
		{key: "main.rs//outer//down//run"},
		{key: "main.rs//own//run"},
	}
	for _, tt := range tests {
		def := definitionOf(t, ix, tt.key)
		assert.Equal(t, tt.locals, def.Locals, tt.key)
		assert.Equal(t, tt.params, def.Params, tt.key)
	}
}

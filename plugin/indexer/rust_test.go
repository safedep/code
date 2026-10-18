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

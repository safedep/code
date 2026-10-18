package indexer

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
)

func TestFileIndexPHP(t *testing.T) {
	source := strings.Join([]string{
		"<?php",
		"namespace App\\Http;",
		"use App\\Jobs\\Mailer;",
		"require_once __DIR__ . '/helpers.php';",
		"include $path;",
		"#[Route('/a')]",
		"class Api extends Base implements Handler {",
		"    use Audits;",
		"    private Store $store;",
		"    public function __construct(private Mailer $mailer) {}",
		"    public static function make(Request $r): Item { return new static(); }",
		"    public function run() { array_map([$this, 'fmt'], []); usort($xs, 'cmp_items'); Route::get('/x', [Mailer::class, 'send']); eval('1'); }",
		"}",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodePHP, "Api.php", source)

	assert.Equal(t, `App\Http`, ix.Package)
	t.Run("a class has its bases, its traits, its fields and its attributes", func(t *testing.T) {
		api := classOf(t, ix, "Api.php//Api")
		assert.Equal(t, []string{"Base", "Handler", "Audits"}, api.Bases)
		assert.Equal(t, []fileindex.Typed{{Name: "store", Type: "Store"}, {Name: "mailer", Type: "Mailer"}}, api.Fields)
		assert.Equal(t, []string{"@Route"}, definitionOf(t, ix, "Api.php//Api").Decorators)
	})
	t.Run("a method has its static mark, its parameters and its result", func(t *testing.T) {
		make := definitionOf(t, ix, "Api.php//Api//make")
		assert.True(t, make.Static)
		assert.Equal(t, []fileindex.Typed{{Name: "$r", Type: "Request"}}, make.Params)
		assert.Equal(t, "Item", make.Returns)
	})
	t.Run("an include is an import and a computed one is a flag", func(t *testing.T) {
		var modules []string
		for _, imp := range ix.Imports {
			modules = append(modules, string(imp.Kind)+" "+imp.Module)
		}
		assert.Contains(t, modules, "static ./helpers.php")
		assert.Contains(t, modules, "dynamic-computed ")
		assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagComputedImport, Line: 5})
		assert.Contains(t, ix.Flags, fileindex.Flag{Kind: fileindex.FlagEval, Line: 12})
	})
	t.Run("a callable is a reference", func(t *testing.T) {
		var targets []string
		for _, ref := range ix.References {
			targets = append(targets, ref.Targets...)
		}
		assert.Equal(t, []string{"Api.php//Api//fmt", "cmp_items", "App//Jobs//Mailer//send"}, targets)
	})
}

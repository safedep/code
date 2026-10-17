package fileindex

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
)

func TestFileIndexGo(t *testing.T) {
	source := strings.Join([]string{
		"package shop",
		"import \"example.com/shop/store\"",
		"type Saver interface { io.Writer; Save(k string) error }",
		"type Cart struct { Base; *store.Store; items, tags []string; db *store.DB }",
		"type ID = store.ID",
		"func New(name string, opts ...Option) (*Cart, error) { return nil, nil }",
		"func (c *Cart) Add(a, b int) {",
		"  var s Saver = c",
		"  d, err := New(\"x\")",
		"  e := &store.DB{}",
		"  f := new(Cart)",
		"  g := s.(*Cart)",
		"  h := store.Open()",
		"  i, j := 1, 2",
		"  k := c.Add",
		"  run(c.Add, k, f)",
		"}",
		"func Apply[T Saver, U any](t T, u U) { b := NewBox[int](1) }",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeGo, "shop.go", source)

	assert.Equal(t, "shop", ix.Package)
	t.Run("a named type is a class and an interface is an interface", func(t *testing.T) {
		assert.Equal(t, fidx.DefInterface, definitionOf(t, ix, "shop.go//Saver").Kind)
		assert.Equal(t, fidx.DefClass, definitionOf(t, ix, "shop.go//Cart").Kind)
		assert.Equal(t, fidx.DefClass, definitionOf(t, ix, "shop.go//ID").Kind)
	})
	t.Run("an embedded type is a base and a named field has its type", func(t *testing.T) {
		cart := classOf(t, ix, "shop.go//Cart")
		assert.Equal(t, []string{"Base", "store.Store"}, cart.Bases)
		assert.Equal(t, []fidx.Typed{{Name: "db", Type: "store.DB"}}, cart.Fields)
		assert.Equal(t, []string{"io.Writer"}, classOf(t, ix, "shop.go//Saver").Bases)
		assert.Equal(t, []fidx.Signature{{Name: "Save", Arity: 1}}, classOf(t, ix, "shop.go//Saver").Abstract)
		assert.Equal(t, []string{"store.ID"}, classOf(t, ix, "shop.go//ID").Bases)
	})
	t.Run("a function has its signature", func(t *testing.T) {
		def := definitionOf(t, ix, "shop.go//New")
		assert.Equal(t, 2, def.Arity)
		assert.True(t, def.Variadic)
		assert.Equal(t, "Cart", def.Returns)
	})
	t.Run("a method has its receiver first and its typed locals", func(t *testing.T) {
		def := definitionOf(t, ix, "shop.go//Cart//Add")
		assert.Equal(t, "shop.go//Cart", def.Parent)
		assert.Equal(t, []fidx.Typed{{Name: "c", Type: "Cart"}}, def.Params)
		assert.Equal(t, 2, def.Arity)
		assert.Equal(t, []fidx.Typed{
			{Name: "s", Type: "Saver"},
			{Name: "d", Type: "New", Result: true},
			{Name: "e", Type: "store.DB"},
			{Name: "f", Type: "Cart"},
			{Name: "g", Type: "Cart"},
			{Name: "h", Type: "store.Open", Result: true},
		}, def.Locals)
	})
	t.Run("a type parameter stands for its constraint", func(t *testing.T) {
		def := definitionOf(t, ix, "shop.go//Apply")
		assert.Equal(t, []fidx.Typed{{Name: "t", Type: "Saver"}}, def.Params)
		assert.Equal(t, []fidx.Typed{{Name: "b", Type: "NewBox", Result: true}}, def.Locals)
	})
	t.Run("a method value is a reference and a typed value is none", func(t *testing.T) {
		var texts []string
		for _, ref := range ix.References {
			texts = append(texts, ref.Text)
		}
		assert.Equal(t, []string{"c.Add", "c.Add"}, texts)
	})
	t.Run("a call has its arguments", func(t *testing.T) {
		for _, call := range ix.Calls {
			if call.Callee == "run" {
				assert.Len(t, call.Args, 3)
			}
		}
	})
}

package indexer

import (
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/stretchr/testify/assert"
)

func TestFileIndexPythonTypeHints(t *testing.T) {
	source := strings.Join([]string{
		"from dataclasses import dataclass",
		"from typing import Optional",
		"import dep",
		"from dep import Thing",
		"",
		"def typed(t: Thing, o: Optional[Thing], u: Thing | None, d: dep.Doer, n: int, s: str = 'x', *args, **kwargs):",
		"    t.do()",
		"",
		"def two(a: Thing | Runner, lst: list[Thing]):",
		"    pass",
		"",
		"def returns() -> Thing:",
		"    return Thing()",
		"",
		"def forward() -> 'Thing':",
		"    return Thing()",
		"",
		"def optional() -> Optional[Thing]:",
		"    return None",
		"",
		"def pair() -> tuple[Thing, int]:",
		"    return Thing(), 1",
		"",
		"def builtin() -> int:",
		"    return 1",
		"",
		"def locals_():",
		"    a: Thing = make()",
		"    b = dep.new_thing()",
		"    c, n = dep.new_pair()",
		"    d = Thing()",
		"    e = 1",
		"    b.do()",
		"",
		"class App:",
		"    worker: Thing",
		"    count: int = 0",
		"",
		"    def __init__(self, t: Thing):",
		"        self.t = dep.new_thing()",
		"        self.n = 1",
		"        self.k: Thing = t",
		"",
		"    def go(self):",
		"        self.t.do()",
		"",
		"@dataclass",
		"class Job:",
		"    thing: Thing",
		"    name: str",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodePython, "a.py", source)

	t.Run("a parameter with a type hint has the type", func(t *testing.T) {
		assert.Equal(t, []fileindex.Typed{{Name: "t", Type: "Thing"}, {Name: "o", Type: "Thing"}, {Name: "u", Type: "Thing"}, {Name: "d", Type: "dep.Doer"}},
			definitionOf(t, ix, "a.py//typed").Params, "a builtin type, a default of a builtin type and a star parameter name no type")
	})
	t.Run("a union of two classes and a generic container name no type", func(t *testing.T) {
		assert.Empty(t, definitionOf(t, ix, "a.py//two").Params)
	})
	t.Run("a return hint gives the result type", func(t *testing.T) {
		assert.Equal(t, "Thing", definitionOf(t, ix, "a.py//returns").Returns)
		assert.Equal(t, "Thing", definitionOf(t, ix, "a.py//forward").Returns, "a forward reference in a string")
		assert.Equal(t, "Thing", definitionOf(t, ix, "a.py//optional").Returns, "an Optional unwraps")
		assert.Equal(t, "", definitionOf(t, ix, "a.py//builtin").Returns)
	})
	t.Run("a tuple return gives the type of each position", func(t *testing.T) {
		def := definitionOf(t, ix, "a.py//pair")
		assert.Equal(t, "", def.Returns)
		assert.Equal(t, []string{"Thing", ""}, def.Results)
	})
	t.Run("a local has its annotation or the call that gives its value", func(t *testing.T) {
		assert.Equal(t, []fileindex.Typed{
			{Name: "a", Type: "Thing"},
			{Name: "b", Type: "dep.new_thing", Result: true},
			{Name: "c", Type: "dep.new_pair", Result: true},
			{Name: "n", Type: "dep.new_pair", Result: true, Index: 1},
			{Name: "d", Type: "Thing", Result: true},
		}, definitionOf(t, ix, "a.py//locals_").Locals)
	})
	t.Run("a call on a typed local names the local in its scope", func(t *testing.T) {
		assert.Contains(t, callTargets(ix, "a.py//locals_"), "a.py//locals_//b//do")
		assert.Contains(t, callTargets(ix, "a.py//typed"), "a.py//typed//t//do")
		assert.NotContains(t, callTargets(ix, "a.py//locals_"), "dep//new_thing//do")
	})
	t.Run("the fields of a class come from its annotations and its methods", func(t *testing.T) {
		assert.Equal(t, []fileindex.Typed{{Name: "worker", Type: "Thing"}, {Name: "t", Type: "dep.new_thing", Result: true}, {Name: "k", Type: "Thing"}},
			classOf(t, ix, "a.py//App").Fields)
		assert.Equal(t, []fileindex.Typed{{Name: "thing", Type: "Thing"}}, classOf(t, ix, "a.py//Job").Fields)
	})
}

func callTargets(ix *fileindex.Index, caller string) []string {
	var out []string
	for _, c := range ix.Calls {
		if c.Caller == caller {
			out = append(out, c.Targets...)
		}
	}
	return out
}

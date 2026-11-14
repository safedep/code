package projectgraph

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/stretchr/testify/assert"
)

// goValueRows are the forms of a Go expression that gives a function
// value. fill stores the value in fs, and run calls fill and then fs[0]. A
// call of a function value concerns no target. So the search finds the
// function only through the reference that fill records, or through a
// record with the method name of a method value whose receiver the index
// cannot name.
var goValueRows = []receiverRow{
	{
		name:   "method expression",
		source: goValueSource("func(*dep.Thing)", "fs[0] = (*dep.Thing).ValueMethod", "fs[0](nil)"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "found/low",
	},
	{
		name:   "parenthesized function",
		source: goValueSource("func()", "fs[0] = (dep.ValueFunc)", "fs[0]()"),
		target: "example.com//dep//ValueFunc",
		want:   "found/low",
	},
	{
		name:   "generic instantiation",
		source: goValueSource("func()", "fs[0] = dep.ValueGeneric[int]", "fs[0]()"),
		target: "example.com//dep//ValueGeneric",
		want:   "found/low",
	},
	{
		name:   "generic instantiation with two type arguments",
		source: goValueSource("func()", "fs[0] = dep.ValuePair[int, string]", "fs[0]()"),
		target: "example.com//dep//ValuePair",
		want:   "found/low",
	},
	{
		name:   "method value on a call result",
		source: goValueSource("func()", "fs[0] = dep.NewThing().ValueMethod", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "found/low",
	},
	{
		name:   "method value on a type assertion",
		source: goValueSource("func()", "var x any = dep.NewThing()\n\tfs[0] = x.(*dep.Thing).ValueMethod", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "found/low",
	},
	{
		name:   "method value on a composite literal",
		source: goValueSource("func()", "fs[0] = (&dep.Thing{}).ValueMethod", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "found/low",
	},
	{
		name:   "method value on new",
		source: goValueSource("func()", "fs[0] = new(dep.Thing).ValueMethod", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "found/low",
	},
	{
		name:   "method value on a dereference",
		source: goValueSource("func()", "t := dep.NewThing()\n\tfs[0] = (*t).ValueMethod", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "found/low",
	},
	{
		name:   "method value on an element of a slice",
		source: goValueSource("func()", "ts := []*dep.Thing{dep.NewThing()}\n\tfs[0] = ts[0].ValueMethod", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "uncertain/unknown-receiver",
	},
	{
		name:   "method value on a local of no known type",
		source: goValueSource("func()", "ts := []*dep.Thing{dep.NewThing()}\n\tt := ts[0]\n\tfs[0] = t.ValueMethod", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "uncertain/unknown-receiver",
	},
	{
		name:   "method value on a range variable",
		source: goValueSource("func()", "for _, t := range []*dep.Thing{dep.NewThing()} {\n\t\tfs[0] = t.ValueMethod\n\t}", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "uncertain/unknown-receiver",
	},
	{
		name:   "method value on a receive",
		source: goValueSource("func()", "ch := make(chan *dep.Thing, 1)\n\tch <- dep.NewThing()\n\tfs[0] = (<-ch).ValueMethod", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "uncertain/unknown-receiver",
	},
	{
		name:   "method value on a local that shadows a function of another file",
		source: goValueSource("func()", "shadowed := []*dep.Thing{dep.NewThing()}[0]\n\tfs[0] = shadowed.ValueMethod", "fs[0]()"),
		target: "example.com//dep//Thing//ValueMethod",
		want:   "uncertain/unknown-receiver",
	},
	{
		name:   "argument of a generic call with type arguments",
		source: "func id[T any](x T) T { return x }\n\n" + goValueSource("func()", "fs[0] = id[func()](dep.ValueFunc)", "fs[0]()"),
		target: "example.com//dep//ValueFunc",
		want:   "found/low",
	},
	{
		name:   "conversion to a function type",
		source: goValueSource("func()", "fs[0] = (func())(dep.ValueFunc)", "fs[0]()"),
		target: "example.com//dep//ValueFunc",
		want:   "found/low",
	},
	{
		name:   "dereference of the address of a variable",
		source: goValueSource("func()", "p := &dep.ValueHook\n\tfs[0] = *p", "fs[0]()"),
		target: "example.com//dep//ValueHooked",
		want:   "found/low",
	},
	{
		name:   "function of another file",
		source: goValueSource("func()", "fs[0] = helper", "fs[0]()"),
		target: "example.com//dep//ValueFunc",
		want:   "found/low",
	},
	{
		name:   "function of another file after a local of its name in a closed block",
		source: goValueSource("func()", "{\n\t\thelper := 1\n\t\t_ = helper\n\t}\n\tfs[0] = helper", "fs[0]()"),
		target: "example.com//dep//ValueFunc",
		want:   "found/low",
	},
	{
		name:   "package variable that holds a function in parentheses",
		source: "var h = (dep.ValueFunc)\n\nfunc run() {\n\th()\n}",
		target: "example.com//dep//ValueFunc",
		want:   "found",
	},
	{
		name:   "package variable that holds a generic instantiation",
		source: "var h = dep.ValueGeneric[int]\n\nfunc run() {\n\th()\n}",
		target: "example.com//dep//ValueGeneric",
		want:   "found",
	},
	{
		name:   "package variable that holds a method value",
		source: "var h = dep.NewThing().ValueMethod\n\nfunc run() {\n\th()\n}",
		target: "example.com//dep//Thing//ValueMethod",
		want:   "found",
	},
}

// goValueSource is a main.go whose fill stores a value in fs, of the
// element type, with the statements, and whose run calls fill and then the
// element of fs with the call.
func goValueSource(element, statements, call string) string {
	return "var fs = make([]" + element + ", 1)\n\nfunc fill() {\n\t" + statements + "\n}\n\nfunc run() {\n\tfill()\n\t" + call + "\n}"
}

// goValueProject is the receiver project of the source with other.go. Its
// helper calls dep.ValueFunc, and the result of its shadowed has a method of
// the name of a method of dep.Thing.
func goValueProject(source string) map[string]string {
	files := goReceiverProject(source)
	files["other.go"] = "package main\n\nimport \"example.com/dep\"\n\nfunc helper() {\n\tdep.ValueFunc()\n}\n\n" +
		"type local struct{}\n\nfunc (local) ValueMethod() {}\n\nfunc shadowed() local { return local{} }\n"
	return files
}

func TestGoFunctionValues(t *testing.T) {
	languages := caseLanguages(t)
	for _, row := range goValueRows {
		t.Run(row.name, func(t *testing.T) {
			assert.Equal(t, row.want, receiverAnswer(t, languages, core.LanguageCodeGo, goValueProject(row.source), row))
		})
	}
}

package ecmascript_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/lang/ecmascript"
	"github.com/safedep/code/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bs is a backslash. The escape cases build their text with it, so no tool
// on the way to the file decodes an escape in the source of this test.
const bs = `\`

func analyze(t testing.TB, l core.Language, source string) *ecmascript.Analysis {
	tree, err := parser.ParseBytes(context.Background(), l, []byte(source), parser.WithMaxBytes(-1))
	require.NoError(t, err)
	a, err := ecmascript.Analyze(context.Background(), tree, ecmascript.Options{MaxNodes: -1})
	require.NoError(t, err)
	return a
}

func js(t testing.TB, source string) *ecmascript.Analysis {
	l, err := lang.NewJavascriptLanguage()
	require.NoError(t, err)
	return analyze(t, l, source)
}

func ts(t testing.TB, source string) *ecmascript.Analysis {
	l, err := lang.NewTypescriptWithoutJSXLanguage()
	require.NoError(t, err)
	return analyze(t, l, source)
}

var (
	requireV = ecmascript.Value{Global: "require"}
	evalV    = ecmascript.Value{Global: "eval"}
	execV    = ecmascript.Value{Module: "child_process", Member: "exec"}
	cpV      = ecmascript.Value{Module: "child_process"}
)

// has reports a value in a list.
func has(values []ecmascript.Value, want ecmascript.Value) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name   string
		source string
		lookup string
		want   ecmascript.Value
	}{
		{"free name", "x()", "eval", evalV},
		{"const alias of require", "const r = require", "r", requireV},
		{"alias through a comma", "var e = (0, eval)", "e", evalV},
		{"chain of names", "const a = eval; const b = a; const c = b", "c", evalV},
		{"assignment", "let r; r = require", "r", requireV},
		{"assignment as a value", "var r = exports.r = require", "r", requireV},
		{"chained assignment", "const r = a = require", "r", requireV},
		{"global member", "global.r = require", "r", requireV},
		{"globalThis subscript", "globalThis['r'] = require", "r", requireV},
		{"alias of the global object", "const g = globalThis; g.r = eval", "r", evalV},
		{"alias of the global object after its use", "function f() { g.r = eval }\nglobal.g = globalThis", "r", evalV},
		{"member of the global object", "const e = globalThis.eval", "e", evalV},
		{"folded subscript", "const e = globalThis['ev' + 'al']", "e", evalV},
		{"required module", "const c = require('child_process')", "c", cpV},
		{"hex escape in a module name", "const c = require('child" + bs + "x5fprocess')", "c", cpV},
		{"octal escape in a module name", "const c = require('child" + bs + "137process')", "c", cpV},
		{"folded module name", "const c = require('child_' + 'process')", "c", cpV},
		{"renamed require", "const r = require; const c = r('child_process')", "c", cpV},
		{"global member require", "global.r = require; const c = global.r('child_process')", "c", cpV},
		{"module.require", "const c = module.require('child_process')", "c", cpV},
		{"process.mainModule.require", "const c = process.mainModule.require('child_process')", "c", cpV},
		{"require.main.require", "const c = require.main.require('child_process')", "c", cpV},
		{"createRequire", "const { createRequire } = require('module'); const r = createRequire(import.meta.url); const c = r('child_process')", "c", cpV},
		{"default parameter", "const f = (r = require) => r('child_process')", "r", requireV},
		{"logical branch", "const e = x || eval", "e", evalV},
		{"conditional branch", "const e = c ? 0 : eval", "e", evalV},
		{"logical assignment", "let e; e ||= eval", "e", evalV},
		{"bind", "const e = eval.bind(null)", "e", evalV},
		{"destructured require", "const { exec } = require('child_process')", "exec", execV},
		{"renamed destructure", "const { exec: run } = require('child_process')", "run", execV},
		{"destructure with default", "const { exec: run = null } = require('child_process')", "run", execV},
		{"shorthand destructure default", "const { run = eval } = cfg", "run", evalV},
		{"destructure of an alias", "const c = require('child_process'); const { exec } = c", "exec", execV},
		{"destructure of the global object", "const { eval: e } = globalThis", "e", evalV},
		{"member of a module", "const run = require('child_process').exec", "run", execV},
		{"member of a bound module", "const c = require('child_process'); const run = c.exec", "run", execV},
		{"renamed import", "import { exec as run } from 'child_process'", "run", execV},
		{"named import", "import { exec } from 'child_process'", "exec", execV},
		{"string specifier", "import { 'exec' as run } from 'child_process'", "run", execV},
		{"escaped import source", "import { exec } from 'child" + bs + "x5fprocess'", "exec", execV},
		{"namespace import", "import * as c from 'child_process'", "c", cpV},
		{"default import", "import c from 'child_process'", "c", cpV},
		{"default specifier", "import { default as c } from 'child_process'", "c", cpV},
		{"dynamic import", "const c = await import('child_process')", "c", cpV},
		{"default member of a dynamic import", "const { default: c } = await import('child_process')", "c", cpV},
		{"two values", "let x = eval; x = require", "x", requireV},
		{"local binding keeps the global", "function f() { var require = 0 }\nconst { exec } = require('child_process')", "exec", execV},
		{"block binding keeps the global", "{ const eval = f }\neval(x)", "eval", evalV},
		{"block let keeps the global", "if (c) { let eval = f }", "eval", evalV},
		{"cycle with a value", "let a = eval; let b = a; a = b", "b", evalV},
		{"unicode escape in a name", "const e = " + bs + "u0065val", "e", evalV},
		{"unicode escape with braces", "const e = " + bs + "u{65}val", "e", evalV},
		{"unicode escape with leading zeros", "const e = " + bs + "u{00000065}val", "e", evalV},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := js(t, tc.source).Resolve(tc.lookup)
			assert.True(t, has(got, tc.want), "%v", got)
		})
	}
}

func TestResolveGlobals(t *testing.T) {
	cases := []struct {
		source string
		lookup string
		want   []ecmascript.Value
	}{
		{"x()", "x", []ecmascript.Value{{Global: "x"}}},
		{"const x = f()", "x", nil},
		{"const x = eval", "x", []ecmascript.Value{evalV}},
		{"const { x } = f()", "x", nil},
		{"import x from 'm'", "x", []ecmascript.Value{{Module: "m"}}},
		{"function f() { const x = eval }", "x", []ecmascript.Value{{Global: "x"}, evalV}},
		{"x = eval", "x", []ecmascript.Value{{Global: "x"}, evalV}},
		{"if (c) { var x = f() }", "x", nil},
		{"const eval = f; globalThis.eval(x)", "eval", []ecmascript.Value{{Global: "f"}}},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			assert.Equal(t, tc.want, js(t, tc.source).Resolve(tc.lookup))
		})
	}
	a := js(t, "x")
	assert.Nil(t, a.Resolve(""))
	assert.Nil(t, a.Resolve(strings.Repeat("a", 1000)))
}

func TestValueLimitMarksTruncated(t *testing.T) {
	var b strings.Builder
	b.WriteString("var cp;")
	for i := 0; i < 16; i++ {
		b.WriteString("cp = require('m" + strconv.Itoa(i) + "');")
	}
	b.WriteString("var cp = require('child_process'); cp.exec(c)")
	a := js(t, b.String())
	assert.Len(t, a.Resolve("cp"), 16)
	assert.True(t, a.Truncated())

	assert.False(t, js(t, "var cp = require('child_process'); cp.exec(c)").Truncated())
}

// TestValueLimitMarksTruncatedForALateValue fills a name before one of its
// dependencies gets its value.
func TestValueLimitMarksTruncatedForALateValue(t *testing.T) {
	var b strings.Builder
	b.WriteString("var y; var x;")
	for i := 0; i < 16; i++ {
		b.WriteString("x = require('m" + strconv.Itoa(i) + "');")
	}
	b.WriteString("x = y; y = require('child_process');")
	a := js(t, b.String())
	assert.Len(t, a.Resolve("x"), 16)
	assert.True(t, a.Truncated())
}

func TestGlobalAliasRounds(t *testing.T) {
	short := js(t, "global.g1 = globalThis; g1.g2 = globalThis; g2.r = eval")
	assert.True(t, has(short.Resolve("r"), evalV))
	assert.False(t, short.Truncated())

	long := js(t, "global.g1 = globalThis; g1.g2 = globalThis; g2.g3 = globalThis; g3.g4 = globalThis; g4.g5 = globalThis; g5.r = eval")
	assert.True(t, long.Truncated())
}

func TestGlobalObjectMemberReadsTheGlobal(t *testing.T) {
	a := js(t, "const eval = f; globalThis.eval(x)")
	c := lastStatementCall(t, a)
	assert.True(t, has(a.Eval(c.Callee), evalV))
}

func TestResolveIsOrderFree(t *testing.T) {
	a := js(t, chain(40))
	assert.True(t, has(a.Resolve("a20"), evalV))
	assert.True(t, has(a.Resolve("a39"), evalV))

	b := js(t, chain(40))
	assert.True(t, has(b.Resolve("a39"), evalV))
	assert.True(t, has(b.Resolve("a20"), evalV))
}

func TestResolveTypeScript(t *testing.T) {
	cases := []struct {
		source string
		lookup string
		want   ecmascript.Value
	}{
		{"const r = require as any", "r", requireV},
		{"const e = eval!", "e", evalV},
		{"const e = (eval satisfies unknown)", "e", evalV},
		{"const e = <any>eval", "e", evalV},
		{"import fs = require('fs')", "fs", ecmascript.Value{Module: "fs"}},
		{"import { exec as run } from 'child_process'", "run", execV},
		{"import d, * as ns from 'child_process'", "ns", cpV},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			got := ts(t, tc.source).Resolve(tc.lookup)
			assert.True(t, has(got, tc.want), "%v", got)
		})
	}
}

// TestTypeAssertionKeepsTheFile checks that a type assertion does not hide
// the rest of a .ts file. The TSX grammar reads <any>x as JSX.
func TestTypeAssertionKeepsTheFile(t *testing.T) {
	a := ts(t, "(<any>globalThis).eval(code)\nconst { exec } = require('child_process')\nexec(cmd)")
	require.Len(t, a.Calls(), 3)
	assert.True(t, has(a.Eval(a.Calls()[0].Callee), evalV))
	assert.True(t, has(a.Eval(a.Calls()[2].Callee), execV))
}

// lastStatementCall returns the outer call of the last statement.
func lastStatementCall(t *testing.T, a *ecmascript.Analysis) ecmascript.Call {
	var last *ecmascript.Call
	for i, c := range a.Calls() {
		if c.Node.Parent().Type() == "expression_statement" {
			last = &a.Calls()[i]
		}
	}
	require.NotNil(t, last)
	return *last
}

func TestCalls(t *testing.T) {
	cases := []struct {
		source string
		name   string
		isNew  bool
		args   int
		target ecmascript.Value
	}{
		{"eval(x)", "eval", false, 1, evalV},
		{"(0, eval)(x)", "eval", false, 1, evalV},
		{"(0, /* c */ eval /* d */)(x)", "eval", false, 1, evalV},
		{"(e = eval)(x)", "eval", false, 1, evalV},
		{"globalThis.eval(x)", "eval", false, 1, evalV},
		{"globalThis['eval'](x)", "eval", false, 1, evalV},
		{"window[`eval`](x)", "eval", false, 1, evalV},
		{"eval?.(x)", "eval", false, 1, evalV},
		{"eval.call(null, x)", "call", false, 2, evalV},
		{"eval.apply(null, [x])", "apply", false, 2, evalV},
		{"new Function('a', 'b')", "Function", true, 2, ecmascript.Value{Global: "Function"}},
		{"require('child_process').exec(c)", "exec", false, 1, execV},
		{"const { exec: run } = require('child_process'); run(c)", "run", false, 1, execV},
		{"const { exec } = require('child_process'); exec.call(null, c)", "call", false, 2, execV},
		{"(await import('child_process')).default.exec(c)", "exec", false, 1, execV},
		{"f(a, /* c */ b)", "f", false, 2, ecmascript.Value{Global: "f"}},
		{"f`x`", "f", false, 1, ecmascript.Value{Global: "f"}},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			a := js(t, tc.source)
			c := lastStatementCall(t, a)
			assert.Equal(t, tc.name, c.Name)
			assert.Equal(t, tc.isNew, c.New)
			assert.Len(t, c.Args, tc.args)
			got := a.Eval(c.Callee)
			assert.True(t, has(got, tc.target), "%v", got)
		})
	}
}

func TestCallsWithNoTarget(t *testing.T) {
	for _, source := range []string{"f()()", "(f())()", "import(x)"} {
		t.Run(source, func(t *testing.T) {
			a := js(t, source)
			c := lastStatementCall(t, a)
			assert.Empty(t, a.Eval(c.Callee))
		})
	}
	assert.Equal(t, "", lastStatementCall(t, js(t, "f()()")).Name)
	assert.Equal(t, "import", lastStatementCall(t, js(t, "import(x)")).Name)
}

func TestCallArg(t *testing.T) {
	c := js(t, "f(a, b)").Calls()[0]
	assert.Equal(t, "identifier", c.Arg(1).Type())
	assert.Nil(t, c.Arg(2))
	assert.Nil(t, c.Arg(-1))
	assert.Len(t, ecmascript.Args(c.Node), 2)
	assert.Nil(t, ecmascript.Args(nil))
}

func TestAssignments(t *testing.T) {
	cases := []struct {
		source   string
		property string
		computed bool
		global   bool
	}{
		{"global.r = require", "r", false, true},
		{"globalThis['r'] = require", "r", false, true},
		{"(global).r = require", "r", false, true},
		{"global[k] = require", "", true, true},
		{"global[`${k}`] = require", "", true, true},
		{"const g = global; g.r = 1", "r", false, true},
		{"module.exports = {}", "exports", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			a := js(t, tc.source)
			require.Len(t, a.Assignments(), 1)
			as := a.Assignments()[0]
			assert.Equal(t, tc.property, as.Property)
			assert.Equal(t, tc.computed, as.Computed)
			assert.Equal(t, tc.global, as.Global)
		})
	}
}

func TestBindings(t *testing.T) {
	a := js(t, "import { exec } from 'child_process'\nconst { eval: e } = globalThis\nconst r = require\nglobal.f = Function")
	bindings := a.Bindings()
	require.Len(t, bindings, 4)

	var names []string
	for _, b := range bindings {
		names = append(names, b.Name)
	}
	assert.Equal(t, []string{"exec", "e", "r", "f"}, names)
	assert.Nil(t, bindings[0].Value)
	assert.Equal(t, "exec", bindings[0].Member)
	assert.Equal(t, "eval", bindings[1].Member)
	assert.Equal(t, "identifier", bindings[2].Value.Type())
	assert.Equal(t, "", bindings[2].Member)
}

func TestClassifyArg(t *testing.T) {
	cases := map[string]ecmascript.ArgKind{
		"f('x')":       ecmascript.ArgString,
		"f(('x'))":     ecmascript.ArgString,
		"f(`x`)":       ecmascript.ArgString,
		"f(`${a}.js`)": ecmascript.ArgTemplate,
		"f(a)":         ecmascript.ArgName,
		"f(a.b)":       ecmascript.ArgMember,
		"f(1)":         ecmascript.ArgLiteral,
		"f(null)":      ecmascript.ArgLiteral,
		"f(a['b'])":    ecmascript.ArgComputed,
		"f('a' + b)":   ecmascript.ArgComputed,
		"f(g())":       ecmascript.ArgComputed,
		"f()":          ecmascript.ArgNone,
	}
	for source, want := range cases {
		t.Run(source, func(t *testing.T) {
			a := js(t, source)
			assert.Equal(t, want, a.ClassifyArg(a.Calls()[0].Arg(0)))
		})
	}
}

func TestStringValue(t *testing.T) {
	cases := map[string]string{
		`f('a')`:                               "a",
		`f("a` + bs + `"b")`:                   `a"b`,
		`f('child` + bs + `x5fprocess')`:       "child_process",
		`f('` + bs + `145val')`:                "eval",
		`f('` + bs + `u0065val')`:              "eval",
		`f('` + bs + `u{65}val')`:              "eval",
		`f('a` + bs + `0b')`:                   "a\x00b",
		"f(`x`)":                               "x",
		`f(('x'))`:                             "x",
		`f('a` + bs + "\n" + `b')`:             "ab",
		`f('` + bs + `q')`:                     "q",
		`f('ev' + 'al')`:                       "eval",
		`f('a' + ('b' + 'c') + ` + "`d`" + `)`: "abcd",
		`f('` + bs + `u{110000}')`:             "u{110000}",
		`f('trailing` + bs + bs + `')`:         `trailing` + bs,
	}
	for source, want := range cases {
		t.Run(source, func(t *testing.T) {
			a := js(t, source)
			got, ok := a.StringValue(a.Calls()[0].Arg(0))
			require.True(t, ok)
			assert.Equal(t, want, got)
		})
	}
	for _, source := range []string{"f(a)", "f('a' + b)", "f('a' - 'b')", "f(`${a}`)"} {
		a := js(t, source)
		_, ok := a.StringValue(a.Calls()[0].Arg(0))
		assert.False(t, ok, source)
	}
}

func TestStringValueLimit(t *testing.T) {
	a := js(t, "f("+strings.Repeat("'aaaaaaaa' + ", 1000)+"'a')")
	_, ok := a.StringValue(a.Calls()[0].Arg(0))
	assert.False(t, ok)
}

func TestStringPrefix(t *testing.T) {
	cases := map[string]string{
		"f(`./${name}`)":          "./",
		"f(`./plugins/`)":         "./plugins/",
		"f('./x')":                "./x",
		"f(`" + bs + "x2e/${a}`)": "./",
	}
	for source, want := range cases {
		t.Run(source, func(t *testing.T) {
			a := js(t, source)
			got, ok := a.StringPrefix(a.Calls()[0].Arg(0))
			require.True(t, ok)
			assert.Equal(t, want, got)
		})
	}
	a := js(t, "f(a)")
	_, ok := a.StringPrefix(a.Calls()[0].Arg(0))
	assert.False(t, ok)
}

func TestAnalyzeRejectsOtherLanguages(t *testing.T) {
	l, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	tree, err := parser.ParseBytes(context.Background(), l, []byte("x = 1"))
	require.NoError(t, err)
	_, err = ecmascript.Analyze(context.Background(), tree, ecmascript.Options{})
	assert.Error(t, err)
}

func TestAnalyzeNodeLimit(t *testing.T) {
	l, err := lang.NewJavascriptLanguage()
	require.NoError(t, err)
	tree, err := parser.ParseBytes(context.Background(), l, []byte(strings.Repeat("eval(x);", 100)))
	require.NoError(t, err)
	a, err := ecmascript.Analyze(context.Background(), tree, ecmascript.Options{MaxNodes: 50})
	require.NoError(t, err)
	assert.True(t, a.Truncated())
	assert.NotEmpty(t, a.Calls())
	assert.Less(t, len(a.Calls()), 100)
}

func TestAnalyzeStopsOnDoneContext(t *testing.T) {
	l, err := lang.NewJavascriptLanguage()
	require.NoError(t, err)
	tree, err := parser.ParseBytes(context.Background(), l, []byte(strings.Repeat("eval(x);", 10_000)))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ecmascript.Analyze(ctx, tree, ecmascript.Options{})
	assert.ErrorIs(t, err, context.Canceled)
}

// TestAnalyzeHostileShapes checks that the analysis stays linear on input
// that makes a naive walk quadratic. A copy of the callee text for each
// call made x()()() of 1 MB take 72 s in vet.
func TestAnalyzeHostileShapes(t *testing.T) {
	if testing.Short() {
		t.Skip("parses large inputs")
	}
	shapes := map[string]string{
		"call chain":     "x" + strings.Repeat("()", 100_000),
		"member chain":   "x" + strings.Repeat(".a", 100_000) + "()",
		"wide call":      "f(" + strings.Repeat("a,", 100_000) + ")",
		"deep parens":    strings.Repeat("(", 20_000) + "eval" + strings.Repeat(")", 20_000) + "(x)",
		"long sequence":  "(" + strings.Repeat("0,", 100_000) + "eval)(x)",
		"long sum":       "f(" + strings.Repeat("'a' + ", 50_000) + "'a')",
		"deep branches":  "const e = " + strings.Repeat("a || (", 10_000) + "eval" + strings.Repeat(")", 10_000),
		"name chain":     chain(30_000),
		"many bindings":  strings.Repeat("x = eval; x = require;", 20_000) + "x()",
		"name graph":     graph(300),
		"long name":      "const " + strings.Repeat("a", 1_000_000) + " = eval",
		"global members": strings.Repeat("global.a = global.a;", 20_000) + "a()",
	}
	for name, source := range shapes {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			a := js(t, source)
			for _, c := range a.Calls() {
				_ = a.Eval(c.Callee)
				for _, arg := range c.Args {
					_ = a.ClassifyArg(arg)
					_, _ = a.StringValue(arg)
				}
			}
			for _, b := range a.Bindings() {
				_ = a.Resolve(b.Name)
			}
			assert.Less(t, time.Since(start), 20*time.Second)
		})
	}
}

// TestReverseChainStopsAtTheBudget checks a name with many sources that
// read a chain of aliases in reverse order. Each step of the chain changes
// one source, so each step reads all sources again.
func TestReverseChainStopsAtTheBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("parses a large input")
	}
	const n = 20_000
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("var x = a" + strconv.Itoa(i) + ";")
	}
	for i := n - 1; i > 0; i-- {
		b.WriteString("var a" + strconv.Itoa(i) + " = a" + strconv.Itoa(i-1) + ";")
	}
	b.WriteString("var a0 = eval;")
	start := time.Now()
	a := js(t, b.String())
	assert.Less(t, time.Since(start), 20*time.Second)
	assert.True(t, a.Truncated())
}

func TestLongSequenceResolves(t *testing.T) {
	a := js(t, "("+strings.Repeat("0,", 1000)+"eval)(x)")
	c := a.Calls()[0]
	assert.Equal(t, "eval", c.Name)
	assert.True(t, has(a.Eval(c.Callee), evalV))
}

// chain binds a0 to eval and each later name to the name before it.
func chain(n int) string {
	var b strings.Builder
	b.WriteString("const a0 = eval;")
	for i := 1; i < n; i++ {
		b.WriteString("const a" + strconv.Itoa(i) + " = a" + strconv.Itoa(i-1) + ";")
	}
	return b.String()
}

// graph assigns each of n names to each other name.
func graph(n int) string {
	var b strings.Builder
	b.WriteString("v0 = eval;")
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			b.WriteString("v" + strconv.Itoa(i) + " = v" + strconv.Itoa(j) + ";")
		}
	}
	return b.String()
}

func FuzzAnalyze(f *testing.F) {
	for _, s := range []string{
		"", "eval(x)", "(0, eval)(x)", "global.r = require", "const { exec: run } = require('child_process'); run(c)",
		"import * as c from 'child_process'", "a = b; b = a", bs + "u0065val('x')", "f`x${y}`", "<svg/>",
		"x = a || b ? c : d", "'a' + 'b' + c", "module.require('x')", "eval.call(null, x)",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, source string) {
		a := js(t, source)
		for _, c := range a.Calls() {
			_ = a.Eval(c.Callee)
			for _, arg := range c.Args {
				_ = a.ClassifyArg(arg)
				_, _ = a.StringValue(arg)
				_, _ = a.StringPrefix(arg)
			}
		}
		for _, as := range a.Assignments() {
			_ = a.Eval(as.Object)
		}
		for _, b := range a.Bindings() {
			_ = a.Resolve(b.Name)
		}
	})
}

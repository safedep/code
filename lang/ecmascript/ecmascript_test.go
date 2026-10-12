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

func analyze(t testing.TB, code core.LanguageCode, source string) *ecmascript.File {
	l, err := lang.GetLanguage(string(code))
	require.NoError(t, err)
	tree, err := parser.ParseBytes(context.Background(), l, []byte(source), parser.WithMaxBytes(-1))
	require.NoError(t, err)
	f, err := ecmascript.Analyze(context.Background(), tree, ecmascript.Options{})
	require.NoError(t, err)
	return f
}

func js(t testing.TB, source string) *ecmascript.File {
	return analyze(t, core.LanguageCodeJavascript, source)
}

var (
	require_ = ecmascript.Value{Global: "require"}
	eval     = ecmascript.Value{Global: "eval"}
	exec     = ecmascript.Value{Module: "child_process", Member: "exec"}
	cp       = ecmascript.Value{Module: "child_process"}
)

func TestResolve(t *testing.T) {
	cases := []struct {
		name   string
		source string
		lookup string
		want   []ecmascript.Value
	}{
		{"free name is the global", "x()", "eval", []ecmascript.Value{eval}},
		{"const alias of require", "const r = require", "r", []ecmascript.Value{require_}},
		{"let alias of eval", "let e = eval", "e", []ecmascript.Value{eval}},
		{"var alias through parentheses", "var e = (0, eval)", "e", []ecmascript.Value{eval}},
		{"chain of names", "const a = eval; const b = a; const c = b", "c", []ecmascript.Value{eval}},
		{"assignment", "let r; r = require", "r", []ecmascript.Value{require_}},
		{"global member", "global.r = require", "r", []ecmascript.Value{require_}},
		{"globalThis subscript", "globalThis['r'] = require", "r", []ecmascript.Value{require_}},
		{"alias of the global object", "const g = globalThis; g.r = eval", "r", []ecmascript.Value{eval}},
		{"member of the global object", "const e = globalThis.eval", "e", []ecmascript.Value{eval}},
		{"required module", "const c = require('child_process')", "c", []ecmascript.Value{cp}},
		{"escaped module name", `const c = require('child\x5fprocess')`, "c", []ecmascript.Value{cp}},
		{"node prefix", "const c = require('node:child_process')", "c", []ecmascript.Value{{Module: "node:child_process"}}},
		{"renamed require", "const r = require; const c = r('child_process')", "c", []ecmascript.Value{cp}},
		{"global member require", "global.r = require; const c = global.r('child_process')", "c", []ecmascript.Value{cp}},
		{"createRequire", "const { createRequire } = require('module'); const r = createRequire(import.meta.url); const c = r('child_process')", "c", []ecmascript.Value{cp}},
		{"destructured require", "const { exec } = require('child_process')", "exec", []ecmascript.Value{exec}},
		{"renamed destructure", "const { exec: run } = require('child_process')", "run", []ecmascript.Value{exec}},
		{"destructure with default", "const { exec: run = null } = require('child_process')", "run", []ecmascript.Value{exec}},
		{"destructure of an alias", "const c = require('child_process'); const { exec } = c", "exec", []ecmascript.Value{exec}},
		{"member of a module", "const run = require('child_process').exec", "run", []ecmascript.Value{exec}},
		{"member of a bound module", "const c = require('child_process'); const run = c.exec", "run", []ecmascript.Value{exec}},
		{"renamed import", "import { exec as run } from 'child_process'", "run", []ecmascript.Value{exec}},
		{"named import", "import { exec } from 'child_process'", "exec", []ecmascript.Value{exec}},
		{"namespace import", "import * as c from 'child_process'", "c", []ecmascript.Value{cp}},
		{"default import", "import c from 'child_process'", "c", []ecmascript.Value{cp}},
		{"default specifier", "import { default as c } from 'child_process'", "c", []ecmascript.Value{cp}},
		{"string specifier", "import { 'exec' as run } from 'child_process'", "run", []ecmascript.Value{exec}},
		{"escaped import source", `import { exec } from 'child\x5fprocess'`, "exec", []ecmascript.Value{exec}},
		{"dynamic import", "const c = await import('child_process')", "c", []ecmascript.Value{cp}},
		{"result of a call", "const x = f()", "x", nil},
		{"two values", "let x = eval; x = require", "x", []ecmascript.Value{eval, require_}},
		{"global member", "const env = process.env", "env", []ecmascript.Value{{Global: "process", Member: "env"}}},
		{"cycle", "a = b; b = a", "a", nil},
		{"escaped identifier", `const e = eval`, "e", []ecmascript.Value{eval}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, js(t, tc.source).Resolve(tc.lookup))
		})
	}
}

func TestResolveTypeScript(t *testing.T) {
	cases := []struct {
		source string
		lookup string
		want   []ecmascript.Value
	}{
		{"const r = require as any", "r", []ecmascript.Value{require_}},
		{"const e = eval!", "e", []ecmascript.Value{eval}},
		{"const e = (eval satisfies unknown)", "e", []ecmascript.Value{eval}},
		{"import fs = require('fs')", "fs", []ecmascript.Value{{Module: "fs"}}},
		{"import { exec as run } from 'child_process'", "run", []ecmascript.Value{exec}},
		{"import d, * as ns from 'child_process'", "ns", []ecmascript.Value{cp}},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			assert.Equal(t, tc.want, analyze(t, core.LanguageCodeTypescript, tc.source).Resolve(tc.lookup))
		})
	}
}

func TestCalls(t *testing.T) {
	cases := []struct {
		source string
		name   string
		isNew  bool
		args   int
		target []ecmascript.Value
	}{
		{"eval(x)", "eval", false, 1, []ecmascript.Value{eval}},
		{"(0, eval)(x)", "eval", false, 1, []ecmascript.Value{eval}},
		{"(0, /* c */ eval /* d */)(x)", "eval", false, 1, []ecmascript.Value{eval}},
		{"globalThis.eval(x)", "eval", false, 1, []ecmascript.Value{eval}},
		{"globalThis['eval'](x)", "eval", false, 1, []ecmascript.Value{eval}},
		{"window[`eval`](x)", "eval", false, 1, []ecmascript.Value{eval}},
		{"new Function('a', 'b')", "Function", true, 2, []ecmascript.Value{{Global: "Function"}}},
		{"require('child_process').exec(c)", "exec", false, 1, []ecmascript.Value{exec}},
		{"const { exec: run } = require('child_process'); run(c)", "run", false, 1, []ecmascript.Value{exec}},
		{"f()()", "", false, 0, nil},
		{"(f())()", "", false, 0, nil},
		{"import(x)", "import", false, 1, nil},
		{"f(a, /* c */ b)", "f", false, 2, []ecmascript.Value{{Global: "f"}}},
		{"f`x`", "f", false, 1, []ecmascript.Value{{Global: "f"}}},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			f := js(t, tc.source)
			c := lastStatementCall(t, f)
			assert.Equal(t, tc.name, c.Name)
			assert.Equal(t, tc.isNew, c.New)
			assert.Len(t, c.Args, tc.args)
			assert.Equal(t, tc.target, f.Eval(c.Callee))
		})
	}
}

// lastStatementCall returns the outer call of the last statement.
func lastStatementCall(t *testing.T, f *ecmascript.File) ecmascript.Call {
	var last *ecmascript.Call
	for i, c := range f.Calls() {
		if c.Node.Parent().Type() == "expression_statement" {
			last = &f.Calls()[i]
		}
	}
	require.NotNil(t, last)
	return *last
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
		{"global[k] = require", "", true, true},
		{"global[`${k}`] = require", "", true, true},
		{"const g = global; g.r = 1", "r", false, true},
		{"module.exports = {}", "exports", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			f := js(t, tc.source)
			require.Len(t, f.Assignments(), 1)
			a := f.Assignments()[0]
			assert.Equal(t, tc.property, a.Property)
			assert.Equal(t, tc.computed, a.Computed)
			isGlobal := false
			for _, v := range f.Eval(a.Object) {
				isGlobal = isGlobal || v.IsGlobalObject()
			}
			assert.Equal(t, tc.global, isGlobal)
		})
	}
}

func TestBindings(t *testing.T) {
	f := js(t, "import { exec } from 'child_process'\nconst r = require\nglobal.e = eval")
	var names []string
	for _, b := range f.Bindings() {
		names = append(names, b.Name)
	}
	assert.Equal(t, []string{"exec", "r", "e"}, names)
	assert.Nil(t, f.Bindings()[0].Value)
	assert.Equal(t, "identifier", f.Bindings()[1].Value.Type())
}

func TestClassifyArg(t *testing.T) {
	cases := map[string]ecmascript.ArgKind{
		"f('x')":       ecmascript.ArgString,
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
			c := js(t, source).Calls()[0]
			var arg = c.Node.ChildByFieldName("arguments").NamedChild(0)
			assert.Equal(t, want, ecmascript.ClassifyArg(arg))
		})
	}
}

func TestStringValue(t *testing.T) {
	cases := map[string]string{
		`f('a')`:                "a",
		`f("a\"b")`:             `a"b`,
		`f('child\x5fprocess')`: "child_process",
		`f('eval')`:             "eval",
		`f('\u{65}val')`:        "eval",
		"f(`x`)":                "x",
		`f(('x'))`:              "x",
		`f('a\` + "\n" + `b')`:  "ab",
		`f('\q')`:               "q",
	}
	for source, want := range cases {
		t.Run(source, func(t *testing.T) {
			f := js(t, source)
			got, ok := f.StringValue(f.Calls()[0].Args[0])
			require.True(t, ok)
			assert.Equal(t, want, got)
		})
	}
	f := js(t, "f(a)")
	_, ok := f.StringValue(f.Calls()[0].Args[0])
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
	f, err := ecmascript.Analyze(context.Background(), tree, ecmascript.Options{MaxNodes: 50})
	require.NoError(t, err)
	assert.True(t, f.Truncated())
	assert.NotEmpty(t, f.Calls())
	assert.Less(t, len(f.Calls()), 100)
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
		"call chain":     "x" + strings.Repeat("()", 200_000),
		"member chain":   "x" + strings.Repeat(".a", 300_000) + "()",
		"wide call":      "f(" + strings.Repeat("a,", 300_000) + ")",
		"deep parens":    strings.Repeat("(", 50_000) + "eval" + strings.Repeat(")", 50_000) + "(x)",
		"long sequence":  "(" + strings.Repeat("0,", 200_000) + "eval)(x)",
		"name chain":     chain(100_000),
		"many bindings":  strings.Repeat("x = eval; x = require;", 50_000) + "x()",
		"long name":      "const " + strings.Repeat("a", 1_000_000) + " = eval",
		"global members": strings.Repeat("global.a = global.a;", 50_000) + "a()",
	}
	for name, source := range shapes {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			f := js(t, source)
			for _, c := range f.Calls() {
				_ = f.Eval(c.Callee)
				for _, a := range c.Args {
					_ = ecmascript.ClassifyArg(a)
					_, _ = f.StringValue(a)
				}
			}
			for _, b := range f.Bindings() {
				_ = f.Resolve(b.Name)
			}
			assert.Less(t, time.Since(start), 20*time.Second)
		})
	}
}

func TestLongSequenceResolves(t *testing.T) {
	f := js(t, "("+strings.Repeat("0,", 1000)+"eval)(x)")
	c := f.Calls()[0]
	assert.Equal(t, "eval", c.Name)
	assert.Equal(t, []ecmascript.Value{eval}, f.Eval(c.Callee))
}

func chain(n int) string {
	var b strings.Builder
	b.WriteString("const a0 = eval;")
	for i := 1; i < n; i++ {
		b.WriteString("const a")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" = a")
		b.WriteString(strconv.Itoa(i - 1))
		b.WriteString(";")
	}
	return b.String()
}

func FuzzAnalyze(f *testing.F) {
	for _, s := range []string{
		"", "eval(x)", "(0, eval)(x)", "global.r = require", "const { exec: run } = require('child_process'); run(c)",
		"import * as c from 'child_process'", "a = b; b = a", `eval('x')`, "f`x${y}`", "<svg/>",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, source string) {
		file := js(t, source)
		for _, c := range file.Calls() {
			_ = file.Eval(c.Callee)
			for _, a := range c.Args {
				_ = ecmascript.ClassifyArg(a)
				_, _ = file.StringValue(a)
			}
		}
		for _, a := range file.Assignments() {
			_ = file.Eval(a.Object)
		}
		for _, b := range file.Bindings() {
			_ = file.Resolve(b.Name)
		}
	})
}

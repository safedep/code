package callgraph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pythonLanguage(t *testing.T) core.Language {
	language, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	return language
}

func javascriptLanguage(t *testing.T) core.Language {
	language, err := lang.NewJavascriptLanguage()
	require.NoError(t, err)
	return language
}

// typescriptLanguage returns the grammar that the parser picks for a .ts
// file.
func typescriptLanguage(t *testing.T) core.Language {
	language, err := lang.NewTypescriptWithoutJSXLanguage()
	require.NoError(t, err)
	return language
}

func goLanguage(t *testing.T) core.Language {
	language, err := lang.NewGoLanguage()
	require.NoError(t, err)
	return language
}

// buildFromSource builds the call graph of source. The tree lives until the
// end of the test, so the test can read the nodes of the graph.
func buildFromSource(t *testing.T, language core.Language, name, source string) *CallGraph {
	t.Helper()

	tree, err := parser.ParseBytes(t.Context(), language, []byte(source), parser.WithFileName(name))
	require.NoError(t, err)
	t.Cleanup(tree.Close)

	cg, err := buildCallGraph(t.Context(), tree, language, name)
	require.NoError(t, err)
	return cg
}

func truncationKinds(cg *CallGraph) []TruncationKind {
	var kinds []TruncationKind
	for _, truncation := range cg.Truncations() {
		kinds = append(kinds, truncation.Kind)
	}
	return kinds
}

func calleesOf(cg *CallGraph, caller string) []string {
	node, exists := cg.Nodes[caller]
	if !exists {
		return nil
	}
	var callees []string
	for _, ref := range node.CallsTo {
		callees = append(callees, ref.CalleeNamespace)
	}
	return callees
}

func TestLimitNodesStopsTheGraphWithNoPanic(t *testing.T) {
	var source strings.Builder
	for i := range 60_000 {
		fmt.Fprintf(&source, "def f%d():\n    print()\n", i)
	}

	cg := buildFromSource(t, pythonLanguage(t), "big.py", source.String())

	assert.Equal(t, []TruncationKind{TruncationNodes}, truncationKinds(cg))
	assert.True(t, cg.LimitExceeded())
	assert.Len(t, cg.Nodes, maxCallGraphNodes)
	assert.Positive(t, cg.Truncations()[0].Line)
	assert.NotEmpty(t, cg.DFS())
}

func TestLimitAssignmentNodesAddsNothing(t *testing.T) {
	var source strings.Builder
	for i := range maxAssignmentGraphNodes + 10 {
		fmt.Fprintf(&source, "%d\n", i)
	}

	cg := buildFromSource(t, pythonLanguage(t), "literals.py", source.String())

	assert.Equal(t, []TruncationKind{TruncationAssignmentNodes}, truncationKinds(cg))
	assert.Len(t, cg.assignmentGraph.Assignments, maxAssignmentGraphNodes)
}

func TestLimitDepthMarksTheDroppedCall(t *testing.T) {
	var source strings.Builder
	source.WriteString("import yaml\n")
	for depth := range 40 {
		fmt.Fprintf(&source, "%sif True:\n", strings.Repeat("    ", depth))
	}
	fmt.Fprintf(&source, "%syaml.load(data)\n", strings.Repeat("    ", 40))

	cg := buildFromSource(t, pythonLanguage(t), "deep.py", source.String())

	assert.NotContains(t, calleesOf(cg, "deep.py"), "yaml//load")
	require.Equal(t, []TruncationKind{TruncationDepth}, truncationKinds(cg))
	assert.Equal(t, "deep.py", cg.Truncations()[0].Namespace)
	assert.Positive(t, cg.Truncations()[0].Line)
}

func TestLimitMemberAndAttributeDepthGiveNoNamespace(t *testing.T) {
	chain := "a." + strings.Repeat("b.", 12) + "c()\n"

	cases := []struct {
		name     string
		language core.Language
		file     string
		source   string
		kind     TruncationKind
	}{
		{"javascript", javascriptLanguage(t), "deep.js", "const a = require('a');\n" + chain, TruncationMemberDepth},
		{"python", pythonLanguage(t), "deep.py", "import a\n" + chain, TruncationAttributeDepth},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cg := buildFromSource(t, tc.language, tc.file, tc.source)

			assert.Equal(t, []TruncationKind{tc.kind}, truncationKinds(cg))
			for namespace := range cg.Nodes {
				assert.NotContains(t, namespace, ".b.", "no key holds the raw expression")
			}

			var reasons []UnresolvedReason
			for _, unresolved := range cg.Unresolved() {
				reasons = append(reasons, unresolved.Reason)
			}
			assert.Equal(t, []UnresolvedReason{UnresolvedLimit}, reasons)
		})
	}
}

func TestLimitPolymorphicFansOutToTheCap(t *testing.T) {
	var source strings.Builder
	for i := range maxPolymorphicResolutions + 5 {
		fmt.Fprintf(&source, "class A%d {}\nx = A%d;\n", i, i)
	}
	source.WriteString("x.run();\n")

	cg := buildFromSource(t, javascriptLanguage(t), "poly.js", source.String())

	var runs []string
	for _, callee := range calleesOf(cg, "poly.js") {
		if strings.HasSuffix(callee, "//run") {
			runs = append(runs, callee)
		}
	}
	assert.Len(t, runs, maxPolymorphicResolutions)
	assert.Equal(t, "poly.js//A0//run", runs[0])
	assert.Equal(t, []TruncationKind{TruncationPolymorphic}, truncationKinds(cg))
}

func TestLimitDFSResults(t *testing.T) {
	cg := buildFromSource(t, pythonLanguage(t), "calls.py", "def f():\n    pass\n"+strings.Repeat("f()\n", maxDFSResultItems+1))
	require.False(t, cg.LimitExceeded())

	assert.Len(t, cg.DFS(), maxDFSResultItems)
	assert.Equal(t, []TruncationKind{TruncationDFSResults}, truncationKinds(cg))
}

func TestMemberFanOutKeepsEveryTarget(t *testing.T) {
	source := "const a = require('a');\nconst b = require('b');\nlet x = a;\nx = b;\nx.run();\n"

	cg := buildFromSource(t, javascriptLanguage(t), "fan.js", source)

	assert.Subset(t, calleesOf(cg, "fan.js"), []string{"a//run", "b//run"})
	assert.Empty(t, cg.Truncations())
}

func TestUnresolvedCallSites(t *testing.T) {
	type site struct {
		Text     string
		Receiver string
		Method   string
		Reason   UnresolvedReason
	}

	cases := []struct {
		name     string
		language core.Language
		file     string
		source   string
		expected []site
	}{
		{
			name:     "python computed names and a dynamic import",
			language: pythonLanguage(t),
			file:     "dyn.py",
			source:   "import importlib\nobj[name]()\ngetattr(o, n)()\nimportlib.import_module(x)\nimportlib.import_module('json')\n__import__(y)\n",
			expected: []site{
				{"obj[name]", "obj", "", UnresolvedComputedName},
				{"getattr(o, n)", "", "", UnresolvedComputedName},
				{"importlib.import_module(x)", "importlib", "import_module", UnresolvedDynamicImport},
				{"__import__(y)", "", "__import__", UnresolvedDynamicImport},
			},
		},
		{
			name:     "python unknown receiver",
			language: pythonLanguage(t),
			file:     "recv.py",
			source:   "def handle(client):\n    client.send()\n    client.socket.close()\n",
			expected: []site{
				{"client.send", "client", "send", UnresolvedUnknownReceiver},
				{"client.socket.close", "client.socket", "close", UnresolvedUnknownReceiver},
			},
		},
		{
			name:     "python computed receiver and call result",
			language: pythonLanguage(t),
			file:     "recv.py",
			source:   "handlers[name].run()\nf()()\n",
			expected: []site{
				{"handlers[name].run", "handlers[name]", "run", UnresolvedComputedName},
				{"f()", "", "", UnresolvedComputedName},
			},
		},
		{
			name:     "javascript dynamic imports",
			language: javascriptLanguage(t),
			file:     "dyn.js",
			source:   "import(x);\nimport('fs');\nimport(`./${name}`);\nrequire(y);\nrequire('os');\n",
			expected: []site{
				{"import(x)", "", "import", UnresolvedDynamicImport},
				{"import(`./${name}`)", "", "import", UnresolvedDynamicImport},
				{"require(y)", "", "require", UnresolvedDynamicImport},
			},
		},
		{
			name:     "javascript computed names and unknown receivers",
			language: javascriptLanguage(t),
			file:     "recv.js",
			source:   "obj[name]();\nfunction handle(client) { client.send(); }\nfoo()();\n(a || b).run();\nnew (factory())();\n",
			expected: []site{
				{"obj[name]", "obj", "", UnresolvedComputedName},
				{"client.send", "client", "send", UnresolvedUnknownReceiver},
				{"foo()", "", "", UnresolvedComputedName},
				{"(a || b).run", "(a || b)", "run", UnresolvedUnknownReceiver},
				{"(factory())", "", "", UnresolvedComputedName},
			},
		},
		{
			name:     "typescript computed names, unknown receivers and dynamic imports",
			language: typescriptLanguage(t),
			file:     "recv.ts",
			source:   "obj[name]();\nfunction handle(client: Client): void { client.send(); }\n(client as Client).close();\nimport(x);\nfoo<string>()();\n",
			expected: []site{
				{"obj[name]", "obj", "", UnresolvedComputedName},
				{"client.send", "client", "send", UnresolvedUnknownReceiver},
				{"(client as Client).close", "(client as Client)", "close", UnresolvedUnknownReceiver},
				{"import(x)", "", "import", UnresolvedDynamicImport},
				{"foo<string>()", "", "", UnresolvedComputedName},
			},
		},
		{
			name:     "go unknown receivers and computed names",
			language: goLanguage(t),
			file:     "recv.go",
			source:   "package main\n\nimport \"fmt\"\n\nfunc handle(c Client) {\n\tc.Send()\n\tfmt.Println()\n\tfns[0]()\n\tf()()\n\thandlers[k].Serve()\n}\n",
			expected: []site{
				{"c.Send", "c", "Send", UnresolvedUnknownReceiver},
				{"fns[0]", "", "", UnresolvedComputedName},
				{"f()", "", "", UnresolvedComputedName},
				{"handlers[k].Serve", "handlers[k]", "Serve", UnresolvedUnknownReceiver},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cg := buildFromSource(t, tc.language, tc.file, tc.source)

			var actual []site
			for _, unresolved := range cg.Unresolved() {
				actual = append(actual, site{unresolved.Text, unresolved.Receiver, unresolved.Method, unresolved.Reason})
				assert.Positive(t, unresolved.Site.StartLine)
				assert.NotEmpty(t, unresolved.Caller)
			}
			assert.ElementsMatch(t, tc.expected, actual)
		})
	}
}

// TestNoKeyHoldsSourceText checks that a callee or a receiver that is not a
// name gives no key of the graph.
func TestNoKeyHoldsSourceText(t *testing.T) {
	cases := []struct {
		language core.Language
		file     string
		source   string
	}{
		{javascriptLanguage(t), "a.js", "foo()();\n(function () { baz(); })();\n(() => qux())();\n\"a,b\".split(\",\");\n`x`.trim();\n({a: 1}).toString();\n[1].map(f);\n/a/.test(s);\n(a || b).run();\nobj[k]();\nnew (factory())();\nx[i].y.z();\n(0, m.f)(x);\n"},
		{typescriptLanguage(t), "a.ts", "foo()();\n(function (): void { baz(); })();\n\"a\".split(\",\");\n(x as Y).z();\nx!.y();\n"},
		{pythonLanguage(t), "a.py", "f()()\n(lambda: foo())()\n\"\".join(xs)\n[1].append(2)\n(a or b).c()\nx[i].y()\n"},
		{goLanguage(t), "a.go", "package main\n\nfunc main() {\n\tf()()\n\tfunc() { g() }()\n\t(h)()\n\tx[i]()\n\tx[i].Y()\n\tFoo{}.Bar()\n\tx.(T).M()\n}\n"},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			cg := buildFromSource(t, tc.language, tc.file, tc.source)

			for namespace := range cg.Nodes {
				assert.False(t, strings.ContainsAny(namespace, "()[]{}'\"` ,|!"), "key %q holds source text", namespace)
			}
		})
	}
}

func TestLiteralReceiversResolveToTheBuiltinType(t *testing.T) {
	cases := []struct {
		language core.Language
		file     string
		source   string
		callees  []string
	}{
		{javascriptLanguage(t), "a.js", "\"a,b\".split(\",\");\n`x`.trim();\n({a: 1}).toString();\n[1].map(f);\n/a/.test(s);\n(1).toFixed();\n", []string{"String//split", "String//trim", "Object//toString", "Array//map", "RegExp//test", "Number//toFixed"}},
		{typescriptLanguage(t), "a.ts", "\"a,b\".split(\",\");\n[1].map(f);\n", []string{"String//split", "Array//map"}},
		{pythonLanguage(t), "a.py", "\",\".join(xs)\n[1].append(2)\n{1}.add(2)\n{\"a\": 1}.get(\"a\")\n(1, 2).count(1)\n\"a\".strip().upper()\n", []string{"str//join", "list//append", "set//add", "dict//get", "tuple//count", "str//strip", "str//strip//upper"}},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			cg := buildFromSource(t, tc.language, tc.file, tc.source)

			assert.Subset(t, calleesOf(cg, tc.file), tc.callees)
			assert.Empty(t, cg.Unresolved())
		})
	}
}

func TestImmediatelyInvokedFunctionKeepsItsCalls(t *testing.T) {
	cases := []struct {
		language core.Language
		file     string
		source   string
		caller   string
		callees  []string
	}{
		{javascriptLanguage(t), "a.js", "function baz() {}\nfunction qux() {}\n(function () { baz(); })();\n(() => qux())();\n", "a.js", []string{"a.js//baz", "a.js//qux"}},
		{typescriptLanguage(t), "a.ts", "function baz(): void {}\n(function (): void { baz(); })();\n", "a.ts", []string{"a.ts//baz"}},
		{pythonLanguage(t), "a.py", "import os\n(lambda: os.getcwd())()\n", "a.py", []string{"os//getcwd"}},
		{goLanguage(t), "a.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tgo func() { fmt.Println() }()\n\tdefer func() { recover() }()\n}\n", "a.go//main", []string{"fmt//Println", "recover"}},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			cg := buildFromSource(t, tc.language, tc.file, tc.source)

			assert.Subset(t, calleesOf(cg, tc.caller), tc.callees)
			assert.Empty(t, cg.Unresolved())
		})
	}
}

func TestStringIndexNamesTheMember(t *testing.T) {
	cases := []struct {
		name     string
		language core.Language
		file     string
		source   string
		callee   string
	}{
		{"javascript call", javascriptLanguage(t), "a.js", "const yaml = require('js-yaml');\nyaml['load'](x);\n", "js-yaml//load"},
		{"javascript constructor", javascriptLanguage(t), "a.js", "const sqlite3 = require('sqlite3');\nnew sqlite3[\"Database\"](f);\n", "sqlite3//Database"},
		{"typescript call", typescriptLanguage(t), "a.ts", "import yaml from 'js-yaml';\nyaml['load'](x);\n", "js-yaml//load"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cg := buildFromSource(t, tc.language, tc.file, tc.source)

			assert.Contains(t, calleesOf(cg, tc.file), tc.callee)
			assert.Empty(t, cg.Unresolved())
		})
	}
}

func TestUnresolvedTextIsClipped(t *testing.T) {
	cg := buildFromSource(t, pythonLanguage(t), "long.py", "obj["+strings.Repeat("é", 100)+"]()\n")

	require.Len(t, cg.Unresolved(), 1)
	text := cg.Unresolved()[0].Text
	assert.LessOrEqual(t, len(text), maxUnresolvedText)
	assert.True(t, strings.HasPrefix("obj["+strings.Repeat("é", 100), text))
}

func TestUnboundCallee(t *testing.T) {
	cases := []struct {
		name     string
		language core.Language
		file     string
		source   string
		caller   string
		callee   string
		unbound  bool
	}{
		{"javascript hoisting", javascriptLanguage(t), "a.js", "function outer() { inner() }\nfunction inner() {}\n", "a.js//outer", "a.js//outer//inner", true},
		{"javascript bound", javascriptLanguage(t), "b.js", "function inner() {}\nfunction outer() { inner() }\n", "b.js//outer", "b.js//inner", false},
		{"python hoisting", pythonLanguage(t), "a.py", "def outer():\n    inner()\n\ndef inner():\n    pass\n", "a.py//outer", "inner", true},
		{"python builtin", pythonLanguage(t), "b.py", "def outer():\n    print()\n", "b.py//outer", "print", false},
		{"go hoisting", goLanguage(t), "a.go", "package main\n\nfunc outer() { inner() }\n\nfunc inner() {}\n", "a.go//outer", "a.go//outer//inner", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cg := buildFromSource(t, tc.language, tc.file, tc.source)

			require.Contains(t, cg.Nodes, tc.caller)
			var refs []CallReference
			for _, ref := range cg.Nodes[tc.caller].CallsTo {
				if ref.CalleeNamespace == tc.callee {
					refs = append(refs, ref)
				}
			}
			require.Len(t, refs, 1)
			assert.Equal(t, tc.unbound, refs[0].Unbound)
		})
	}
}

func TestUnboundNameStaysUnboundOnTheSecondCall(t *testing.T) {
	cg := buildFromSource(t, javascriptLanguage(t), "a.js", "function outer() { inner(); inner(); }\nfunction inner() {}\n")

	refs := cg.Nodes["a.js//outer"].CallsTo
	require.Len(t, refs, 2)
	for _, ref := range refs {
		assert.True(t, ref.Unbound)
	}
}

func TestDeterministicDFS(t *testing.T) {
	cases := []struct {
		fixture  string
		language core.Language
	}{
		{"testClass.py", pythonLanguage(t)},
		{"testJavascript.js", javascriptLanguage(t)},
		{"testCallGraph.go", goLanguage(t)},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("fixtures", tc.fixture))
			require.NoError(t, err)

			order := func() []string {
				cg := buildFromSource(t, tc.language, tc.fixture, string(source))
				var items []string
				for _, item := range cg.DFS() {
					items = append(items, fmt.Sprintf("%s<-%s@%d/%t", item.Namespace, item.Caller.Namespace, item.Depth, item.Terminal))
				}
				return items
			}

			first := order()
			require.NotEmpty(t, first)
			for range 19 {
				assert.Equal(t, first, order())
			}
		})
	}
}

func TestArrowFunctionWithAnExpressionBody(t *testing.T) {
	cases := []struct {
		name   string
		source string
		caller string
		callee string
	}{
		{"argument", "x((k) => parseInt(k));\n", "", "parseInt"},
		{"callback of a member call", "p.then((n) => console.log(n));\n", "", "console//log"},
		{"callback of a builtin", "setTimeout(() => process.exit(0));\n", "", "process//exit"},
		{"declared", "function foo() {}\nconst f = () => foo();\n", "//f", "//foo"},
	}

	languages := []struct {
		language core.Language
		file     string
	}{
		{javascriptLanguage(t), "a.js"},
		{typescriptLanguage(t), "a.ts"},
	}

	for _, l := range languages {
		for _, tc := range cases {
			t.Run(l.file+" "+tc.name, func(t *testing.T) {
				cg := buildFromSource(t, l.language, l.file, tc.source)

				callee := tc.callee
				if strings.HasPrefix(callee, namespaceSeparator) {
					callee = l.file + callee
				}
				assert.Contains(t, calleesOf(cg, l.file+tc.caller), callee)
			})
		}
	}
}

func TestRuntimeGlobalsAreKnownReceivers(t *testing.T) {
	source := `process.exit(0);
window.alert(1);
document.querySelector("a");
navigator.sendBeacon(u);
global.gc();
globalThis.queueMicrotask(f);
Buffer.from("x");
module.require("x");
exports.run();
crypto.randomUUID();
new URL(u);
fetch(u);
new Error("e");
new TypeError("e");
new AbortController();
new TextEncoder();
new TextDecoder();
require("os");
`
	languages := []struct {
		language core.Language
		file     string
	}{
		{javascriptLanguage(t), "a.js"},
		{typescriptLanguage(t), "a.ts"},
	}

	for _, l := range languages {
		t.Run(l.file, func(t *testing.T) {
			cg := buildFromSource(t, l.language, l.file, source)

			assert.Empty(t, cg.Unresolved())
			assert.Subset(t, calleesOf(cg, l.file), []string{"process//exit", "Buffer//from", "crypto//randomUUID", "URL", "fetch", "Error", "require"})
			for _, ref := range cg.Nodes[l.file].CallsTo {
				assert.False(t, ref.Unbound, ref.CalleeNamespace)
			}
		})
	}
}

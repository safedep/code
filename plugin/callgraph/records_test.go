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

func goLanguage(t *testing.T) core.Language {
	language, err := lang.NewGoLanguage()
	require.NoError(t, err)
	return language
}

// buildFromSource builds the call graph of source. The tree lives until the
// end of the test, so the test can read the nodes of the graph.
func buildFromSource(t *testing.T, language core.Language, name, source string) *CallGraph {
	t.Helper()
	loadBuiltinOnce.Do(initBuiltins)

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
		Text   string
		Reason UnresolvedReason
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
			source:   "import importlib\nobj[name]()\ngetattr(o, n)()\nimportlib.import_module(x)\nimportlib.import_module('json')\n",
			expected: []site{
				{"obj[name]", UnresolvedComputedName},
				{"getattr(o, n)", UnresolvedComputedName},
				{"importlib.import_module(x)", UnresolvedDynamicImport},
			},
		},
		{
			name:     "python unknown receiver",
			language: pythonLanguage(t),
			file:     "recv.py",
			source:   "def handle(client):\n    client.send()\n",
			expected: []site{{"client.send", UnresolvedUnknownReceiver}},
		},
		{
			name:     "python computed receiver",
			language: pythonLanguage(t),
			file:     "recv.py",
			source:   "handlers[name].run()\n",
			expected: []site{{"handlers[name].run", UnresolvedComputedName}},
		},
		{
			name:     "javascript dynamic imports",
			language: javascriptLanguage(t),
			file:     "dyn.js",
			source:   "import(x);\nimport('fs');\nimport(`./${name}`);\nrequire(y);\nrequire('os');\n",
			expected: []site{
				{"import(x)", UnresolvedDynamicImport},
				{"import(`./${name}`)", UnresolvedDynamicImport},
				{"require(y)", UnresolvedDynamicImport},
			},
		},
		{
			name:     "javascript computed name and unknown receiver",
			language: javascriptLanguage(t),
			file:     "recv.js",
			source:   "obj[name]();\nfunction handle(client) { client.send(); }\n",
			expected: []site{
				{"obj[name]", UnresolvedComputedName},
				{"client.send", UnresolvedUnknownReceiver},
			},
		},
		{
			name:     "go unknown receiver",
			language: goLanguage(t),
			file:     "recv.go",
			source:   "package main\n\nimport \"fmt\"\n\nfunc handle(c Client) {\n\tc.Send()\n\tfmt.Println()\n}\n",
			expected: []site{{"c.Send", UnresolvedUnknownReceiver}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cg := buildFromSource(t, tc.language, tc.file, tc.source)

			var actual []site
			for _, unresolved := range cg.Unresolved() {
				actual = append(actual, site{unresolved.Text, unresolved.Reason})
				assert.Positive(t, unresolved.Site.StartLine)
				assert.NotEmpty(t, unresolved.Caller)
			}
			assert.ElementsMatch(t, tc.expected, actual)
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

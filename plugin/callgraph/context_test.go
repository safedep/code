package callgraph

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/safedep/dry/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nodeDeadline passes its deadline when the graph has processed a number of
// nodes. A test with it depends neither on the clock nor on how often a
// resolver reads Err.
type nodeDeadline struct {
	context.Context
	graph *CallGraph
	nodes int
}

func (c *nodeDeadline) Err() error {
	if c.graph.visited < c.nodes {
		return nil
	}
	return context.DeadlineExceeded
}

// expiringContext passes its deadline after its Err has returned nil a
// number of times. A test with it does not depend on the clock.
type expiringContext struct {
	context.Context
	live int
}

func (c *expiringContext) Err() error {
	if c.live > 0 {
		c.live--
		return nil
	}
	return context.DeadlineExceeded
}

const deadlineCalls = 100_000

var deadlineSource = "def f():\n    pass\n" + strings.Repeat("f()\n", deadlineCalls)

func TestLimitDeadlineStopsBuildCallGraph(t *testing.T) {
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	tree, err := parser.ParseBytes(t.Context(), python, []byte(deadlineSource), parser.WithFileName("calls.py"))
	require.NoError(t, err)
	defer tree.Close()

	full, err := Build(t.Context(), tree)
	require.NoError(t, err)
	assert.Empty(t, full.Truncations())
	assert.Len(t, full.RootNode.CallsTo, deadlineCalls)

	imports, err := python.Resolvers().ResolveImports(t.Context(), tree)
	require.NoError(t, err)
	cg := newCallGraph("calls.py", python.Meta().Code)
	treeData, err := tree.Data()
	require.NoError(t, err)

	cg.build(&nodeDeadline{Context: t.Context(), graph: cg, nodes: 2 * ctxCheckInterval}, tree.Tree().RootNode(), imports, *treeData)

	assert.Equal(t, 2*ctxCheckInterval, cg.visited, "the walk stops at the first check past the deadline")
	assert.ErrorIs(t, cg.ctxErr, context.DeadlineExceeded)
	assert.Equal(t, []Truncation{{Kind: TruncationDeadline, Namespace: "calls.py"}}, cg.Truncations())
	assert.NotEmpty(t, cg.RootNode.CallsTo)
	assert.Less(t, len(cg.RootNode.CallsTo), deadlineCalls/10)
}

func TestDeadlineFailsTheFileAndGivesThePartialGraph(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "calls.py"), []byte(deadlineSource), 0o600))
	treeWalker, fileSystem, err := test.SetupBasicPluginContext(dir, nil, []core.LanguageCode{core.LanguageCodePython})
	require.NoError(t, err)

	var partial *CallGraph
	executor := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		NewCallGraphPlugin(func(_ context.Context, cg *CallGraph) error {
			partial = cg
			return nil
		}),
	})

	// The parse and the import queries read Err less often than live, so
	// the deadline passes during the walk of the tree.
	report, err := executor.Execute(&expiringContext{Context: t.Context(), live: 16}, fileSystem)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	require.NotNil(t, partial)
	assert.Equal(t, []TruncationKind{TruncationDeadline}, truncationKinds(partial))
	assert.Less(t, len(partial.RootNode.CallsTo), deadlineCalls)

	require.Len(t, report.Failed, 1)
	assert.Equal(t, "calls.py", report.Failed[0].File)
	assert.Equal(t, core.Stage("CallgraphPlugin"), report.Failed[0].Stage)
	assert.ErrorIs(t, report.Failed[0], context.DeadlineExceeded)
}

func TestDoneContextStopsBuildCallGraphAtTheImports(t *testing.T) {
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	tree, err := parser.ParseBytes(t.Context(), python, []byte("import os\nos.getcwd()\n"))
	require.NoError(t, err)
	defer tree.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = Build(ctx, tree)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestAnalyzeGraphFailsOnAPartialGraphAsAnalyzeTreeDoes(t *testing.T) {
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	tree, err := parser.ParseBytes(t.Context(), python, []byte(deadlineSource), parser.WithFileName("calls.py"))
	require.NoError(t, err)
	defer tree.Close()
	imports, err := python.Resolvers().ResolveImports(t.Context(), tree)
	require.NoError(t, err)
	treeData, err := tree.Data()
	require.NoError(t, err)
	partial := newCallGraph("calls.py", python.Meta().Code)
	partial.build(&nodeDeadline{Context: t.Context(), graph: partial, nodes: 2 * ctxCheckInterval}, tree.Tree().RootNode(), imports, *treeData)
	full, err := Build(t.Context(), tree)
	require.NoError(t, err)

	var got []*CallGraph
	pl := NewCallGraphPlugin(func(_ context.Context, cg *CallGraph) error {
		got = append(got, cg)
		return nil
	})
	require.NoError(t, pl.AnalyzeGraph(t.Context(), full))
	err = pl.AnalyzeGraph(t.Context(), partial)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, []*CallGraph{full, partial}, got, "the callback gets the graph of the caller, the partial one too")
}

// A debug line for each call made the debug log of a large project run to
// hundreds of thousands of lines. The log of a build must not grow with
// the number of calls.
func TestBuildLogsNoLineForEachCall(t *testing.T) {
	for _, tc := range []struct {
		name     string
		language func() (core.Language, error)
		file     string
		source   func(calls int) string
	}{
		{"go", func() (core.Language, error) { return lang.NewGoLanguage() }, "main.go", func(n int) string {
			return "package main\n\nfunc f() {}\n\nfunc g() {}\n\nfunc main() {\n" + strings.Repeat("\tf()\n", n) + "}\n"
		}},
		{"javascript", func() (core.Language, error) { return lang.NewJavascriptLanguage() }, "main.js", func(n int) string {
			return "function f() {}\nconst g = () => {};\n" + strings.Repeat("f();\nnew Map();\n", n)
		}},
		{"python", func() (core.Language, error) { return lang.NewPythonLanguage() }, "main.py", func(n int) string {
			return "class C:\n    pass\n\ndef f():\n    pass\n\n" + strings.Repeat("f()\nC()\n", n)
		}},
		{"java", func() (core.Language, error) { return lang.NewJavaLanguage() }, "Main.java", func(n int) string {
			return "class Main {\n  void f() {}\n  void m() {\n" + strings.Repeat("    f();\n    new Main();\n    this.f();\n", n) + "  }\n}\n"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			language, err := tc.language()
			require.NoError(t, err)
			lines := func(calls int) int {
				tree, err := parser.ParseBytes(t.Context(), language, []byte(tc.source(calls)), parser.WithFileName(tc.file))
				require.NoError(t, err)
				defer tree.Close()
				var buf strings.Builder
				restore := log.SwapGlobalForTest(&buf)
				defer restore()
				cg, err := Build(t.Context(), tree)
				require.NoError(t, err)
				require.NotEmpty(t, cg.Nodes)
				return strings.Count(buf.String(), "\n")
			}
			assert.Equal(t, lines(1), lines(50))
		})
	}
}

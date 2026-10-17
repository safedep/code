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

	full, err := buildCallGraph(t.Context(), tree, python, "calls.py")
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
	executor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		NewCallGraphPlugin(func(_ context.Context, cg *CallGraph) error {
			partial = cg
			return nil
		}),
	})
	require.NoError(t, err)

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
	_, err = buildCallGraph(ctx, tree, python, "a.py")
	assert.ErrorIs(t, err, context.Canceled)
}

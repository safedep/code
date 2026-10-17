package callgraph

import (
	"context"
	"strings"
	"testing"

	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestLimitDeadlineStopsBuildCallGraph(t *testing.T) {
	const calls = 100_000
	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	tree, err := parser.ParseBytes(t.Context(), python, []byte("def f():\n    pass\n"+strings.Repeat("f()\n", calls)), parser.WithFileName("calls.py"))
	require.NoError(t, err)
	defer tree.Close()

	full, err := buildCallGraph(t.Context(), tree, python, "calls.py")
	require.NoError(t, err)
	assert.Empty(t, full.Truncations())
	assert.Len(t, full.RootNode.CallsTo, calls)

	// The import queries read Err twice before the walk of the tree.
	cg, err := buildCallGraph(&expiringContext{Context: t.Context(), live: 2}, tree, python, "calls.py")
	require.NoError(t, err)
	assert.True(t, cg.LimitExceeded())
	assert.Equal(t, []Truncation{{Kind: TruncationDeadline, Namespace: "calls.py"}}, cg.Truncations())
	assert.NotEmpty(t, cg.RootNode.CallsTo)
	assert.Less(t, len(cg.RootNode.CallsTo), calls/10)
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

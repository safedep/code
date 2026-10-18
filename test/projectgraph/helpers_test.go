package projectgraph

import (
	"context"
	"os"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/plugin/indexer"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/stretchr/testify/require"
)

// indexProject indexes the files of a file system. The run itself must
// succeed, and a file that fails is in the report of the result.
func indexProject(t testing.TB, fsys core.ImportAwareFileSystem, opts indexer.Options) *indexer.Result {
	t.Helper()
	indexed, err := (&indexer.Indexer{}).Run(context.Background(), fsys, opts)
	require.NoError(t, err)
	return indexed
}

func targetsOf(t testing.TB, keys ...string) []reach.Target {
	t.Helper()
	targets, err := reach.TargetsByKey(keys...)
	require.NoError(t, err)
	return targets
}

// linkProject links the result of an index run with the manifests under
// root. The statuses of the project files and of the dependency files both
// go to the link step, so a failed file of either kind is a budget cause.
func linkProject(t testing.TB, root string, indexed *indexer.Result, opts projectgraph.Options) *projectgraph.Graph {
	t.Helper()
	opts.Root, opts.Files = os.DirFS(root), indexed.Statuses()
	g, err := projectgraph.Build(context.Background(), indexed.Indexes, opts)
	require.NoError(t, err)
	return g
}

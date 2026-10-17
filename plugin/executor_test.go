package plugin_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// treeFunc is a tree plugin for Python that calls fn for each tree.
type treeFunc func(context.Context, core.ParseTree) error

func (f treeFunc) Name() string { return "treeFunc" }
func (f treeFunc) SupportedLanguages() []core.LanguageCode {
	return []core.LanguageCode{core.LanguageCodePython}
}
func (f treeFunc) AnalyzeTree(ctx context.Context, tree core.ParseTree) error { return f(ctx, tree) }

func writeFiles(t *testing.T, files map[string]string) string {
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	return dir
}

func TestExecuteStopsWhenTheContextIsCancelled(t *testing.T) {
	files := map[string]string{}
	for i := range 10 {
		files[fmt.Sprintf("f%d.py", i)] = "import os\n"
	}
	dir := writeFiles(t, files)

	treeWalker, fileSystem, err := test.SetupBasicPluginContext([]string{dir}, []core.LanguageCode{core.LanguageCodePython})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	visited := 0
	executor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		treeFunc(func(context.Context, core.ParseTree) error {
			visited++
			cancel()
			return nil
		}),
	})
	require.NoError(t, err)

	err = executor.Execute(ctx, fileSystem)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, visited)
}

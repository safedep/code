package plugin_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// treeFunc is a tree plugin for Python that calls fn for each tree.
type treeFunc struct {
	name string
	fn   func(context.Context, core.ParseTree) error
}

func (f treeFunc) Name() string { return f.name }
func (f treeFunc) SupportedLanguages() []core.LanguageCode {
	return []core.LanguageCode{core.LanguageCodePython}
}
func (f treeFunc) AnalyzeTree(ctx context.Context, tree core.ParseTree) error { return f.fn(ctx, tree) }

// fileNames records the base name of each file that a plugin gets.
func fileNames(names *[]string) treeFunc {
	return treeFunc{name: "names", fn: func(_ context.Context, tree core.ParseTree) error {
		file, err := tree.File()
		if err != nil {
			return err
		}
		*names = append(*names, filepath.Base(file.Name()))
		return nil
	}}
}

func writeFiles(t *testing.T, files map[string]string) string {
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	return dir
}

func execute(t *testing.T, dir string, plugins []core.Plugin, opts ...parser.Option) *plugin.Report {
	treeWalker, fileSystem, err := test.SetupBasicPluginContext(dir, nil, []core.LanguageCode{core.LanguageCodePython}, opts...)
	require.NoError(t, err)
	executor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, plugins)
	require.NoError(t, err)

	report, err := executor.Execute(t.Context(), fileSystem)
	require.NoError(t, err)
	return report
}

func baseNames(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, filepath.Base(name))
	}
	return out
}

func TestExecuteStopsWhenTheContextIsCancelled(t *testing.T) {
	files := map[string]string{}
	for i := range 10 {
		files[fmt.Sprintf("f%d.py", i)] = "import os\n"
	}
	dir := writeFiles(t, files)

	treeWalker, fileSystem, err := test.SetupBasicPluginContext(dir, nil, []core.LanguageCode{core.LanguageCodePython})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	visited := 0
	executor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		treeFunc{name: "cancel", fn: func(context.Context, core.ParseTree) error {
			visited++
			cancel()
			return nil
		}},
	})
	require.NoError(t, err)

	report, err := executor.Execute(ctx, fileSystem)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, visited)
	assert.Len(t, report.Files, 1)
}

func TestExecuteFailsTheFileThatTheContextStops(t *testing.T) {
	dir := writeFiles(t, map[string]string{"a.py": "import os\n", "b.py": "import sys\n"})
	treeWalker, fileSystem, err := test.SetupBasicPluginContext(dir, nil, []core.LanguageCode{core.LanguageCodePython})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	executor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		treeFunc{name: "stopped", fn: func(ctx context.Context, _ core.ParseTree) error {
			cancel()
			return ctx.Err()
		}},
	})
	require.NoError(t, err)

	report, err := executor.Execute(ctx, fileSystem)
	assert.ErrorIs(t, err, context.Canceled)
	require.Len(t, report.Failed, 1)
	assert.Equal(t, "a.py", report.Failed[0].File)
	assert.Equal(t, core.Stage("stopped"), report.Failed[0].Stage)
	assert.ErrorIs(t, report.Failed[0], context.Canceled)
}

func TestExecuteGoesOnPastAFileThatDoesNotParse(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a_large.py": strings.Repeat("a = 1\n", 100),
		"b_good.py":  "import os\n",
	})

	var names []string
	report := execute(t, dir, []core.Plugin{fileNames(&names)}, parser.WithMaxBytes(64))

	assert.Equal(t, []string{"b_good.py"}, names)
	assert.Equal(t, []string{"b_good.py"}, baseNames(report.Files))
	assert.Empty(t, report.Skipped)
	require.Len(t, report.Failed, 1)
	assert.Equal(t, "a_large.py", filepath.Base(report.Failed[0].File))
	assert.Equal(t, core.StageParse, report.Failed[0].Stage)
	assert.ErrorIs(t, report.Failed[0], parser.ErrInputTooLarge)
}

func TestExecuteGoesOnPastAPluginError(t *testing.T) {
	dir := writeFiles(t, map[string]string{"a.py": "import os\n", "b.py": "import sys\n"})

	errFailed := errors.New("plugin failed")
	var names []string
	failing := treeFunc{name: "failing", fn: func(_ context.Context, tree core.ParseTree) error {
		file, err := tree.File()
		if err != nil {
			return err
		}
		if filepath.Base(file.Name()) == "a.py" {
			return errFailed
		}
		return nil
	}}
	report := execute(t, dir, []core.Plugin{failing, fileNames(&names)})

	assert.Equal(t, []string{"a.py", "b.py"}, names, "the next plugin and the next file still run")
	assert.Equal(t, []string{"a.py", "b.py"}, baseNames(report.Files))
	require.Len(t, report.Failed, 1)
	assert.Equal(t, "a.py", filepath.Base(report.Failed[0].File))
	assert.Equal(t, core.Stage("failing"), report.Failed[0].Stage)
	assert.ErrorIs(t, report.Failed[0], errFailed)
}

func TestExecuteSkipsAFileThatNoLanguageParses(t *testing.T) {
	dir := writeFiles(t, map[string]string{"a.js": "a()\n", "b.py": "import os\n"})

	python, err := lang.NewPythonLanguage()
	require.NoError(t, err)
	javascript, err := lang.NewJavascriptLanguage()
	require.NoError(t, err)
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{AppDirectories: []string{dir}})
	require.NoError(t, err)
	sourceWalker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, []core.Language{python, javascript})
	require.NoError(t, err)
	treeWalker, err := parser.NewWalkingParser(sourceWalker, []core.Language{python})
	require.NoError(t, err)
	executor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, nil)
	require.NoError(t, err)

	report, err := executor.Execute(t.Context(), fileSystem)
	require.NoError(t, err)
	assert.Equal(t, []string{"a.js"}, baseNames(report.Skipped))
	assert.Equal(t, []string{"b.py"}, baseNames(report.Files))
	assert.Empty(t, report.Failed)
}

func TestExecuteGoesOnPastAnUnreadableDirectory(t *testing.T) {
	dir := writeFiles(t, map[string]string{"a/a.py": "import os\n", "b/b.py": "import os\n", "c.py": "import os\n"})
	locked := filepath.Join(dir, "b")
	require.NoError(t, os.Chmod(locked, 0))
	t.Cleanup(func() { require.NoError(t, os.Chmod(locked, 0o700)) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("this user reads a directory with no permissions, as root does")
	}

	var names []string
	report := execute(t, dir, []core.Plugin{fileNames(&names)})
	assert.Equal(t, []string{"a.py", "c.py"}, names)
	assert.Empty(t, report.Failed)
}

// closeRecorder is a tree that records Close.
type closeRecorder struct {
	core.ParseTree
	closed bool
}

func (r *closeRecorder) Close() {
	r.closed = true
	r.ParseTree.Close()
}

// recordingWalker gives the visitor each tree in a closeRecorder.
type recordingWalker struct {
	core.TreeWalker
	trees []*closeRecorder
}

func (w *recordingWalker) Walk(ctx context.Context, fsys core.ImportAwareFileSystem, visitor core.TreeVisitor) error {
	return w.TreeWalker.Walk(ctx, fsys, &recordingVisitor{TreeVisitor: visitor, walker: w})
}

type recordingVisitor struct {
	core.TreeVisitor
	walker *recordingWalker
}

func (v *recordingVisitor) VisitTree(ctx context.Context, tree core.ParseTree) error {
	recorder := &closeRecorder{ParseTree: tree}
	v.walker.trees = append(v.walker.trees, recorder)
	return v.TreeVisitor.VisitTree(ctx, recorder)
}

func TestExecuteClosesEachTreeAfterTheLastPlugin(t *testing.T) {
	dir := writeFiles(t, map[string]string{"a.py": "import os\n", "b.py": "import sys\n"})
	treeWalker, fileSystem, err := test.SetupBasicPluginContext(dir, nil, []core.LanguageCode{core.LanguageCodePython})
	require.NoError(t, err)
	walker := &recordingWalker{TreeWalker: treeWalker}

	var seen []core.ParseTree
	openInPlugin := func(_ context.Context, tree core.ParseTree) error {
		seen = append(seen, tree)
		assert.False(t, tree.(*closeRecorder).closed, "a plugin gets an open tree")
		return nil
	}
	executor, err := plugin.NewTreeWalkPluginExecutor(walker, []core.Plugin{
		treeFunc{name: "first", fn: openInPlugin},
		treeFunc{name: "last", fn: openInPlugin},
	})
	require.NoError(t, err)

	report, err := executor.Execute(t.Context(), fileSystem)
	require.NoError(t, err)
	assert.Empty(t, report.Failed)
	assert.Len(t, seen, 4)
	require.Len(t, walker.trees, 2)
	for _, tree := range walker.trees {
		assert.True(t, tree.closed)
	}
}

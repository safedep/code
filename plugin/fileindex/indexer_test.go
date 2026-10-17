package fileindex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/projectgraph/store"
	"github.com/safedep/code/projectgraph/store/localdbstore"
	"github.com/safedep/code/projectgraph/store/memstore"
	"github.com/safedep/dry/localdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var projectFiles = map[string]string{
	"main.js":       "import { helper } from './lib/helper';\nhelper();\n",
	"lib/helper.js": "export function helper() {}\n",
	"app/util.ts":   "export const util = (): number => 1;\n",
	"tool.py":       "import os\n\ndef run():\n    os.getcwd()\n",
	"README.md":     "not source\n",
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
		require.NoError(t, os.WriteFile(file, []byte(content), 0o644))
	}
}

func appFileSystem(t *testing.T, dirs ...string) core.ImportAwareFileSystem {
	t.Helper()
	fsys, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{AppDirectories: dirs})
	require.NoError(t, err)
	return fsys
}

func languages(t *testing.T, codes ...core.LanguageCode) []core.Language {
	t.Helper()
	var languages []core.Language
	for _, code := range codes {
		language, err := lang.GetLanguage(string(code))
		require.NoError(t, err)
		languages = append(languages, language)
	}
	return languages
}

func projectLanguages(t *testing.T) []core.Language {
	return languages(t, core.LanguageCodeJavascript, core.LanguageCodeTypescript, core.LanguageCodePython)
}

// countingIndexer returns an indexer that counts its parses.
func countingIndexer() (*Indexer, *int) {
	parses := 0
	return &Indexer{
		parse: func(ctx context.Context, language core.Language, data []byte, opts ...parser.Option) (core.ParseTree, error) {
			parses++
			return parser.ParseBytes(ctx, language, data, opts...)
		},
	}, &parses
}

func statuses(status fidx.FileStatus, names ...string) map[string]fidx.FileStatus {
	files := map[string]fidx.FileStatus{}
	for _, name := range names {
		files[name] = status
	}
	return files
}

var sourceNames = []string{"app/util.ts", "lib/helper.js", "main.js", "tool.py"}

func TestIndexerWarmRunParsesNothing(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, projectFiles)
	fsys := appFileSystem(t, dir)
	opts := Options{Languages: projectLanguages(t), Store: memstore.New()}

	indexer, parses := countingIndexer()
	cold, err := indexer.Run(context.Background(), fsys, opts)
	require.NoError(t, err)
	assert.Equal(t, 4, *parses)
	assert.Equal(t, statuses(fidx.FileIndexed, sourceNames...), cold.Files)
	assert.Empty(t, cold.Report.Failed)
	require.Len(t, cold.Indexes, 4)
	for i, name := range sourceNames {
		assert.Equal(t, name, cold.Indexes[i].Path)
	}

	*parses = 0
	warm, err := indexer.Run(context.Background(), fsys, opts)
	require.NoError(t, err)
	assert.Zero(t, *parses)
	assert.Equal(t, statuses(fidx.FileCached, sourceNames...), warm.Files)
	assert.Equal(t, cold.Indexes, warm.Indexes)
	assert.Empty(t, warm.Report.Visited)

	t.Run("a changed file parses again", func(t *testing.T) {
		writeFiles(t, dir, map[string]string{"main.js": "import { helper } from './lib/helper';\nhelper();\nhelper();\n"})
		*parses = 0
		changed, err := indexer.Run(context.Background(), fsys, opts)
		require.NoError(t, err)
		assert.Equal(t, 1, *parses)
		assert.Equal(t, fidx.FileIndexed, changed.Files["main.js"])
		assert.Equal(t, fidx.FileCached, changed.Files["tool.py"])
		assert.Len(t, changed.Indexes[2].Calls, 2)
	})
}

func TestIndexerWithNoStoreParsesEveryFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, projectFiles)
	indexer, parses := countingIndexer()
	for range 2 {
		result, err := indexer.Run(context.Background(), appFileSystem(t, dir), Options{Languages: projectLanguages(t)})
		require.NoError(t, err)
		assert.Equal(t, statuses(fidx.FileIndexed, sourceNames...), result.Files)
	}
	assert.Equal(t, 8, *parses)
}

func TestIndexerWithLocalDBStoreReadsTheStore(t *testing.T) {
	dir, dbDir := t.TempDir(), t.TempDir()
	writeFiles(t, dir, projectFiles)
	fsys := appFileSystem(t, dir)

	run := func() (*Result, int) {
		mgr := localdb.New(localdb.Config{Dir: dbDir})
		defer func() { assert.NoError(t, mgr.Close()) }()
		s, err := localdbstore.New(context.Background(), mgr)
		require.NoError(t, err)

		indexer, parses := countingIndexer()
		result, err := indexer.Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s})
		require.NoError(t, err)
		return result, *parses
	}

	cold, coldParses := run()
	assert.Equal(t, 4, coldParses)
	warm, warmParses := run()
	assert.Zero(t, warmParses)
	assert.Equal(t, statuses(fidx.FileCached, sourceNames...), warm.Files)
	assert.Equal(t, cold.Indexes, warm.Indexes)
}

// cancelingStore cancels the run at its first Put.
type cancelingStore struct {
	store.Store
	cancel context.CancelFunc
}

func (s *cancelingStore) Put(ctx context.Context, indexes []*fidx.Index) error {
	defer s.cancel()
	return s.Store.Put(ctx, indexes)
}

func TestIndexerStopsAfterTheDeadline(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}
	for i := range batchSize + 44 {
		files[fmt.Sprintf("f%03d.js", i)] = fmt.Sprintf("function f%d() {}\n", i)
	}
	writeFiles(t, dir, files)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var indexer Indexer
	result, err := indexer.Run(ctx, appFileSystem(t, dir), Options{
		Languages: languages(t, core.LanguageCodeJavascript),
		Store:     &cancelingStore{Store: memstore.New(), cancel: cancel},
	})
	require.ErrorIs(t, err, context.Canceled)

	count := map[fidx.FileStatus]int{}
	for _, status := range result.Files {
		count[status]++
	}
	assert.Equal(t, map[fidx.FileStatus]int{fidx.FileIndexed: batchSize, fidx.FileMissing: 44}, count)
	assert.Len(t, result.Indexes, batchSize)
}

// recordingPlugin records the name of each file that it analyzes.
type recordingPlugin struct {
	mu    sync.Mutex
	files []string
	paths []string
}

func (p *recordingPlugin) Name() string { return "RecordingPlugin" }

func (p *recordingPlugin) SupportedLanguages() []core.LanguageCode {
	return []core.LanguageCode{core.LanguageCodeJavascript, core.LanguageCodeTypescript, core.LanguageCodePython}
}

func (p *recordingPlugin) AnalyzeTree(_ context.Context, tree core.ParseTree) error {
	file, err := tree.File()
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.files = append(p.files, file.Name())
	p.paths = append(p.paths, file.Path())
	return nil
}

func TestIndexerRunsExtraPluginsOnEveryFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, projectFiles)
	fsys := appFileSystem(t, dir)
	opts := Options{Languages: projectLanguages(t), Store: memstore.New()}

	var indexer Indexer
	_, err := indexer.Run(context.Background(), fsys, opts)
	require.NoError(t, err)

	extra := &recordingPlugin{}
	opts.Plugins = []core.Plugin{extra}
	result, err := indexer.Run(context.Background(), fsys, opts)
	require.NoError(t, err)
	assert.ElementsMatch(t, sourceNames, extra.files)
	assert.Equal(t, statuses(fidx.FileIndexed, sourceNames...), result.Files)
	for _, p := range extra.paths {
		assert.True(t, filepath.IsAbs(p), "a plugin sees the path of the file, not %s", p)
	}
}

func TestIndexerRejectsTwoFilesOfOneName(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeFiles(t, first, map[string]string{"src/x.js": "x();\n"})
	writeFiles(t, second, map[string]string{"src/x.js": "y();\n"})

	var indexer Indexer
	_, err := indexer.Run(context.Background(), appFileSystem(t, first, second), Options{Languages: projectLanguages(t)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "two files have the name src/x.js")
}

func TestIndexerFailsAFileOverTheLimit(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"big.js":   strings.Repeat("x", parser.DefaultMaxBytes+1),
		"small.js": "x();\n",
	})

	var indexer Indexer
	result, err := indexer.Run(context.Background(), appFileSystem(t, dir), Options{Languages: projectLanguages(t)})
	require.NoError(t, err)
	assert.Equal(t, map[string]fidx.FileStatus{"big.js": fidx.FileFailed, "small.js": fidx.FileIndexed}, result.Files)
	require.Len(t, result.Report.Failed, 1)
	assert.Equal(t, core.StageParse, result.Report.Failed[0].Stage)
	assert.ErrorIs(t, result.Report.Failed[0], parser.ErrInputTooLarge)
}

func TestIndexerNeedsALanguage(t *testing.T) {
	var indexer Indexer
	_, err := indexer.Run(context.Background(), appFileSystem(t, t.TempDir()), Options{})
	assert.Error(t, err)
}

package indexer

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store/memstore"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingPlugin is a cached plugin that keeps the number of lines of each
// file. It records which files it analyzed from a tree and which it
// restored from the store.
type countingPlugin struct {
	version  string
	analyzed []string
	restored []string
	lines    map[string]string
	fail     string
}

func (p *countingPlugin) Name() string    { return "CountingPlugin" }
func (p *countingPlugin) Version() string { return p.version }

func (p *countingPlugin) SupportedLanguages() []core.LanguageCode {
	return []core.LanguageCode{core.LanguageCodeJavascript, core.LanguageCodeTypescript}
}

func (p *countingPlugin) Analyze(_ context.Context, file core.File, tree core.ParseTree) ([]byte, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, err
	}
	p.analyzed = append(p.analyzed, file.Name())
	return []byte(strconv.Itoa(strings.Count(string(*data), "\n"))), nil
}

func (p *countingPlugin) Restore(_ context.Context, file core.File, data []byte) error {
	if file.Name() == p.fail {
		return errors.New("bad artifact")
	}
	p.restored = append(p.restored, file.Name())
	p.lines[file.Name()] = string(data)
	return nil
}

func newCountingPlugin(version string) *countingPlugin {
	return &countingPlugin{version: version, lines: map[string]string{}}
}

// ecmascriptNames are the files of the plugin's languages, in path order.
var ecmascriptNames = []string{"app/util.ts", "lib/helper.js", "main.js"}

func TestIndexerServesACachedPluginFromTheStore(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, projectFiles)
	fsys := appFileSystem(t, dir)
	s := memstore.New()
	var indexer Indexer

	cold := newCountingPlugin("1")
	result, err := indexer.Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{cold}})
	require.NoError(t, err)
	assert.Equal(t, sourceNames, result.Report.Visited)
	assert.Equal(t, ecmascriptNames, cold.analyzed)
	assert.Empty(t, cold.restored)

	warm := newCountingPlugin("1")
	result, err = indexer.Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{warm}})
	require.NoError(t, err)
	assert.Empty(t, result.Report.Visited, "a warm run with a cached plugin parses no file")
	assert.Empty(t, result.Report.Failed)
	assert.Equal(t, statuses(fileindex.FileCached, sourceNames...), result.Files)
	assert.Empty(t, warm.analyzed)
	assert.Equal(t, ecmascriptNames, warm.restored)
	assert.Equal(t, map[string]string{"app/util.ts": "1", "lib/helper.js": "1", "main.js": "2"}, warm.lines)

	t.Run("a new plugin version parses again", func(t *testing.T) {
		next := newCountingPlugin("2")
		result, err := indexer.Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{next}})
		require.NoError(t, err)
		assert.Equal(t, ecmascriptNames, result.Report.Visited, "the files of the plugin's languages parse, the Python file stays cached")
		assert.Equal(t, ecmascriptNames, next.analyzed)
		assert.Equal(t, fileindex.FileCached, result.Files["tool.py"])
		assert.Equal(t, fileindex.FileIndexed, result.Files["main.js"])
	})

	t.Run("a plugin that is not cached forces a parse", func(t *testing.T) {
		extra := &recordingPlugin{}
		again := newCountingPlugin("1")
		result, err := indexer.Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{again, extra}})
		require.NoError(t, err)
		assert.Equal(t, sourceNames, result.Report.Visited)
		assert.Equal(t, ecmascriptNames, again.analyzed, "a parsed file goes to Analyze, not to Restore")
		assert.Empty(t, again.restored)
	})

	t.Run("a restore that fails goes to the report", func(t *testing.T) {
		broken := newCountingPlugin("1")
		broken.fail = "main.js"
		result, err := indexer.Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{broken}})
		require.NoError(t, err)
		assert.Empty(t, result.Report.Visited)
		assert.Equal(t, []plugin.FileError{{File: "main.js", Stage: "CountingPlugin", Err: errors.New("bad artifact")}}, result.Report.Failed)
		assert.Equal(t, fileindex.FileCached, result.Files["main.js"], "the index of the file is still served")
	})
}

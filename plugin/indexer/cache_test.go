package indexer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/fileindex/store/memstore"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin/callgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingPlugin is a cached plugin that keeps the number of lines of each
// file. It records which files it analyzed from a tree and which it
// restored from the store.
type countingPlugin struct {
	name     string
	version  string
	analyzed []string
	restored []string
	lines    map[string]string
	fail     string
}

func (p *countingPlugin) Name() string    { return p.name }
func (p *countingPlugin) Version() string { return p.version }

func (p *countingPlugin) SupportedLanguages() []core.LanguageCode {
	return []core.LanguageCode{core.LanguageCodeJavascript, core.LanguageCodeTypescript}
}

func (p *countingPlugin) Analyze(_ context.Context, file core.File, tree core.ParseTree) ([]byte, error) {
	p.analyzed = append(p.analyzed, file.Name())
	return lineCount(tree)
}

func lineCount(tree core.ParseTree) ([]byte, error) {
	data, err := tree.Data()
	if err != nil {
		return nil, err
	}
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
	return &countingPlugin{name: "CountingPlugin", version: version, lines: map[string]string{}}
}

// ecmascriptNames are the files of the plugin's languages, in path order.
var ecmascriptNames = []string{"app/util.ts", "lib/helper.js", "main.js"}

func TestIndexerServesACachedPluginFromTheStore(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, projectFiles)
	fsys := appFileSystem(t, dir)
	s := memstore.New()

	cold := newCountingPlugin("1")
	result, err := runIndexer(t, context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{cold}})
	require.NoError(t, err)
	assert.Equal(t, sourceNames, result.Report.Visited)
	assert.Equal(t, ecmascriptNames, cold.analyzed)
	assert.Empty(t, cold.restored)

	warm := newCountingPlugin("1")
	result, err = runIndexer(t, context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{warm}})
	require.NoError(t, err)
	assert.Empty(t, result.Report.Visited, "a warm run with a cached plugin parses no file")
	assert.Empty(t, result.Report.Failed)
	assert.Equal(t, statuses(fileindex.FileCached, sourceNames...), result.Files)
	assert.Empty(t, warm.analyzed)
	assert.Equal(t, ecmascriptNames, warm.restored)
	assert.Equal(t, map[string]string{"app/util.ts": "1", "lib/helper.js": "1", "main.js": "2"}, warm.lines)

	t.Run("a new plugin version parses again", func(t *testing.T) {
		next := newCountingPlugin("2")
		result, err := runIndexer(t, context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{next}})
		require.NoError(t, err)
		assert.Equal(t, ecmascriptNames, result.Report.Visited, "the files of the plugin's languages parse, the Python file stays cached")
		assert.Equal(t, ecmascriptNames, next.analyzed)
		assert.Equal(t, fileindex.FileCached, result.Files["tool.py"])
		assert.Equal(t, fileindex.FileIndexed, result.Files["main.js"])
	})

	t.Run("a plugin that is not cached forces a parse", func(t *testing.T) {
		extra := &recordingPlugin{}
		again := newCountingPlugin("1")
		result, err := runIndexer(t, context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{again, extra}})
		require.NoError(t, err)
		assert.Equal(t, sourceNames, result.Report.Visited)
		assert.Equal(t, ecmascriptNames, again.analyzed, "a parsed file goes to Analyze, not to Restore")
		assert.Empty(t, again.restored)
	})

	t.Run("a restore that fails parses the file again", func(t *testing.T) {
		broken := newCountingPlugin("1")
		broken.fail = "main.js"
		result, err := runIndexer(t, context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{broken}})
		require.NoError(t, err)
		assert.Equal(t, []string{"main.js"}, result.Report.Visited)
		assert.Empty(t, result.Report.Failed)
		assert.Equal(t, fileindex.FileIndexed, result.Files["main.js"])
		assert.Equal(t, []string{"main.js"}, broken.analyzed, "the plugin runs on the tree and the run stores a fresh artifact")
		assert.Equal(t, []string{"app/util.ts", "lib/helper.js"}, broken.restored)
	})

	t.Run("a plugin that took its artifact skips the tree of a file that parses again", func(t *testing.T) {
		good := newCountingPlugin("1")
		broken := newCountingPlugin("1")
		broken.name = "BrokenPlugin"
		_, err := runIndexer(t, context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{good, broken}})
		require.NoError(t, err)

		good, broken = newCountingPlugin("1"), newCountingPlugin("1")
		broken.name = "BrokenPlugin"
		broken.fail = "main.js"
		result, err := runIndexer(t, context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{good, broken}})
		require.NoError(t, err)
		assert.Equal(t, []string{"main.js"}, result.Report.Visited)
		assert.Equal(t, ecmascriptNames, good.restored)
		assert.Empty(t, good.analyzed, "the plugin that took its artifact does not see main.js twice")
		assert.Equal(t, []string{"main.js"}, broken.analyzed)
	})
}

// graphCountingPlugin is a countingPlugin that reads the call graph of the
// index. It records the file of each graph that it got.
type graphCountingPlugin struct {
	*countingPlugin
	graphs []string
}

func (p *graphCountingPlugin) AnalyzeGraph(ctx context.Context, file core.File, tree core.ParseTree, cg *callgraph.CallGraph) ([]byte, error) {
	built, err := callgraph.Build(ctx, tree)
	if err != nil {
		return nil, err
	}
	var got, want bytes.Buffer
	if err := cg.Dump(&got); err != nil {
		return nil, err
	}
	if err := built.Dump(&want); err != nil {
		return nil, err
	}
	if cg.FileName != file.Name() || len(cg.Nodes) == 0 || got.String() != want.String() {
		return nil, errors.New("not the call graph of the file")
	}
	p.graphs = append(p.graphs, file.Name())
	return lineCount(tree)
}

func TestIndexerGivesAGraphPluginTheCallGraphOfTheIndex(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, projectFiles)
	fsys := appFileSystem(t, dir)
	s := memstore.New()

	pl := &graphCountingPlugin{countingPlugin: newCountingPlugin("1")}
	result, err := runIndexer(t, context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{pl}})
	require.NoError(t, err)
	assert.Empty(t, result.Report.Failed)
	assert.Equal(t, ecmascriptNames, pl.graphs)
	assert.Empty(t, pl.analyzed, "the run builds no second call graph through Analyze")

	warm := &graphCountingPlugin{countingPlugin: newCountingPlugin("1")}
	_, err = runIndexer(t, context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Plugins: []core.Plugin{warm}})
	require.NoError(t, err)
	assert.Equal(t, ecmascriptNames, warm.restored, "the artifact of AnalyzeGraph serves the next run")
	assert.Equal(t, map[string]string{"app/util.ts": "1", "lib/helper.js": "1", "main.js": "2"}, warm.lines)
}

func TestAnalyzeKeepsTheCallGraphOnlyForAGraphPlugin(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"main.js": projectFiles["main.js"]})
	langs := projectLanguages(t)
	files, _, err := enumerate(ctx, appFileSystem(t, dir), Options{Languages: langs})
	require.NoError(t, err)
	require.Len(t, files, 1)
	byExtension, err := parser.LanguagesByExtension(langs)
	require.NoError(t, err)
	data, err := read(files[0], parser.DefaultMaxBytes)
	require.NoError(t, err)
	p, err := parser.NewParser(langs)
	require.NoError(t, err)
	defer p.Close()

	file := pending{file: files[0], language: byExtension[".js"], data: data}
	for _, keep := range []bool{false, true} {
		o := analyze(ctx, p, file, keep)
		require.NoError(t, o.err)
		require.NotNil(t, o.index)
		o.tree.Close()
		assert.Equal(t, keep, o.graph != nil, "a run with no GraphPlugin drops the graph after the index")
	}
}

// storeContents returns the index that the store holds for each file, and
// the artifact of the plugin for each file. The files are JavaScript.
func storeContents(t *testing.T, s store.Store, files map[string]string, pl CachedPlugin) (map[string]*fileindex.Index, map[string]string) {
	t.Helper()
	var keys []fileindex.Key
	var artifactKeys []store.ArtifactKey
	for name, content := range files {
		key := fileindex.NewKey(name, fileindex.HashContent([]byte(content)), langcode.Javascript)
		keys = append(keys, key)
		artifactKeys = append(artifactKeys, store.ArtifactKey{Key: key, Plugin: pl.Name(), Version: pl.Version()})
	}
	stored, err := s.Get(context.Background(), keys)
	require.NoError(t, err)
	indexes := map[string]*fileindex.Index{}
	for key, ix := range stored {
		indexes[key.Path] = ix
	}
	data, err := s.GetArtifacts(context.Background(), artifactKeys)
	require.NoError(t, err)
	artifacts := map[string]string{}
	for key, d := range data {
		artifacts[key.Path] = string(d)
	}
	return indexes, artifacts
}

// finished returns the files that a run indexed, by name.
func finished(r *Result) map[string]*fileindex.Index {
	out := map[string]*fileindex.Index{}
	for _, ix := range r.Indexes {
		if r.Files[ix.Path] == fileindex.FileIndexed {
			out[ix.Path] = ix
		}
	}
	return out
}

// cuttingPlugin is a countingPlugin that ends the run when it analyzes the
// file at. After the end of ctx it returns a partial artifact with no
// error, as a plugin that stops at the deadline can.
type cuttingPlugin struct {
	*countingPlugin
	at     string
	cancel context.CancelFunc
}

func (p *cuttingPlugin) Analyze(ctx context.Context, file core.File, tree core.ParseTree) ([]byte, error) {
	if file.Name() == p.at {
		p.cancel()
	}
	if ctx.Err() != nil {
		return []byte("partial"), nil
	}
	return p.countingPlugin.Analyze(ctx, file, tree)
}

func TestIndexerStoresTheFinishedFilesOfACutBatch(t *testing.T) {
	files := map[string]string{}
	var names []string
	for i := range 10 {
		name := fmt.Sprintf("f%d.js", i)
		files[name] = fmt.Sprintf("function f%d() {}\n", i)
		names = append(names, name)
	}
	dir := t.TempDir()
	writeFiles(t, dir, files)
	fsys := appFileSystem(t, dir)
	langs := languages(t, core.LanguageCodeJavascript)

	for _, workers := range []int{1, 4} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			s := memstore.New()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			pl := &cuttingPlugin{countingPlugin: newCountingPlugin("1"), at: "f4.js", cancel: cancel}
			cut, err := runIndexer(t, ctx, fsys, Options{Languages: langs, Store: s, Plugins: []core.Plugin{pl}, Workers: workers})
			require.ErrorIs(t, err, context.Canceled)

			done := finished(cut)
			var rest []string
			for _, name := range names {
				if _, ok := done[name]; !ok {
					assert.Equal(t, fileindex.FileMissing, cut.Files[name], name)
					rest = append(rest, name)
				}
			}
			assert.Subset(t, slices.Collect(maps.Keys(done)), names[:5], "the files up to the cut finish")

			indexes, artifacts := storeContents(t, s, files, pl)
			assert.Equal(t, done, indexes, "the store holds the index of each finished file and of no other file")
			assert.Equal(t, map[string]string{"f0.js": "1", "f1.js": "1", "f2.js": "1", "f3.js": "1"}, artifacts,
				"the store holds no artifact that the plugin made after the cut")

			next, err := runIndexer(t, context.Background(), fsys, Options{Languages: langs, Store: s, Workers: workers})
			require.NoError(t, err)
			assert.Equal(t, rest, next.Report.Visited, "the next run parses only the files that the cut stopped")
			for name := range done {
				assert.Equal(t, fileindex.FileCached, next.Files[name], name)
			}
		})
	}
}

// graphKeepingPlugin is a GraphPlugin that keeps the number of lines of a
// file, or "partial" for a partial call graph, as a plugin that does not
// check the graph can.
type graphKeepingPlugin struct {
	*countingPlugin
}

func (p *graphKeepingPlugin) AnalyzeGraph(_ context.Context, _ core.File, tree core.ParseTree, cg *callgraph.CallGraph) ([]byte, error) {
	for _, t := range cg.Truncations() {
		if t.Kind == callgraph.TruncationDeadline {
			return []byte("partial"), nil
		}
	}
	return lineCount(tree)
}

func TestIndexerStoresNoPartialIndexOrGraphArtifact(t *testing.T) {
	var big strings.Builder
	for i := range 20000 {
		_, err := fmt.Fprintf(&big, "f%d();\n", i)
		require.NoError(t, err)
	}
	files := map[string]string{
		"a.js": "function a() {}\n",
		"b.js": "function b() {}\n",
		// The deadline cuts the build of the call graph of this file.
		"c.js": big.String(),
		"d.js": "function d() {}\n",
	}
	dir := t.TempDir()
	writeFiles(t, dir, files)
	fsys := appFileSystem(t, dir)

	for _, workers := range []int{1, 4} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			s := memstore.New()
			pl := &graphKeepingPlugin{countingPlugin: newCountingPlugin("1")}
			ctx := &cutContext{Context: context.Background(), at: []string{callGraphBuild}}
			result, err := runIndexer(t, ctx, fsys, Options{Languages: languages(t, core.LanguageCodeJavascript), Store: s, Plugins: []core.Plugin{pl}, Workers: workers})
			require.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, fileindex.FileMissing, result.Files["c.js"])

			indexes, artifacts := storeContents(t, s, files, pl)
			assert.Equal(t, finished(result), indexes, "the store holds the index of each finished file and of no other file")
			assert.NotContains(t, indexes, "c.js")
			for name, data := range artifacts {
				assert.Equal(t, "1", data, name)
				assert.Contains(t, indexes, name, "an artifact comes with the index of its file")
			}
			if workers == 1 {
				assert.Contains(t, indexes, "a.js", "one worker finishes the files before the cut")
				assert.Contains(t, indexes, "b.js", "one worker finishes the files before the cut")
			}
		})
	}
}

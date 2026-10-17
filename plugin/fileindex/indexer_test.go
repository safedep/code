package fileindex

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

	var indexer Indexer
	cold, err := indexer.Run(context.Background(), fsys, opts)
	require.NoError(t, err)
	assert.Equal(t, sourceNames, cold.Report.Visited)
	assert.Equal(t, statuses(fidx.FileIndexed, sourceNames...), cold.Files)
	assert.Empty(t, cold.Report.Failed)
	require.Len(t, cold.Indexes, 4)
	for i, name := range sourceNames {
		assert.Equal(t, name, cold.Indexes[i].Path)
	}

	warm, err := indexer.Run(context.Background(), fsys, opts)
	require.NoError(t, err)
	assert.Equal(t, statuses(fidx.FileCached, sourceNames...), warm.Files)
	assert.Equal(t, cold.Indexes, warm.Indexes)
	assert.Empty(t, warm.Report.Visited)

	t.Run("a changed file parses again", func(t *testing.T) {
		writeFiles(t, dir, map[string]string{"main.js": "import { helper } from './lib/helper';\nhelper();\nhelper();\n"})
		changed, err := indexer.Run(context.Background(), fsys, opts)
		require.NoError(t, err)
		assert.Equal(t, []string{"main.js"}, changed.Report.Visited)
		assert.Equal(t, fidx.FileIndexed, changed.Files["main.js"])
		assert.Equal(t, fidx.FileCached, changed.Files["tool.py"])
		assert.Len(t, changed.Indexes[2].Calls, 2)
	})
}

func TestIndexerWithNoStoreParsesEveryFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, projectFiles)
	var indexer Indexer
	for range 2 {
		result, err := indexer.Run(context.Background(), appFileSystem(t, dir), Options{Languages: projectLanguages(t)})
		require.NoError(t, err)
		assert.Equal(t, statuses(fidx.FileIndexed, sourceNames...), result.Files)
		assert.Equal(t, sourceNames, result.Report.Visited)
	}
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

		var indexer Indexer
		result, err := indexer.Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s})
		require.NoError(t, err)
		return result, len(result.Report.Visited)
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

func TestIndexerTakesAFileSizeLimit(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"big.js":   strings.Repeat("x", 65) + "();\n",
		"small.js": "x();\n",
	})

	var indexer Indexer
	result, err := indexer.Run(context.Background(), appFileSystem(t, dir), Options{Languages: projectLanguages(t), MaxFileBytes: 64})
	require.NoError(t, err)
	assert.Equal(t, map[string]fidx.FileStatus{"big.js": fidx.FileFailed, "small.js": fidx.FileIndexed}, result.Files)
	require.Len(t, result.Report.Failed, 1)
	assert.ErrorIs(t, result.Report.Failed[0], parser.ErrInputTooLarge)
}

func TestIndexerNeedsALanguage(t *testing.T) {
	var indexer Indexer
	_, err := indexer.Run(context.Background(), appFileSystem(t, t.TempDir()), Options{})
	assert.Error(t, err)
}

// output is the result of a run as text, so two runs compare byte for byte.
func output(t *testing.T, r *Result) string {
	t.Helper()
	failed := make([]string, len(r.Report.Failed))
	for i, f := range r.Report.Failed {
		failed[i] = f.Error()
	}
	data, err := json.Marshal(map[string]any{
		"indexes": r.Indexes,
		"files":   r.Files,
		"visited": r.Report.Visited,
		"skipped": r.Report.Skipped,
		"failed":  failed,
	})
	require.NoError(t, err)
	return string(data)
}

// generatedProject writes more files than two batches, in two languages,
// with imports across files, one file over the size limit and one file that
// no language parses.
func generatedProject(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		"big.js":    strings.Repeat("x", 4096) + "();\n",
		"notes.txt": "not source\n",
	}
	for i := range batchSize + 300 {
		next := (i + 1) % (batchSize + 300)
		files[fmt.Sprintf("js/m%03d.js", i)] = fmt.Sprintf(
			"import { f%d } from './m%03d';\nexport function f%d(x) {\n  return f%d(x.y) + g(x);\n}\nclass C%d { run() { this.go(); } }\n",
			next, next, i, next, i)
		files[fmt.Sprintf("py/m%03d.py", i)] = fmt.Sprintf(
			"from .m%03d import f%d\n\ndef f%d(x):\n    return f%d(x.y) + g(x)\n\nclass C%d:\n    def run(self):\n        self.go()\n",
			next, next, i, next, i)
	}
	dir := t.TempDir()
	writeFiles(t, dir, files)
	return dir
}

func TestIndexerGivesOneResultForAnyNumberOfWorkers(t *testing.T) {
	fsys := appFileSystem(t, generatedProject(t))
	run := func(workers int) (string, []string) {
		extra := &recordingPlugin{}
		var indexer Indexer
		result, err := indexer.Run(context.Background(), fsys, Options{
			Languages:    projectLanguages(t),
			Plugins:      []core.Plugin{extra},
			MaxFileBytes: 1024,
			Workers:      workers,
		})
		require.NoError(t, err)
		require.Len(t, result.Report.Failed, 1)
		assert.Equal(t, result.Report.Visited, extra.files, "the extra plugins run in path order")
		return output(t, result), extra.files
	}

	serial, serialFiles := run(1)
	assert.Len(t, serialFiles, 2*(batchSize+300))
	for _, workers := range []int{2, 4, 7} {
		parallel, parallelFiles := run(workers)
		assert.Equal(t, serial, parallel, "workers=%d", workers)
		assert.Equal(t, serialFiles, parallelFiles, "workers=%d", workers)
	}
}

// deadlineStore starts the deadline of the run when the run asks for the
// first batch, just before the parse. A deadline on the whole run can land
// before the parse on a slow machine.
type deadlineStore struct {
	store.Store
	cancel  context.CancelFunc
	after   time.Duration
	started time.Time
}

func (s *deadlineStore) Get(ctx context.Context, keys []fidx.Key) (map[fidx.Key]*fidx.Index, error) {
	if s.started.IsZero() {
		s.started = time.Now()
		time.AfterFunc(s.after, s.cancel)
	}
	return s.Store.Get(ctx, keys)
}

func TestIndexerCutsAParseAtTheDeadline(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		// The parse of this file takes more than a second.
		"a_big.py": strings.Repeat("def f(a):\n    return g(a, b) + h[c]\n", 100000),
		"b.py":     "def b():\n    pass\n",
		"c.py":     "def c():\n    pass\n",
	})

	for _, workers := range []int{1, 4} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &deadlineStore{Store: memstore.New(), cancel: cancel, after: 100 * time.Millisecond}
			var indexer Indexer
			result, err := indexer.Run(ctx, appFileSystem(t, dir), Options{Languages: projectLanguages(t), Store: s, Workers: workers})
			require.ErrorIs(t, err, context.Canceled)
			assert.Less(t, time.Since(s.started), time.Second, "the deadline stops the parse")

			assert.Equal(t, fidx.FileMissing, result.Files["a_big.py"])
			require.NotEmpty(t, result.Report.Failed)
			cut := result.Report.Failed[0]
			assert.Equal(t, "a_big.py", cut.File)
			assert.Equal(t, core.StageParse, cut.Stage)
			assert.ErrorIs(t, cut, context.Canceled)
			for _, ix := range result.Indexes {
				assert.NotEqual(t, "a_big.py", ix.Path)
				assert.Equal(t, fidx.FileIndexed, result.Files[ix.Path])
			}
			assert.Len(t, result.Files, 3)
		})
	}
}

func TestIndexerIndexesTheImportDirectories(t *testing.T) {
	const unit = "npm:lodash@4.17.21"
	pkg := map[string]string{
		"index.js":    "export { template } from './template';\n",
		"template.js": "export function template() { return 1; }\n",
	}
	project := func() (string, core.ImportAwareFileSystem) {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{
			"index.js":                    "import { template } from 'lodash';\ntemplate();\n",
			"node_modules/lodash/x.txt":   "not source\n",
			"node_modules/other/index.js": "export const other = 1;\n",
		})
		writeFiles(t, filepath.Join(dir, "node_modules", "lodash"), pkg)
		lodash, other := filepath.Join(dir, "node_modules", "lodash"), filepath.Join(dir, "node_modules", "other")
		fsys, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
			AppDirectories:    []string{dir},
			ImportDirectories: []string{lodash, other},
			ImportUnits:       map[string]string{lodash: unit},
		})
		require.NoError(t, err)
		return dir, fsys
	}

	_, fsys := project()
	s := memstore.New()
	result, err := (&Indexer{}).Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s})
	require.NoError(t, err)
	assert.Equal(t, statuses(fidx.FileIndexed, "index.js"), result.Files, "the run reads no import file unless the caller asks")
	assert.Empty(t, result.Dependencies)

	result, err = (&Indexer{}).Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Imports: true})
	require.NoError(t, err)
	assert.Equal(t, map[string]fidx.FileStatus{"index.js": fidx.FileCached}, result.Files)
	assert.Equal(t, statuses(fidx.FileIndexed, unit+"//index.js", unit+"//template.js"), result.Dependencies)
	assert.Equal(t, map[string]fidx.FileStatus{"index.js": fidx.FileCached, unit + "//index.js": fidx.FileIndexed, unit + "//template.js": fidx.FileIndexed}, result.Statuses())
	assert.Contains(t, result.Report.Skipped, "index.js", "an import file with no unit has no key")

	byPath := map[string]*fidx.Index{}
	for _, ix := range result.Indexes {
		byPath[ix.Path] = ix
	}
	require.Len(t, byPath, 3, "a file of the package and a file of the project with one relative path do not collide")
	assert.Empty(t, byPath["index.js"].Unit)
	assert.Equal(t, unit, byPath[unit+"//template.js"].Unit)
	assert.Equal(t, unit+"//template.js//template", byPath[unit+"//template.js"].Definitions[1].Key)

	_, fsys = project()
	result, err = (&Indexer{}).Run(context.Background(), fsys, Options{Languages: projectLanguages(t), Store: s, Imports: true})
	require.NoError(t, err)
	assert.Equal(t, statuses(fidx.FileCached, unit+"//index.js", unit+"//template.js"), result.Dependencies, "a second project with the package version reads its indexes from the store")
}

//go:build projectgraph

package projectgraph

import (
	"context"
	"encoding/json"
	iofs "io/fs"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/fileindex/store/localdbstore"
	"github.com/safedep/code/fileindex/store/memstore"
	codefs "github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/langcode"
	"github.com/safedep/code/plugin/indexer"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/safedep/code/projectgraph/summary"
	"github.com/safedep/code/projectgraph/summary/summarytest"
	"github.com/safedep/dry/localdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// benchFilesEnv lists the project sizes of the benchmark, as 1000,10000.
	benchFilesEnv = "PROJECTGRAPH_BENCH_FILES"

	// benchWorkersEnv lists the numbers of workers of the cold index step,
	// as 1,4. The other index steps use the last one. The default is 1,4,
	// for the 4 cores of the machine of the report.
	benchWorkersEnv = "PROJECTGRAPH_BENCH_WORKERS"
)

var benchTestPaths = []string{"tests", "test", "test_*.py", "*.test.js", "*.test.ts", "*_test.go", "*Test.java", "*Test.kt", "*Tests.cs", "*Test.php", "*_test.rb"}

func benchSizes(b *testing.B) []int {
	return envInts(b, benchFilesEnv, 1000)
}

func envInts(b *testing.B, name string, fallback ...int) []int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	var values []int
	for _, field := range strings.Split(value, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(field))
		require.NoError(b, err, name)
		values = append(values, n)
	}
	return values
}

// BenchmarkProjectGraph measures each step on a generated project per
// language and size. index-cold parses every file with no store, once for
// each number of workers of PROJECTGRAPH_BENCH_WORKERS. index-cold-store
// parses every file and puts each index in an empty store. index-warm finds
// every index in the store. link and reach run on the indexes of the warm
// store. Each step reports the peak heap of its run. The index steps other
// than index-cold use the last number of workers.
func BenchmarkProjectGraph(b *testing.B) {
	for _, code := range langcode.All() {
		for _, files := range benchSizes(b) {
			b.Run(string(code)+"/files="+strconv.Itoa(files), func(b *testing.B) {
				benchProject(b, code, files)
			})
		}
	}
}

func benchProject(b *testing.B, code core.LanguageCode, files int) {
	ctx := context.Background()
	language, err := lang.GetLanguage(string(code))
	require.NoError(b, err)
	languages := []core.Language{language}

	root := b.TempDir()
	written, err := generate(code, files).write(root)
	require.NoError(b, err)
	fsys, err := codefs.NewLocalFileSystem(codefs.LocalFileSystemConfig{AppDirectories: []string{root}})
	require.NoError(b, err)

	coldWorkers := envInts(b, benchWorkersEnv, 1, 4)
	workers := coldWorkers[len(coldWorkers)-1]
	index := func(s store.Store, workers int) *indexer.Result {
		indexed := indexProject(b, fsys, indexer.Options{Languages: languages, Store: s, Workers: workers})
		require.Empty(b, indexed.Report.Failed)
		require.Len(b, indexed.Indexes, written)
		return indexed
	}
	newStore := func(dir string) store.Store {
		mgr := localdb.New(localdb.Config{Dir: dir})
		b.Cleanup(func() { require.NoError(b, mgr.Close()) })
		s, err := localdbstore.New(ctx, mgr)
		require.NoError(b, err)
		return s
	}

	warm := newStore(b.TempDir())
	indexed := index(warm, workers)
	link := func() *projectgraph.Graph {
		return linkProject(b, root, indexed, projectgraph.Options{TestPaths: benchTestPaths})
	}
	g := link()
	targets := reachTargets(code)

	for _, n := range coldWorkers {
		b.Run("index-cold/workers="+strconv.Itoa(n), func(b *testing.B) {
			retained := measure(b, written, func() any { return index(nil, n) })
			b.ReportMetric(float64(retained)/(1<<10)/float64(written), "retained-KiB/file")
		})
	}
	b.Run("index-cold-store", func(b *testing.B) {
		measure(b, written, func() any { return index(newStore(b.TempDir()), workers) })
	})
	b.Run("index-warm", func(b *testing.B) {
		retained := measure(b, written, func() any {
			r := index(warm, workers)
			require.Equal(b, written, count(r.Files, fileindex.FileCached))
			return r
		})
		b.ReportMetric(float64(retained)/(1<<10)/float64(written), "retained-KiB/file")
		b.ReportMetric(jsonKiB(b, indexed.Indexes)/float64(written), "index-KiB/file")
	})
	b.Run("link", func(b *testing.B) {
		retained := measure(b, written, func() any { return link() })
		b.ReportMetric(float64(retained)/float64(len(g.Edges)), "retained-B/edge")
		b.ReportMetric(float64(len(g.Edges)), "edges")
		sites, unresolved := callSites(g, indexed.Indexes)
		b.ReportMetric(float64(unresolved), "unresolved")
		b.ReportMetric(float64(unresolved)/float64(sites), "unresolved-rate")
		b.ReportMetric(float64(len(g.Roots)), "roots")
	})
	b.Run("reach", func(b *testing.B) {
		var results []reach.Result
		measure(b, written, func() any {
			results, err = reach.Reach(ctx, g, reach.Query{Targets: targetsOf(b, targets...)})
			require.NoError(b, err)
			return results
		})
		found := 0
		for _, r := range results {
			if r.Status == reach.Found {
				found++
			}
		}
		b.ReportMetric(float64(len(targets)), "targets")
		b.ReportMetric(float64(found), "found")
	})
	unit, ok := benchUnits[code]
	if !ok {
		return
	}
	b.Run("summary", func(b *testing.B) {
		// The summary reads the project as one unit of dependency code.
		unitFS, err := codefs.NewLocalFileSystem(codefs.LocalFileSystemConfig{ImportDirectories: []string{root}, ImportUnits: map[string]string{root: unit}})
		require.NoError(b, err)
		dependency, err := runIndexer(b, ctx, unitFS, indexer.Options{Languages: languages, Workers: workers, Imports: true})
		require.NoError(b, err)
		require.Len(b, dependency.Indexes, written)
		var s *summary.Summary
		measure(b, written, func() any {
			s, err = summary.Summarize(ctx, dependency.Indexes, projectgraph.Options{Files: dependency.Dependencies, Languages: dependency.Languages, Units: map[string]iofs.FS{unit: os.DirFS(root)}})
			require.NoError(b, err)
			return s
		})
		reaches := 0
		for _, sym := range s.Symbols {
			reaches += len(sym.Reaches)
		}
		b.ReportMetric(float64(len(s.Symbols)), "symbols")
		b.ReportMetric(float64(reaches), "reaches")
	})
}

// TestSummaryStatusMatchesReachOnASyntheticPackage checks the summary of
// a generated Python package of 100 files against a reach query from each
// of its symbols, for every target.
func TestSummaryStatusMatchesReachOnASyntheticPackage(t *testing.T) {
	ctx := context.Background()
	language, err := lang.GetLanguage(string(core.LanguageCodePython))
	require.NoError(t, err)
	root := t.TempDir()
	written, err := generate(core.LanguageCodePython, 100).write(root)
	require.NoError(t, err)
	unit := benchUnits[core.LanguageCodePython]
	unitFS, err := codefs.NewLocalFileSystem(codefs.LocalFileSystemConfig{ImportDirectories: []string{root}, ImportUnits: map[string]string{root: unit}})
	require.NoError(t, err)
	dependency := indexProject(t, unitFS, indexer.Options{Languages: []core.Language{language}, Workers: 4, Imports: true})
	require.Len(t, dependency.Indexes, written)
	r, err := summary.Build(ctx, dependency.Indexes, projectgraph.Options{Files: dependency.Dependencies, Languages: dependency.Languages, Units: map[string]iofs.FS{unit: os.DirFS(root)}})
	require.NoError(t, err)
	require.NotEmpty(t, r.Summary.Uncertainties, "the generated package has dynamic calls")
	summarytest.AssertStatusMatchesReach(t, r)
}

// benchUnits name the generated project of each language that the link
// step links as dependency code, as the unit of its summary.
var benchUnits = map[core.LanguageCode]string{
	core.LanguageCodePython:     "pypi:bench@1.0.0",
	core.LanguageCodeJavascript: "npm:bench-app@1.0.0",
	core.LanguageCodeTypescript: "npm:bench-app@1.0.0",
	core.LanguageCodeGo:         "go:" + goModulePath + "@v1.0.0",
}

// jsonKiB is the size of the JSON form of the indexes, as the store keeps
// them.
func jsonKiB(b *testing.B, indexes []*fileindex.Index) float64 {
	size := 0
	for _, ix := range indexes {
		data, err := json.Marshal(ix)
		require.NoError(b, err)
		size += len(data)
	}
	return float64(size) / (1 << 10)
}

func count(files map[string]fileindex.FileStatus, status fileindex.FileStatus) int {
	n := 0
	for _, s := range files {
		if s == status {
			n++
		}
	}
	return n
}

// measure runs op b.N times and reports the time per 1,000 files, the heap
// before the run and the peak heap. The base holds the indexes and the graph
// that the link and the reach step read. A goroutine samples
// runtime.MemStats, because MemStats keeps no peak of HeapAlloc. The sample
// stops the world for a few microseconds, so it runs every 5 ms only.
//
// measure returns the heap that the result of the last op holds after a
// collection, over the base.
func measure(b *testing.B, files int, op func() any) int64 {
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	peak := newPeakHeap()
	b.ResetTimer()
	start := time.Now()
	var last any
	for i := range b.N {
		if i < b.N-1 {
			op()
			continue
		}
		last = op()
	}
	elapsed := time.Since(start)
	b.StopTimer()
	b.ReportMetric(elapsed.Seconds()/float64(b.N)/float64(files)*1000, "s/1k-files")
	b.ReportMetric(float64(peak.stop())/(1<<20), "peak-heap-MiB")
	b.ReportMetric(float64(base.HeapAlloc)/(1<<20), "base-heap-MiB")

	// The second collection frees the objects that a finalizer of the
	// first one released, as the trees of a parse.
	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(last)
	return int64(after.HeapAlloc) - int64(base.HeapAlloc)
}

type peakHeap struct {
	done chan struct{}
	wg   sync.WaitGroup
	max  uint64
}

func newPeakHeap() *peakHeap {
	p := &peakHeap{done: make(chan struct{})}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			p.sample()
			select {
			case <-p.done:
				return
			case <-ticker.C:
			}
		}
	}()
	return p
}

func (p *peakHeap) sample() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	p.max = max(p.max, m.HeapAlloc)
}

func (p *peakHeap) stop() uint64 {
	close(p.done)
	p.wg.Wait()
	p.sample()
	return p.max
}

// firstReadStore is an empty store that records the time of its first
// read and calls onFirstRead then.
type firstReadStore struct {
	store.Store
	started     time.Time
	onFirstRead func()
}

func (s *firstReadStore) Get(ctx context.Context, keys []fileindex.Key) (map[fileindex.Key]*fileindex.Index, error) {
	if s.started.IsZero() {
		s.started = time.Now()
		if s.onFirstRead != nil {
			s.onFirstRead()
		}
	}
	return s.Store.Get(ctx, keys)
}

// TestDegradesAtHalfTheColdTime gives the index step half the time of a cold
// run. The files that the deadline cuts are missing, the link step takes
// the partial result, and the reach query reports each target that it does
// not find as uncertain with a budget cause, never as not found.
//
// A partial graph can find a path that the whole graph does not. A site
// with more than 20 field candidates in the whole project is ambiguous,
// and with fewer files it has field edges.
func TestDegradesAtHalfTheColdTime(t *testing.T) {
	for _, code := range langcode.All() {
		t.Run(string(code), func(t *testing.T) {
			language, err := lang.GetLanguage(string(code))
			require.NoError(t, err)
			root := t.TempDir()
			written, err := generate(code, 1000).write(root)
			require.NoError(t, err)
			fsys, err := codefs.NewLocalFileSystem(codefs.LocalFileSystemConfig{AppDirectories: []string{root}})
			require.NoError(t, err)
			targets := reachTargets(code)

			search := func(indexed *indexer.Result) []reach.Result {
				g := linkProject(t, root, indexed, projectgraph.Options{TestPaths: benchTestPaths})
				results, err := reach.Reach(context.Background(), g, reach.Query{Targets: targetsOf(t, targets...)})
				require.NoError(t, err)
				return results
			}

			// The clock of both runs starts at the first read of the
			// store, just before the first parse, so the walk of the
			// files does not count. A deadline on the whole run can land
			// before the parse on a slow machine.
			coldStore := &firstReadStore{Store: memstore.New()}
			_, err = runIndexer(t, context.Background(), fsys, indexer.Options{Languages: []core.Language{language}, Workers: 4, Store: coldStore})
			require.NoError(t, err)
			cold := time.Since(coldStore.started)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cutStore := &firstReadStore{Store: memstore.New(), onFirstRead: func() { time.AfterFunc(cold/2, cancel) }}
			partial, err := runIndexer(t, ctx, fsys, indexer.Options{Languages: []core.Language{language}, Workers: 4, Store: cutStore})
			require.ErrorIs(t, err, context.Canceled)
			indexed, missing := count(partial.Files, fileindex.FileIndexed), count(partial.Files, fileindex.FileMissing)
			t.Logf("cold %v: %d indexed, %d missing at %v", cold, indexed, missing, cold/2)
			assert.Positive(t, indexed)
			assert.Positive(t, missing)
			assert.Equal(t, written, indexed+missing)
			assert.Len(t, partial.Indexes, indexed)

			for _, r := range search(partial) {
				if r.Status == reach.Found {
					continue
				}
				assert.Equal(t, reach.Uncertain, r.Status, r.Target)
				assert.True(t, slices.ContainsFunc(r.Uncertainties, func(u reach.Uncertainty) bool { return u.Kind == reach.Budget }), "budget cause for %s", r.Target)
			}
		})
	}
}

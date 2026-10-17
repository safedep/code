//go:build projectgraph

package projectgraph

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	codefs "github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/plugin/fileindex"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/safedep/code/projectgraph/store"
	"github.com/safedep/code/projectgraph/store/localdbstore"
	"github.com/safedep/dry/localdb"
	"github.com/stretchr/testify/require"
)

// benchFilesEnv lists the project sizes of the benchmark, as 1000,10000.
const benchFilesEnv = "PROJECTGRAPH_BENCH_FILES"

var benchTestPaths = []string{"tests", "test_*.py", "*.test.js", "*.test.ts", "*_test.go"}

func benchSizes(b *testing.B) []int {
	value := os.Getenv(benchFilesEnv)
	if value == "" {
		return []int{1000}
	}
	var sizes []int
	for _, field := range strings.Split(value, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(field))
		require.NoError(b, err, benchFilesEnv)
		sizes = append(sizes, n)
	}
	return sizes
}

// BenchmarkProjectGraph measures each step on a generated project per
// language and size. index-cold parses every file with no store.
// index-cold-store parses every file and puts each index in an empty store.
// index-warm finds every index in the store. link and reach run on the
// indexes of the warm store. Each step reports the peak heap of its run.
func BenchmarkProjectGraph(b *testing.B) {
	for _, code := range languageCodes {
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

	index := func(s store.Store) *fileindex.Result {
		indexed, err := (&fileindex.Indexer{}).Run(ctx, fsys, fileindex.Options{Languages: languages, Store: s})
		require.NoError(b, err)
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
	indexed := index(warm)
	link := func() *projectgraph.Graph {
		g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{
			Root:      os.DirFS(root),
			Files:     indexed.Files,
			TestPaths: benchTestPaths,
		})
		require.NoError(b, err)
		return g
	}
	g := link()
	targets := reachTargets(code)

	b.Run("index-cold", func(b *testing.B) {
		measure(b, written, func() { index(nil) })
	})
	b.Run("index-cold-store", func(b *testing.B) {
		measure(b, written, func() { index(newStore(b.TempDir())) })
	})
	b.Run("index-warm", func(b *testing.B) {
		measure(b, written, func() {
			r := index(warm)
			require.Equal(b, written, count(r.Files, fidx.FileCached))
		})
		b.ReportMetric(jsonKiB(b, indexed.Indexes)/float64(written), "index-KiB/file")
	})
	b.Run("link", func(b *testing.B) {
		measure(b, written, func() { link() })
		b.ReportMetric(float64(len(g.Edges)), "edges")
		sites, unresolved := callSites(g, indexed.Indexes)
		b.ReportMetric(float64(unresolved), "unresolved")
		b.ReportMetric(float64(unresolved)/float64(sites), "unresolved-rate")
		b.ReportMetric(float64(len(g.Roots)), "roots")
	})
	b.Run("reach", func(b *testing.B) {
		var results []reach.Result
		measure(b, written, func() {
			results, err = reach.Reach(ctx, g, reach.Query{Targets: targets})
			require.NoError(b, err)
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
}

// jsonKiB is the size of the JSON form of the indexes, as the store keeps
// them.
func jsonKiB(b *testing.B, indexes []*fidx.Index) float64 {
	size := 0
	for _, ix := range indexes {
		data, err := json.Marshal(ix)
		require.NoError(b, err)
		size += len(data)
	}
	return float64(size) / (1 << 10)
}

func count(files map[string]fidx.FileStatus, status fidx.FileStatus) int {
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
// that the link and the reach step read. A goroutine samples runtime.MemStats, because MemStats keeps
// no peak of HeapAlloc. The sample stops the world for a few microseconds,
// so it runs every 5 ms only.
func measure(b *testing.B, files int, op func()) {
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	peak := newPeakHeap()
	b.ResetTimer()
	start := time.Now()
	for range b.N {
		op()
	}
	elapsed := time.Since(start)
	b.StopTimer()
	b.ReportMetric(elapsed.Seconds()/float64(b.N)/float64(files)*1000, "s/1k-files")
	b.ReportMetric(float64(peak.stop())/(1<<20), "peak-heap-MiB")
	b.ReportMetric(float64(base.HeapAlloc)/(1<<20), "base-heap-MiB")
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

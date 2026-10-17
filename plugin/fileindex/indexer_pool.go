package fileindex

import (
	"context"
	"slices"
	"sync"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
)

// contentParser parses the bytes of a file that the run already read.
type contentParser interface {
	ParseContent(ctx context.Context, file core.File, data []byte) (core.ParseTree, error)
	Close()
}

// pool parses and indexes files on a fixed set of workers. Each worker owns
// a parser, because a sitter parser is not safe for concurrent use.
type pool struct {
	size int
	jobs chan job
	wg   sync.WaitGroup
}

// job is one file for a worker. The worker sends the outcome on out, which
// has room for one value, so a worker never waits for the caller.
type job struct {
	file pending
	out  chan outcome
}

// outcome is what a worker did with one file. An outcome with no tree and
// no error is a file that the worker did not start, because ctx was done.
type outcome struct {
	tree  core.ParseTree
	index *fidx.Index

	// indexErr is the failure of the index plugin, at the stage stage.
	indexErr error
	stage    core.Stage

	err error
}

func newPool(ctx context.Context, languages []core.Language, maxBytes, size int) (*pool, error) {
	parsers := make([]contentParser, 0, max(size, 1))
	for range cap(parsers) {
		p, err := parser.NewParser(languages, parser.WithMaxBytes(maxBytes))
		if err != nil {
			for _, p := range parsers {
				p.Close()
			}
			return nil, err
		}
		parsers = append(parsers, p)
	}

	pl := &pool{size: len(parsers), jobs: make(chan job)}
	for _, p := range parsers {
		pl.wg.Add(1)
		go func() {
			defer pl.wg.Done()
			defer p.Close()
			for j := range pl.jobs {
				j.out <- analyze(ctx, p, j.file)
			}
		}()
	}
	return pl, nil
}

// close stops the workers and frees their parsers.
func (pl *pool) close() {
	close(pl.jobs)
	pl.wg.Wait()
}

// each gives the files to the workers and calls fn with each outcome in the
// order of the files, on the goroutine of the caller. It keeps at most two
// files per worker in flight, which bounds the trees that wait for fn. It
// gives no new file to a worker after ctx is done, and it waits for each
// file that a worker took, so fn closes every tree.
func (pl *pool) each(ctx context.Context, files []pending, fn func(pending, outcome)) {
	window := 2 * pl.size
	jobs := make([]job, 0, len(files))
	for _, p := range files {
		if ctx.Err() != nil {
			break
		}
		if len(jobs) >= window {
			j := jobs[len(jobs)-window]
			fn(j.file, <-j.out)
		}
		j := job{file: p, out: make(chan outcome, 1)}
		jobs = append(jobs, j)
		pl.jobs <- j
	}
	for _, j := range jobs[max(0, len(jobs)-window):] {
		fn(j.file, <-j.out)
	}
}

// analyze parses one file and runs the index plugin on its tree.
func analyze(ctx context.Context, p contentParser, file pending) outcome {
	if ctx.Err() != nil {
		return outcome{}
	}
	tree, err := p.ParseContent(ctx, file.file, file.data)
	if err != nil {
		return outcome{err: err}
	}

	var index *fidx.Index
	indexPlugin := NewFileIndexPlugin(func(_ context.Context, ix *fidx.Index) error {
		index = ix
		return nil
	})
	if !slices.Contains(indexPlugin.SupportedLanguages(), file.language.Meta().Code) {
		return outcome{tree: tree}
	}
	err = plugin.Analyze(ctx, indexPlugin, file.file, tree)
	return outcome{tree: tree, index: index, indexErr: err, stage: core.Stage(indexPlugin.Name())}
}

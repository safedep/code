package fileindex

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"runtime"
	"slices"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
	"github.com/safedep/code/projectgraph/store"
)

const (
	// batchSize is the number of files of one round trip to the store.
	batchSize = 256

	// maxBatchBytes bounds the bytes that a batch holds, because the
	// indexer keeps the bytes of a file from the read to the parse.
	maxBatchBytes = 64 << 20
)

// Options configure one run of an Indexer.
type Options struct {
	// Languages are the languages to index. The run reads only the files
	// that one of them parses.
	Languages []core.Language

	// Store keeps the indexes between runs. A nil store parses every file.
	Store store.Store

	// Plugins run on the tree of each file, next to the index plugin. A
	// run with plugins parses every file, because a plugin needs the tree.
	Plugins []core.Plugin

	// MaxFileBytes is the size limit of a file. A larger file fails at the
	// parse. Zero selects parser.DefaultMaxBytes.
	MaxFileBytes int

	// Workers is the number of files that the run parses and indexes at
	// once. Zero selects runtime.GOMAXPROCS(0). The result does not depend
	// on it. The extra plugins run on one goroutine in path order, so a
	// plugin need not be safe for concurrent use.
	Workers int
}

// Result is what one run of an Indexer did.
type Result struct {
	// Indexes are the complete indexes of the run, in path order.
	Indexes []*fidx.Index

	// Files holds the status of each file of the run by name. A file that
	// no language parses has no status, and the report lists it as skipped.
	Files map[string]fidx.FileStatus

	// Report lists the files that the run parsed, skipped and failed.
	Report *plugin.Report
}

// Indexer indexes the files of a file system and keeps the indexes in a
// store. The zero value is ready to use.
type Indexer struct{}

// pending is a file that a run read and has not indexed yet.
type pending struct {
	file     core.File
	language core.Language
	data     []byte
	key      fidx.Key
}

// run is the state of one call of Run. Only the goroutine of Run changes
// the result. The workers send each outcome back to it.
type run struct {
	opts   Options
	result *Result
	pool   *pool
	strs   interner
}

// Run indexes the app files of fsys. It reads each file once and asks the
// store for the index of its bytes. A hit with no extra plugin needs no
// parse. Each new index goes to the store. A file that fails does not stop
// the run.
//
// When ctx is done, Run stops, marks each file that it did not index as
// missing, and returns the result so far with the error of ctx. Run fails
// when two files of fsys have one name, as the same relative path under
// two app roots, because the index keys a file by its name.
func (*Indexer) Run(ctx context.Context, fsys core.ImportAwareFileSystem, opts Options) (*Result, error) {
	if len(opts.Languages) == 0 {
		return nil, errors.New("no language to index")
	}
	byExtension, err := parser.LanguagesByExtension(opts.Languages)
	if err != nil {
		return nil, err
	}
	files, err := enumerate(ctx, fsys, opts.Languages)
	if err != nil {
		return nil, err
	}

	r := &run{
		opts:   opts,
		result: &Result{Files: make(map[string]fidx.FileStatus, len(files)), Report: &plugin.Report{}},
		strs:   interner{},
	}
	if r.pool, err = newPool(ctx, opts.Languages, r.maxBytes(), cmp.Or(opts.Workers, runtime.GOMAXPROCS(0))); err != nil {
		return nil, err
	}
	defer r.pool.close()
	err = r.index(ctx, files, byExtension)
	for _, file := range files {
		_, done := r.result.Files[file.Name()]
		if !done && !slices.Contains(r.result.Report.Skipped, file.Name()) {
			r.result.Files[file.Name()] = fidx.FileMissing
		}
	}
	slices.SortFunc(r.result.Indexes, func(a, b *fidx.Index) int { return cmp.Compare(a.Path, b.Path) })
	return r.result, err
}

// enumerate returns the app files that one of the languages parses. It
// fails at the second file of a name.
func enumerate(ctx context.Context, fsys core.ImportAwareFileSystem, languages []core.Language) ([]core.File, error) {
	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, languages)
	if err != nil {
		return nil, err
	}
	collector := &fileCollector{byName: map[string]core.File{}}
	if err := walker.Walk(ctx, fsys, collector); err != nil {
		return nil, err
	}
	return collector.files, nil
}

type fileCollector struct {
	files  []core.File
	byName map[string]core.File
}

func (c *fileCollector) VisitFile(_ context.Context, file core.File) error {
	if other, ok := c.byName[file.Name()]; ok {
		return fmt.Errorf("two files have the name %s, %s and %s: index one app root at a time", file.Name(), other.Path(), file.Path())
	}
	c.byName[file.Name()] = file
	c.files = append(c.files, file)
	return nil
}

// index reads the files in batches and indexes each batch.
func (r *run) index(ctx context.Context, files []core.File, byExtension map[string]core.Language) error {
	var batch []pending
	bytes := 0
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		language, ok := byExtension[path.Ext(file.Name())]
		if !ok {
			r.result.Report.Skipped = append(r.result.Report.Skipped, file.Name())
			continue
		}
		data, err := read(file, r.maxBytes())
		if err != nil {
			r.fail(ctx, file, core.StageRead, err)
			continue
		}

		batch = append(batch, pending{
			file:     file,
			language: language,
			data:     data,
			key:      fidx.NewKey(file.Name(), fidx.HashContent(data), language.Meta().Code),
		})
		bytes += len(data)
		if len(batch) == batchSize || bytes >= maxBatchBytes {
			if err := r.flush(ctx, batch); err != nil {
				return err
			}
			batch, bytes = nil, 0
		}
	}
	return r.flush(ctx, batch)
}

// read reads a file and closes it. It reads one byte over the limit at
// most, so the parse can report the file as too large.
func read(file core.File, limit int) ([]byte, error) {
	reader, err := file.Reader()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	return data, errors.Join(err, reader.Close())
}

func (r *run) maxBytes() int {
	return cmp.Or(r.opts.MaxFileBytes, parser.DefaultMaxBytes)
}

// flush serves the files of a batch from the store, parses the rest and
// puts their indexes.
func (r *run) flush(ctx context.Context, batch []pending) error {
	if len(batch) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	stored, err := r.stored(ctx, batch)
	if err != nil {
		return err
	}

	var parse []pending
	for _, p := range batch {
		if ix, ok := stored[p.key]; ok && len(r.opts.Plugins) == 0 {
			r.done(p.file, ix, fidx.FileCached)
			continue
		}
		parse = append(parse, p)
	}

	put, err := r.parse(ctx, parse)
	if err != nil {
		return err
	}
	if r.opts.Store == nil || len(put) == 0 {
		return nil
	}
	if err := r.opts.Store.Put(ctx, put); err != nil {
		return fmt.Errorf("failed to store file indexes: %w", err)
	}
	return nil
}

func (r *run) stored(ctx context.Context, batch []pending) (map[fidx.Key]*fidx.Index, error) {
	if r.opts.Store == nil {
		return nil, nil
	}
	keys := make([]fidx.Key, len(batch))
	for i, p := range batch {
		keys[i] = p.key
	}
	stored, err := r.opts.Store.Get(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to read file indexes: %w", err)
	}
	return stored, nil
}

// parse sends the files to the workers and takes the outcomes in the order
// of the files, so the report and the extra plugins see the files in the
// order of a run with one worker. It returns the new indexes in that order.
func (r *run) parse(ctx context.Context, files []pending) ([]*fidx.Index, error) {
	var put []*fidx.Index
	r.pool.each(ctx, files, func(p pending, o outcome) {
		if ix := r.collect(ctx, p, o); ix != nil {
			put = append(put, ix)
		}
	})
	return put, ctx.Err()
}

// collect records the outcome of one file, runs the extra plugins on its
// tree and closes the tree. It returns the index of the file, or nil when
// the file failed.
func (r *run) collect(ctx context.Context, p pending, o outcome) *fidx.Index {
	if o.err != nil {
		r.fail(ctx, p.file, core.StageParse, o.err)
		return nil
	}
	if o.tree == nil {
		return nil
	}
	defer o.tree.Close()

	report := r.result.Report
	report.Visited = append(report.Visited, p.file.Name())
	failed := o.indexErr != nil
	if failed {
		report.Failed = append(report.Failed, plugin.FileError{File: p.file.Name(), Stage: o.stage, Err: o.indexErr})
	}
	for _, pl := range r.opts.Plugins {
		if !slices.Contains(pl.SupportedLanguages(), p.language.Meta().Code) {
			continue
		}
		if err := plugin.Analyze(ctx, pl, p.file, o.tree); err != nil {
			report.Failed = append(report.Failed, plugin.FileError{File: p.file.Name(), Stage: core.Stage(pl.Name()), Err: err})
		}
	}

	if o.index == nil || failed {
		r.result.Files[p.file.Name()] = failure(ctx)
		return nil
	}
	r.done(p.file, o.index, fidx.FileIndexed)
	return o.index
}

func (r *run) done(file core.File, ix *fidx.Index, status fidx.FileStatus) {
	r.strs.index(ix)
	r.result.Files[file.Name()] = status
	r.result.Indexes = append(r.result.Indexes, ix)
}

func (r *run) fail(ctx context.Context, file core.File, stage core.Stage, err error) {
	r.result.Report.Failed = append(r.result.Report.Failed, plugin.FileError{File: file.Name(), Stage: stage, Err: err})
	r.result.Files[file.Name()] = failure(ctx)
}

// failure is the status of a file that failed. A failure after ctx is done
// is a stop, so the file is missing.
func failure(ctx context.Context) fidx.FileStatus {
	if ctx.Err() != nil {
		return fidx.FileMissing
	}
	return fidx.FileFailed
}

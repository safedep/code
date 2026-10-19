package indexer

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"runtime"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
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
	// plugin needs the tree, so a run with a plugin parses every file of
	// the languages of the plugin, unless the plugin is a CachedPlugin and
	// the store holds its artifact for the file.
	Plugins []core.Plugin

	// MaxFileBytes is the size limit of a file. A larger file fails at the
	// parse. Zero selects parser.DefaultMaxBytes.
	MaxFileBytes int

	// Workers is the number of files that the run parses and indexes at
	// once. Zero selects runtime.GOMAXPROCS(0). The result does not depend
	// on it. The extra plugins run on one goroutine in path order, so a
	// plugin need not be safe for concurrent use.
	Workers int

	// Imports indexes the files of the import directories of the file
	// system too. The run indexes only an import file whose name holds a
	// package unit, as fs.LocalFileSystemConfig.ImportUnits gives it,
	// because a key of dependency code needs its unit. The report lists
	// any other import file as skipped.
	Imports bool
}

// Result is what one run of an Indexer did.
type Result struct {
	// Indexes are the complete indexes of the run, in path order.
	Indexes []*fileindex.Index

	// Files holds the status of each file of the project by name. A file
	// that no language parses has no status, and the report lists it as
	// skipped.
	Files map[string]fileindex.FileStatus

	// Dependencies holds the status of each file of dependency code by
	// name, as Files does for the project. A failed or a missing file of a
	// package makes only the paths through that package uncertain, so the
	// two sets stay apart.
	Dependencies map[string]fileindex.FileStatus

	// Report lists the files that the run parsed, skipped and failed.
	Report *plugin.Report

	// Excluded lists the Go files of dependency code that no Go build
	// reads by default, in walk order: test files, files with //go:build
	// ignore and files that need a tag the go command never sets. They have
	// no status, as no program runs their code.
	Excluded []string
}

// Statuses returns the status of each file of the run, of the project and
// of dependency code, as projectgraph.Options.Files takes them.
func (r *Result) Statuses() map[string]fileindex.FileStatus {
	out := make(map[string]fileindex.FileStatus, len(r.Files)+len(r.Dependencies))
	maps.Copy(out, r.Files)
	maps.Copy(out, r.Dependencies)
	return out
}

// Unit returns the indexes and the file statuses of one unit of dependency
// code, as summary.Summarize takes them.
func (r *Result) Unit(unit string) ([]*fileindex.Index, map[string]fileindex.FileStatus) {
	var indexes []*fileindex.Index
	for _, ix := range r.Indexes {
		if ix.Unit == unit {
			indexes = append(indexes, ix)
		}
	}
	files := map[string]fileindex.FileStatus{}
	for p, status := range r.Dependencies {
		if fileindex.PackageOf(p) == unit {
			files[p] = status
		}
	}
	return indexes, files
}

// statuses returns the map of the status of a file of the name.
func (r *Result) statuses(name string) map[string]fileindex.FileStatus {
	if unit, _ := fileindex.SplitUnit(name); unit != "" {
		return r.Dependencies
	}
	return r.Files
}

// Indexer indexes the files of a file system and keeps the indexes in a
// store. New makes one from its Options, and Run indexes one file system.
type Indexer struct {
	opts        Options
	byExtension map[string]core.Language
}

// New validates the options and returns an Indexer that holds what every
// run derives from them: the language of each file extension, and the
// defaults of MaxFileBytes and Workers.
func New(opts Options) (*Indexer, error) {
	switch {
	case len(opts.Languages) == 0:
		return nil, errors.New("no language to index")
	case opts.MaxFileBytes < 0:
		return nil, fmt.Errorf("max file bytes %d is negative", opts.MaxFileBytes)
	case opts.Workers < 0:
		return nil, fmt.Errorf("workers %d is negative", opts.Workers)
	}
	byExtension, err := parser.LanguagesByExtension(opts.Languages)
	if err != nil {
		return nil, err
	}
	opts.Languages = slices.Clone(opts.Languages)
	opts.Plugins = slices.Clone(opts.Plugins)
	opts.MaxFileBytes = cmp.Or(opts.MaxFileBytes, parser.DefaultMaxBytes)
	opts.Workers = cmp.Or(opts.Workers, runtime.GOMAXPROCS(0))
	return &Indexer{opts: opts, byExtension: byExtension}, nil
}

// pending is a file that a run read and has not indexed yet.
type pending struct {
	file     core.File
	language core.Language
	data     []byte
	key      fileindex.Key

	// restored names the cached plugins that took their stored artifact
	// before another plugin rejected its own. They skip the tree.
	restored map[string]bool
}

// run is the state of one call of Run. Only the goroutine of Run changes
// the result. The workers send each outcome back to it. opts are the
// options of the Indexer, with their defaults filled.
type run struct {
	opts   Options
	result *Result
	pool   *pool
	strs   interner
	// excluded holds the names of Result.Excluded.
	excluded map[string]bool
}

// Run indexes the app files of fsys, and its import files with
// Options.Imports. An Indexer can run on several file systems, one at a
// time. It reads each file once and asks the store for the index
// of its bytes and for the artifacts of the cached plugins. A hit of all of
// them needs no parse. Each new index and artifact goes to the store. A
// file that fails does not stop the run.
//
// The extra plugins run on the goroutine of Run. In a batch, the files
// that the store serves come first, in path order, and then the parsed
// files, in path order.
//
// When ctx is done, Run stops, marks each file that it did not index as
// missing, and returns the result so far with the error of ctx. Run fails
// when two files of fsys have one name, as the same relative path under
// two app roots, because the index keys a file by its name.
func (ix *Indexer) Run(ctx context.Context, fsys core.ImportAwareFileSystem) (*Result, error) {
	files, skipped, err := enumerate(ctx, fsys, ix.opts)
	if err != nil {
		return nil, err
	}

	r := &run{
		opts:     ix.opts,
		excluded: map[string]bool{},
		result: &Result{
			Files:        make(map[string]fileindex.FileStatus, len(files)),
			Dependencies: map[string]fileindex.FileStatus{},
			Report:       &plugin.Report{Skipped: skipped},
		},
		strs: interner{},
	}
	if r.pool, err = newPool(ctx, ix.opts.Languages, ix.opts.MaxFileBytes, ix.opts.Workers); err != nil {
		return nil, err
	}
	defer r.pool.close()
	err = r.index(ctx, files, ix.byExtension)
	for _, file := range files {
		statuses := r.result.statuses(file.Name())
		_, done := statuses[file.Name()]
		if !done && !r.excluded[file.Name()] && !slices.Contains(r.result.Report.Skipped, file.Name()) {
			statuses[file.Name()] = fileindex.FileMissing
		}
	}
	slices.SortFunc(r.result.Indexes, func(a, b *fileindex.Index) int { return cmp.Compare(a.Path, b.Path) })
	return r.result, err
}

// enumerate returns the files that one of the languages parses, and the
// import files with no unit. It fails at the second file of a name.
func enumerate(ctx context.Context, fsys core.ImportAwareFileSystem, opts Options) ([]core.File, []string, error) {
	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{IncludeImports: opts.Imports}, opts.Languages)
	if err != nil {
		return nil, nil, err
	}
	collector := &fileCollector{byName: map[string]core.File{}}
	if err := walker.Walk(ctx, fsys, collector); err != nil {
		return nil, nil, err
	}
	return collector.files, collector.noUnit, nil
}

type fileCollector struct {
	files  []core.File
	byName map[string]core.File
	noUnit []string
}

func (c *fileCollector) VisitFile(_ context.Context, file core.File) error {
	if unit, _ := fileindex.SplitUnit(file.Name()); file.IsImport() && unit == "" {
		c.noUnit = append(c.noUnit, file.Name())
		return nil
	}
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
		data, err := read(file, r.opts.MaxFileBytes)
		if err != nil {
			r.fail(ctx, file, core.StageRead, err)
			continue
		}
		if goExcluded(file, data) {
			r.excluded[file.Name()] = true
			r.result.Excluded = append(r.result.Excluded, file.Name())
			continue
		}

		batch = append(batch, pending{
			file:     file,
			language: language,
			data:     data,
			key:      fileindex.NewKey(file.Name(), fileindex.HashContent(data), language.Meta().Code),
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

// flush serves the files of a batch from the store, parses the rest and
// puts their indexes.
func (r *run) flush(ctx context.Context, batch []pending) error {
	if len(batch) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	stored, artifacts, err := r.stored(ctx, batch)
	if err != nil {
		return err
	}

	var parse []pending
	for _, p := range batch {
		if ix, ok := stored[p.key]; ok && !r.needsParse(p, artifacts) {
			restored, ok := r.restore(ctx, p, artifacts)
			if ok {
				r.done(p.file, ix, fileindex.FileCached)
				continue
			}
			p.restored = restored
		}
		parse = append(parse, p)
	}

	put, made, err := r.parse(ctx, parse)
	if err != nil {
		return err
	}
	if r.opts.Store == nil {
		return nil
	}
	if len(put) > 0 {
		if err := r.opts.Store.Put(ctx, put); err != nil {
			return fmt.Errorf("failed to store file indexes: %w", err)
		}
	}
	if len(made) > 0 {
		if err := r.opts.Store.PutArtifacts(ctx, made); err != nil {
			return fmt.Errorf("failed to store plugin artifacts: %w", err)
		}
	}
	return nil
}

// stored asks the store for the index of each file of the batch and for
// the artifact of each cached plugin of the file.
func (r *run) stored(ctx context.Context, batch []pending) (map[fileindex.Key]*fileindex.Index, map[store.ArtifactKey][]byte, error) {
	if r.opts.Store == nil {
		return nil, nil, nil
	}
	keys := make([]fileindex.Key, len(batch))
	var artifactKeys []store.ArtifactKey
	for i, p := range batch {
		keys[i] = p.key
		for _, pl := range r.opts.Plugins {
			if cached, ok := pl.(CachedPlugin); ok && supports(pl, p) {
				artifactKeys = append(artifactKeys, artifactKey(p, cached))
			}
		}
	}
	stored, err := r.opts.Store.Get(ctx, keys)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file indexes: %w", err)
	}
	var artifacts map[store.ArtifactKey][]byte
	if len(artifactKeys) > 0 {
		if artifacts, err = r.opts.Store.GetArtifacts(ctx, artifactKeys); err != nil {
			return nil, nil, fmt.Errorf("failed to read plugin artifacts: %w", err)
		}
	}
	return stored, artifacts, nil
}

// parse sends the files to the workers and takes the outcomes in the order
// of the files, so the report and the extra plugins see the files in the
// order of a run with one worker. It returns the new indexes and the new
// artifacts in that order.
func (r *run) parse(ctx context.Context, files []pending) ([]*fileindex.Index, []store.Artifact, error) {
	var put []*fileindex.Index
	var made []store.Artifact
	r.pool.each(ctx, files, func(p pending, o outcome) {
		ix, artifacts := r.collect(ctx, p, o)
		if ix != nil {
			put = append(put, ix)
		}
		made = append(made, artifacts...)
	})
	return put, made, ctx.Err()
}

// collect records the outcome of one file, runs the extra plugins on its
// tree and closes the tree. It returns the index of the file, or nil when
// the file failed, and the artifacts of the cached plugins.
func (r *run) collect(ctx context.Context, p pending, o outcome) (*fileindex.Index, []store.Artifact) {
	if o.err != nil {
		r.fail(ctx, p.file, core.StageParse, o.err)
		return nil, nil
	}
	if o.tree == nil {
		return nil, nil
	}
	defer o.tree.Close()

	report := r.result.Report
	report.Visited = append(report.Visited, p.file.Name())
	failed := o.indexErr != nil
	if failed {
		report.Failed = append(report.Failed, plugin.FileError{File: p.file.Name(), Stage: o.stage, Err: o.indexErr})
	}
	var artifacts []store.Artifact
	for _, pl := range r.opts.Plugins {
		if !supports(pl, p) {
			continue
		}
		if cached, ok := pl.(CachedPlugin); ok {
			if p.restored[pl.Name()] {
				continue
			}
			data, err := cached.Analyze(ctx, p.file, o.tree)
			if err != nil {
				report.Failed = append(report.Failed, failure(p.file, pl, err))
			} else {
				artifacts = append(artifacts, store.Artifact{Key: artifactKey(p, cached), Data: data})
			}
			continue
		}
		if err := plugin.Analyze(ctx, pl, p.file, o.tree); err != nil {
			report.Failed = append(report.Failed, failure(p.file, pl, err))
		}
	}

	if o.index == nil || failed {
		r.result.statuses(p.file.Name())[p.file.Name()] = status(ctx)
		return nil, nil
	}
	r.done(p.file, o.index, fileindex.FileIndexed)
	return o.index, artifacts
}

// failure is the report entry of a plugin that failed on a file.
func failure(file core.File, pl core.Plugin, err error) plugin.FileError {
	return plugin.FileError{File: file.Name(), Stage: core.Stage(pl.Name()), Err: err}
}

func (r *run) done(file core.File, ix *fileindex.Index, status fileindex.FileStatus) {
	r.strs.index(ix)
	r.result.statuses(file.Name())[file.Name()] = status
	r.result.Indexes = append(r.result.Indexes, ix)
}

func (r *run) fail(ctx context.Context, file core.File, stage core.Stage, err error) {
	r.result.Report.Failed = append(r.result.Report.Failed, plugin.FileError{File: file.Name(), Stage: stage, Err: err})
	r.result.statuses(file.Name())[file.Name()] = status(ctx)
}

// status is the status of a file that failed. A failure after ctx is done
// is a stop, so the file is missing.
func status(ctx context.Context) fileindex.FileStatus {
	if ctx.Err() != nil {
		return fileindex.FileMissing
	}
	return fileindex.FileFailed
}

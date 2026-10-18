// Command projectgraph builds the call graph of a directory and prints the
// path from a root to each target, as main.py:3 -> handlers.py:12 ->
// yaml//load. It keeps the file indexes in a localdb file in the -store
// directory, so a second run parses only the files that changed. With
// -overlay it merges a file of llm edges into the graph before the query.
// Each -source names an entry point of the project that the root rules do
// not mark, and -sources-only searches from the -source entry points
// alone.
//
// Each -deps dir=unit, as -deps node_modules/lodash=npm:lodash@4.17.21,
// adds the code of a dependency to the graph, so a path goes on into the
// package. A -deps directory under an -import-dir, as
// -import-dir venv/site-packages -deps venv/site-packages/yaml=pypi:PyYAML@6.0.1,
// keeps its path under the import directory, as yaml/__init__.py. Any
// other -deps directory is an import directory of its own, and a pypi
// package given that way is its package. With -summary it prints the
// summary of each such unit as JSON.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	iofs "io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/fileindex/store/localdbstore"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/plugin/indexer"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/overlay"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/safedep/code/projectgraph/summary"
	"github.com/safedep/dry/localdb"
)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// targetList is the -target flags. A bad key fails at the flag, so the
// run never starts with a target that the query rejects.
type targetList []reach.Target

func (l *targetList) String() string {
	parts := make([]string, 0, len(*l))
	for _, t := range *l {
		parts = append(parts, t.Key())
	}
	return strings.Join(parts, ",")
}

func (l *targetList) Set(value string) error {
	t, err := reach.TargetByKey(value)
	if err != nil {
		return err
	}
	*l = append(*l, t)
	return nil
}

// sourceList is the -source flags. A value with // is a key, as
// app/server.py//main. Any other value is a file path and a dotted name
// after a colon, as app/server.py:main or src/api.ts:Router.handle, or a
// file path alone for its module top level.
type sourceList []reach.Source

func (l *sourceList) String() string {
	parts := make([]string, 0, len(*l))
	for _, s := range *l {
		parts = append(parts, s.Key())
	}
	return strings.Join(parts, ",")
}

func (l *sourceList) Set(value string) error {
	var s reach.Source
	var err error
	if strings.Contains(value, fileindex.Separator) {
		s, err = reach.SourceByKey(value)
	} else {
		path, name, _ := strings.Cut(value, ":")
		var names []string
		if name != "" {
			names = strings.Split(name, ".")
		}
		s, err = reach.SourceInFile(path, names...)
	}
	if err != nil {
		return err
	}
	*l = append(*l, s)
	return nil
}

// depList is the -deps flags: the unit of each directory of dependency
// code.
type depList map[string]string

func (d depList) String() string {
	var parts []string
	for _, dir := range slices.Sorted(maps.Keys(d)) {
		parts = append(parts, dir+"="+d[dir])
	}
	return strings.Join(parts, ",")
}

func (d depList) Set(value string) error {
	dir, unit, ok := strings.Cut(value, "=")
	if !ok || dir == "" || unit == "" {
		return fmt.Errorf("%q is not dir=unit", value)
	}
	d[dir] = unit
	return nil
}

type config struct {
	dir          string
	store        string
	overlay      string
	targets      targetList
	sources      sourceList
	sourcesOnly  bool
	importDirs   stringList
	deps         depList
	summary      bool
	includeTests bool
	json         bool
	workers      int
}

func main() {
	cfg := config{deps: depList{}}
	flag.StringVar(&cfg.dir, "dir", "", "directory of the project")
	flag.StringVar(&cfg.store, "store", "", "directory of the localdb file that keeps the file indexes")
	flag.StringVar(&cfg.overlay, "overlay", "", "JSON file of llm edges to merge into the graph: edges that a model proposed for unresolved call sites, as the Overlay section of docs/projectgraph.md describes")
	flag.Var(&cfg.targets, "target", "external symbol to reach, as yaml//load (repeatable)")
	flag.Var(&cfg.sources, "source", "entry point of the project to search from, as app/server.py:main, src/api.ts:Router.handle or a key as app/server.py//main (repeatable)")
	flag.BoolVar(&cfg.sourcesOnly, "sources-only", false, "search from the -source entry points alone, not from the roots that the rules mark")
	flag.Var(cfg.deps, "deps", "directory of a dependency and its unit, as node_modules/lodash=npm:lodash@4.17.21 (repeatable)")
	flag.Var(&cfg.importDirs, "import-dir", "directory that holds -deps directories, as venv/site-packages (repeatable)")
	flag.BoolVar(&cfg.summary, "summary", false, "print the summary of each unit of -deps as JSON")
	flag.BoolVar(&cfg.includeTests, "include-tests", false, "count the test files as roots")
	flag.BoolVar(&cfg.json, "json", false, "print the reach results as JSON")
	flag.IntVar(&cfg.workers, "workers", runtime.NumCPU(), "number of files to parse at once")
	flag.Parse()

	if cfg.dir == "" || len(cfg.targets) == 0 && !cfg.summary {
		fmt.Fprintln(os.Stderr, "projectgraph needs -dir, and -target or -summary")
		flag.Usage()
		os.Exit(2)
	}
	if cfg.sourcesOnly && len(cfg.sources) == 0 {
		fmt.Fprintln(os.Stderr, "projectgraph needs a -source with -sources-only")
		flag.Usage()
		os.Exit(2)
	}
	if err := run(context.Background(), cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config, out io.Writer) (err error) {
	root, err := filepath.Abs(cfg.dir)
	if err != nil {
		return err
	}
	fsCfg := fs.LocalFileSystemConfig{AppDirectories: []string{root}, ImportUnits: map[string]string{}, ExcludePatterns: fs.DefaultSkipPatterns(root)}
	var importDirs []string
	for _, dir := range cfg.importDirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		importDirs = append(importDirs, abs)
	}
	units := map[string]iofs.FS{}
	for dir, unit := range cfg.deps {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		importDir := abs
		if i := slices.IndexFunc(importDirs, func(d string) bool { return within(d, abs) }); i >= 0 {
			importDir = importDirs[i]
		} else {
			importDirs = append(importDirs, abs)
		}
		fsCfg.ImportUnits[abs] = unit
		units[unit] = os.DirFS(importDir)
	}
	for _, dir := range importDirs {
		fsCfg.ImportDirectories = append(fsCfg.ImportDirectories, dir)
		// The patterns of root do not apply to the walk of a dependency,
		// so the code that the dependency installs below it stays out.
		fsCfg.ExcludePatterns = append(fsCfg.ExcludePatterns, fs.SkipPattern(dir))
	}
	slices.Sort(fsCfg.ImportDirectories)
	fsys, err := fs.NewLocalFileSystem(fsCfg)
	if err != nil {
		return err
	}

	languages, err := lang.AllLanguages()
	if err != nil {
		return err
	}

	var s store.Store
	if cfg.store != "" {
		mgr := localdb.New(localdb.Config{Dir: cfg.store})
		defer func() { err = errors.Join(err, mgr.Close()) }()
		if s, err = localdbstore.New(ctx, mgr); err != nil {
			return err
		}
	}

	ix, err := indexer.New(indexer.Options{Languages: languages, Store: s, Workers: cfg.workers, Imports: len(cfg.deps) > 0})
	if err != nil {
		return err
	}
	indexed, err := ix.Run(ctx, fsys)
	if err != nil {
		return fmt.Errorf("failed to index %s: %w", root, err)
	}
	if err := warnIndex(os.Stderr, indexed); err != nil {
		return err
	}
	if len(cfg.targets) > 0 {
		if err := printReach(ctx, cfg, out, root, indexed, units); err != nil {
			return err
		}
	}
	if cfg.summary {
		return printSummaries(ctx, out, indexed, units)
	}
	return nil
}

// printReach links the project with its dependencies and writes the result
// of each target.
func printReach(ctx context.Context, cfg config, out io.Writer, root string, indexed *indexer.Result, units map[string]iofs.FS) error {
	g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{
		Root:      os.DirFS(root),
		Files:     indexed.Statuses(),
		TestPaths: fileindex.DefaultTestPaths,
		Units:     units,
	})
	if err != nil {
		return fmt.Errorf("failed to link %s: %w", root, err)
	}
	if err := warnGraph(os.Stderr, g, cfg.sourcesOnly); err != nil {
		return err
	}
	if cfg.overlay != "" {
		if err := mergeOverlay(g, cfg.overlay, os.Stderr); err != nil {
			return err
		}
	}
	q := reach.Query{Targets: cfg.targets, Sources: cfg.sources, IncludeTests: cfg.includeTests}
	if cfg.sourcesOnly {
		q.SourceMode = reach.SourcesOnly
	}
	results, err := reach.Reach(ctx, g, q)
	if err != nil {
		return err
	}

	if cfg.json {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	}
	for _, r := range results {
		if r.Searched.Stopped {
			if _, err := fmt.Fprintf(os.Stderr, "the search for %s stopped at the deadline\n", r.Target); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out, r); err != nil {
			return err
		}
	}
	return nil
}

// printSummaries writes the summary of each unit as one JSON array, in the
// order of the units. Each unit gets one index and one link of its own
// files.
func printSummaries(ctx context.Context, out io.Writer, indexed *indexer.Result, units map[string]iofs.FS) error {
	summaries := []*summary.Summary{}
	for _, unit := range slices.Sorted(maps.Keys(units)) {
		indexes, files := indexed.Unit(unit)
		if len(indexes) == 0 && len(files) == 0 {
			return fmt.Errorf("the unit %s has no file to summarize", unit)
		}
		s, err := summary.Summarize(ctx, indexes, projectgraph.Options{Files: files, Units: map[string]iofs.FS{unit: units[unit]}})
		if err != nil {
			return fmt.Errorf("failed to summarize %s: %w", unit, err)
		}
		summaries = append(summaries, s)
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summaries)
}

// warnIndex writes the files that the index step failed or skipped to warn.
func warnIndex(warn io.Writer, indexed *indexer.Result) error {
	for _, f := range indexed.Report.Failed {
		if _, err := fmt.Fprintln(warn, "failed:", f); err != nil {
			return err
		}
	}
	if n := len(indexed.Report.Skipped); n > 0 {
		if _, err := fmt.Fprintf(warn, "skipped %d files that no language parses\n", n); err != nil {
			return err
		}
	}
	return nil
}

// warnGraph writes the problems and the truncations of the link step to
// warn, and says what to do when no root rule matched. A search from the
// sources alone does not use the fallback roots, so it gets no such advice.
func warnGraph(warn io.Writer, g *projectgraph.Graph, sourcesOnly bool) error {
	for _, p := range g.Problems {
		if _, err := fmt.Fprintf(warn, "problem: %s: %s\n", p.Path, p.Text); err != nil {
			return err
		}
	}
	for _, t := range g.Truncated {
		if _, err := fmt.Fprintf(warn, "truncated: the link step hit its %s limit at %s %s\n", t.Kind, t.Key, t.Site); err != nil {
			return err
		}
	}
	if g.Stats.RootFallback && !sourcesOnly {
		if _, err := fmt.Fprintln(warn, "no root rule matched: every exported symbol is a root. Add a manifest entry, as a package.json bin or a pyproject.toml script, or a main guard, or pass -include-tests"); err != nil {
			return err
		}
	}
	return nil
}

// mergeOverlay merges the overlay at path into g and writes each edge that
// the merge drops to warn.
func mergeOverlay(g *projectgraph.Graph, path string, warn io.Writer) error {
	f, err := overlay.ReadFile(path)
	if err != nil {
		return err
	}
	report, err := overlay.Merge(g, f)
	if err != nil {
		return fmt.Errorf("failed to merge %s: %w", path, err)
	}
	for _, d := range report.Diagnostics {
		if _, err := fmt.Fprintln(warn, d); err != nil {
			return err
		}
	}
	return nil
}

// within reports whether path is dir or a path below it.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

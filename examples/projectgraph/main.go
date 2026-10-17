// Command projectgraph builds the call graph of a directory and prints the
// path from a root to each target, as main.py:3 -> handlers.py:12 ->
// yaml//load. It keeps the file indexes in a localdb file in the -store
// directory, so a second run parses only the files that changed. With
// -overlay it merges a file of llm edges into the graph before the query.
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
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/plugin/fileindex"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/overlay"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/safedep/code/projectgraph/store"
	"github.com/safedep/code/projectgraph/store/localdbstore"
	"github.com/safedep/code/projectgraph/summary"
	"github.com/safedep/dry/localdb"
)

// installedDirs hold installed code or caches at any depth.
var installedDirs = []string{
	"node_modules", "bower_components", ".git", ".hg", ".svn", "venv", ".venv", ".tox", ".nox",
	"__pycache__", ".mypy_cache", ".pytest_cache", "site-packages", ".next", ".nuxt",
}

// rootDirs hold vendored or built code at the project root. A package of the
// project can have one of these names below the root, as src/build, and its
// files are first-party.
var rootDirs = []string{"vendor", "third_party", "env", "dist", "build", "out", "target", "coverage"}

// testPaths mark the test files. A test file gives roots only with
// -include-tests.
var testPaths = []string{
	"test", "tests", "__tests__", "spec", "test_*.py", "*_test.py", "conftest.py",
	"*.test.js", "*.spec.js", "*.test.ts", "*.spec.ts", "*.test.tsx", "*.spec.tsx", "*_test.go", "*Test.java", "*Tests.java", "*Test.kt", "*Tests.kt",
	"*Test.cs", "*Tests.cs", "*Test.php", "*_test.rb", "*_spec.rb", "benches",
}

var languageCodes = []core.LanguageCode{core.LanguageCodePython, core.LanguageCodeJavascript, core.LanguageCodeTypescript, core.LanguageCodeGo, core.LanguageCodeJava, core.LanguageCodeKotlin, core.LanguageCodeCSharp, core.LanguageCodePHP, core.LanguageCodeRuby, core.LanguageCodeRust}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
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
	targets      stringList
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
	flag.StringVar(&cfg.overlay, "overlay", "", "JSON file of llm edges to merge into the graph")
	flag.Var(&cfg.targets, "target", "external symbol to reach, as yaml//load (repeatable)")
	flag.Var(cfg.deps, "deps", "directory of a dependency and its unit, as node_modules/lodash=npm:lodash@4.17.21 (repeatable)")
	flag.Var(&cfg.importDirs, "import-dir", "directory that holds -deps directories, as venv/site-packages (repeatable)")
	flag.BoolVar(&cfg.summary, "summary", false, "print the summary of each unit of -deps as JSON")
	flag.BoolVar(&cfg.includeTests, "include-tests", false, "count the test files as roots")
	flag.BoolVar(&cfg.json, "json", false, "print the reach results as JSON")
	flag.IntVar(&cfg.workers, "workers", runtime.NumCPU(), "number of files to parse at once")
	flag.Parse()

	if cfg.dir == "" || len(cfg.targets) == 0 && !cfg.summary {
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
	fsCfg := fs.LocalFileSystemConfig{AppDirectories: []string{root}, ImportUnits: map[string]string{}, ExcludePatterns: skipPatterns(root)}
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
		fsCfg.ExcludePatterns = append(fsCfg.ExcludePatterns, installedPattern(dir))
	}
	slices.Sort(fsCfg.ImportDirectories)
	fsys, err := fs.NewLocalFileSystem(fsCfg)
	if err != nil {
		return err
	}

	var languages []core.Language
	for _, code := range languageCodes {
		language, err := lang.GetLanguage(string(code))
		if err != nil {
			return err
		}
		languages = append(languages, language)
	}

	var s store.Store
	if cfg.store != "" {
		mgr := localdb.New(localdb.Config{Dir: cfg.store})
		defer func() { err = errors.Join(err, mgr.Close()) }()
		if s, err = localdbstore.New(ctx, mgr); err != nil {
			return err
		}
	}

	indexed, err := (&fileindex.Indexer{}).Run(ctx, fsys, fileindex.Options{Languages: languages, Store: s, Workers: cfg.workers, Imports: len(cfg.deps) > 0})
	if err != nil {
		return fmt.Errorf("failed to index %s: %w", root, err)
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
func printReach(ctx context.Context, cfg config, out io.Writer, root string, indexed *fileindex.Result, units map[string]iofs.FS) error {
	g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{
		Root:      os.DirFS(root),
		Files:     indexed.Statuses(),
		TestPaths: testPaths,
		Units:     units,
	})
	if err != nil {
		return fmt.Errorf("failed to link %s: %w", root, err)
	}
	if cfg.overlay != "" {
		if err := mergeOverlay(g, cfg.overlay, os.Stderr); err != nil {
			return err
		}
	}
	results, err := reach.Reach(ctx, g, reach.Query{Targets: cfg.targets, IncludeTests: cfg.includeTests})
	if err != nil {
		return err
	}

	if cfg.json {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	}
	for _, r := range results {
		if err := printResult(out, r); err != nil {
			return err
		}
	}
	return nil
}

// printSummaries writes the summary of each unit as one JSON array, in the
// order of the units. Each unit gets one index and one link of its own
// files.
func printSummaries(ctx context.Context, out io.Writer, indexed *fileindex.Result, units map[string]iofs.FS) error {
	summaries := []*summary.Summary{}
	for _, unit := range slices.Sorted(maps.Keys(units)) {
		var indexes []*fidx.Index
		for _, ix := range indexed.Indexes {
			if ix.Unit == unit {
				indexes = append(indexes, ix)
			}
		}
		files := map[string]fidx.FileStatus{}
		for p, status := range indexed.Dependencies {
			if fidx.PackageOf(p) == unit {
				files[p] = status
			}
		}
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

// sep matches a path separator of any platform.
const sep = `[\\/]`

// skipPatterns match the installed directories at any depth below root and
// the vendored or built directories directly under root. A root under a
// directory named build keeps its code.
func skipPatterns(root string) []*regexp.Regexp {
	return []*regexp.Regexp{
		installedPattern(root),
		regexp.MustCompile(`^` + regexp.QuoteMeta(root) + sep + alternation(rootDirs) + `(?:` + sep + `|$)`),
	}
}

// installedPattern matches the installed directories at any depth below dir.
func installedPattern(dir string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(dir) + sep + `(?:.*` + sep + `)?` + alternation(installedDirs) + `(?:` + sep + `|$)`)
}

func alternation(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = regexp.QuoteMeta(name)
	}
	return `(?:` + strings.Join(quoted, "|") + `)`
}

func printResult(out io.Writer, r reach.Result) error {
	lines := []string{fmt.Sprintf("%s: %s", r.Target, r.Status)}
	if len(r.Path) > 0 {
		lines = append(lines, "  "+pathText(r.Path))
	}
	if len(r.TestPath) > 0 {
		lines = append(lines, "  test only: "+pathText(r.TestPath))
	}
	for _, u := range r.Uncertainties {
		lines = append(lines, fmt.Sprintf("  %s: %s %s", u.Kind, siteText(u.Site), u.Text))
	}
	if r.Status != reach.Found {
		for _, c := range r.CallSites {
			if !c.Reached {
				lines = append(lines, fmt.Sprintf("  call at %s in %s, which no root reaches", siteText(c.Site), c.Caller))
			}
		}
		lines = append(lines, fmt.Sprintf("  searched %d roots, %d nodes, %d edges", len(r.Searched.Roots), r.Searched.Nodes, r.Searched.Edges))
	}
	_, err := fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}

// pathText writes a path as the call site of each hop and then the target,
// as main.py:3 -> handlers.py:12 -> yaml//load. A hop with low confidence
// names its edge kind, as main.py:3 (name), so a reader sees the weak hop.
// An llm hop also names its model and its prompt. A package hop names its
// kind at the definition in the package, so a reader sees where the path
// enters dependency code.
func pathText(hops []reach.Hop) string {
	parts := make([]string, 0, len(hops))
	for _, h := range hops[1:] {
		part := siteText(h.Site)
		switch {
		case h.EdgeKind == projectgraph.EdgeLLM:
			part += " (" + string(h.EdgeKind) + ", " + h.Resolver + ")"
		case h.Confidence == projectgraph.ConfidenceLow || h.EdgeKind == projectgraph.EdgePackage:
			part += " (" + string(h.EdgeKind) + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(append(parts, hops[len(hops)-1].Key), " -> ")
}

func siteText(s projectgraph.Site) string {
	if s.Line == 0 {
		return s.Path
	}
	return fmt.Sprintf("%s:%d", s.Path, s.Line)
}

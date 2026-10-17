// Command projectgraph builds the call graph of a directory and prints the
// path from a root to each target, as main.py:3 -> handlers.py:12 ->
// yaml//load. It keeps the file indexes in a localdb file in the -store
// directory, so a second run parses only the files that changed.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/plugin/fileindex"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/safedep/code/projectgraph/store"
	"github.com/safedep/code/projectgraph/store/localdbstore"
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
	"*.test.js", "*.spec.js", "*.test.ts", "*.spec.ts", "*.test.tsx", "*.spec.tsx",
}

var languageCodes = []core.LanguageCode{core.LanguageCodePython, core.LanguageCodeJavascript, core.LanguageCodeTypescript}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type config struct {
	dir          string
	store        string
	targets      stringList
	includeTests bool
	json         bool
}

func main() {
	var cfg config
	flag.StringVar(&cfg.dir, "dir", "", "directory of the project")
	flag.StringVar(&cfg.store, "store", "", "directory of the localdb file that keeps the file indexes")
	flag.Var(&cfg.targets, "target", "external symbol to reach, as yaml//load (repeatable)")
	flag.BoolVar(&cfg.includeTests, "include-tests", false, "count the test files as roots")
	flag.BoolVar(&cfg.json, "json", false, "print the reach results as JSON")
	flag.Parse()

	if cfg.dir == "" || len(cfg.targets) == 0 {
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
	fsys, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories:  []string{root},
		ExcludePatterns: skipPatterns(root),
	})
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

	indexed, err := (&fileindex.Indexer{}).Run(ctx, fsys, fileindex.Options{Languages: languages, Store: s})
	if err != nil {
		return fmt.Errorf("failed to index %s: %w", root, err)
	}
	g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{
		Root:      os.DirFS(root),
		Files:     indexed.Files,
		TestPaths: testPaths,
	})
	if err != nil {
		return fmt.Errorf("failed to link %s: %w", root, err)
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

// skipPatterns match the installed directories at any depth below root and
// the vendored or built directories directly under root. A root under a
// directory named build keeps its code.
func skipPatterns(root string) []*regexp.Regexp {
	const sep = `[\\/]`
	prefix := `^` + regexp.QuoteMeta(root) + sep
	return []*regexp.Regexp{
		regexp.MustCompile(prefix + `(?:.*` + sep + `)?` + alternation(installedDirs) + `(?:` + sep + `|$)`),
		regexp.MustCompile(prefix + alternation(rootDirs) + `(?:` + sep + `|$)`),
	}
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
// as main.py:3 -> handlers.py:12 -> yaml//load.
func pathText(hops []reach.Hop) string {
	parts := make([]string, 0, len(hops))
	for _, h := range hops[1:] {
		parts = append(parts, siteText(h.Site))
	}
	return strings.Join(append(parts, hops[len(hops)-1].Key), " -> ")
}

func siteText(s projectgraph.Site) string {
	if s.Line == 0 {
		return s.Path
	}
	return fmt.Sprintf("%s:%d", s.Path, s.Line)
}

package indexer

import (
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/internal/testutil"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "write the golden files")

// indexFiles runs the index plugin over files under root and returns the
// index of each file by path.
func indexFiles(t *testing.T, root string, files []string, codes ...core.LanguageCode) map[string]*fileindex.Index {
	t.Helper()
	walker, fsys, err := test.SetupBasicPluginContext(root, files, codes)
	require.NoError(t, err)

	var mu sync.Mutex
	indexes := map[string]*fileindex.Index{}
	executor := plugin.NewTreeWalkPluginExecutor(walker, []core.Plugin{
		NewFileIndexPlugin(func(_ context.Context, ix *fileindex.Index) error {
			mu.Lock()
			defer mu.Unlock()
			indexes[ix.Path] = ix
			return nil
		}),
	})

	report, err := executor.Execute(context.Background(), fsys)
	require.NoError(t, err)
	require.Empty(t, report.Failed)
	require.Len(t, indexes, len(files))
	return indexes
}

// assertGolden compares the JSON of ix with the golden file of its path.
// The flag -update writes the golden file.
func assertGolden(t *testing.T, root string, ix *fileindex.Index) {
	t.Helper()
	testutil.AssertGolden(t, filepath.Join(root, "golden", ix.Path+".json"), *update, ix)
}

func TestFileIndexGolden(t *testing.T) {
	cases := []struct {
		file string
		code core.LanguageCode
	}{
		{"App.java", core.LanguageCodeJava},
		{"App.kt", core.LanguageCodeKotlin},
		{"App.cs", core.LanguageCodeCSharp},
		{"app.php", core.LanguageCodePHP},
		{"app.rb", core.LanguageCodeRuby},
		{"app.rs", core.LanguageCodeRust},
		{"app.go", core.LanguageCodeGo},
		{"app.js", core.LanguageCodeJavascript},
		{"app.ts", core.LanguageCodeTypescript},
		{"app.py", core.LanguageCodePython},
		{"exports.js", core.LanguageCodeJavascript},
		{"exports.ts", core.LanguageCodeTypescript},
		{"exports.py", core.LanguageCodePython},
		{"flags.js", core.LanguageCodeJavascript},
		{"flags.ts", core.LanguageCodeTypescript},
		{"flags.py", core.LanguageCodePython},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			indexes := indexFiles(t, "testdata", []string{tc.file}, tc.code)
			ix := indexes[tc.file]
			require.NotNil(t, ix)
			assert.Equal(t, tc.code, ix.Language)
			assertGolden(t, "testdata", ix)
		})
	}
}

// goldenHashes records the hash of the golden files of each
// fileindex.Version. A change of a golden file is a change of a fill rule,
// and a store must not serve an index of the old rules for it. So a change
// of the golden files needs a new Version and a new row. A row of an old
// Version stays as it is.
var goldenHashes = map[string]string{
	"10": "6053fe7c2b8fef2888806debc47372948eec366beb800419c1e435f021711fa0",
	"11": "141e6d6a6f6d369635bf4f6781033acc269e59fe5f722558192b8e952c7e7bd9",
	"12": "974943df4185f9c3070dcfdfedff8e91912e257d255a936f7c27608bb63a69f0",
	"13": "97614d8b96782e65813ee409e4a3f33c4655157b5dfa58baf17cd3677958c619",
	"14": "26f22c80fd4120cd04f228a53188accb1695434d0ed1ff5654b110981f8f44dd",
	"15": "327ac05cb53e44b88a8e5e0185bdd1fb32ac0cdb9e8a8eb27b9804843bdca48d",
}

func TestFileIndexGoldenHashFollowsVersion(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "golden", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, names)

	want, ok := goldenHashes[fileindex.Version]
	require.True(t, ok, "no row of goldenHashes for fileindex.Version %s: add one", fileindex.Version)
	assert.Equal(t, want, testutil.HashFiles(t, names), "the golden files changed: bump fileindex.Version and add a row to goldenHashes")
}

func TestFileIndexRecords(t *testing.T) {
	ix := indexFiles(t, "testdata", []string{"app.js"}, core.LanguageCodeJavascript)["app.js"]
	require.NotNil(t, ix)

	data, err := os.ReadFile("testdata/app.js")
	require.NoError(t, err)
	assert.Equal(t, fileindex.HashContent(data), ix.ContentHash)

	callsAt := func(line int) fileindex.Call {
		for _, call := range ix.Calls {
			if call.Line == line {
				return call
			}
		}
		require.Failf(t, "no call", "line %d", line)
		return fileindex.Call{}
	}

	t.Run("an unbound call rebinds to a later definition", func(t *testing.T) {
		assert.Equal(t, []string{"app.js//later"}, callsAt(32).Targets)
	})
	t.Run("an unbound call with no definition keeps a bare name", func(t *testing.T) {
		call := callsAt(33)
		assert.Equal(t, "missing", call.Callee)
		assert.Empty(t, call.Targets)
	})
	t.Run("a method call on this reaches the method", func(t *testing.T) {
		assert.Equal(t, []string{"app.js//Parser//parse"}, callsAt(20).Targets)
	})
	t.Run("an import call reaches the import namespace", func(t *testing.T) {
		assert.Equal(t, []string{"js-yaml//load"}, callsAt(31).Targets)
	})
}

func TestFileIndexRebindSkipsAClassScope(t *testing.T) {
	source := "class A:\n    def m(self):\n        g()\n\n    def g(self):\n        pass\n\n\ndef g():\n    pass\n"
	ix := indexSource(t, core.LanguageCodePython, "a.py", source)

	var targets []string
	for _, call := range ix.Calls {
		if call.Callee == "g" {
			targets = call.Targets
		}
	}
	assert.Equal(t, []string{"a.py//g"}, targets)
}

func TestFileIndexOverTheNodeLimit(t *testing.T) {
	var source strings.Builder
	for i := range 50001 {
		_, err := fmt.Fprintf(&source, "f%d();\n", i)
		require.NoError(t, err)
	}

	var ix *fileindex.Index
	require.NotPanics(t, func() {
		ix = indexSource(t, core.LanguageCodeJavascript, "big.js", source.String())
	})
	require.NotEmpty(t, ix.Truncated)
	assert.Equal(t, fileindex.TruncationNodes, ix.Truncated[0].Kind)
}

func TestFileIndexPartialAtTheDeadline(t *testing.T) {
	var source strings.Builder
	for i := range 20000 {
		_, err := fmt.Fprintf(&source, "f%d();\n", i)
		require.NoError(t, err)
	}
	tree := parseSource(t, core.LanguageCodeJavascript, "slow.js", source.String())

	ctx := &buildCutContext{Context: context.Background()}

	var got *fileindex.Index
	err := NewFileIndexPlugin(func(_ context.Context, ix *fileindex.Index) error {
		got = ix
		return nil
	}).AnalyzeTree(ctx, tree)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, got)
	assert.True(t, partial(got))
}

// buildCutContext is done from the first check inside the build of the
// call graph on. The import queries before the build see it live, so the
// build starts and the deadline cuts it. It reads the stack, so it does not
// depend on the number of checks before the build.
type buildCutContext struct {
	context.Context
	cut bool
}

func (c *buildCutContext) Err() error {
	if !c.cut {
		c.cut = insideBuild()
	}
	if c.cut {
		return context.Canceled
	}
	return nil
}

func insideBuild() bool {
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.Function, "callgraph.(*CallGraph).build") {
			return true
		}
		if !more {
			return false
		}
	}
}

func parseSource(t *testing.T, code core.LanguageCode, name, source string) core.ParseTree {
	t.Helper()
	language, err := lang.GetLanguage(string(code))
	require.NoError(t, err)
	tree, err := parser.ParseBytes(context.Background(), language, []byte(source), parser.WithFileName(name))
	require.NoError(t, err)
	t.Cleanup(tree.Close)
	return tree
}

func indexSource(t *testing.T, code core.LanguageCode, name, source string) *fileindex.Index {
	t.Helper()
	var got *fileindex.Index
	err := NewFileIndexPlugin(func(_ context.Context, ix *fileindex.Index) error {
		got = ix
		return nil
	}).AnalyzeTree(context.Background(), parseSource(t, code, name, source))
	require.NoError(t, err)
	require.NotNil(t, got)
	return got
}

func TestFileIndexTypeOnlyImports(t *testing.T) {
	source := strings.Join([]string{
		"import type { A } from './a';",
		"import { type B, c } from './b';",
		"import typeof D from './d';",
		"import e from './e';",
		"export type { F } from './f';",
		"export { type G, h } from './g';",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeTypescript, "main.ts", source)

	kinds := map[string]fileindex.ImportKind{}
	for _, imp := range ix.Imports {
		kinds[cmp.Or(imp.Item, imp.Alias)] = imp.Kind
	}
	assert.Equal(t, map[string]fileindex.ImportKind{
		"A": fileindex.ImportTypeOnly,
		"B": fileindex.ImportTypeOnly,
		"c": fileindex.ImportStatic,
		"D": fileindex.ImportTypeOnly,
		"e": fileindex.ImportStatic,
	}, kinds)
	assert.Equal(t, []fileindex.Export{{Name: "h", From: "./g", Item: "h", Line: 6}}, ix.Exports)
}

func TestFileIndexStaticClassCode(t *testing.T) {
	source := strings.Join([]string{
		"import { load } from 'js-yaml';",
		"class A {",
		"  static x = load('a');",
		"  y = load('b');",
		"  static { load('c'); }",
		"  static f = () => load('d');",
		"}",
		"function make() {",
		"  return class B { static z = load('e'); };",
		"}",
	}, "\n") + "\n"
	for _, code := range []core.LanguageCode{core.LanguageCodeJavascript, core.LanguageCodeTypescript} {
		t.Run(string(code), func(t *testing.T) {
			ix := indexSource(t, code, "a.js", source)
			callers := map[int]string{}
			for _, call := range ix.Calls {
				callers[call.Line] = call.Caller
			}
			assert.Equal(t, map[int]string{
				3: "a.js",
				4: "a.js//A",
				5: "a.js",
				6: "a.js//A//f",
				9: "a.js//make",
			}, callers)
		})
	}
}

func TestFileIndexRequireCalls(t *testing.T) {
	source := strings.Join([]string{
		"const fs = require('fs');",
		"require('js-yaml').load(fs.readFileSync('a'));",
		"require(\"./parse\").parse.strict();",
	}, "\n") + "\n"
	ix := indexSource(t, core.LanguageCodeJavascript, "main.js", source)

	targets := map[string][]string{}
	for _, call := range ix.Calls {
		targets[call.Callee] = call.Targets
	}
	assert.NotContains(t, targets, "require", "a require call is an import, not a call")
	assert.Equal(t, []string{"js-yaml//load"}, targets["require('js-yaml').load"])
	assert.Equal(t, []string{".//parse//parse//strict"}, targets["require(\"./parse\").parse.strict"])
	assert.Equal(t, []string{"fs//readFileSync"}, targets["fs.readFileSync"])
}

func TestFileIndexModuleVariables(t *testing.T) {
	tests := []struct {
		name   string
		code   core.LanguageCode
		file   string
		source string
		want   map[string][]string
	}{
		{
			name:   "python",
			code:   core.LanguageCodePython,
			file:   "app.py",
			source: "import yaml\nfrom flask import Flask\n\nclass Parser:\n    pass\n\nparser = Parser()\napp = Flask(__name__)\nLOAD = yaml.load\nN = 3\n\ndef f():\n    local = 1\n",
			want: map[string][]string{
				"app.py//parser": {"app.py//Parser"},
				"app.py//app":    {"flask//Flask"},
				"app.py//LOAD":   {"yaml//load"},
				"app.py//N":      nil,
			},
		},
		{
			name:   "javascript",
			code:   core.LanguageCodeJavascript,
			file:   "app.js",
			source: "const yaml = require('js-yaml');\nclass Parser {}\nconst parser = new Parser();\nexport let load = yaml.load;\nconst handler = () => 1;\n",
			want: map[string][]string{
				"app.js//parser": {"app.js//Parser"},
				"app.js//load":   {"js-yaml//load"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := indexSource(t, tt.code, tt.file, tt.source)
			got := map[string][]string{}
			for _, def := range ix.Definitions {
				if def.Kind == fileindex.DefVariable {
					got[def.Key] = def.Values
				}
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFileIndexOfDependencyCode indexes the golden files as the files of a
// package unit. Each index has the records of the golden file, with the unit
// first in each key, because the shape of the index does not change for
// dependency code.
func TestFileIndexOfDependencyCode(t *testing.T) {
	const unit = "pypi:fixture@1.0.0"
	fsys, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		ImportDirectories: []string{"testdata"},
		ImportUnits:       map[string]string{"testdata": unit},
	})
	require.NoError(t, err)
	result, err := runIndexer(t, context.Background(), fsys, Options{Languages: allLanguages(t), Imports: true, Workers: 2})
	require.NoError(t, err)
	assert.Empty(t, result.Files)

	golden, err := filepath.Glob(filepath.Join("testdata", "golden", "*.json"))
	require.NoError(t, err)
	require.Len(t, result.Indexes, len(golden))
	for _, ix := range result.Indexes {
		_, name := fileindex.SplitUnit(ix.Path)
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, unit, ix.Unit)
			assert.Equal(t, fileindex.FileIndexed, result.Dependencies[ix.Path])
			for _, def := range ix.Definitions {
				assert.Equal(t, ix.Path, fileindex.Unit(def.Key))
			}

			got, err := json.MarshalIndent(ix, "", "  ")
			require.NoError(t, err)
			text := strings.ReplaceAll(string(got), unit+fileindex.Separator, "")
			text = strings.Replace(text, "  \"unit\": \""+unit+"\",\n", "", 1)
			want, err := os.ReadFile(filepath.Join("testdata", "golden", name+".json"))
			require.NoError(t, err)
			assert.Equal(t, string(want), text+"\n")
		})
	}
}

func allLanguages(t *testing.T) []core.Language {
	t.Helper()
	var languages []core.Language
	for _, code := range []core.LanguageCode{
		core.LanguageCodeJava, core.LanguageCodeKotlin, core.LanguageCodeCSharp, core.LanguageCodePHP, core.LanguageCodeRuby,
		core.LanguageCodeRust, core.LanguageCodeGo, core.LanguageCodeJavascript, core.LanguageCodeTypescript, core.LanguageCodePython,
	} {
		language, err := lang.GetLanguage(string(code))
		require.NoError(t, err)
		languages = append(languages, language)
	}
	return languages
}

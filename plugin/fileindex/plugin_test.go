package fileindex

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/safedep/code/core"
	fidx "github.com/safedep/code/fileindex"
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
func indexFiles(t *testing.T, root string, files []string, codes ...core.LanguageCode) map[string]*fidx.Index {
	t.Helper()
	walker, fsys, err := test.SetupBasicPluginContext(root, files, codes)
	require.NoError(t, err)

	var mu sync.Mutex
	indexes := map[string]*fidx.Index{}
	executor, err := plugin.NewTreeWalkPluginExecutor(walker, []core.Plugin{
		NewFileIndexPlugin(func(_ context.Context, ix *fidx.Index) error {
			mu.Lock()
			defer mu.Unlock()
			indexes[ix.Path] = ix
			return nil
		}),
	})
	require.NoError(t, err)

	report, err := executor.Execute(context.Background(), fsys)
	require.NoError(t, err)
	require.Empty(t, report.Failed)
	require.Len(t, indexes, len(files))
	return indexes
}

// assertGolden compares the JSON of ix with the golden file of its path.
// The flag -update writes the golden file.
func assertGolden(t *testing.T, root string, ix *fidx.Index) {
	t.Helper()
	got, err := json.MarshalIndent(ix, "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')

	golden := filepath.Join(root, "golden", ix.Path+".json")
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
		require.NoError(t, os.WriteFile(golden, got, 0o644))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

func TestFileIndexGolden(t *testing.T) {
	cases := []struct {
		file string
		code core.LanguageCode
	}{
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

// goldenVersion and goldenHash pin the golden files to fileindex.Version. A
// change of a golden file is a change of a fill rule, and a store must not
// serve an index of the old rules for it. So a change of the golden files
// needs a new Version, and both constants change with it.
const (
	goldenVersion = "4"
	goldenHash    = "79cbaa8e1866df050e6163a5fe4b47b6f3e3b68b6e222dd765d98de46940b650"
)

func TestFileIndexGoldenHashFollowsVersion(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "golden", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, names)

	sum := sha256.New()
	for _, name := range names {
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		_, err = fmt.Fprintf(sum, "%s\n%d\n", filepath.Base(name), len(data))
		require.NoError(t, err)
		_, err = sum.Write(data)
		require.NoError(t, err)
	}
	assert.Equal(t, goldenVersion, fidx.Version, "the golden files belong to another Version")
	assert.Equal(t, goldenHash, hex.EncodeToString(sum.Sum(nil)), "the golden files changed: bump fileindex.Version, goldenVersion and goldenHash")
}

func TestFileIndexRecords(t *testing.T) {
	ix := indexFiles(t, "testdata", []string{"app.js"}, core.LanguageCodeJavascript)["app.js"]
	require.NotNil(t, ix)

	data, err := os.ReadFile("testdata/app.js")
	require.NoError(t, err)
	assert.Equal(t, fidx.HashContent(data), ix.ContentHash)

	callsAt := func(line int) fidx.Call {
		for _, call := range ix.Calls {
			if call.Line == line {
				return call
			}
		}
		require.Failf(t, "no call", "line %d", line)
		return fidx.Call{}
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

	var ix *fidx.Index
	require.NotPanics(t, func() {
		ix = indexSource(t, core.LanguageCodeJavascript, "big.js", source.String())
	})
	require.NotEmpty(t, ix.Truncated)
	assert.Equal(t, fidx.TruncationNodes, ix.Truncated[0].Kind)
}

func TestFileIndexPartialAtTheDeadline(t *testing.T) {
	var source strings.Builder
	for i := range 20000 {
		_, err := fmt.Fprintf(&source, "f%d();\n", i)
		require.NoError(t, err)
	}
	tree := parseSource(t, core.LanguageCodeJavascript, "slow.js", source.String())

	// The context turns done after the eight import queries and the first
	// check of the build of the call graph.
	ctx := &countdownContext{Context: context.Background(), left: 9}

	var got *fidx.Index
	err := NewFileIndexPlugin(func(_ context.Context, ix *fidx.Index) error {
		got = ix
		return nil
	}).AnalyzeTree(ctx, tree)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, got)
	assert.True(t, partial(got))
}

// countdownContext is done after its Err has returned nil left times.
type countdownContext struct {
	context.Context
	left int
}

func (c *countdownContext) Err() error {
	if c.left <= 0 {
		return context.Canceled
	}
	c.left--
	return nil
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

func indexSource(t *testing.T, code core.LanguageCode, name, source string) *fidx.Index {
	t.Helper()
	var got *fidx.Index
	err := NewFileIndexPlugin(func(_ context.Context, ix *fidx.Index) error {
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

	kinds := map[string]fidx.ImportKind{}
	for _, imp := range ix.Imports {
		kinds[cmp.Or(imp.Item, imp.Alias)] = imp.Kind
	}
	assert.Equal(t, map[string]fidx.ImportKind{
		"A": fidx.ImportTypeOnly,
		"B": fidx.ImportTypeOnly,
		"c": fidx.ImportStatic,
		"D": fidx.ImportTypeOnly,
		"e": fidx.ImportStatic,
	}, kinds)
	assert.Equal(t, []fidx.Export{{Name: "h", From: "./g", Item: "h", Line: 6}}, ix.Exports)
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
				if def.Kind == fidx.DefVariable {
					got[def.Key] = def.Values
				}
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

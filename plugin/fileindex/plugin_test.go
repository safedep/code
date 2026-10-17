package fileindex

import (
	"context"
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

package stripcomments

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
)

type StripCommentsTestcase struct {
	Language          core.LanguageCode
	CommentedFilePath string
	StrippedFilePath  string
}

var testcases = []StripCommentsTestcase{
	{
		Language:          core.LanguageCodePython,
		CommentedFilePath: "fixtures/commented.py",
		StrippedFilePath:  "fixtures/stripped.py",
	},
	{
		Language:          core.LanguageCodeJavascript,
		CommentedFilePath: "fixtures/commented.js",
		StrippedFilePath:  "fixtures/stripped.js",
	},
}

func TestStripComments(t *testing.T) {
	// run for each testcase
	for _, testcase := range testcases {
		t.Run(fmt.Sprintf("%s (%s)", testcase.CommentedFilePath, testcase.Language), func(t *testing.T) {
			filePaths := []string{testcase.CommentedFilePath}
			treeWalker, fileSystem, err := test.SetupBasicPluginContext(filePaths, []core.LanguageCode{testcase.Language})
			assert.NoError(t, err)

			readers := []io.Reader{}
			var stripCommentsCallback StripCommentsCallback = func(ctx context.Context, strippedData *StripCommentsPluginData) error {
				readers = append(readers, strippedData.Reader)
				return nil
			}

			pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
				NewStripCommentsPlugin(stripCommentsCallback),
			})
			assert.NoError(t, err)

			err = pluginExecutor.Execute(context.Background(), fileSystem)
			assert.NoError(t, err)

			assert.Equal(t, 1, len(readers))

			expectedReader, err := os.Open(testcase.StrippedFilePath)
			assert.NoError(t, err)

			strippedBytes, err := io.ReadAll(readers[0])
			assert.NoError(t, err)

			defer expectedReader.Close()
			expectedBytes, err := io.ReadAll(expectedReader)
			assert.NoError(t, err)

			assert.Equal(t, string(expectedBytes), string(strippedBytes))
		})
	}
}

func benchmarkStripComments(b *testing.B) {
	treeWalker, fileSystem, err := test.SetupBasicPluginContext([]string{
		"fixtures/commented.js",
		"fixtures/commented.py",
	}, []core.LanguageCode{core.LanguageCodeJavascript})
	assert.NoError(b, err)

	pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		NewStripCommentsPlugin(func(ctx context.Context, scpd *StripCommentsPluginData) error {
			assert.NotNil(b, scpd)
			assert.NotNil(b, scpd.File)
			assert.NotNil(b, scpd.Reader)
			return nil
		}),
	})
	assert.NoError(b, err)

	err = pluginExecutor.Execute(context.Background(), fileSystem)
	assert.NoError(b, err)
}

func BenchmarkStripComments(b *testing.B) {
	n := runtime.NumCPU()
	defer runtime.GOMAXPROCS(n)

	// Use a single core of CPU
	runtime.GOMAXPROCS(1)

	for i := 0; i < b.N; i++ {
		benchmarkStripComments(b)
	}
}

package depsusage

import (
	"context"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
)

type DepsTestcase struct {
	Language          core.LanguageCode
	FilePath          string
	ExpectedEvicences []UsageEvidence
}

var testcases = []DepsTestcase{
	{
		Language: core.LanguageCodePython,
		FilePath: "fixtures/testcases.py",
		ExpectedEvicences: []UsageEvidence{
			*newUsageEvidence("seaborn", "*", "*", "seaborn", "fixtures/testcases.py", 60, true),
			*newUsageEvidence("flask", "*", "*", "helpers", "fixtures/testcases.py", 61, true),
			*newUsageEvidence("xyz", "*", "*", "pqr.mno", "fixtures/testcases.py", 62, true),
			*newUsageEvidence("sys", "sys", "", "", "fixtures/testcases.py", 6, false),
			*newUsageEvidence("math", "sqrt", "sqrt", "sqrt", "fixtures/testcases.py", 13, false),
			*newUsageEvidence("pandas", "pd", "pd", "pandas", "fixtures/testcases.py", 18, false),
			*newUsageEvidence("matplotlib", "plt", "plt", "pyplot", "fixtures/testcases.py", 22, false),
			*newUsageEvidence("slumber", "sl", "sl", "API", "fixtures/testcases.py", 27, false),
			*newUsageEvidence("sklearn", "ds", "ds", "datasets", "fixtures/testcases.py", 29, false),
			*newUsageEvidence("sklearn", "met", "met", "metrics", "fixtures/testcases.py", 30, false),
			*newUsageEvidence("random", "randint", "randint", "randint", "fixtures/testcases.py", 35, false),
			*newUsageEvidence("collections", "deque", "deque", "deque", "fixtures/testcases.py", 37, false),
			*newUsageEvidence("collections", "defaultdict", "defaultdict", "defaultdict", "fixtures/testcases.py", 39, false),
			*newUsageEvidence("collections", "namedtuple", "namedtuple", "namedtuple", "fixtures/testcases.py", 40, false),
			*newUsageEvidence("json", "JSONEncoder", "JSONEncoder", "JSONEncoder", "fixtures/testcases.py", 46, false),
			*newUsageEvidence("urllib", "urlsplit", "urlsplit", "urlsplit", "fixtures/testcases.py", 47, false),
			*newUsageEvidence("ujson", "ujson", "", "", "fixtures/testcases.py", 52, false),
			*newUsageEvidence("simplejson", "smpjson", "smpjson", "simplejson", "fixtures/testcases.py", 56, false),
		},
	},
}

func TestDepsusage(t *testing.T) {
	// run for each testcase
	for _, testcase := range testcases {
		t.Run(string(testcase.Language), func(t *testing.T) {
			fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
				AppDirectories: []string{testcase.FilePath},
			})

			if err != nil {
				t.Fatalf("failed to create file system: %v", err)
			}

			language, err := lang.GetLanguage(string(testcase.Language))
			if err != nil {
				t.Fatalf("failed to get language: %v", err)
			}

			walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, language)
			if err != nil {
				t.Fatalf("failed to create source walker: %v", err)
			}

			treeWalker, err := parser.NewWalkingParser(walker, language)
			if err != nil {
				t.Fatalf("failed to create tree walker: %v", err)
			}

			evidences := []UsageEvidence{}
			usageCallback := func(evidence *UsageEvidence) {
				evidences = append(evidences, *evidence)
			}

			pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
				NewDependencyUsagePlugin(usageCallback),
			})

			if err != nil {
				t.Fatalf("failed to create plugin executor: %v", err)
			}

			err = pluginExecutor.Execute(context.Background(), fileSystem)
			if err != nil {
				t.Fatalf("failed to execute depsusage via plugin executor: %v", err)
			}

			assert.Equal(t, testcase.ExpectedEvicences, evidences)
		})
	}
}

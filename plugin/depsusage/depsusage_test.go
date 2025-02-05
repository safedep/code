package depsusage

import (
	"context"
	"fmt"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
)

type DepsTestcase struct {
	Language          core.LanguageCode
	FilePath          string
	ExpectedEvicences []*UsageEvidence
}

var testcases = []DepsTestcase{
	{
		Language: core.LanguageCodePython,
		FilePath: "fixtures/testcases.py",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("seaborn", "seaborn", "", "", true, "", "fixtures/testcases.py", 60),
			newUsageEvidence("flask", "flask.helpers", "", "", true, "", "fixtures/testcases.py", 61),
			newUsageEvidence("xyz", "xyz.pqr.mno", "", "", true, "", "fixtures/testcases.py", 62),
			newUsageEvidence("sys", "sys", "", "sys", false, "sys", "fixtures/testcases.py", 6),
			newUsageEvidence("math", "math", "sqrt", "sqrt", false, "sqrt", "fixtures/testcases.py", 13),
			newUsageEvidence("pandas", "pandas", "", "pd", false, "pd", "fixtures/testcases.py", 18),
			newUsageEvidence("matplotlib", "matplotlib.pyplot", "", "plt", false, "plt", "fixtures/testcases.py", 22),
			newUsageEvidence("slumber", "slumber", "API", "sl", false, "sl", "fixtures/testcases.py", 27),
			newUsageEvidence("sklearn", "sklearn", "datasets", "ds", false, "ds", "fixtures/testcases.py", 29),
			newUsageEvidence("sklearn", "sklearn", "metrics", "met", false, "met", "fixtures/testcases.py", 30),
			newUsageEvidence("random", "random", "randint", "randint", false, "randint", "fixtures/testcases.py", 35),
			newUsageEvidence("collections", "collections", "deque", "deque", false, "deque", "fixtures/testcases.py", 37),
			newUsageEvidence("collections", "collections", "defaultdict", "defaultdict", false, "defaultdict", "fixtures/testcases.py", 39),
			newUsageEvidence("collections", "collections", "namedtuple", "namedtuple", false, "namedtuple", "fixtures/testcases.py", 40),
			newUsageEvidence("json", "json.encoder.implementation", "JSONEncoder", "JSONEncoder", false, "JSONEncoder", "fixtures/testcases.py", 46),
			newUsageEvidence("urllib", "urllib.parse", "urlsplit", "urlsplit", false, "urlsplit", "fixtures/testcases.py", 47),
			newUsageEvidence("ujson", "ujson", "", "ujson", false, "ujson", "fixtures/testcases.py", 52),
			newUsageEvidence("simplejson", "simplejson", "", "smpjson", false, "smpjson", "fixtures/testcases.py", 56),
		},
	},
}

func TestDepsusageEvidences(t *testing.T) {
	// run for each testcase
	for _, testcase := range testcases {
		t.Run(string(testcase.Language), func(t *testing.T) {
			filePaths := []string{testcase.FilePath}
			treeWalker, fileSystem, err := test.SetupBasicPluginContext(filePaths, []core.LanguageCode{testcase.Language})
			assert.NoError(t, err)

			evidences := []UsageEvidence{}
			var usageCallback DependencyUsageCallback = func(ctx context.Context, evidence *UsageEvidence) error {
				evidences = append(evidences, *evidence)
				return nil
			}

			pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
				NewDependencyUsagePlugin(usageCallback),
			})
			assert.NoError(t, err)

			err = pluginExecutor.Execute(context.Background(), fileSystem)
			assert.NoError(t, err)

			assert.Equal(t, len(testcase.ExpectedEvicences), len(evidences))
			for i, expectedEvidence := range testcase.ExpectedEvicences {
				assert.Equal(t, expectedEvidence, &evidences[i])
			}
		})
	}
}

func TestAbortedDepsusage(t *testing.T) {
	t.Run("AbortExecutionForWildcardEvidence", func(t *testing.T) {
		filePaths := []string{"fixtures/testcases.py"}
		treeWalker, fileSystem, err := test.SetupBasicPluginContext(filePaths, []core.LanguageCode{core.LanguageCodePython})

		if err != nil {
			t.Fatalf("failed to setup plugin context: %v", err)
		}

		var usageCallback DependencyUsageCallback = func(ctx context.Context, evidence *UsageEvidence) error {
			if evidence.IsWildCardUsage {
				return fmt.Errorf("aborting due to user err in callback")
			}
			return nil
		}

		pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
			NewDependencyUsagePlugin(usageCallback),
		})

		if err != nil {
			t.Fatalf("failed to create plugin executor: %v", err)
		}

		err = pluginExecutor.Execute(context.Background(), fileSystem)

		assert.Error(t, err)
	})

	t.Run("AbortExecutionForAstEvidence", func(t *testing.T) {
		filePaths := []string{"fixtures/testcases.py"}
		treeWalker, fileSystem, err := test.SetupBasicPluginContext(filePaths, []core.LanguageCode{core.LanguageCodePython})
		assert.NoError(t, err)

		var usageCallback DependencyUsageCallback = func(ctx context.Context, evidence *UsageEvidence) error {
			if evidence.IsWildCardUsage {
				return nil
			}
			return fmt.Errorf("aborting due to user err in callback")
		}

		pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
			NewDependencyUsagePlugin(usageCallback),
		})
		assert.NoError(t, err)

		err = pluginExecutor.Execute(context.Background(), fileSystem)
		assert.Error(t, err)
	})
}

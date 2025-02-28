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
	{
		Language: core.LanguageCodeGo,
		FilePath: "fixtures/testcases.go",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("embed", "embed", "", "", true, "", "fixtures/testcases.go", 12),
			newUsageEvidence("math", "math", "", "", true, "", "fixtures/testcases.go", 13),
			newUsageEvidence("github.com/labstack/echo-contrib/pprof", "github.com/labstack/echo-contrib/pprof", "", "", true, "", "fixtures/testcases.go", 18),
			newUsageEvidence("net/http", "net/http", "", "", true, "", "fixtures/testcases.go", 20),
			newUsageEvidence("fmt", "fmt", "", "fmt", false, "fmt", "fixtures/testcases.go", 25),
			newUsageEvidence("github.com/safedep/code/lang", "github.com/safedep/code/lang", "", "lang", false, "lang", "fixtures/testcases.go", 25),
			newUsageEvidence("os", "os", "", "osalias", false, "osalias", "fixtures/testcases.go", 27),
			newUsageEvidence("crypto", "crypto", "", "cryptoalias", false, "cryptoalias", "fixtures/testcases.go", 33),
			newUsageEvidence("strings", "strings", "", "strings", false, "strings", "fixtures/testcases.go", 39),
		},
	},
	{
		Language: core.LanguageCodeJavascript,
		FilePath: "fixtures/testcases.js",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("express", "express", "", "express", false, "express", "fixtures/testcases.js", 10),
			newUsageEvidence("cluster", "cluster", "", "Cluster", false, "Cluster", "fixtures/testcases.js", 11),
			newUsageEvidence("@gilbarbara/eslint-config", "@gilbarbara/eslint-config", "", "EslintConfig", false, "EslintConfig", "fixtures/testcases.js", 14),
			newUsageEvidence("./config.js", "./config.js", "", "config", false, "config", "fixtures/testcases.js", 20),
			newUsageEvidence("./utils.js", "./utils.js", "", "utils", false, "utils", "fixtures/testcases.js", 21),
			newUsageEvidence("../utils/helper.js", "../utils/helper.js", "", "helper", false, "helper", "fixtures/testcases.js", 27),
			newUsageEvidence("../utils/sideeffects.js", "../utils/sideeffects.js", "", "sideeffects", false, "sideeffects", "fixtures/testcases.js", 28),
			newUsageEvidence("./data1.json", "./data1.json", "", "jsonData", false, "jsonData", "fixtures/testcases.js", 34),
			newUsageEvidence("./data2.json", "./data2.json", "", "jsonData2", false, "jsonData2", "fixtures/testcases.js", 35),
			newUsageEvidence("lodash", "lodash", "", "lodash", false, "lodash", "fixtures/testcases.js", 42),
			newUsageEvidence("./math-utils", "./math-utils", "", "mathUtils", false, "mathUtils", "fixtures/testcases.js", 43),
			newUsageEvidence("./dynamic-module.js", "./dynamic-module.js", "", "dynamicModule", false, "dynamicModule", "fixtures/testcases.js", 46),
			newUsageEvidence("./dynamic-module.js", "./dynamic-module.js", "", "dynamicModule", false, "dynamicModule", "fixtures/testcases.js", 48),
			newUsageEvidence("react-dom", "react-dom", "flushSync", "flushIt", false, "flushIt", "fixtures/testcases.js", 53),
			newUsageEvidence("react-dom", "react-dom", "render", "render", false, "render", "fixtures/testcases.js", 54),
			newUsageEvidence("react-dom", "react-dom", "", "ReactDOM", false, "ReactDOM", "fixtures/testcases.js", 56),
			newUsageEvidence("constants", "constants", "EADDRINUSE", "EADDRINUSE", false, "EADDRINUSE", "fixtures/testcases.js", 66),
			newUsageEvidence("chalk/ansi-styles", "chalk/ansi-styles", "hex", "hex", false, "hex", "fixtures/testcases.js", 67),
			newUsageEvidence("@xyz/xyz", "@xyz/xyz", "", "b", false, "b", "fixtures/testcases.js", 67),
			newUsageEvidence("virtual-dom", "virtual-dom", "patch", "patch", false, "patch", "fixtures/testcases.js", 68),
			newUsageEvidence("react", "react", "useState", "useMyState", false, "useMyState", "fixtures/testcases.js", 75),
			newUsageEvidence("react", "react", "useEffect", "useEffect", false, "useEffect", "fixtures/testcases.js", 76),
			newUsageEvidence("@xyz/pqr", "@xyz/pqr", "foo", "fooAlias", false, "fooAlias", "fixtures/testcases.js", 78),
			newUsageEvidence("@xyz/pqr", "@xyz/pqr", "bar", "bar", false, "bar", "fixtures/testcases.js", 78),
			newUsageEvidence("dotenv", "dotenv", "", "DotEnv", false, "DotEnv", "fixtures/testcases.js", 86),
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

package depsusage

import (
	"context"
	"fmt"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DepsTestcase struct {
	Language          core.LanguageCode
	FilePath          string
	ExpectedEvicences []*UsageEvidence
}

var testcases = []DepsTestcase{
	{
		Language: core.LanguageCodePython,
		FilePath: "testcases.py",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("seaborn", "seaborn", "", "", true, "", "testcases.py", 60),
			newUsageEvidence("flask", "flask.helpers", "", "", true, "", "testcases.py", 61),
			newUsageEvidence("xyz", "xyz.pqr.mno", "", "", true, "", "testcases.py", 62),
			newUsageEvidence("sys", "sys", "", "sys", false, "sys", "testcases.py", 6),
			newUsageEvidence("math", "math", "sqrt", "sqrt", false, "sqrt", "testcases.py", 13),
			newUsageEvidence("pandas", "pandas", "", "pd", false, "pd", "testcases.py", 18),
			newUsageEvidence("matplotlib", "matplotlib.pyplot", "", "plt", false, "plt", "testcases.py", 22),
			newUsageEvidence("slumber", "slumber", "API", "sl", false, "sl", "testcases.py", 27),
			newUsageEvidence("scikit-learn", "sklearn", "datasets", "ds", false, "ds", "testcases.py", 29),
			newUsageEvidence("scikit-learn", "sklearn", "metrics", "met", false, "met", "testcases.py", 30),
			newUsageEvidence("random", "random", "randint", "randint", false, "randint", "testcases.py", 35),
			newUsageEvidence("collections", "collections", "deque", "deque", false, "deque", "testcases.py", 37),
			newUsageEvidence("collections", "collections", "defaultdict", "defaultdict", false, "defaultdict", "testcases.py", 39),
			newUsageEvidence("collections", "collections", "namedtuple", "namedtuple", false, "namedtuple", "testcases.py", 40),
			newUsageEvidence("json", "json.encoder.implementation", "JSONEncoder", "JSONEncoder", false, "JSONEncoder", "testcases.py", 46),
			newUsageEvidence("urllib", "urllib.parse", "urlsplit", "urlsplit", false, "urlsplit", "testcases.py", 47),
			newUsageEvidence("ujson", "ujson", "", "ujson", false, "ujson", "testcases.py", 52),
			newUsageEvidence("simplejson", "simplejson", "", "smpjson", false, "smpjson", "testcases.py", 56),
		},
	},
	{
		Language: core.LanguageCodeGo,
		FilePath: "testcases.go",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("embed", "embed", "", "", true, "", "testcases.go", 12),
			newUsageEvidence("math", "math", "", "", true, "", "testcases.go", 13),
			newUsageEvidence("github.com/labstack/echo-contrib", "github.com/labstack/echo-contrib/pprof", "", "", true, "", "testcases.go", 18),
			newUsageEvidence("net", "net/http", "", "", true, "", "testcases.go", 20),
			newUsageEvidence("fmt", "fmt", "", "fmt", false, "fmt", "testcases.go", 25),
			newUsageEvidence("github.com/safedep/code", "github.com/safedep/code/lang", "", "lang", false, "lang", "testcases.go", 25),
			newUsageEvidence("os", "os", "", "osalias", false, "osalias", "testcases.go", 27),
			newUsageEvidence("crypto", "crypto", "", "cryptoalias", false, "cryptoalias", "testcases.go", 33),
			newUsageEvidence("strings", "strings", "", "strings", false, "strings", "testcases.go", 39),
		},
	},
	{
		Language: core.LanguageCodeJavascript,
		FilePath: "testcases.js",
		ExpectedEvicences: []*UsageEvidence{
			// The destructuring assignment of bundled code loads vue with no binding.
			newUsageEvidence("vue", "vue", "", "", true, "", "testcases.js", 85),
			newUsageEvidence("express", "express", "", "express", false, "express", "testcases.js", 10),
			newUsageEvidence("cluster", "cluster", "", "Cluster", false, "Cluster", "testcases.js", 11),
			newUsageEvidence("@gilbarbara/eslint-config", "@gilbarbara/eslint-config", "", "EslintConfig", false, "EslintConfig", "testcases.js", 14),
			newUsageEvidence("./config.js", "./config.js", "", "config", false, "config", "testcases.js", 20),
			newUsageEvidence("./utils.js", "./utils.js", "", "utils", false, "utils", "testcases.js", 21),
			newUsageEvidence("../utils/helper.js", "../utils/helper.js", "", "helper", false, "helper", "testcases.js", 27),
			newUsageEvidence("../utils/sideeffects.js", "../utils/sideeffects.js", "", "sideeffects", false, "sideeffects", "testcases.js", 28),
			newUsageEvidence("./data1.json", "./data1.json", "", "jsonData", false, "jsonData", "testcases.js", 34),
			newUsageEvidence("./data2.json", "./data2.json", "", "jsonData2", false, "jsonData2", "testcases.js", 35),
			newUsageEvidence("lodash", "lodash", "", "lodash", false, "lodash", "testcases.js", 42),
			newUsageEvidence("./math-utils", "./math-utils", "", "mathUtils", false, "mathUtils", "testcases.js", 43),
			newUsageEvidence("./dynamic-module.js", "./dynamic-module.js", "", "dynamicModule", false, "dynamicModule", "testcases.js", 48),
			newUsageEvidence("react-dom", "react-dom", "flushSync", "flushIt", false, "flushIt", "testcases.js", 53),
			newUsageEvidence("react-dom", "react-dom", "render", "render", false, "render", "testcases.js", 54),
			newUsageEvidence("react-dom", "react-dom", "", "ReactDOM", false, "ReactDOM", "testcases.js", 56),
			newUsageEvidence("constants", "constants", "EADDRINUSE", "EADDRINUSE", false, "EADDRINUSE", "testcases.js", 66),
			newUsageEvidence("chalk", "chalk/ansi-styles", "hex", "hex", false, "hex", "testcases.js", 67),
			newUsageEvidence("@xyz/xyz", "@xyz/xyz", "", "b", false, "b", "testcases.js", 67),
			newUsageEvidence("virtual-dom", "virtual-dom", "patch", "patch", false, "patch", "testcases.js", 68),
			newUsageEvidence("react", "react", "useState", "useMyState", false, "useMyState", "testcases.js", 75),
			newUsageEvidence("react", "react", "useEffect", "useEffect", false, "useEffect", "testcases.js", 76),
			newUsageEvidence("@xyz/pqr", "@xyz/pqr", "foo", "fooAlias", false, "fooAlias", "testcases.js", 78),
			newUsageEvidence("@xyz/pqr", "@xyz/pqr", "bar", "bar", false, "bar", "testcases.js", 78),
			newUsageEvidence("dotenv", "dotenv", "", "DotEnv", false, "DotEnv", "testcases.js", 86),
		},
	},
	{
		Language: core.LanguageCodeJava,
		FilePath: "testcases.java",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("java.util", "java.util", "", "", true, "", "testcases.java", 12),
			newUsageEvidence("java.awt", "java.awt.print", "", "", true, "", "testcases.java", 13),
			newUsageEvidence("", "org.springframework.beans.factory.annotation", "", "", true, "", "testcases.java", 14),
			newUsageEvidence("java.lang", "java.lang.Math", "", "", true, "", "testcases.java", 15),
			newUsageEvidence("java.util", "java.util.List", "", "List", false, "List", "testcases.java", 19),
			newUsageEvidence("java.util", "java.util.stream.Collectors", "", "Collectors", false, "Collectors", "testcases.java", 20),
			newUsageEvidence("", "com.sun.activation.registries.MailcapFile", "", "MailcapFile", false, "MailcapFile", "testcases.java", 21),
			newUsageEvidence("", "com.sun.activation.registries.MailcapFile", "", "MailcapFile", false, "MailcapFile", "testcases.java", 21),
			newUsageEvidence("java.lang", "java.lang.Math.PI", "", "PI", false, "PI", "testcases.java", 23),
			newUsageEvidence("", "org.junit.jupiter.api.Assertions.assertEquals", "", "assertEquals", false, "assertEquals", "testcases.java", 24),
		},
	},
	{
		Language: core.LanguageCodeCSharp,
		FilePath: "testcases.cs",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("System", "System", "", "", true, "", "testcases.cs", 1),
			newUsageEvidence("System.Text.Json", "System.Text.Json", "", "", true, "", "testcases.cs", 2),
			newUsageEvidence("System.Math", "System.Math", "", "", true, "", "testcases.cs", 3),
			newUsageEvidence("Microsoft.Extensions.Logging", "Microsoft.Extensions.Logging", "", "", true, "", "testcases.cs", 4),
			newUsageEvidence("OpenAI.Chat", "OpenAI.Chat", "", "", true, "", "testcases.cs", 9),
		},
	},
	{
		Language: core.LanguageCodeRust,
		FilePath: "testcases.rs",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("tokio", "tokio", "", "", true, "", "testcases.rs", 4),
			newUsageEvidence("serde_json", "serde_json", "", "", false, "serde_json", "testcases.rs", 13),
			newUsageEvidence("std", "std::collections::HashMap", "", "HashMap", false, "HashMap", "testcases.rs", 13),
			newUsageEvidence("serde_json", "serde_json", "", "", false, "serde_json", "testcases.rs", 14),
		},
	},
	{
		Language: core.LanguageCodePHP,
		FilePath: "testcases.php",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("GuzzleHttp\\Client", "GuzzleHttp\\Client", "", "HttpClient", false, "HttpClient", "testcases.php", 11),
			newUsageEvidence("Illuminate\\Support", "Illuminate\\Support", "Str", "Str", false, "Str", "testcases.php", 16),
		},
	},
	{
		Language: core.LanguageCodeRuby,
		FilePath: "testcases.rb",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("ruby-openai", "openai", "", "", true, "", "testcases.rb", 1),
			newUsageEvidence("net", "net/http", "", "", true, "", "testcases.rb", 2),
			newUsageEvidence("google-cloud-storage", "google/cloud/storage", "", "", true, "", "testcases.rb", 7),
		},
	},
	{
		Language: core.LanguageCodeKotlin,
		FilePath: "testcases.kt",
		ExpectedEvicences: []*UsageEvidence{
			newUsageEvidence("", "kotlinx.coroutines", "", "", true, "", "testcases.kt", 5),
			newUsageEvidence("", "okhttp3.OkHttpClient", "", "OkHttpClient", false, "OkHttpClient", "testcases.kt", 8),
		},
	},
}

func TestDepsusageEvidences(t *testing.T) {
	// run for each testcase
	for _, testcase := range testcases {
		t.Run(string(testcase.Language), func(t *testing.T) {
			filePaths := []string{testcase.FilePath}
			treeWalker, fileSystem, err := test.SetupBasicPluginContext("fixtures", filePaths, []core.LanguageCode{testcase.Language})
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

			report, err := pluginExecutor.Execute(context.Background(), fileSystem)
			assert.NoError(t, err)
			assert.Empty(t, report.Failed)

			assert.Equal(t, len(testcase.ExpectedEvicences), len(evidences))
			for i, expectedEvidence := range testcase.ExpectedEvicences {
				assert.Equal(t, expectedEvidence, &evidences[i])
			}
		})
	}
}

func TestAbortedDepsusage(t *testing.T) {
	t.Run("AbortExecutionForWildcardEvidence", func(t *testing.T) {
		filePaths := []string{"testcases.py"}
		treeWalker, fileSystem, err := test.SetupBasicPluginContext("fixtures", filePaths, []core.LanguageCode{core.LanguageCodePython})

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

		report, err := pluginExecutor.Execute(context.Background(), fileSystem)
		require.NoError(t, err)
		assertCallbackFailure(t, report)
	})

	t.Run("AbortExecutionForAstEvidence", func(t *testing.T) {
		filePaths := []string{"testcases.py"}
		treeWalker, fileSystem, err := test.SetupBasicPluginContext("fixtures", filePaths, []core.LanguageCode{core.LanguageCodePython})
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

		report, err := pluginExecutor.Execute(context.Background(), fileSystem)
		require.NoError(t, err)
		assertCallbackFailure(t, report)
	})
}

// assertCallbackFailure checks that the error of a callback fails the file
// at the stage of the plugin, and does not stop the walk.
func assertCallbackFailure(t *testing.T, report *plugin.Report) {
	require.Len(t, report.Failed, 1)
	failure := report.Failed[0]
	assert.Equal(t, "testcases.py", failure.File)
	assert.Equal(t, core.Stage("DependencyUsagePlugin"), failure.Stage)
	assert.ErrorContains(t, failure, "aborting due to user err in callback")
	assert.Equal(t, []string{"testcases.py"}, report.Files)
}

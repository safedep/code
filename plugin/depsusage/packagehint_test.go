package depsusage

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/stretchr/testify/assert"
)

func TestResolvePackageHint(t *testing.T) {
	t.Run("resolvePackageHint", func(t *testing.T) {
		languageWiseTests := map[core.LanguageCode]map[string]string{
			core.LanguageCodePython: {
				"foo":             "foo",
				"foo.bar":         "foo",
				"foo.bar.baz":     "foo",
				"foo.bar.baz.qux": "foo",
			},
			core.LanguageCodeGo: {
				"os":                                         "os",
				"filepath":                                   "filepath",
				"os/signal":                                  "os",
				"github.com/sjwhitworth/golearn/x/y/z":       "github.com/sjwhitworth/golearn",
				"github.com/safedep/code/utils/helpers":      "github.com/safedep/code",
				"github.com/robfig/cron/v3":                  "github.com/robfig/cron/v3",
				"github.com/robfig/cron/v28":                 "github.com/robfig/cron/v28",
				"github.com/robfig/cron/v3/a/b/c/d":          "github.com/robfig/cron/v3",
				"gopkg.in/natefinch/lumberjack":              "gopkg.in/natefinch/lumberjack",
				"gopkg.in/natefinch/lumberjack/xyz":          "gopkg.in/natefinch/lumberjack",
				"gopkg.in/natefinch/lumberjack.v2":           "gopkg.in/natefinch/lumberjack.v2",
				"go.etcd.io/etcd/client":                     "go.etcd.io/etcd/client",
				"go.etcd.io/etcd/client/v3":                  "go.etcd.io/etcd/client/v3",
				"go.opentelemetry.io/otel":                   "go.opentelemetry.io/otel",
				"go.opentelemetry.io/otel/xyz":               "go.opentelemetry.io/otel",
				"gocv.io/x/gocv":                             "gocv.io/x/gocv",
				"gocv.io/x/gocv/abc/def":                     "gocv.io/x/gocv",
				"golang.org/x/net/context":                   "golang.org/x/net",
				"golang.org/x/tools":                         "golang.org/x/tools",
				"go.uber.org/multierr":                       "go.uber.org/multierr",
				"go.uber.org/multierr/xyz":                   "go.uber.org/multierr",
				"go.uber.org/zap":                            "go.uber.org/zap",
				"go.uber.org/zap/abc":                        "go.uber.org/zap",
				"gopkg.in/yaml.v3":                           "gopkg.in/yaml.v3",
				"k8s.io/apiextensions-apiserver/xyz":         "k8s.io/apiextensions-apiserver",
				"bitbucket.org/bertimus9/systemstat/abc/xyz": "bitbucket.org/bertimus9/systemstat",
			},
			core.LanguageCodeJavascript: {
				"lodash":                                 "lodash",
				"lodash/fp":                              "lodash",
				"express/lib/router":                     "express",
				"react-dom/server":                       "react-dom",
				"crypto/randomBytes/async":               "crypto",
				"@fortawesome/fa-icon-chooser-react":     "@fortawesome/fa-icon-chooser-react",
				"@fortawesome/fa-icon-chooser-react/abc": "@fortawesome/fa-icon-chooser-react",
				"./utils":                                "./utils",
				"../utils":                               "../utils",
				"../../utils":                            "../../utils",
			},
		}

		for langCode, tests := range languageWiseTests {
			language, err := lang.GetLanguage(string(langCode))
			assert.NoError(t, err)
			// Empty modulename must give error
			_, err = resolvePackageHint("", language)
			assert.Error(t, err)
			for moduleName, expected := range tests {
				packageHint, err := resolvePackageHint(moduleName, language)
				assert.NoError(t, err)
				assert.Equal(t, expected, packageHint)
			}
		}
	})
}

package depsusage

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/stretchr/testify/assert"
)

func TestResolvePackageHint(t *testing.T) {
	t.Run("resolvePackageHint", func(t *testing.T) {
		type testCase struct {
			moduleName          string
			expectedPackageHint string
			expectedError       bool
		}
		languageWiseTests := map[core.LanguageCode][]testCase{
			core.LanguageCodePython: []testCase{
				{"foo", "foo", false},
				{"foo.bar", "foo", false},
				{"foo.bar.baz", "foo", false},
				{"foo.bar.baz.qux", "foo", false},
				{"foo.bar.baz.qux.v1", "foo", false},
			},
			core.LanguageCodeGo: {
				{"os", "os", false},
				{"filepath", "filepath", false},
				{"os/signal", "os", false},
				{"github.com/sjwhitworth/golearn/x/y/z", "github.com/sjwhitworth/golearn", false},
				{"github.com/safedep/code/utils/helpers", "github.com/safedep/code", false},
				{"github.com/robfig/cron/v3", "github.com/robfig/cron/v3", false},
				{"github.com/robfig/cron/v28", "github.com/robfig/cron/v28", false},
				{"github.com/robfig/cron/v3/a/b/c/d", "github.com/robfig/cron/v3", false},
				{"gopkg.in/natefinch/lumberjack", "gopkg.in/natefinch/lumberjack", false},
				{"gopkg.in/natefinch/lumberjack/xyz", "gopkg.in/natefinch/lumberjack", false},
				{"gopkg.in/natefinch/lumberjack.v2", "gopkg.in/natefinch/lumberjack.v2", false},
				{"go.etcd.io/etcd/client", "go.etcd.io/etcd/client", false},
				{"go.etcd.io/etcd/client/v3", "go.etcd.io/etcd/client/v3", false},
				{"go.opentelemetry.io/otel", "go.opentelemetry.io/otel", false},
				{"go.opentelemetry.io/otel/xyz", "go.opentelemetry.io/otel", false},
				{"gocv.io/x/gocv", "gocv.io/x/gocv", false},
				{"gocv.io/x/gocv/abc/def", "gocv.io/x/gocv", false},
				{"golang.org/x/net/context", "golang.org/x/net", false},
				{"golang.org/x/tools", "golang.org/x/tools", false},
				{"go.uber.org/multierr", "go.uber.org/multierr", false},
				{"go.uber.org/multierr/xyz", "go.uber.org/multierr", false},
				{"go.uber.org/zap", "go.uber.org/zap", false},
				{"go.uber.org/zap/abc", "go.uber.org/zap", false},
				{"gopkg.in/yaml.v3", "gopkg.in/yaml.v3", false},
				{"k8s.io/apiextensions-apiserver/xyz", "k8s.io/apiextensions-apiserver", false},
				{"bitbucket.org/bertimus9/systemstat/abc/xyz", "bitbucket.org/bertimus9/systemstat", false},
			},
			core.LanguageCodeJavascript: {
				{"lodash", "lodash", false},
				{"lodash/fp", "lodash", false},
				{"express/lib/router", "express", false},
				{"react-dom/server", "react-dom", false},
				{"crypto/randomBytes/async", "crypto", false},
				{"@fortawesome/fa-icon-chooser-react", "@fortawesome/fa-icon-chooser-react", false},
				{"@fortawesome/fa-icon-chooser-react/abc", "@fortawesome/fa-icon-chooser-react", false},
				{"./utils", "./utils", false},
				{"../utils", "../utils", false},
				{"../../utils", "../../utils", false},
			},
			core.LanguageCodeJava: {
				{"java.util", "java.util", false},
				{"java.util.concurrent", "java.util", false},
				{"java.util.concurrent.atomic", "java.util", false},
				{"jdk.javadoc", "jdk.javadoc", false},
				{"jdk.javadoc.doclet", "jdk.javadoc", false},
				{"com.google.common.collect", "", true},
				{"org.springframework.ai.chat.client.ChatClient", "", true},
				{"lombok.extern.slf4j.Slf4j", "", true},
			},
		}

		for langCode, tests := range languageWiseTests {
			language, err := lang.GetLanguage(string(langCode))
			assert.NoError(t, err)

			// Empty modulename must give error
			_, err = resolvePackageHint("", language)
			assert.Error(t, err)

			for _, test := range tests {
				packageHint, err := resolvePackageHint(test.moduleName, language)
				if test.expectedError {
					assert.Error(t, err)
					assert.Equal(t, "", packageHint)
				} else {
					assert.NoError(t, err)
					assert.NotEmpty(t, packageHint)
					assert.Equal(t, test.expectedPackageHint, packageHint)
				}
			}
		}
	})
}

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
				"":                "",
				"foo":             "foo",
				"foo.bar":         "foo",
				"foo.bar.baz":     "foo",
				"foo.bar.baz.qux": "foo",
			},
		}
		for langCode, tests := range languageWiseTests {
			language, err := lang.GetLanguage(string(langCode))
			assert.NoError(t, err)
			for moduleName, expected := range tests {
				assert.Equal(t, expected, resolvePackageHint(moduleName, language))
			}
		}
	})
}

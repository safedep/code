package lang

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/stretchr/testify/assert"
)

type ImportExpectations struct {
	filePath string
	imports  []string
}

var resolveLanguageTestcases = []struct {
	filePath             string
	exists               bool
	expectedLanguageCode core.LanguageCode
}{
	{filePath: "test.py", exists: true, expectedLanguageCode: core.LanguageCodePython},
	{filePath: "test.js", exists: true, expectedLanguageCode: core.LanguageCodeJavascript},
	{filePath: "test.cjs", exists: true, expectedLanguageCode: core.LanguageCodeJavascript},
	{filePath: "test.mjs", exists: true, expectedLanguageCode: core.LanguageCodeJavascript},
	{
		filePath:             "test.go",
		exists:               false,
		expectedLanguageCode: "",
	},
	{
		filePath:             "README.md",
		exists:               false,
		expectedLanguageCode: "",
	},
	{
		filePath:             "withoutextension",
		exists:               false,
		expectedLanguageCode: "",
	},
}

func TestResolveLanguage(t *testing.T) {
	t.Run("ResolveLanguage", func(t *testing.T) {
		for _, testcase := range resolveLanguageTestcases {
			l, exists := ResolveLanguageFromPath(testcase.filePath)
			assert.Equal(t, testcase.exists, exists)
			if testcase.exists {
				assert.Equal(t, testcase.expectedLanguageCode, l.Meta().Code)
			} else {
				assert.Nil(t, l)
			}
		}
	})
}

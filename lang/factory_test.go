package lang

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/stretchr/testify/assert"
)

var resolveLanguageTestcases = []struct {
	filePath             string
	exists               bool
	expectedLanguageCode core.LanguageCode
}{
	{filePath: "test.py", exists: true, expectedLanguageCode: core.LanguageCodePython},
	{filePath: "test.js", exists: true, expectedLanguageCode: core.LanguageCodeJavascript},
	{filePath: "test.cjs", exists: true, expectedLanguageCode: core.LanguageCodeJavascript},
	{filePath: "test.mjs", exists: true, expectedLanguageCode: core.LanguageCodeJavascript},
	{filePath: "test.go", exists: true, expectedLanguageCode: core.LanguageCodeGo},
	{filePath: "test.java", exists: true, expectedLanguageCode: core.LanguageCodeJava},
	{filePath: "test.rs", exists: true, expectedLanguageCode: core.LanguageCodeRust},
	{filePath: "test.cs", exists: true, expectedLanguageCode: core.LanguageCodeCSharp},
	{filePath: "test.php", exists: true, expectedLanguageCode: core.LanguageCodePHP},
	{filePath: "test.rb", exists: true, expectedLanguageCode: core.LanguageCodeRuby},
	{filePath: "build.gradle.kts", exists: true, expectedLanguageCode: core.LanguageCodeKotlin},
	{filePath: "test.swift", exists: false, expectedLanguageCode: ""},
	{filePath: "README.md", exists: false, expectedLanguageCode: ""},
	{filePath: "withoutextension", exists: false, expectedLanguageCode: ""},
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

func TestGetLanguage(t *testing.T) {
	t.Run("GetLanguage", func(t *testing.T) {
		allLangs, err := AllLanguages()
		assert.NoError(t, err)
		for _, lang := range allLangs {
			l, err := GetLanguage(string(lang.Meta().Code))
			assert.NoError(t, err)
			assert.Equal(t, lang.Meta().Code, l.Meta().Code)
		}
		_, err = GetLanguage("unknown")
		assert.Error(t, err)
	})
}

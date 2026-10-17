package depsusage

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageProfiles(t *testing.T) {
	codes := []core.LanguageCode{
		core.LanguageCodePython,
		core.LanguageCodeGo,
		core.LanguageCodeJavascript,
		core.LanguageCodeJava,
		core.LanguageCodeTypescript,
		core.LanguageCodeCSharp,
		core.LanguageCodeRust,
		core.LanguageCodePHP,
		core.LanguageCodeRuby,
		core.LanguageCodeKotlin,
	}

	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			profile, ok := usageProfilesByCode[code]
			require.True(t, ok, "the code has a profile")
			assert.Equal(t, code, profile.code)
			assert.NotEmpty(t, profile.evidentNodeTypes)

			_, err := lang.GetLanguage(string(code))
			assert.NoError(t, err, "the library parses the language of the profile")
		})
	}

	assert.Equal(t, codes, NewDependencyUsagePlugin(nil).SupportedLanguages(), "the plugin supports exactly the registered codes")
	assert.Len(t, usageProfilesByCode, len(usageProfiles), "each code has one profile")
}

package callgraph

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/lang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLanguageProfiles(t *testing.T) {
	cases := []struct {
		code            core.LanguageCode
		instanceKeyword string
		builtins        bool
	}{
		{core.LanguageCodePython, "self", true},
		{core.LanguageCodeJava, "this", true},
		{core.LanguageCodeGo, "", true},
		{core.LanguageCodeJavascript, "this", true},
		{core.LanguageCodeTypescript, "this", true},
		{core.LanguageCodeCSharp, "this", true},
		{core.LanguageCodeRust, "self", true},
		{core.LanguageCodePHP, "$this", true},
		{core.LanguageCodeRuby, "self", true},
		{core.LanguageCodeKotlin, "this", true},
	}

	var codes []core.LanguageCode
	for _, tc := range cases {
		codes = append(codes, tc.code)
		t.Run(string(tc.code), func(t *testing.T) {
			profile, ok := profilesByCode[tc.code]
			require.True(t, ok, "the code has a profile")
			assert.Equal(t, tc.code, profile.code)
			assert.NotEmpty(t, profile.separators)
			assert.NotEmpty(t, profile.processors)
			assert.NotEmpty(t, profile.reachable)
			assert.NotEmpty(t, profile.literals)
			assert.NotEmpty(t, profile.ignored)
			assert.NotEmpty(t, profile.argumentList)
			assert.Equal(t, tc.instanceKeyword, profile.instanceKeyword)
			assert.Equal(t, tc.builtins, len(profile.builtins) > 0)

			_, err := lang.GetLanguage(string(tc.code))
			assert.NoError(t, err, "the library parses the language of the profile")
		})
	}

	assert.Equal(t, codes, NewCallGraphPlugin(nil).SupportedLanguages(), "the plugin supports exactly the registered codes")
	assert.Len(t, profilesByCode, len(languageProfiles), "each code has one profile")
}

func TestOnlyTypeScriptFallsBackToOtherSignatures(t *testing.T) {
	for _, profile := range languageProfiles {
		want := core.LanguageCode("")
		if profile.code == core.LanguageCodeTypescript {
			want = core.LanguageCodeJavascript
		}
		assert.Equal(t, want, profile.signatureFallback, profile.code)
	}
}

func TestProfileOfUnknownLanguageHasNoProcessors(t *testing.T) {
	unknown := core.LanguageCode("cobol")
	profile := profileOf(unknown)
	assert.Equal(t, unknown, profile.code)
	assert.Empty(t, profile.processors)
	assert.Empty(t, profile.builtins)
}

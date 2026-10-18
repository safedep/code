package indexer

import (
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryLanguageHasRules(t *testing.T) {
	for _, code := range langcode.All() {
		rules, ok := languageRulesByCode[code]
		require.True(t, ok, "%s has no rules", code)
		assert.NotEmpty(t, rules.kinds, "%s names no definition kinds", code)
		for _, v := range rules.visitors {
			assert.NotNil(t, v, "%s has a nil visitor", code)
		}
	}
	assert.Len(t, languageRulesByCode, len(langcode.All()), "a language with rules has no code")
}

func TestRulesOfUnknownLanguageAreEmpty(t *testing.T) {
	p := &projection{ix: &fileindex.Index{Language: "cobol"}}
	assert.Empty(t, p.rules().kinds)
	assert.Nil(t, p.rules().packageOf)
}

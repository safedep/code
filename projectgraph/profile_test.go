package projectgraph

import (
	"testing"

	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
)

func TestProfiles(t *testing.T) {
	for _, p := range profiles {
		assert.Same(t, p, profileOf(p.code), p.code)
		assert.NotEmpty(t, p.family, p.code)
		assert.NotNil(t, p.resolver, "%s has a module resolver", p.code)
	}
	assert.Equal(t, langcode.Javascript, languageFamily(langcode.Typescript))
	assert.Equal(t, langcode.Ruby, languageFamily(langcode.Ruby), "a language with no profile is a family of its own")
	assert.Same(t, noProfile, profileOf(langcode.Ruby))
	assert.True(t, isInstanceKeyword(langcode.Python, "cls"))
	assert.False(t, isInstanceKeyword(langcode.Go, "this"))
}

func TestModulesOfALanguageWithNoResolver(t *testing.T) {
	b, err := newBuilder(nil, Options{})
	assert.NoError(t, err)
	assert.True(t, b.mayBeFirstParty(langcode.Ruby, "app/models"), "a language with no resolver can name any file")
	assert.False(t, b.mayBeFirstParty(langcode.Python, "yaml"))
}

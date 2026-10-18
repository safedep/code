package projectgraph

import (
	"github.com/safedep/code/fileindex"
	"testing"

	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
)

func TestProfiles(t *testing.T) {
	for _, p := range profiles {
		assert.Same(t, p, profileOf(p.code), p.code)
		assert.NotNil(t, p.resolver, "%s has a module resolver", p.code)
	}
	assert.Same(t, noProfile, profileOf(langcode.Code("swift")))
	assert.True(t, isInstanceKeyword(langcode.Python, "cls"))
	assert.False(t, isInstanceKeyword(langcode.Go, "this"))
}

func TestModulesOfALanguageWithNoResolver(t *testing.T) {
	b, err := newBuilder(nil, Options{})
	assert.NoError(t, err)
	assert.True(t, b.mayBeFirstParty(&fileindex.Index{Path: "app.swift", Language: "swift"}, "app/models"), "a language with no resolver can name any file")
	assert.False(t, b.mayBeFirstParty(&fileindex.Index{Path: "app.py", Language: langcode.Python}, "yaml"))
}

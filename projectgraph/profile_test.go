package projectgraph

import (
	"testing"

	"github.com/safedep/code/fileindex"

	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestLangStateIsOnePerLanguage(t *testing.T) {
	b, err := newBuilder(nil, Options{})
	require.NoError(t, err)
	assert.Same(t, b.goState(), b.goState(), "the second call returns the first state")
	assert.NotNil(t, b.goState().implemented)
	assert.Same(t, b.rustState(), langState[rustState](b, langcode.Rust))
	assert.Nil(t, b.goState().methods, "a table that no lookup filled is empty")
}

func TestAmbiguousSitesKeepTheMethodNameWhereOnlyAMethodRuns(t *testing.T) {
	for _, tt := range []struct {
		lang     langcode.Code
		receiver string
		want     bool
	}{
		{lang: langcode.Go, receiver: "s", want: true},
		{lang: langcode.Go, receiver: "", want: true},
		{lang: langcode.Rust, receiver: "s", want: true},
		{lang: langcode.Rust, receiver: "", want: false},
		{lang: langcode.Python, receiver: "s", want: false},
		{lang: langcode.Javascript, receiver: "s", want: false},
		{lang: langcode.Typescript, receiver: "s", want: false},
		{lang: langcode.Ruby, receiver: "s", want: false},
		{lang: langcode.PHP, receiver: "s", want: false},
	} {
		assert.Equal(t, tt.want, profileOf(tt.lang).keepsAmbiguousMethod(tt.receiver), "%s with the receiver %q", tt.lang, tt.receiver)
	}
}

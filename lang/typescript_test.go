package lang

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/stretchr/testify/assert"
)

func TestTypescriptLanguageMeta(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		l := &typescriptLanguage{}
		assert.Equal(t, typescriptLanguageName, l.Name())
	})

	t.Run("Code", func(t *testing.T) {
		l := &typescriptLanguage{}
		assert.Equal(t, core.LanguageCodeTypescript, l.Meta().Code)
	})

	t.Run("ObjectOriented", func(t *testing.T) {
		l := &typescriptLanguage{}
		assert.True(t, l.Meta().ObjectOriented)
	})

	t.Run("SourceFileExtensions", func(t *testing.T) {
		l := &typescriptLanguage{}
		assert.ElementsMatch(t, []string{".ts", ".tsx", ".mts", ".cts"}, l.Meta().SourceFileExtensions)
	})
}

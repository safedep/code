package lang

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/stretchr/testify/assert"
)

func TestGoLanguageMeta(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		l := &goLanguage{}
		assert.Equal(t, goLanguageName, l.Name())
	})

	t.Run("Code", func(t *testing.T) {
		l := &goLanguage{}
		assert.Equal(t, core.LanguageCodeGo, l.Meta().Code)
	})

	t.Run("ObjectOriented", func(t *testing.T) {
		l := &goLanguage{}
		assert.False(t, l.Meta().ObjectOriented)
	})
}

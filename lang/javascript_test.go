package lang

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/stretchr/testify/assert"
)

func TestJavascriptLanguageMeta(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		l := &javascriptLanguage{}
		assert.Equal(t, javascriptLanguageName, l.Name())
	})

	t.Run("Code", func(t *testing.T) {
		l := &javascriptLanguage{}
		assert.Equal(t, core.LanguageCodeJavascript, l.Meta().Code)
	})

	t.Run("ObjectOriented", func(t *testing.T) {
		l := &javascriptLanguage{}
		assert.True(t, l.Meta().ObjectOriented)
	})
}

package lang

import (
	"testing"

	"github.com/safedep/code/core"
	"github.com/stretchr/testify/assert"
)

func TestPythonLanguageMeta(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		l := &pythonLanguage{}
		assert.Equal(t, pythonLanguageName, l.Name())
	})

	t.Run("Code", func(t *testing.T) {
		l := &pythonLanguage{}
		assert.Equal(t, core.LanguageCodePython, l.Meta().Code)
	})

	t.Run("ObjectOriented", func(t *testing.T) {
		l := &pythonLanguage{}
		assert.True(t, l.Meta().ObjectOriented)
	})
}

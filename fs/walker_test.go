package fs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSourceWalker(t *testing.T) {
	t.Run("NewSourceWalker", func(t *testing.T) {
		t.Run("should return a new SourceWalker", func(t *testing.T) {
			config := SourceWalkerConfig{
				IncludeImports: true,
			}

			result, err := NewSourceWalker(config, nil)
			assert.NoError(t, err)
			assert.NotNil(t, result)
		})
	})
}

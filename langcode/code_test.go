package langcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllHasEveryCodeOnce(t *testing.T) {
	seen := map[Code]bool{}
	for _, c := range All() {
		assert.False(t, seen[c], "duplicate code %q", c)
		seen[c] = true
	}
	assert.Len(t, seen, 10)
}

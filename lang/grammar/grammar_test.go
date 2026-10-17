package grammar

import (
	"os"
	"strings"
	"testing"

	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"
)

func TestVersionMatchesGoModPin(t *testing.T) {
	data, err := os.ReadFile("../../go.mod")
	require.NoError(t, err)

	mod, err := modfile.Parse("go.mod", data, nil)
	require.NoError(t, err)

	pins := map[string]string{}
	for _, r := range mod.Require {
		pins[r.Mod.Path] = r.Mod.Version
	}

	for _, code := range langcode.All() {
		path, version, ok := strings.Cut(Version(code), "@")
		require.True(t, ok, "no grammar version for %q", code)
		assert.Equal(t, pins[path], version, "grammar version of %q differs from the go.mod pin of %s", code, path)
	}
}

func TestVersionOfUnknownLanguageIsEmpty(t *testing.T) {
	assert.Empty(t, Version("cobol"))
}

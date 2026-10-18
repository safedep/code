// Package testutil holds the helpers that the tests of several packages
// share.
package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertGolden compares the indented JSON form of v with the golden file at
// path. With update it writes the file first.
func AssertGolden(t testing.TB, path string, update bool, v any) {
	t.Helper()
	got, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')

	if update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "%s has no golden file: run the test with -update", path)
	assert.Equal(t, string(want), string(got))
}

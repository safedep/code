package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPrintsTheGraph(t *testing.T) {
	dir := t.TempDir()
	cache := t.TempDir()
	for name, data := range map[string]string{
		filepath.Join(dir, "go.mod"):                        "module example.com/app\n\ngo 1.22\n\nrequire example.com/dep v1.0.0\n",
		filepath.Join(dir, "main.go"):                       "package main\n\nimport _ \"example.com/dep\"\n",
		filepath.Join(cache, "example.com/dep@v1.0.0/d.go"): "package dep\n\nimport _ \"fmt\"\n",
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(name), 0o755))
		require.NoError(t, os.WriteFile(name, []byte(data), 0o600))
	}

	var out strings.Builder
	require.NoError(t, run(&out, dir, cache, false))
	assert.Contains(t, out.String(), "reached   imports   example.com/dep@v1.0.0")
	assert.Contains(t, out.String(), "example.com/app -> example.com/dep (imports, main.go:3)")
	assert.Contains(t, out.String(), "example.com/dep -> std (imports, d.go:3)")

	out.Reset()
	require.NoError(t, run(&out, dir, cache, true))
	assert.Contains(t, out.String(), `"Path": "example.com/dep"`)
}

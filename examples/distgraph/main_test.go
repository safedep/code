package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/safedep/code/distgraph"
)

func TestRunPrintsTheGraph(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, ".venv", "lib", "python3.12", "site-packages")
	for name, data := range map[string]string{
		filepath.Join(dir, "main.py"):                              "import requests\n",
		filepath.Join(dir, ".venv", "pyvenv.cfg"):                  "home = /usr/bin\n",
		filepath.Join(sp, "requests", "__init__.py"):               "import urllib3\n",
		filepath.Join(sp, "requests-2.31.0.dist-info", "METADATA"): "Name: requests\nVersion: 2.31.0\n",
		filepath.Join(sp, "requests-2.31.0.dist-info", "RECORD"):   "requests/__init__.py,,\n",
		filepath.Join(sp, "urllib3", "__init__.py"):                "import socket\n",
		filepath.Join(sp, "urllib3-2.0.7.dist-info", "METADATA"):   "Name: urllib3\nVersion: 2.0.7\n",
		filepath.Join(sp, "urllib3-2.0.7.dist-info", "RECORD"):     "urllib3/__init__.py,,\n",
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(name), 0o755))
		require.NoError(t, os.WriteFile(name, []byte(data), 0o600))
	}

	var out strings.Builder
	require.NoError(t, run(&out, dir, "app", []string{sp}, false, false))
	assert.Contains(t, out.String(), "reached   imports   requests@2.31.0")
	assert.Contains(t, out.String(), "app -> requests (imports, main.py:1)")
	assert.Contains(t, out.String(), "urllib3 -> std (imports, urllib3/__init__.py:1)")
	assert.Contains(t, out.String(), "3 files, 2 distributions")

	out.Reset()
	require.NoError(t, run(&out, dir, "", []string{sp}, false, true))
	var g distgraph.Graph
	require.NoError(t, json.Unmarshal([]byte(out.String()), &g))
	assert.Equal(t, filepath.Base(dir), g.Main.Name, "the project takes the name of its directory")
	urllib3, ok := g.Node("urllib3")
	require.True(t, ok)
	assert.True(t, urllib3.Reached)
	assert.Equal(t, distgraph.StateImports, urllib3.State)
	assert.Equal(t, []string{g.Main.Name, "requests", "urllib3"}, g.Path([]string{g.Main.Name}, "urllib3"))
}

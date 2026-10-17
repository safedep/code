package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/safedep/code/projectgraph/reach"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const acceptanceCase = "../../test/projectgraph/cases/python/helper-module/project"

func buildExample(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "projectgraph")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	require.NoError(t, err, string(out))
	return bin
}

func TestExamplePrintsThePath(t *testing.T) {
	bin := buildExample(t)
	store := t.TempDir()

	for _, run := range []string{"cold", "warm"} {
		t.Run(run, func(t *testing.T) {
			out, err := exec.Command(bin, "-dir", acceptanceCase, "-store", store, "-target", "yaml//load").CombinedOutput()
			require.NoError(t, err, string(out))
			assert.Equal(t, "yaml//load: found\n  main.py:3 -> handlers.py:12 -> yaml//load\n", string(out))
		})
	}
	assert.FileExists(t, filepath.Join(store, "local.db"))
}

func TestExamplePrintsJSON(t *testing.T) {
	out, err := exec.Command(buildExample(t), "-dir", acceptanceCase, "-target", "yaml//load", "-target", "yaml//dump", "-json").Output()
	require.NoError(t, err)

	var results []reach.Result
	require.NoError(t, json.Unmarshal(out, &results))
	require.Len(t, results, 2)
	assert.Equal(t, reach.Found, results[0].Status)
	assert.Equal(t, reach.NotFound, results[1].Status)
}

func TestSkipPatterns(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "ci", "build", "app")
	patterns := skipPatterns(root)
	skipped := func(p string) bool {
		for _, re := range patterns {
			if re.MatchString(p) {
				return true
			}
		}
		return false
	}

	tests := []struct {
		path string
		skip bool
	}{
		{filepath.Join(root, "main.py"), false},
		{filepath.Join(root, "src", "builder.py"), false},
		{filepath.Join(root, "node_modules", "yaml", "index.js"), true},
		{filepath.Join(root, "web", "node_modules"), true},
		{filepath.Join(root, ".venv", "lib", "yaml.py"), true},
		{filepath.Join(root, "pkg", "__pycache__", "a.pyc"), true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.skip, skipped(tt.path), tt.path)
	}
}

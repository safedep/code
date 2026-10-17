package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/reach"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	acceptanceCase = "../../test/projectgraph/cases/python/helper-module/project"
	overlayCase    = "../../test/projectgraph/cases/python/llm-overlay"
)

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

func TestExampleMergesAnOverlay(t *testing.T) {
	cmd := exec.Command(buildExample(t), "-dir", filepath.Join(overlayCase, "project"), "-overlay", filepath.Join(overlayCase, "overlay.json"), "-target", "yaml//safe_load")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err, stderr.String())

	assert.Equal(t, "yaml//safe_load: found\n  main.py:9 -> main.py:5 (llm, model test-model-1, prompt v2) -> parsers.py:5 -> yaml//safe_load\n", string(out))
	assert.Equal(t, "overlay edge 1 at main.py:5 from main.py//dispatch to parsers.py//load: undefined-callee\n"+
		"overlay edge 2 at main.py:9 from main.py to parsers.py//parse: no-unresolved-site\n", stderr.String())
}

func TestExampleGivesOneOutputForAnyNumberOfWorkers(t *testing.T) {
	bin := buildExample(t)
	run := func(workers string) string {
		out, err := exec.Command(bin, "-dir", acceptanceCase, "-target", "yaml//load", "-json", "-workers", workers).Output()
		require.NoError(t, err)
		return string(out)
	}
	assert.Equal(t, run("1"), run("4"))
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
		{filepath.Join(root, "build", "app.js"), true},
		{filepath.Join(root, "src", "build", "helper.py"), false},
		{filepath.Join(root, "env"), true},
		{filepath.Join(root, "app", "env", "settings.py"), false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.skip, skipped(tt.path), tt.path)
	}
}

func TestPathTextNamesAWeakHop(t *testing.T) {
	hops := []reach.Hop{
		{Key: "main.py"},
		{Key: "app.py//Parser//parse", Site: projectgraph.Site{Path: "main.py", Line: 3}, EdgeKind: projectgraph.EdgeName, Confidence: projectgraph.ConfidenceLow},
		{Key: "yaml//load", Site: projectgraph.Site{Path: "app.py", Line: 6}, EdgeKind: projectgraph.EdgeDirect, Confidence: projectgraph.ConfidenceHigh},
	}
	assert.Equal(t, "main.py:3 (name) -> app.py:6 -> yaml//load", pathText(hops))

	hops[1].EdgeKind, hops[1].Resolver = projectgraph.EdgeLLM, "model m1, prompt v2"
	assert.Equal(t, "main.py:3 (llm, model m1, prompt v2) -> app.py:6 -> yaml//load", pathText(hops))
}

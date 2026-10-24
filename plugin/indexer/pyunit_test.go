package indexer

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/fileindex/store/memstore"
	"github.com/safedep/code/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexerLeavesOutTheTestsOfAPypiUnit(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"main.py": "import kit\n"})
	site := filepath.Join(dir, "venv", "site-packages")
	writeFiles(t, site, map[string]string{
		"kit/__init__.py":               "from .core import run\n",
		"kit/core.py":                   "def run():\n    pass\n",
		"kit/test/client.py":            "def client():\n    pass\n",
		"kit/tests/test_core.py":        "def test_run():\n    pass\n",
		"kit/conftest.py":               "x = 1\n",
		"kit/core_test.py":              "x = 1\n",
		"kit/__pycache__/core.py":       "x = 1\n",
		"kit-1.0.0.dist-info/setup.py":  "x = 1\n",
		"kit-1.0.0.data/scripts/kit.py": "x = 1\n",
		"kit-1.0.0.dist-info/METADATA":  "Name: kit\nVersion: 1.0.0\n",
	})
	fsys, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories:    []string{dir},
		ImportDirectories: []string{site},
		ImportUnits:       map[string]string{filepath.Join(site, "kit"): "pypi:kit@1.0.0"},
		ExcludePatterns:   []*regexp.Regexp{fs.SkipPattern(dir)},
	})
	require.NoError(t, err)
	result, err := runIndexer(t, context.Background(), fsys, Options{Languages: languages(t, core.LanguageCodePython), Store: memstore.New(), Imports: true})
	require.NoError(t, err)

	var indexed []string
	for p, status := range result.Dependencies {
		_, inner := fileindex.SplitUnit(p)
		assert.Equal(t, fileindex.FileIndexed, status, p)
		indexed = append(indexed, inner)
	}
	var excluded []string
	for _, p := range result.Excluded {
		_, inner := fileindex.SplitUnit(p)
		excluded = append(excluded, inner)
	}
	assert.ElementsMatch(t, []string{"kit/__init__.py", "kit/core.py", "kit/test/client.py"}, indexed)
	assert.ElementsMatch(t, []string{"kit/tests/test_core.py", "kit/conftest.py", "kit/core_test.py", "kit/__pycache__/core.py"}, excluded)
	require.Len(t, result.Files, 1, "a project file keeps its status")
	for _, status := range result.Files {
		assert.Equal(t, fileindex.FileIndexed, status)
	}
}

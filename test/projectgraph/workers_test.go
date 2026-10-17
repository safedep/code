package projectgraph

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/plugin/fileindex"
	"github.com/safedep/code/projectgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// indexAndLink indexes the project with the given workers, links it and
// returns the result of both steps as JSON, so two runs compare byte for
// byte.
func indexAndLink(t testing.TB, project string, languages []core.Language, testPaths []string, workers int) []byte {
	t.Helper()
	ctx := context.Background()
	fsys, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{AppDirectories: []string{project}})
	require.NoError(t, err)
	indexed, err := (&fileindex.Indexer{}).Run(ctx, fsys, fileindex.Options{Languages: languages, Workers: workers})
	require.NoError(t, err)
	g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{
		Root:      os.DirFS(project),
		Files:     indexed.Files,
		TestPaths: testPaths,
	})
	require.NoError(t, err)

	failed := make([]string, len(indexed.Report.Failed))
	for i, f := range indexed.Report.Failed {
		failed[i] = f.Error()
	}
	data, err := json.Marshal(map[string]any{
		"indexes": indexed.Indexes,
		"files":   indexed.Files,
		"visited": indexed.Report.Visited,
		"skipped": indexed.Report.Skipped,
		"failed":  failed,
		"graph":   g,
	})
	require.NoError(t, err)
	return data
}

func caseLanguages(t testing.TB) []core.Language {
	t.Helper()
	languages := make([]core.Language, 0, len(languageCodes))
	for _, code := range languageCodes {
		language, err := lang.GetLanguage(string(code))
		require.NoError(t, err)
		languages = append(languages, language)
	}
	return languages
}

func TestCasesGiveOneOutputForAnyNumberOfWorkers(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("cases", "*", "*", "project"))
	require.NoError(t, err)
	require.NotEmpty(t, dirs)

	languages := caseLanguages(t)
	for _, dir := range dirs {
		t.Run(filepath.ToSlash(filepath.Dir(dir)), func(t *testing.T) {
			serial := indexAndLink(t, dir, languages, nil, 1)
			assert.Equal(t, string(serial), string(indexAndLink(t, dir, languages, nil, 4)))
		})
	}
}

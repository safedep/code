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

// indexAndLink indexes the project and the units of dependency code of the
// expectation with the given workers, links it and returns the result of
// both steps as JSON, so two runs compare byte for byte.
func indexAndLink(t testing.TB, project string, want expectation, languages []core.Language, testPaths []string, workers int) []byte {
	t.Helper()
	ctx := context.Background()
	cfg, roots := fileSystemConfig(project, want)
	fsys, err := fs.NewLocalFileSystem(cfg)
	require.NoError(t, err)
	indexed, err := (&fileindex.Indexer{}).Run(ctx, fsys, fileindex.Options{Languages: languages, Workers: workers, Imports: len(want.Units) > 0})
	require.NoError(t, err)
	g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{
		Root:      os.DirFS(project),
		Files:     indexed.Statuses(),
		TestPaths: testPaths,
		Units:     roots,
	})
	require.NoError(t, err)

	failed := make([]string, len(indexed.Report.Failed))
	for i, f := range indexed.Report.Failed {
		failed[i] = f.Error()
	}
	data, err := json.Marshal(map[string]any{
		"indexes": indexed.Indexes,
		"files":   indexed.Statuses(),
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
			want := readExpectation(t, filepath.Dir(dir))
			serial := indexAndLink(t, dir, want, languages, nil, 1)
			assert.Equal(t, string(serial), string(indexAndLink(t, dir, want, languages, nil, 4)))
		})
	}
}

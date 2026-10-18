package projectgraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/internal/testutil"
	"github.com/safedep/code/projectgraph"
	"github.com/safedep/code/projectgraph/summary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// graphHashes records the hash of the graphs of every case for each pair of
// fileindex.Version and projectgraph.Version. The graph of a case changes
// with a fill rule of the index or a rule of the link step, so a change of
// the hash needs a bump of the Version that owns the rule and a new row. A
// row of an old pair stays as it is.
var graphHashes = map[string]string{
	"10/10": "fa32ae11c72bdf01c93d4b7afe3564b1d59b2436306e3b3503f01fc4c308e5aa",
}

// summaryHashes records the hash of the golden summaries of the cases for
// each triple of fileindex.Version, projectgraph.Version and
// summary.Version.
var summaryHashes = map[string]string{
	"10/10/1": "69c0f5f2082a61a0aceb28b1da7bffb4f662c31f35c6b5913a249c7b43410d25",
}

func TestGraphHashFollowsVersion(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("cases", "*", "*"))
	require.NoError(t, err)
	require.NotEmpty(t, dirs)

	languages := caseLanguages(t)
	h := testutil.NewHasher()
	for _, dir := range dirs {
		_, _, g, _ := caseGraph(t, dir, languages)
		data, err := json.Marshal(g)
		require.NoError(t, err)
		h.Add(t, filepath.ToSlash(dir), data)
	}
	key := fileindex.Version + "/" + projectgraph.Version
	want, ok := graphHashes[key]
	require.True(t, ok, "no row of graphHashes for the versions %s: add one", key)
	assert.Equal(t, want, h.Sum(), "the graphs of the cases changed: bump fileindex.Version or projectgraph.Version and add a row to graphHashes")
}

func TestSummaryHashFollowsVersion(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("cases", "*", "*", "summaries", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, names)

	h := testutil.NewHasher()
	for _, name := range names {
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		h.Add(t, filepath.ToSlash(name), data)
	}
	key := strings.Join([]string{fileindex.Version, projectgraph.Version, summary.Version}, "/")
	want, ok := summaryHashes[key]
	require.True(t, ok, "no row of summaryHashes for the versions %s: add one", key)
	assert.Equal(t, want, h.Sum(), "the golden summaries changed: bump summary.Version and add a row to summaryHashes")
}

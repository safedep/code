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
	"github.com/safedep/code/projectgraph/reach"
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
	"11/10": "421c6441093c8f82e4db8a2342884b5e42faf360951985d3600d99c96202625b",
	"11/11": "19e0527969cf2702f5abfab8234310b254ac25a6a539ab6e86cf6febabfb8757",
	"11/12": "14d5bbbaca07ab950f75c665f48e71ce9195da2ebd853cd1b0b745d42c4852cc",
	"12/12": "1cb170ec83fe3517e6f71755983c040210f83bb7a0e69528b3d7007dcc69d5dc",
	"13/13": "9383a0bc5c6e8b760beb443112cae1400c39212c1da5c2d155c22488eb00eaa6",
	"14/14": "0f3c9c38b39f4de1835f34e8a58bcea85461f37e7aa2d5a2dd484569e670ff48",
	"14/15": "bbc708aba767ccf554c75a7e06816abda3fc1804ad6d0876ad8ce9575f4c3a88",
	"14/16": "823b1f82c4aed8ba27d19275505c8d1dee755086be48fcda64d323f55b4c62a5",
	"15/17": "ca24a262671d86221ad25aad86efbf27c7efa3750335dc0f6c8999ba23ff2cf3",
}

// resultHashes records the hash of the reach results of every case for
// each triple of fileindex.Version, projectgraph.Version and
// reach.Version. A result changes with a rule of any of the three steps.
var resultHashes = map[string]string{
	"11/10/1": "4beb6248ef1b8dc3572939a41d1aea11c6504f5007a5f24fd1261db026708186",
	"11/11/1": "4beb6248ef1b8dc3572939a41d1aea11c6504f5007a5f24fd1261db026708186",
	"11/11/2": "58d8fd9c5b67bf02a44778a5173828515c9424a1fddd9ff3f644d8fc13ebcfbb",
	"11/12/2": "8341f2a1965f9d3b19d976ff1ba63ed26c1e15026e02079ec6195dfc0d762dba",
	"12/12/2": "b89a142078d885eeb73fe15c731315c60d42b38270efd12a7857be44b9ccc83c",
	"13/13/2": "2f81b00e1708cbb7e0589f34eaa7866a5658a6cfc0a7608d532381ca4267ba06",
	"14/14/2": "a832ea709bf3df852814375a56e6329c35e2ff4c49ae34ee49afe042e9502d17",
	"14/15/3": "77dfd0ea8ab71869ef612d41b8c2e46852d4fb67a49458cb645321692af77ea0",
	"14/16/3": "13b6eecb686ddec3c2ba470fc9752ea66cff445d5e18e0772a71b15ae7fe1c9c",
	"15/17/4": "96f5a284da37b626dda6eb66c6d031145d9ee2b2bc737a1ec2da402b51b28c5b",
}

// summaryHashes records the hash of the golden summaries of the cases for
// each triple of fileindex.Version, projectgraph.Version and
// summary.Version.
var summaryHashes = map[string]string{
	"10/10/1": "69c0f5f2082a61a0aceb28b1da7bffb4f662c31f35c6b5913a249c7b43410d25",
	"11/10/1": "f3c6c7e7f2390a0c8ec81ab6673b6b8a02cadf95e0ca36dc963d57b72834589c",
	"11/10/2": "8047cc4e4d95ce6744993a20c7f4224a78b2d3531c2e31415f371b16608f19a5",
	"11/11/2": "c0aeb7f515b2c546fd6b355663d5a7bb446c7f6bdfb9ab3ef343dd252e4302bf",
	"11/12/2": "f4102bd40a876fa387f7ff1ce81505e1f04b64cd8165602d0726f897e2b88a7f",
	"12/12/2": "cb2147fda018110f38254519bbc75a814b99e0318f5c7485fbce78a667bb4287",
	"13/13/2": "dc844b1cd13e7873f54d646f27384c78b64707e13ae9c9ef27a4557ba7ba4dfc",
	"14/14/2": "27424c4e2525450479faba78421959738748811d70b9a93b81e2bbdf1733bf0d",
	"14/15/2": "5969f3cda15b666cbd902bbc2db953cbdef8c0ce48d5e77446cf243cdd0bb0e7",
	"14/16/2": "fb678202978212aae932e50a56a0d3dc7f0f794ce908988bf605ce9a6e04bc51",
	"15/17/2": "69d9b0f80c4f5811cc07608a7a1f224499e5f4e3b3eb5b62d2c2cf842df142e7",
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

func TestResultHashFollowsVersion(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("cases", "*", "*"))
	require.NoError(t, err)
	require.NotEmpty(t, dirs)

	languages := caseLanguages(t)
	h := testutil.NewHasher()
	for _, dir := range dirs {
		want, _, g, _ := caseGraph(t, dir, languages)
		data, err := json.Marshal(caseResults(t, g, want))
		require.NoError(t, err)
		h.Add(t, filepath.ToSlash(dir), data)
	}
	key := strings.Join([]string{fileindex.Version, projectgraph.Version, reach.Version}, "/")
	want, ok := resultHashes[key]
	require.True(t, ok, "no row of resultHashes for the versions %s: add one", key)
	assert.Equal(t, want, h.Sum(), "the reach results of the cases changed: bump the Version that owns the rule and add a row to resultHashes")
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

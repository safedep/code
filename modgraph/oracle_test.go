package modgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/safedep/code/fileindex/store/memstore"
)

// The oracle test compares the graph with the module edges of go list. Each
// edge that go list gives must be in the graph, unless the node that it
// leaves is open, and the node that it reaches must be reached. go list
// runs for each platform pair below, with the test packages of the main
// module. The test runs on this repository. MODGRAPH_ORACLE_DIRS adds more
// projects, as a list of directories that filepath.SplitList reads. A
// project with vendor/modules.txt reads its dependencies from vendor. The
// test needs the go command and a module cache that holds the build list,
// and skips with neither. It never downloads: GOPROXY is off.

var oraclePlatforms = [][2]string{
	{"linux", "amd64"},
	{"darwin", "arm64"},
	{"windows", "amd64"},
	{"js", "wasm"},
}

func TestBuildHoldsEachEdgeOfGoList(t *testing.T) {
	if testing.Short() {
		t.Skip("the oracle test runs the go command")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	dirs := []string{root}
	for _, d := range filepath.SplitList(os.Getenv("MODGRAPH_ORACLE_DIRS")) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			sources, ok := oracleSources(t, dir)
			if !ok {
				t.Skip("the module cache does not hold the build list")
			}
			st := memstore.New()
			start := time.Now()
			g, err := Build(context.Background(), Input{MainFS: os.DirFS(dir), Modules: sources, Store: st})
			require.NoError(t, err)
			cold := time.Since(start)
			start = time.Now()
			warm, err := Build(context.Background(), Input{MainFS: os.DirFS(dir), Modules: sources, Store: st})
			require.NoError(t, err)
			assert.Equal(t, g.Edges, warm.Edges, "the cache gives the same graph")
			t.Logf("Build: %d files, %d packages, %v; with the cache: %d files, %d cached packages, %v",
				g.Stats.Files, g.Stats.Packages, cold.Round(time.Millisecond), warm.Stats.Files, warm.Stats.CachedPackages, time.Since(start).Round(time.Millisecond))
			for _, n := range g.Nodes {
				if n.Open() && n.Reached && !strings.HasPrefix(n.Reason, "only an open node") {
					t.Logf("open: %s: %s", n.Module.Path, n.Reason)
				}
			}
			edges := map[[2]string]bool{}
			for _, e := range g.Edges {
				edges[[2]string{e.From.Path, e.To.Path}] = true
			}
			for _, p := range oraclePlatforms {
				listed := goListEdges(t, dir, p[0], p[1])
				t.Logf("%s/%s: go list has %d module edges, the graph %d, with %d nodes", p[0], p[1], len(listed), len(g.Edges), len(g.Nodes))
				for e := range listed {
					from, _ := g.Node(e[0])
					to, ok := g.Node(e[1])
					assert.True(t, ok && to.Reached, "%s/%s: go list reaches %s, the graph does not", p[0], p[1], e[1])
					assert.True(t, edges[e] || from.Open(), "%s/%s: go list has the edge %s -> %s", p[0], p[1], e[0], e[1])
				}
			}

			// With no source, each node holds only its go.mod, and each module
			// that go list reaches must still be reached.
			for i := range sources {
				sources[i].FS = nil
			}
			g, err = Build(context.Background(), Input{MainFS: os.DirFS(dir), Modules: sources})
			require.NoError(t, err)
			for e := range goListEdges(t, dir, "linux", "amd64") {
				to, ok := g.Node(e[1])
				assert.True(t, ok && to.Reached, "go.mod only: go list reaches %s, the graph does not", e[1])
			}
		})
	}
}

// oracleSources finds the sources with Discover and the module cache of
// the go command.
func oracleSources(t *testing.T, dir string) ([]Source, bool) {
	t.Helper()
	out, err := goCommand(dir, nil, "env", "GOMODCACHE")
	if err != nil {
		t.Logf("go env: %v", err)
		return nil, false
	}
	sources, err := Discover(DiscoverOptions{Dir: dir, ModuleCache: strings.TrimSpace(string(out)), LocalRoot: dir})
	require.NoError(t, err)
	return sources, true
}

type listPackage struct {
	ImportPath string
	Standard   bool
	Module     *struct{ Path string }
	Imports    []string
}

// goListEdges returns the module edges of the packages that go list -deps
// -test gives for a platform.
func goListEdges(t *testing.T, dir, goos, goarch string) map[[2]string]bool {
	t.Helper()
	out, err := goCommand(dir, []string{"GOOS=" + goos, "GOARCH=" + goarch, "CGO_ENABLED=1"}, "list", "-e", "-deps", "-test", "-json", "./...")
	require.NoError(t, err, "go list -deps")
	var pkgs []listPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p listPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else {
			require.NoError(t, err)
		}
		pkgs = append(pkgs, p)
	}
	moduleOf := map[string]string{}
	for _, p := range pkgs {
		switch {
		case p.Standard:
			moduleOf[p.ImportPath] = StdPath
		case p.Module != nil:
			moduleOf[p.ImportPath] = p.Module.Path
		}
	}
	edges := map[[2]string]bool{}
	for _, p := range pkgs {
		from, ok := moduleOf[p.ImportPath]
		if !ok || from == StdPath {
			continue
		}
		for _, imp := range p.Imports {
			if to, ok := moduleOf[imp]; ok && to != from && imp != "C" {
				edges[[2]string{from, to}] = true
			}
		}
	}
	return edges
}

func goCommand(dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=", "GOWORK=off")
	cmd.Env = append(cmd.Env, env...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Join(err, errors.New(stderr.String()))
	}
	return out, nil
}

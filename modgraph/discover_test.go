package modgraph

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(data), 0o600))
	}
}

func sourceOf(t *testing.T, sources []Source, path string) Source {
	t.Helper()
	for _, s := range sources {
		if s.Module.Path == path {
			return s
		}
	}
	require.Failf(t, "no source", "%s", path)
	return Source{}
}

func hasFile(t *testing.T, fsys fs.FS, name string) bool {
	t.Helper()
	require.NotNil(t, fsys)
	_, err := fs.Stat(fsys, name)
	return err == nil
}

func TestDiscoverFindsEachSourceAsTheGoCommandBuildsIt(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	writeFiles(t, root, map[string]string{
		"app/go.mod": `module example.com/app

go 1.22

require (
	example.com/cached v1.0.0
	example.com/modonly v1.0.0
	example.com/missing v1.0.0
	github.com/Upper/case v1.2.0
	example.com/vendored v1.1.0
	example.com/local v1.0.0
	example.com/outside v1.0.0
	example.com/forked v1.0.0
	example.com/exact v1.0.0
)

replace example.com/local => ../lib
replace example.com/outside => ../../elsewhere
replace example.com/forked => example.com/fork v2.0.0
replace example.com/exact v0.9.0 => example.com/never v1.0.0
`,
		"app/go.sum":                           "example.com/cached v1.0.0 h1:source=\nexample.com/cached v1.0.0/go.mod h1:gomod=\n",
		"app/vendor/modules.txt":               "# example.com/vendored v1.1.0\n## explicit; go 1.21\nexample.com/vendored\n",
		"app/vendor/example.com/vendored/v.go": "package vendored\n",
		"lib/go.mod":                           "module example.com/local\n\ngo 1.22\n",
		"lib/l.go":                             "package local\n",
	})
	writeFiles(t, cache, map[string]string{
		"example.com/cached@v1.0.0/c.go":                   "package cached\n",
		"cache/download/example.com/cached/@v/v1.0.0.mod":  "module example.com/cached\n",
		"cache/download/example.com/modonly/@v/v1.0.0.mod": "module example.com/modonly\n",
		"github.com/!upper/case@v1.2.0/u.go":               "package case\n",
		"example.com/fork@v2.0.0/f.go":                     "package fork\n",
		"example.com/exact@v1.0.0/e.go":                    "package exact\n",
	})

	sources, err := Discover(DiscoverOptions{Dir: filepath.Join(root, "app"), ModuleCache: cache, LocalRoot: root})
	require.NoError(t, err)
	require.Len(t, sources, 9)

	cached := sourceOf(t, sources, "example.com/cached")
	assert.True(t, hasFile(t, cached.FS, "c.go"))
	assert.True(t, cached.Immutable)
	assert.Equal(t, "h1:source=", cached.Sum, "the hash of the source, not of its go.mod")
	assert.Equal(t, "module example.com/cached\n", string(cached.GoMod))

	modonly := sourceOf(t, sources, "example.com/modonly")
	assert.Nil(t, modonly.FS)
	assert.Equal(t, "module example.com/modonly\n", string(modonly.GoMod))

	missing := sourceOf(t, sources, "example.com/missing")
	assert.Nil(t, missing.FS)
	assert.Nil(t, missing.GoMod)

	assert.True(t, hasFile(t, sourceOf(t, sources, "github.com/Upper/case").FS, "u.go"), "the module cache escapes upper case")

	vendored := sourceOf(t, sources, "example.com/vendored")
	assert.True(t, hasFile(t, vendored.FS, "v.go"))
	assert.False(t, vendored.Immutable, "a vendor directory can change")

	local := sourceOf(t, sources, "example.com/local")
	assert.True(t, hasFile(t, local.FS, "l.go"))
	assert.False(t, local.Immutable)
	assert.Equal(t, Module{Path: "example.com/local", Version: "v1.0.0"}, local.Module, "the node keeps the path that imports use")

	assert.Nil(t, sourceOf(t, sources, "example.com/outside").FS, "a local replacement outside the root is not read")

	forked := sourceOf(t, sources, "example.com/forked")
	assert.True(t, hasFile(t, forked.FS, "f.go"))
	assert.Equal(t, "example.com/forked", forked.Module.Path)
	assert.Equal(t, Module{Path: "example.com/fork", Version: "v2.0.0"}, forked.Origin, "the cache keys the fork by its own module")
	assert.Equal(t, Module{}, cached.Origin)

	assert.True(t, hasFile(t, sourceOf(t, sources, "example.com/exact").FS, "e.go"), "a replace of another version does not apply")
}

func TestDiscoverReadsAVendorDirectoryOnlyAtTheVersionOfGoMod(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                    "module example.com/app\n\ngo 1.22\n\nrequire example.com/v v1.2.0\n",
		"vendor/modules.txt":        "# example.com/v v1.1.0\nexample.com/v\n",
		"vendor/example.com/v/v.go": "package v\n",
	})
	sources, err := Discover(DiscoverOptions{Dir: root})
	require.NoError(t, err)
	assert.Nil(t, sources[0].FS, "a vendor copy of another version proves nothing")
}

func TestDiscoverReadsALocalReplacementFromVendorInVendorMode(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"app/go.mod":                        "module example.com/app\n\ngo 1.22\n\nrequire example.com/local v1.0.0\n\nreplace example.com/local => ../lib\n",
		"app/vendor/modules.txt":            "# example.com/local v1.0.0 => ../lib\nexample.com/local\n",
		"app/vendor/example.com/local/v.go": "package local\n",
		"lib/l.go":                          "package local\n",
	})
	sources, err := Discover(DiscoverOptions{Dir: filepath.Join(root, "app"), LocalRoot: root})
	require.NoError(t, err)
	assert.True(t, hasFile(t, sources[0].FS, "v.go"))
	assert.False(t, hasFile(t, sources[0].FS, "l.go"))
}

func TestDiscoverDoesNotFollowALinkOutOfTheLocalRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFiles(t, root, map[string]string{
		"app/go.mod": "module example.com/app\n\ngo 1.22\n\nrequire example.com/local v1.0.0\n\nreplace example.com/local => ./lib\n",
	})
	writeFiles(t, outside, map[string]string{"l.go": "package local\n"})
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "app", "lib")))
	sources, err := Discover(DiscoverOptions{Dir: filepath.Join(root, "app"), LocalRoot: root})
	require.NoError(t, err)
	assert.Nil(t, sources[0].FS)
}

func TestDiscoverConfinesEachReadOfALocalReplacement(t *testing.T) {
	cases := []struct {
		name     string
		imp      string
		link     func(t *testing.T, lib, outside string)
		wantOpen bool
	}{
		{"a link to a file outside the root", "example.com/local", func(t *testing.T, lib, outside string) {
			require.NoError(t, os.Symlink(filepath.Join(outside, "o.go"), filepath.Join(lib, "o.go")))
		}, true},
		{"a link to a package outside the root", "example.com/local/sub", func(t *testing.T, lib, outside string) {
			require.NoError(t, os.Symlink(outside, filepath.Join(lib, "sub")))
		}, true},
		{"a link inside the root", "example.com/local", func(t *testing.T, lib, _ string) {
			require.NoError(t, os.Symlink(filepath.Join(lib, "real.go"), filepath.Join(lib, "o.go")))
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			writeFiles(t, root, map[string]string{
				"app/go.mod":      "module example.com/app\n\ngo 1.22\n\nrequire example.com/local v1.0.0\n\nreplace example.com/local => ./lib\n",
				"app/main.go":     "package main\n\nimport _ \"" + tc.imp + "\"\n",
				"app/lib/go.mod":  "module example.com/local\n",
				"app/lib/real.go": "package local\n",
			})
			writeFiles(t, outside, map[string]string{"o.go": "package local\n\nimport _ \"example.com/secret\"\n"})
			tc.link(t, filepath.Join(root, "app", "lib"), outside)

			dir := filepath.Join(root, "app")
			sources, err := Discover(DiscoverOptions{Dir: dir, LocalRoot: root})
			require.NoError(t, err)
			require.NotNil(t, sources[0].FS)
			g, err := Build(context.Background(), Input{MainFS: os.DirFS(dir), Modules: sources})
			require.NoError(t, err)
			n, ok := g.Node("example.com/local")
			require.True(t, ok)
			assert.Equal(t, tc.wantOpen, n.State == StateOpen, n.Reason)
			assert.Equal(t, StateImports, node(t, g, "example.com/app").State)
			assert.NotContains(t, n.Reason, "example.com/secret", "Build reads no file outside the root")
		})
	}
}

func TestDiscoverNeedsAGoMod(t *testing.T) {
	_, err := Discover(DiscoverOptions{Dir: t.TempDir()})
	assert.ErrorContains(t, err, "read go.mod")
}

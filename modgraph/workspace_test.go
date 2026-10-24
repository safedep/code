package modgraph

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cachedModule(path, version, gomod string, files map[string]string) map[string]string {
	out := map[string]string{"cache/download/" + path + "/@v/" + version + ".mod": gomod}
	for name, data := range files {
		out[path+"@"+version+"/"+name] = data
	}
	return out
}

func mergeFiles(sets ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, s := range sets {
		for k, v := range s {
			out[k] = v
		}
	}
	return out
}

func TestDiscoverGivesTheBuildListOfTheWorkspace(t *testing.T) {
	root, cache := t.TempDir(), t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.work":     "go 1.22\n\nuse (\n\t./a\n\t./b\n)\n",
		"a/go.mod":    "module example.com/a\n\ngo 1.22\n\nrequire (\n\texample.com/x v1.0.0\n\texample.com/b v0.1.0\n)\n",
		"a/a.go":      "package a\n\nimport _ \"example.com/b\"\n",
		"b/go.mod":    "module example.com/b\n\ngo 1.22\n\nrequire (\n\texample.com/x v1.2.0\n\texample.com/y v1.0.0\n\texample.com/old v1.0.0\n)\n",
		"b/b.go":      "package b\n\nimport _ \"example.com/x\"\n",
		"b/go.sum":    "example.com/x v1.2.0 h1:x=\n",
		"go.work.sum": "example.com/y v1.0.0 h1:y=\n",
	})
	writeFiles(t, cache, mergeFiles(
		cachedModule("example.com/x", "v1.0.0", "module example.com/x\n\ngo 1.22\n", map[string]string{"x.go": "package x\n"}),
		cachedModule("example.com/x", "v1.2.0", "module example.com/x\n\ngo 1.22\n", map[string]string{"x.go": "package x\n\nimport _ \"example.com/z\"\n"}),
		cachedModule("example.com/y", "v1.0.0", "module example.com/y\n\ngo 1.22\n\nrequire example.com/z v1.1.0\n", nil),
		cachedModule("example.com/old", "v1.0.0", "module example.com/old\n\ngo 1.16\n\nrequire example.com/w v1.0.0\n", nil),
		cachedModule("example.com/w", "v1.0.0", "module example.com/w\n\ngo 1.22\n\nrequire example.com/q v1.3.0\n", nil),
		cachedModule("example.com/b", "v0.1.0", "module example.com/b\n\ngo 1.22\n", nil),
		cachedModule("example.com/q", "v1.3.0", "module example.com/q\n\ngo 1.22\n", nil),
		cachedModule("example.com/z", "v1.1.0", "module example.com/z\n\ngo 1.22\n", map[string]string{"z.go": "package z\n"}),
	))

	sources, err := Discover(DiscoverOptions{Dir: filepath.Join(root, "a"), Work: filepath.Join(root, "go.work"), ModuleCache: cache, LocalRoot: root})
	require.NoError(t, err)
	got := map[string]string{}
	for _, s := range sources {
		got[s.Module.Path] = s.Module.Version
	}
	assert.Equal(t, map[string]string{
		"example.com/b":   "",
		"example.com/x":   "v1.2.0",
		"example.com/y":   "v1.0.0",
		"example.com/z":   "v1.1.0",
		"example.com/old": "v1.0.0",
		"example.com/w":   "v1.0.0",
		"example.com/q":   "v1.3.0",
	}, got, "the highest version in the graph, the go.mod of a requirement, and the go.mod chain below go 1.17")

	b := sourceOf(t, sources, "example.com/b")
	assert.True(t, hasFile(t, b.FS, "b.go"), "another module of the workspace is local code")
	assert.Equal(t, Module{Path: filepath.ToSlash(resolved(filepath.Join(root, "b")))}, b.Origin)
	assert.Equal(t, "h1:x=", sourceOf(t, sources, "example.com/x").Sum, "each go.sum of the workspace gives a hash")
	assert.Equal(t, "h1:y=", sourceOf(t, sources, "example.com/y").Sum, "go.work.sum gives a hash")

	g, err := Build(context.Background(), Input{MainFS: os.DirFS(filepath.Join(root, "a")), Modules: sources})
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com/a", "example.com/b", "example.com/x", "example.com/z"}, g.Path([]string{"example.com/a"}, "example.com/z"),
		"the code of x at the version of the workspace imports z")
}

func TestDiscoverAppliesTheReplacementsOfTheWorkspace(t *testing.T) {
	cases := []struct {
		name    string
		work    string
		modA    string
		modB    string
		want    Module
		wantErr string
	}{
		{
			name: "go.work wins over a module",
			work: "replace example.com/x => example.com/workfork v1.0.0\n",
			modA: "replace example.com/x v1.0.0 => example.com/modfork v1.0.0\n",
			want: Module{Path: "example.com/workfork", Version: "v1.0.0"},
		},
		{
			name: "a local replace of go.work is relative to its directory",
			work: "replace example.com/x => ./fork\n",
			want: Module{Path: "./fork"},
		},
		{
			name: "a replace of one module",
			modB: "replace example.com/x => example.com/modfork v1.0.0\n",
			want: Module{Path: "example.com/modfork", Version: "v1.0.0"},
		},
		{
			name:    "two modules replace it with other code",
			modA:    "replace example.com/x => example.com/modfork v1.0.0\n",
			modB:    "replace example.com/x => example.com/workfork v1.0.0\n",
			wantErr: "replace example.com/x@v1.0.0 with other code",
		},
		{
			name: "two modules replace it with one directory",
			modA: "replace example.com/x => ../fork\n",
			modB: "replace example.com/x => ./../fork\n",
			want: Module{Path: "./../fork"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, cache := t.TempDir(), t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.work":      "go 1.22\n\nuse ./a\nuse ./b\n" + tc.work,
				"a/go.mod":     "module example.com/a\n\ngo 1.22\n\nrequire example.com/x v1.0.0\n" + tc.modA,
				"b/go.mod":     "module example.com/b\n\ngo 1.22\n" + tc.modB,
				"fork/go.mod":  "module example.com/x\n\ngo 1.22\n",
				"fork/fork.go": "package x\n",
			})
			writeFiles(t, cache, mergeFiles(
				cachedModule("example.com/workfork", "v1.0.0", "module example.com/x\n\ngo 1.22\n", map[string]string{"x.go": "package x\n"}),
				cachedModule("example.com/modfork", "v1.0.0", "module example.com/x\n\ngo 1.22\n", map[string]string{"x.go": "package x\n"}),
			))
			sources, err := Discover(DiscoverOptions{Dir: filepath.Join(root, "a"), Work: filepath.Join(root, "go.work"), ModuleCache: cache, LocalRoot: root})
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			x := sourceOf(t, sources, "example.com/x")
			assert.Equal(t, tc.want, x.Origin)
			assert.NotNil(t, x.FS)
		})
	}
}

func TestDiscoverReadsTheVendorDirectoryOfTheWorkspace(t *testing.T) {
	cases := []struct {
		name     string
		work     string
		modules  string
		vendored bool
	}{
		{"go work vendor wrote it", "go 1.21", "## workspace\n# example.com/v v1.0.0\n## explicit; go 1.21\nexample.com/v\n", true},
		{"a vendor directory of one module", "go 1.22", "# example.com/v v1.0.0\n## explicit; go 1.21\nexample.com/v\n", false},
		{"a go.work before go 1.14", "go 1.13", "## workspace\n# example.com/v v1.0.0\nexample.com/v\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, cache := t.TempDir(), t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.work":                   tc.work + "\n\nuse ./a\n",
				"a/go.mod":                  "module example.com/a\n\ngo 1.21\n\nrequire example.com/v v1.0.0\n",
				"vendor/modules.txt":        tc.modules,
				"vendor/example.com/v/v.go": "package v\n",
			})
			writeFiles(t, cache, cachedModule("example.com/v", "v1.0.0", "module example.com/v\n\ngo 1.21\n", map[string]string{"c.go": "package v\n"}))
			sources, err := Discover(DiscoverOptions{Dir: filepath.Join(root, "a"), Work: filepath.Join(root, "go.work"), ModuleCache: cache, LocalRoot: root})
			require.NoError(t, err)
			v := sourceOf(t, sources, "example.com/v")
			assert.Equal(t, "v1.0.0", v.Module.Version)
			assert.Equal(t, tc.vendored, hasFile(t, v.FS, "v.go"))
			assert.Equal(t, !tc.vendored, hasFile(t, v.FS, "c.go"))
		})
	}
}

func TestDiscoverReadsAReplacedModuleFromTheVendorDirectoryOfTheWorkspace(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.work":  "go 1.22\n\nuse ./a\n",
		"a/go.mod": "module example.com/a\n\ngo 1.22\n\nrequire example.com/x v1.0.0\n\nreplace example.com/x => ../fork\n",
		// go work vendor writes the local path relative to go.work, and adds
		// a line for the replace of each version.
		"vendor/modules.txt":        "## workspace\n# example.com/x v1.0.0 => ./fork\n## explicit; go 1.22\nexample.com/x\n# example.com/x => ./fork\n",
		"vendor/example.com/x/x.go": "package x\n",
		"fork/go.mod":               "module example.com/x\n\ngo 1.22\n",
		"fork/f.go":                 "package x\n",
	})
	sources, err := Discover(DiscoverOptions{Dir: filepath.Join(root, "a"), Work: filepath.Join(root, "go.work"), LocalRoot: root})
	require.NoError(t, err)
	x := sourceOf(t, sources, "example.com/x")
	assert.Equal(t, "v1.0.0", x.Module.Version, "the wildcard line keeps the version")
	assert.True(t, hasFile(t, x.FS, "x.go"), "the go command builds the vendor copy")
}

func TestDiscoverSkipsTheVendorDirectoryOfAWorkspaceOutsideWorkspaceMode(t *testing.T) {
	root, cache := t.TempDir(), t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                    "module example.com/a\n\ngo 1.22\n\nrequire example.com/v v1.0.0\n",
		"vendor/modules.txt":        "## workspace\n# example.com/v v1.0.0\nexample.com/v\n",
		"vendor/example.com/v/v.go": "package v\n",
	})
	writeFiles(t, cache, cachedModule("example.com/v", "v1.0.0", "module example.com/v\n", map[string]string{"c.go": "package v\n"}))
	sources, err := Discover(DiscoverOptions{Dir: root, ModuleCache: cache})
	require.NoError(t, err)
	assert.True(t, hasFile(t, sources[0].FS, "c.go"))
}

func TestDiscoverRefusesAWorkspaceWhoseBuildListItCannotKnow(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		dir     string
		wantErr string
	}{
		{
			name:    "a go.mod that the graph needs is missing",
			files:   map[string]string{"go.work": "go 1.22\n\nuse ./a\n", "a/go.mod": "module example.com/a\n\ngo 1.22\n\nrequire example.com/gone v1.0.0\n"},
			wantErr: "no go.mod of example.com/gone@v1.0.0 is on the disk",
		},
		{
			name:    "an exclude directive",
			files:   map[string]string{"go.work": "go 1.22\n\nuse ./a\n", "a/go.mod": "module example.com/a\n\ngo 1.22\n\nexclude example.com/x v1.0.0\n"},
			wantErr: "exclude directive",
		},
		{
			name:    "a workspace that does not use the module",
			files:   map[string]string{"go.work": "go 1.22\n\nuse ./b\n", "a/go.mod": "module example.com/a\n\ngo 1.22\n", "b/go.mod": "module example.com/b\n\ngo 1.22\n"},
			wantErr: "does not use the module",
		},
		{
			name:    "a module of the workspace with no go.mod",
			files:   map[string]string{"go.work": "go 1.22\n\nuse ./a\nuse ./b\n", "a/go.mod": "module example.com/a\n\ngo 1.22\n"},
			wantErr: "no go.mod inside the local root",
		},
		{
			name:    "a go.work that does not parse",
			files:   map[string]string{"go.work": "go 1.22\n\nuse (\n", "a/go.mod": "module example.com/a\n\ngo 1.22\n"},
			wantErr: "parse go.work",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, tc.files)
			_, err := Discover(DiscoverOptions{Dir: filepath.Join(root, "a"), Work: filepath.Join(root, "go.work"), ModuleCache: t.TempDir(), LocalRoot: root})
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestDiscoverReadsNoWorkspaceModuleOutsideTheLocalRoot(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	writeFiles(t, outside, map[string]string{"go.mod": "module example.com/b\n\ngo 1.22\n", "b.go": "package b\n"})
	writeFiles(t, root, map[string]string{
		"go.work":  "go 1.22\n\nuse ./a\nuse " + filepath.ToSlash(outside) + "\n",
		"a/go.mod": "module example.com/a\n\ngo 1.22\n",
	})
	_, err := Discover(DiscoverOptions{Dir: filepath.Join(root, "a"), Work: filepath.Join(root, "go.work"), LocalRoot: root})
	assert.ErrorContains(t, err, "no go.mod inside the local root", "its requirements are part of the build list")
}

func TestDiscoverTakesRelativePaths(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"ws/go.work":  "go 1.22\n\nuse ./a\nuse ./b\n",
		"ws/a/go.mod": "module example.com/a\n\ngo 1.22\n",
		"ws/b/go.mod": "module example.com/b\n\ngo 1.22\n",
		"ws/b/b.go":   "package b\n",
		"other/x.txt": "",
	})
	t.Chdir(filepath.Join(root, "other"))
	sources, err := Discover(DiscoverOptions{Dir: "../ws/a", Work: "../ws/go.work", LocalRoot: "../ws"})
	require.NoError(t, err)
	assert.True(t, hasFile(t, sourceOf(t, sources, "example.com/b").FS, "b.go"))
}

package modgraph

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomodule "golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

// proxyModule is one module version of a test proxy, with its go.mod.
type proxyModule struct {
	path, version, gomod string
}

// writeProxy writes a module proxy that a file:// GOPROXY serves: a list,
// an info file, a go.mod and a zip for each module version.
func writeProxy(t *testing.T, dir string, mods []proxyModule) {
	t.Helper()
	versions := map[string][]string{}
	for _, m := range mods {
		esc, err := gomodule.EscapePath(m.path)
		require.NoError(t, err)
		at := filepath.Join(dir, filepath.FromSlash(esc), "@v")
		src := t.TempDir()
		name := m.path[strings.LastIndex(m.path, "/")+1:]
		writeFiles(t, src, map[string]string{"go.mod": m.gomod, name + ".go": "package " + name + "\n"})
		writeFiles(t, at, map[string]string{
			m.version + ".info": `{"Version":"` + m.version + `","Time":"2026-01-01T00:00:00Z"}`,
			m.version + ".mod":  m.gomod,
		})
		f, err := os.Create(filepath.Join(at, m.version+".zip"))
		require.NoError(t, err)
		require.NoError(t, modzip.CreateFromDir(f, gomodule.Version{Path: m.path, Version: m.version}, src))
		require.NoError(t, f.Close())
		versions[at] = append(versions[at], m.version)
	}
	for at, vs := range versions {
		writeFiles(t, at, map[string]string{"list": strings.Join(vs, "\n") + "\n"})
	}
}

func mod(path, version string, requires ...string) proxyModule {
	gomod := "module " + path + "\n\ngo 1.22\n"
	for _, r := range requires {
		gomod += "\nrequire " + r + "\n"
	}
	return proxyModule{path: path, version: version, gomod: gomod}
}

// The go command decides the build list of a workspace. Each case runs it
// on a proxy on the disk, and compares Discover with go list -m all.
func TestDiscoverGivesTheBuildListOfTheGoCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("the test runs the go command")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	cases := []struct {
		name    string
		files   map[string]string
		modules []proxyModule
	}{
		{
			name: "a module of the workspace requires a lower version than another",
			files: map[string]string{
				"go.work":  "go 1.22\n\nuse ./a\nuse ./b\n",
				"a/go.mod": "module example.com/a\n\ngo 1.22\n\nrequire (\n\texample.com/x v1.0.0\n\texample.com/z v1.0.0\n)\n",
				"b/go.mod": "module example.com/b\n\ngo 1.22\n\nrequire example.com/x v1.2.0\n",
			},
			modules: []proxyModule{
				mod("example.com/x", "v1.0.0"),
				mod("example.com/x", "v1.2.0", "example.com/y v1.0.0"),
				mod("example.com/y", "v1.0.0", "example.com/z v1.5.0"),
				mod("example.com/z", "v1.0.0"),
				mod("example.com/z", "v1.5.0"),
			},
		},
		{
			name: "a requirement on a module of the workspace at a version",
			files: map[string]string{
				"go.work":  "go 1.22\n\nuse ./a\nuse ./b\n",
				"a/go.mod": "module example.com/a\n\ngo 1.22\n\nrequire (\n\texample.com/b v0.1.0\n\texample.com/z v1.0.0\n)\n",
				"b/go.mod": "module example.com/b\n\ngo 1.22\n\nrequire example.com/z v1.0.0\n",
			},
			modules: []proxyModule{
				mod("example.com/b", "v0.1.0", "example.com/z v1.5.0"),
				mod("example.com/z", "v1.0.0"),
				mod("example.com/z", "v1.5.0"),
			},
		},
		{
			name: "a module of the workspace below go 1.17",
			files: map[string]string{
				"go.work":  "go 1.22\n\nuse ./a\nuse ./c\n",
				"a/go.mod": "module example.com/a\n\ngo 1.22\n\nrequire (\n\texample.com/x v1.0.0\n\texample.com/z v1.0.0\n)\n",
				"c/go.mod": "module example.com/c\n\ngo 1.16\n\nrequire example.com/x v1.0.0\n",
			},
			modules: []proxyModule{
				mod("example.com/x", "v1.0.0", "example.com/y v1.0.0"),
				mod("example.com/y", "v1.0.0", "example.com/z v1.5.0"),
				mod("example.com/z", "v1.0.0"),
				mod("example.com/z", "v1.5.0"),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, proxy, cache := t.TempDir(), t.TempDir(), t.TempDir()
			writeFiles(t, root, tc.files)
			writeProxy(t, proxy, tc.modules)
			work := filepath.Join(root, "go.work")
			env := []string{
				"GOWORK=" + work, "GOMODCACHE=" + cache, "GOPROXY=file://" + filepath.ToSlash(proxy),
				"GOSUMDB=off", "GONOSUMDB=*", "GOFLAGS=-modcacherw",
			}
			dir := filepath.Join(root, "a")
			_, err := goCommand(dir, env, "mod", "download", "all")
			require.NoError(t, err)
			want := goListBuildList(t, dir, env)
			sources, err := Discover(DiscoverOptions{Dir: dir, Work: work, ModuleCache: cache, LocalRoot: root})
			require.NoError(t, err)
			got := buildListOf("example.com/a", sources)
			assert.Equal(t, want, got)
		})
	}
}

package modgraph

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/safedep/code/fileindex/store"
	"github.com/safedep/code/fileindex/store/memstore"
)

const mainPath = "example.com/app"

// goSrc returns a Go file of a package with the imports.
func goSrc(pkg string, imports ...string) *fstest.MapFile {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	for _, imp := range imports {
		fmt.Fprintf(&b, "import _ %q\n", imp)
	}
	return &fstest.MapFile{Data: []byte(b.String())}
}

func mainFS(files fstest.MapFS) fstest.MapFS {
	return withGoMod(files, "module "+mainPath+"\n\ngo 1.22\n")
}

func withGoMod(files fstest.MapFS, gomod string) fstest.MapFS {
	out := fstest.MapFS{"go.mod": {Data: []byte(gomod)}}
	for k, v := range files {
		out[k] = v
	}
	return out
}

func source(path string, files fstest.MapFS) Source {
	return Source{Module: Module{Path: path, Version: "v1.0.0"}, FS: files}
}

func build(t *testing.T, in Input) *Graph {
	t.Helper()
	g, err := Build(context.Background(), in)
	require.NoError(t, err)
	return g
}

// edgeList writes each edge as "from -> to" with a mark for a test-only
// edge or a require edge.
func edgeList(g *Graph) []string {
	var out []string
	for _, e := range g.Edges {
		s := e.From.Path + " -> " + e.To.Path
		if e.TestOnly {
			s += " (test)"
		}
		if e.Kind == StateRequires {
			s += " (requires)"
		}
		out = append(out, s)
	}
	return out
}

func node(t *testing.T, g *Graph, path string) Node {
	t.Helper()
	n, ok := g.Node(path)
	require.True(t, ok, "no node %s", path)
	return n
}

func TestBuildFollowsTheImportsOfTheReachedPackages(t *testing.T) {
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "fmt", "example.com/a/x")}),
		Modules: []Source{
			source("example.com/a", fstest.MapFS{
				"x/x.go": goSrc("x", "example.com/b"),
				"y/y.go": goSrc("y", "example.com/c"),
			}),
			source("example.com/b", fstest.MapFS{"b.go": goSrc("b", "net/http")}),
			source("example.com/c", fstest.MapFS{"c.go": goSrc("c")}),
		},
	})

	assert.Equal(t, []string{
		"example.com/a -> example.com/b",
		"example.com/app -> example.com/a",
		"example.com/app -> std",
		"example.com/b -> std",
	}, edgeList(g), "the package y of a is not reached, so its import of c gives no edge")
	assert.True(t, node(t, g, "example.com/b").Reached)
	assert.False(t, node(t, g, "example.com/c").Reached)
	assert.Equal(t, StateImports, node(t, g, "example.com/c").State)
	assert.Equal(t, Evidence{File: "x/x.go", Line: 3}, g.Edges[0].Evidence)
	assert.Equal(t, Version, g.Version)
	assert.Equal(t, Module{Path: mainPath}, g.Main)
}

func TestBuildReadsEachPackageOfTheMainModule(t *testing.T) {
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{
			"main.go":                      goSrc("main"),
			"third_party/forked/y/wrap.go": goSrc("y", "example.com/y"),
			"vendor/example.com/z/z.go":    goSrc("z", "example.com/z"),
			"testdata/t.go":                goSrc("t", "example.com/z"),
			"_old/o.go":                    goSrc("o", "example.com/z"),
			".hidden/h.go":                 goSrc("h", "example.com/z"),
			"_skip.go":                     goSrc("main", "example.com/z"),
			"tools/go.mod":                 {Data: []byte("module example.com/app/tools\n\ngo 1.22\n")},
			"tools/tools.go":               goSrc("tools", "example.com/z"),
		}),
		Modules: []Source{
			source("example.com/y", fstest.MapFS{"y.go": goSrc("y")}),
			source("example.com/z", fstest.MapFS{"z.go": goSrc("z")}),
		},
	})

	assert.Equal(t, []string{"example.com/app -> example.com/y"}, edgeList(g))
	assert.False(t, node(t, g, "example.com/z").Reached)
}

func TestBuildMarksTheTestOnlyEdgesOfTheMainModule(t *testing.T) {
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{
			"a.go":      goSrc("app", "example.com/both"),
			"a_test.go": goSrc("app", "example.com/test", "example.com/both"),
		}),
		Modules: []Source{
			source("example.com/both", fstest.MapFS{"b.go": goSrc("both")}),
			source("example.com/test", fstest.MapFS{"t.go": goSrc("test"), "t_test.go": goSrc("test", "example.com/onlytests")}),
			source("example.com/onlytests", fstest.MapFS{"o.go": goSrc("onlytests")}),
		},
	})

	assert.Equal(t, []string{
		"example.com/app -> example.com/both",
		"example.com/app -> example.com/test (test)",
	}, edgeList(g), "the test files of a dependency never count")
	assert.True(t, node(t, g, "example.com/test").Reached)
	assert.False(t, node(t, g, "example.com/onlytests").Reached)
}

func TestBuildResolvesAnImportPath(t *testing.T) {
	cases := []struct {
		name    string
		imp     string
		modules []Source
		want    []string
	}{
		{
			name: "the longest module path that has the directory",
			imp:  "github.com/a/b/c/d",
			modules: []Source{
				source("github.com/a/b", fstest.MapFS{"c/d/d.go": goSrc("d")}),
				source("github.com/a/b/c", fstest.MapFS{"c.go": goSrc("c")}),
			},
			want: []string{"example.com/app -> github.com/a/b"},
		},
		{
			name: "a longer module with no source may hold the package",
			imp:  "github.com/a/b/c/d",
			modules: []Source{
				source("github.com/a/b", fstest.MapFS{"c/d/d.go": goSrc("d")}),
				{Module: Module{Path: "github.com/a/b/c", Version: "v0.1.0"}},
			},
			want: []string{"example.com/app -> github.com/a/b", "example.com/app -> github.com/a/b/c"},
		},
		{
			name: "a module path with no dot is not the standard library",
			imp:  "corp/lib/x",
			modules: []Source{
				source("corp/lib", fstest.MapFS{"x/x.go": goSrc("x")}),
			},
			want: []string{"example.com/app -> corp/lib"},
		},
		{
			name: "a module root",
			imp:  "gopkg.in/yaml.v3",
			modules: []Source{
				source("gopkg.in/yaml.v3", fstest.MapFS{"yaml.go": goSrc("yaml")}),
			},
			want: []string{"example.com/app -> gopkg.in/yaml.v3"},
		},
		{
			name: "a nested module of a dependency is not its package",
			imp:  "github.com/a/b/sub",
			modules: []Source{
				source("github.com/a/b", fstest.MapFS{"sub/go.mod": {Data: []byte("module github.com/a/b/sub\n")}, "sub/s.go": goSrc("sub")}),
				source("github.com/a/b/sub", fstest.MapFS{"s.go": goSrc("sub")}),
			},
			want: []string{"example.com/app -> github.com/a/b/sub"},
		},
		{
			name:    "cgo",
			imp:     "C",
			modules: nil,
			want:    nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := build(t, Input{MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", tc.imp)}), Modules: tc.modules})
			assert.Equal(t, tc.want, edgeList(g))
			assert.Equal(t, StateImports, node(t, g, mainPath).State)
		})
	}
}

func TestBuildOpensAModuleThatImportsWhatItCannotResolve(t *testing.T) {
	cases := []struct {
		name string
		imp  string
	}{
		{"a path of no module", "example.com/missing/x"},
		{"a package directory that its module does not have", "example.com/a/missing"},
		{"a relative path", "./local"},
		{"a directory of a module nested in the dependency", "example.com/a/sub"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := build(t, Input{
				MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
				Modules: []Source{
					source("example.com/a", fstest.MapFS{
						"a.go":       goSrc("a", tc.imp),
						"sub/go.mod": {Data: []byte("module example.com/a/sub\n")},
						"sub/s.go":   goSrc("sub"),
					}),
					source("example.com/unused", fstest.MapFS{"u.go": goSrc("unused")}),
				},
			})
			a := node(t, g, "example.com/a")
			assert.Equal(t, StateOpen, a.State)
			assert.Contains(t, a.Reason, tc.imp)
			unused := node(t, g, "example.com/unused")
			assert.True(t, unused.Reached, "an open node can reach each module")
			assert.Equal(t, StateOpen, unused.State, "no walk read its packages")
			assert.True(t, g.Reaches([]string{mainPath}, "example.com/unused"))
		})
	}
}

func TestBuildReadsTheGoModOfAModuleWithNoSource(t *testing.T) {
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
		Modules: []Source{
			{Module: Module{Path: "example.com/a", Version: "v1.0.0"}, GoMod: []byte("module example.com/a\n\ngo 1.21\n\nrequire (\n\texample.com/b v1.0.0\n\texample.com/outside v1.0.0\n)\n")},
			source("example.com/b", fstest.MapFS{"b.go": goSrc("b")}),
			source("example.com/c", fstest.MapFS{"c.go": goSrc("c")}),
		},
	})

	assert.Equal(t, []string{
		"example.com/a -> example.com/b (requires)",
		"example.com/app -> example.com/a",
	}, edgeList(g), "a require of a module outside the build list gives no edge")
	a := node(t, g, "example.com/a")
	assert.Equal(t, StateRequires, a.State)
	assert.True(t, a.Open(), "go.mod need not list each module that the code imports")
	assert.True(t, node(t, g, "example.com/c").Reached)
	assert.Equal(t, Evidence{File: "go.mod", Line: 6}, g.Edges[0].Evidence)
}

func TestBuildOpensAModuleWithNoSourceAndNoGoMod(t *testing.T) {
	g := build(t, Input{
		MainFS:  mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
		Modules: []Source{{Module: Module{Path: "example.com/a", Version: "v1.0.0"}}},
	})
	a := node(t, g, "example.com/a")
	assert.Equal(t, StateOpen, a.State)
	assert.True(t, a.Reached)
}

func TestBuildKeepsAFileUnlessItsConstraintCanNeverHold(t *testing.T) {
	cases := []struct {
		name, file, src string
		counts          bool
	}{
		{"no constraint", "a.go", "package a\n\nimport _ \"example.com/dep\"\n", true},
		{"ignore", "a.go", "//go:build ignore\n\npackage a\n\nimport _ \"example.com/dep\"\n", false},
		{"ignore with a platform", "a.go", "//go:build ignore && linux\n\npackage a\n\nimport _ \"example.com/dep\"\n", false},
		{"a tag that the go command never sets", "a.go", "//go:build purego\n\npackage a\n\nimport _ \"example.com/dep\"\n", true},
		{"a contradiction", "a.go", "//go:build linux && !linux\n\npackage a\n\nimport _ \"example.com/dep\"\n", false},
		{"a file name suffix never drops a file", "a_windows.go", "//go:build !windows\n\npackage a\n\nimport _ \"example.com/dep\"\n", true},
		{"a +build line", "a.go", "// +build ignore\n\npackage a\n\nimport _ \"example.com/dep\"\n", false},
		{"a +build line with no space", "a.go", "//+build ignore\n\npackage a\n\nimport _ \"example.com/dep\"\n", false},
		{"+build lines that each hold", "a.go", "// +build linux darwin\n// +build amd64\n\npackage a\n\nimport _ \"example.com/dep\"\n", true},
		{"+build lines that cannot hold together", "a.go", "// +build linux\n// +build !linux\n\npackage a\n\nimport _ \"example.com/dep\"\n", false},
		{"a //go:build line wins over a +build line", "a.go", "//go:build linux\n// +build ignore\n\npackage a\n\nimport _ \"example.com/dep\"\n", true},
		{"a +build line of the package doc comment", "a.go", "// +build ignore\npackage a\n\nimport _ \"example.com/dep\"\n", true},
		{"a //go:build line of the package doc comment", "a.go", "//go:build ignore\npackage a\n\nimport _ \"example.com/dep\"\n", false},
		{"a +build line after a block comment", "a.go", "/* license */\n\n// +build ignore\n\npackage a\n\nimport _ \"example.com/dep\"\n", true},
		{"a +build line before a block comment", "a.go", "// +build ignore\n/* x */\n\npackage a\n\nimport _ \"example.com/dep\"\n", true},
		{"a //go:build line after a block comment", "a.go", "/* license */\n//go:build ignore\n\npackage a\n\nimport _ \"example.com/dep\"\n", false},
		{"two //go:build lines", "a.go", "//go:build ignore\n//go:build ignore\n\npackage a\n\nimport _ \"example.com/dep\"\n", true},
		{"a constraint that does not parse", "a.go", "//go:build linux &&\n\npackage a\n\nimport _ \"example.com/dep\"\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := build(t, Input{
				MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
				Modules: []Source{
					source("example.com/a", fstest.MapFS{tc.file: {Data: []byte(tc.src)}}),
					source("example.com/dep", fstest.MapFS{"d.go": goSrc("dep")}),
				},
			})
			assert.Equal(t, tc.counts, slices.Contains(edgeList(g), "example.com/a -> example.com/dep"))
		})
	}
}

func TestSatisfiableCountsAConstraintWithManyTags(t *testing.T) {
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
		Modules: []Source{
			source("example.com/a", fstest.MapFS{"a.go": {Data: []byte("//go:build ignore && (t1 || t2 || t3 || t4 || t5 || t6 || t7 || t8 || t9 || t10 || t11 || t12 || t13 || t14 || t15 || t16 || t17)\n\npackage a\n\nimport _ \"example.com/dep\"\n")}}),
			source("example.com/dep", fstest.MapFS{"d.go": goSrc("dep")}),
		},
	})
	assert.Contains(t, edgeList(g), "example.com/a -> example.com/dep", "past the tag cap the file counts")
}

func TestBuildReadsEachImportOfALongFile(t *testing.T) {
	long := "package a\n\nimport _ \"example.com/first\"\n\n// " + strings.Repeat("x", 70<<10) + "\n\nimport _ \"example.com/dep\"\n"
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
		Modules: []Source{
			source("example.com/a", fstest.MapFS{"a.go": {Data: []byte(long)}}),
			source("example.com/first", fstest.MapFS{"f.go": goSrc("first")}),
			source("example.com/dep", fstest.MapFS{"d.go": goSrc("dep")}),
		},
	})
	assert.Contains(t, edgeList(g), "example.com/a -> example.com/dep", "an import after a long comment counts")
	assert.Contains(t, edgeList(g), "example.com/a -> example.com/first")
}

func TestBuildReadsTheLinksOfTheMainModule(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"app/go.mod":       "module " + mainPath + "\n\ngo 1.22\n",
		"app/main.go":      "package main\n\nimport _ \"" + mainPath + "/linked\"\n",
		"shared/s.go":      "package real\n\nimport _ \"example.com/viafile\"\n",
		"outside/o.go":     "package linked\n\nimport _ \"example.com/viadir\"\n",
		"app/real/keep.go": "package real\n",
	})
	require.NoError(t, os.Symlink(filepath.Join(root, "shared", "s.go"), filepath.Join(root, "app", "real", "link.go")))
	require.NoError(t, os.Symlink(filepath.Join(root, "outside"), filepath.Join(root, "app", "linked")))
	g := build(t, Input{
		MainFS: os.DirFS(filepath.Join(root, "app")),
		Modules: []Source{
			source("example.com/viafile", fstest.MapFS{"v.go": goSrc("viafile")}),
			source("example.com/viadir", fstest.MapFS{"v.go": goSrc("viadir")}),
		},
	})
	assert.Contains(t, edgeList(g), mainPath+" -> example.com/viafile", "a link to a file is a file of the package")
	assert.Contains(t, edgeList(g), mainPath+" -> example.com/viadir", "a package that an import reaches through a link to a directory is read")
	assert.Equal(t, StateImports, node(t, g, mainPath).State)
}

func TestBuildReadsATestdataPackageThatAnImportNames(t *testing.T) {
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{
			"main.go":               goSrc("main", mainPath+"/testdata/fixture"),
			"testdata/fixture/f.go": goSrc("fixture", "example.com/dep"),
		}),
		Modules: []Source{source("example.com/dep", fstest.MapFS{"d.go": goSrc("dep")})},
	})
	assert.Equal(t, []string{mainPath + " -> example.com/dep"}, edgeList(g))
	assert.Equal(t, StateImports, node(t, g, mainPath).State)
}

func TestBuildOpensAModuleWithAFileThatDoesNotParse(t *testing.T) {
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
		Modules: []Source{
			source("example.com/a", fstest.MapFS{"a.go": {Data: []byte("package a\n\nimport (\n")}}),
		},
	})
	a := node(t, g, "example.com/a")
	assert.Equal(t, StateOpen, a.State)
	assert.Contains(t, a.Reason, "does not parse")
}

func TestBuildNeedsAPrunedGoMod(t *testing.T) {
	cases := []struct {
		name, gomod string
		err         error
	}{
		{"go 1.16", "module " + mainPath + "\n\ngo 1.16\n", ErrUnpruned},
		{"no go directive", "module " + mainPath + "\n", ErrUnpruned},
		{"go 1.17", "module " + mainPath + "\n\ngo 1.17\n", nil},
		{"go 1.25.1", "module " + mainPath + "\n\ngo 1.25.1\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(context.Background(), Input{MainFS: withGoMod(fstest.MapFS{"main.go": goSrc("main")}, tc.gomod)})
			if tc.err == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.err)
			}
		})
	}
}

func TestBuildRejectsABadInput(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		msg  string
	}{
		{"no main module", Input{}, "no main module"},
		{"no go.mod", Input{MainFS: fstest.MapFS{"main.go": goSrc("main")}}, "read the go.mod"},
		{"no module directive", Input{MainFS: withGoMod(nil, "go 1.22\n")}, "no module directive"},
		{"a module twice", Input{MainFS: mainFS(nil), Modules: []Source{source("example.com/a", nil), source("example.com/a", nil)}}, "twice"},
		{"the reserved std path", Input{MainFS: mainFS(nil), Modules: []Source{source(StdPath, nil)}}, "reserved"},
		{"a module with no path", Input{MainFS: mainFS(nil), Modules: []Source{{}}}, "no path"},
		{"the main module in the build list", Input{MainFS: mainFS(nil), Modules: []Source{source(mainPath, nil)}}, "twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(context.Background(), tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.msg)
		})
	}
}

func TestBuildOpensAModulePastTheFileBudget(t *testing.T) {
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
		Modules: []Source{
			source("example.com/a", fstest.MapFS{"a.go": goSrc("a")}),
		},
		MaxFiles: 1,
	})
	assert.True(t, g.Stats.Truncated)
	assert.Equal(t, StateOpen, node(t, g, "example.com/a").State)
	assert.Equal(t, 1, g.Stats.Files)
}

func TestBuildDoesNotDependOnTheOrderOfTheBuildList(t *testing.T) {
	mods := []Source{
		source("example.com/a", fstest.MapFS{"a.go": goSrc("a", "example.com/b", "example.com/c")}),
		source("example.com/b", fstest.MapFS{"b.go": goSrc("b", "example.com/c")}),
		source("example.com/c", fstest.MapFS{"c.go": goSrc("c")}),
	}
	in := Input{MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a", "example.com/b")}), Modules: mods}
	want := build(t, in)
	slices.Reverse(in.Modules)
	assert.Equal(t, want, build(t, in))
}

func TestBuildCachesThePackagesOfAnImmutableSource(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	immutable := func(s Source) Source { s.Immutable = true; return s }
	in := Input{
		MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a", "example.com/v")}),
		Modules: []Source{
			immutable(source("example.com/a", fstest.MapFS{"a.go": goSrc("a", "example.com/b")})),
			immutable(source("example.com/b", fstest.MapFS{"b.go": goSrc("b")})),
			source("example.com/v", fstest.MapFS{"v.go": goSrc("v")}),
		},
		Store: st,
	}
	first := build(t, in)
	assert.Equal(t, 4, first.Stats.Files)
	assert.Equal(t, 0, first.Stats.CachedPackages)

	second := build(t, in)
	assert.Equal(t, 2, second.Stats.Files, "the main module and the mutable source are read again")
	assert.Equal(t, 2, second.Stats.CachedPackages)
	assert.Equal(t, first.Edges, second.Edges)

	in.Modules[0].Sum = "h1:other"
	third := build(t, in)
	assert.Equal(t, 1, third.Stats.CachedPackages, "another go.sum hash is another entry")

	found, err := st.GetArtifacts(ctx, []store.ArtifactKey{cacheKey(pkgRef{mod: &module{src: in.Modules[2]}, dir: "."})})
	require.NoError(t, err)
	assert.Empty(t, found, "a mutable source has no entry")
}

func TestBuildCachesAReplacementByItsOwnModule(t *testing.T) {
	st := memstore.New()
	original := Source{Module: Module{Path: "example.com/a", Version: "v1.0.0"}, FS: fstest.MapFS{"a.go": goSrc("a")}, Immutable: true}
	fork := Source{Module: Module{Path: "example.com/a", Version: "v1.0.0"}, Origin: Module{Path: "example.com/fork", Version: "v1.0.0"}, FS: fstest.MapFS{"a.go": goSrc("a", "example.com/dep")}, Immutable: true}
	dep := source("example.com/dep", fstest.MapFS{"d.go": goSrc("dep")})
	main := mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")})

	build(t, Input{MainFS: main, Modules: []Source{original, dep}, Store: st})
	g := build(t, Input{MainFS: main, Modules: []Source{fork, dep}, Store: st})
	assert.Contains(t, edgeList(g), "example.com/a -> example.com/dep", "the fork is not served the imports of the original")
}

func TestBuildCachesNoPackageThatFailed(t *testing.T) {
	st := memstore.New()
	bad := Source{Module: Module{Path: "example.com/a", Version: "v1.0.0"}, FS: fstest.MapFS{"a.go": {Data: []byte("package a\nimport (\n")}}, Immutable: true}
	in := Input{MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}), Modules: []Source{bad}, Store: st}
	build(t, in)
	g := build(t, in)
	assert.Equal(t, 0, g.Stats.CachedPackages)
	assert.Equal(t, StateOpen, node(t, g, "example.com/a").State)
}

func TestGraphPath(t *testing.T) {
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
		Modules: []Source{
			source("example.com/a", fstest.MapFS{"a.go": goSrc("a", "example.com/b")}),
			source("example.com/b", fstest.MapFS{"b.go": goSrc("b")}),
			source("example.com/c", fstest.MapFS{"c.go": goSrc("c", "example.com/b")}),
		},
	})
	assert.Equal(t, []string{mainPath, "example.com/a", "example.com/b"}, g.Path([]string{mainPath}, "example.com/b"))
	assert.True(t, g.Reaches([]string{"example.com/a"}, "example.com/b"))
	assert.False(t, g.Reaches([]string{"example.com/b"}, "example.com/a"))
	assert.False(t, g.Reaches([]string{mainPath}, "example.com/c"))
	assert.False(t, g.Reaches([]string{"example.com/b"}, "example.com/b"), "a module is not on a path from itself")
	assert.Equal(t, []string{"example.com/c", "example.com/b"}, g.Path([]string{"example.com/c"}, "example.com/b"),
		"Build read no package of c, so its edges are not known and it can reach each module")
	assert.False(t, node(t, g, "example.com/c").Walked)
	assert.True(t, node(t, g, "example.com/a").Walked)
}

func TestImportsEndNeedsAWholeTokenAfterTheImports(t *testing.T) {
	cases := []struct {
		name string
		head string
		want bool
	}{
		{"a declaration after the imports", "package a\n\nimport _ \"x\"\n\nfunc f() {}\n", true},
		{"a cut inside a comment", "package a\n\nimport _ \"x\"\n\n// comment", false},
		{"a cut inside the word import", "package a\n\nimport _ \"x\"\n\nimp", false},
		{"a cut after the word import, which does not parse", "package a\n\nimport _ \"x\"\n\nimport", false},
		{"a cut inside a word that is not import", "package a\n\nimport _ \"x\"\n\nfun", false},
		{"no import and a declaration", "package a\n\nvar v = 1\n", true},
		{"a cut inside a block comment, which does not parse", "package a\n\nimport _ \"x\"\n\n/* open", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "a.go", tc.head, parser.ImportsOnly)
			if strings.Contains(tc.name, "does not parse") {
				assert.Error(t, err, "Build reads the whole file")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, importsEnd(fset, f, []byte(tc.head)))
		})
	}
}

func TestBuildReadsAnImportThatTheHeadCuts(t *testing.T) {
	prefix := "package a\n\nimport _ \"example.com/first\"\n// "
	src := prefix + strings.Repeat("x", headSize-3-len(prefix)-1) + "\nimport _ \"example.com/dep\"\n"
	require.Equal(t, "imp", src[headSize-3:headSize], "the head ends inside the word import")
	g := build(t, Input{
		MainFS: mainFS(fstest.MapFS{"main.go": goSrc("main", "example.com/a")}),
		Modules: []Source{
			source("example.com/a", fstest.MapFS{"a.go": {Data: []byte(src)}}),
			source("example.com/first", fstest.MapFS{"f.go": goSrc("first")}),
			source("example.com/dep", fstest.MapFS{"d.go": goSrc("dep")}),
		},
	})
	assert.Contains(t, edgeList(g), "example.com/a -> example.com/dep")
}

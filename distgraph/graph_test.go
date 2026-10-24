package distgraph

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// py returns a Python file with one import statement per module.
func py(imports ...string) *fstest.MapFile {
	var b strings.Builder
	for _, imp := range imports {
		b.WriteString("import " + imp + "\n")
	}
	return &fstest.MapFile{Data: []byte(b.String())}
}

func text(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

// sitePackages builds a site-packages directory. Each distribution is a
// name, a version, the files of its RECORD and the Requires-Dist lines of
// its METADATA. The files get the content of the files map, or an empty
// file.
func sitePackages(files fstest.MapFS, dists ...distInfo) fstest.MapFS {
	out := fstest.MapFS{}
	for k, v := range files {
		out[k] = v
	}
	for _, d := range dists {
		dir := strings.ReplaceAll(d.name, "-", "_") + "-" + d.version + ".dist-info"
		meta := "Name: " + d.name + "\nVersion: " + d.version + "\n"
		for _, r := range d.requires {
			meta += "Requires-Dist: " + r + "\n"
		}
		out[dir+"/METADATA"] = text(meta)
		record := ""
		for _, f := range d.files {
			record += f + ",,\n"
			if _, ok := out[f]; !ok && strings.HasSuffix(f, ".py") {
				out[f] = py()
			}
		}
		out[dir+"/RECORD"] = text(record)
	}
	return out
}

type distInfo struct {
	name, version string
	files         []string
	requires      []string
}

func build(t *testing.T, in Input) *Graph {
	t.Helper()
	g, err := Build(context.Background(), in)
	require.NoError(t, err)
	return g
}

// edgeList writes each edge as "from -> to" with a mark for a test-only
// edge or a requires edge.
func edgeList(g *Graph) []string {
	var out []string
	for _, e := range g.Edges {
		s := e.From.Name + " -> " + e.To.Name
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

func node(t *testing.T, g *Graph, name string) Node {
	t.Helper()
	n, ok := g.Node(name)
	require.True(t, ok, "no node %s", name)
	return n
}

func TestBuildFollowsTheImportsOfTheReachedDistributions(t *testing.T) {
	project := fstest.MapFS{
		"app/__init__.py": py(),
		"app/main.py":     py("os", "requests", "app.util", "yaml"),
		"app/util.py":     py("json"),
	}
	sp := sitePackages(fstest.MapFS{
		"requests/__init__.py": py("urllib3", "requests.api"),
		"requests/api.py":      text("try:\n    import simplejson as json\nexcept ImportError:\n    import json\n"),
		"urllib3/__init__.py":  py("socket"),
		"six.py":               py("yaml"),
		"yaml/__init__.py":     py(),
	},
		distInfo{name: "requests", version: "2.31.0", files: []string{"requests/__init__.py", "requests/api.py"}},
		distInfo{name: "urllib3", version: "2.0.7", files: []string{"urllib3/__init__.py"}},
		distInfo{name: "six", version: "1.16.0", files: []string{"six.py"}},
		distInfo{name: "PyYAML", version: "6.0.1", files: []string{"yaml/__init__.py"}},
	)
	g := build(t, Input{ProjectFS: project, ProjectName: "app", SitePackages: []fs.FS{sp}})

	assert.Equal(t, Distribution{Name: "app"}, g.Main)
	assert.Equal(t, []string{"app -> PyYAML", "app -> requests", "app -> std", "requests -> std", "requests -> urllib3", "urllib3 -> std"}, edgeList(g),
		"a guarded import of a module that nothing holds gives no edge, and a module of the project gives none")
	for _, name := range []string{"app", "requests", "urllib3", "PyYAML", "std"} {
		n := node(t, g, name)
		assert.Equal(t, StateImports, n.State, name)
		assert.True(t, n.Reached, name)
		assert.True(t, n.Walked, name)
	}
	six := node(t, g, "six")
	assert.False(t, six.Reached, "no import reaches six")
	assert.False(t, six.Walked, "Build does not read a distribution that no import reaches")
	assert.Equal(t, StateImports, six.State)
	assert.Equal(t, 4, g.Stats.Distributions)
	assert.Equal(t, 7, g.Stats.Files, "the three files of the project and the four of the reached distributions")
	assert.Equal(t, Evidence{File: "app/main.py", Line: 2}, g.Edges[1].Evidence)
	assert.Equal(t, []string{"app", "requests", "urllib3"}, g.Path([]string{"app"}, "urllib3"))
	assert.Nil(t, g.Path([]string{"app"}, "six"))
	assert.Equal(t, "2.0.7", node(t, g, "URLLIB3").Distribution.Version, "a node is found by any spelling of its name")
}

func TestBuildMarksTheTestOnlyEdgesOfTheProject(t *testing.T) {
	project := fstest.MapFS{
		"app.py":              py("requests"),
		"tests/test_app.py":   py("pytest", "requests"),
		"tests/conftest.py":   py("six"),
		"src/pkg/__init__.py": py("pkg.inner"),
		"src/pkg/inner.py":    py(),
	}
	sp := sitePackages(nil,
		distInfo{name: "requests", version: "2.31.0", files: []string{"requests/__init__.py"}},
		distInfo{name: "pytest", version: "7.0.0", files: []string{"pytest/__init__.py"}},
		distInfo{name: "six", version: "1.16.0", files: []string{"six.py"}},
	)
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}})
	assert.Equal(t, []string{"project -> pytest (test)", "project -> requests", "project -> six (test)"}, edgeList(g),
		"an edge that a test file and a program file give is no test-only edge, and a package under src is a name of the project")
	assert.Equal(t, "project", g.Main.Name, "the project node has a default name")
}

func TestBuildResolvesANamespacePackage(t *testing.T) {
	project := fstest.MapFS{
		"main.py":  py("google.cloud.storage", "google.auth.crypt"),
		"other.py": py("google"),
	}
	sp := sitePackages(nil,
		distInfo{name: "google-auth", version: "2.0.0", files: []string{"google/auth/__init__.py", "google/auth/crypt.py"}},
		distInfo{name: "google-cloud-storage", version: "2.1.0", files: []string{"google/cloud/storage/__init__.py"}},
		distInfo{name: "google-cloud-core", version: "2.3.0", files: []string{"google/cloud/__init__.py"}},
	)
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}})
	assert.Equal(t, []string{"project -> google-auth", "project -> google-cloud-core", "project -> google-cloud-storage"}, edgeList(g))
	assert.Equal(t, Evidence{File: "main.py", Line: 1}, g.Edges[2].Evidence, "google.cloud.storage goes to the distribution whose RECORD holds the module")
	assert.Equal(t, Evidence{File: "other.py", Line: 1}, g.Edges[1].Evidence, "a bare import of the namespace goes to each distribution that shares it")
}

func TestBuildOpensADistributionWithAnExtensionModule(t *testing.T) {
	project := fstest.MapFS{"main.py": py("lxml")}
	sp := sitePackages(fstest.MapFS{"lxml/etree.cpython-311-x86_64-linux-gnu.so": text("")},
		distInfo{name: "lxml", version: "4.9.0", files: []string{"lxml/__init__.py", "lxml/etree.cpython-311-x86_64-linux-gnu.so"}, requires: []string{"six", "cssselect ; extra == 'cssselect'"}},
		distInfo{name: "six", version: "1.16.0", files: []string{"six.py"}},
		distInfo{name: "cssselect", version: "1.2.0", files: []string{"cssselect/__init__.py"}},
	)
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}})
	lxml := node(t, g, "lxml")
	assert.Equal(t, StateOpen, lxml.State)
	assert.Equal(t, "the extension module lxml/etree can import any module", lxml.Reason)
	assert.Equal(t, []string{"lxml -> six (requires)", "project -> lxml"}, edgeList(g), "the Requires-Dist lines of an open node are evidence of edges")
	assert.True(t, node(t, g, "cssselect").Reached, "a reached open node reaches each distribution")
	assert.True(t, node(t, g, "cssselect").Walked, "and Build reads each one")
	assert.Equal(t, []string{"project", "lxml", "cssselect"}, g.Path([]string{"project"}, "cssselect"))

	g = build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}, TrustCompiled: true})
	assert.Equal(t, StateImports, node(t, g, "lxml").State)
	assert.Equal(t, []string{"project -> lxml"}, edgeList(g))
	assert.False(t, node(t, g, "six").Reached)
}

func TestBuildOpensADistributionThatImportsAComputedName(t *testing.T) {
	project := fstest.MapFS{"main.py": py("plugins", "loader")}
	sp := sitePackages(fstest.MapFS{
		"plugins/__init__.py": text("import importlib\n\ndef load(name):\n    return importlib.import_module(name)\n"),
		"loader/__init__.py":  text("import importlib\n\nyaml = importlib.import_module(\"yaml\")\nsix = __import__('six')\n"),
	},
		distInfo{name: "plugins", version: "1.0.0", files: []string{"plugins/__init__.py"}},
		distInfo{name: "loader", version: "1.0.0", files: []string{"loader/__init__.py"}},
		distInfo{name: "PyYAML", version: "6.0.1", files: []string{"yaml/__init__.py"}},
		distInfo{name: "six", version: "1.16.0", files: []string{"six.py"}},
	)
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}})
	plugins := node(t, g, "plugins")
	assert.Equal(t, StateOpen, plugins.State)
	assert.Equal(t, "plugins/__init__.py:4 imports a computed name", plugins.Reason)
	assert.Equal(t, StateImports, node(t, g, "loader").State, "an import of a string names its module")
	assert.Equal(t, []string{"loader -> PyYAML", "loader -> six", "loader -> std", "plugins -> std", "project -> loader", "project -> plugins"}, edgeList(g))
}

func TestBuildOpensADistributionThatImportsWhatNothingHolds(t *testing.T) {
	project := fstest.MapFS{"main.py": py("dep")}
	sp := sitePackages(fstest.MapFS{
		"dep/__init__.py": text("import missing\ntry:\n    import optional\nexcept (ImportError, OSError):\n    optional = None\ntry:\n    import other\nexcept:\n    pass\n"),
	},
		distInfo{name: "dep", version: "1.0.0", files: []string{"dep/__init__.py"}},
	)
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}})
	dep := node(t, g, "dep")
	assert.Equal(t, StateOpen, dep.State)
	assert.Equal(t, "dep/__init__.py:1 imports missing, which no distribution holds", dep.Reason, "the guarded imports open nothing")
	assert.Equal(t, []string{"project -> dep"}, edgeList(g))
}

func TestBuildOpensADistributionWithAFileThatDoesNotParse(t *testing.T) {
	project := fstest.MapFS{"main.py": py("dep")}
	sp := sitePackages(fstest.MapFS{"dep/__init__.py": text("import yaml\ndef (:\n")},
		distInfo{name: "dep", version: "1.0.0", files: []string{"dep/__init__.py"}},
		distInfo{name: "PyYAML", version: "6.0.1", files: []string{"yaml/__init__.py"}},
	)
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}})
	dep := node(t, g, "dep")
	assert.Equal(t, StateOpen, dep.State)
	assert.Equal(t, "dep/__init__.py does not parse", dep.Reason)
	assert.Equal(t, []string{"dep -> PyYAML", "project -> dep"}, edgeList(g), "the imports that the tree shows still give edges")
}

func TestBuildOpensANodePastTheFileBudget(t *testing.T) {
	project := fstest.MapFS{"a.py": py("dep"), "b.py": py(), "c.py": py()}
	sp := sitePackages(nil, distInfo{name: "dep", version: "1.0.0", files: []string{"dep/__init__.py"}})
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}, MaxFiles: 2})
	assert.True(t, g.Stats.Truncated)
	assert.Equal(t, StateOpen, node(t, g, "project").State)
	assert.Equal(t, "the file budget ran out", node(t, g, "project").Reason)
	assert.Equal(t, 2, g.Stats.Files)
}

func TestBuildMarksADistributionWithNoPythonFileAsRequires(t *testing.T) {
	project := fstest.MapFS{"main.py": py("fast")}
	sp := sitePackages(fstest.MapFS{"fast.cpython-311-x86_64-linux-gnu.so": text("")},
		distInfo{name: "fast", version: "1.0.0", files: []string{"fast.cpython-311-x86_64-linux-gnu.so"}, requires: []string{"six>=1.0", "fast"}},
		distInfo{name: "six", version: "1.16.0", files: []string{"six.py"}},
	)
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}, TrustCompiled: true})
	fast := node(t, g, "fast")
	assert.Equal(t, StateRequires, fast.State)
	assert.Equal(t, "no Python file", fast.Reason)
	assert.Equal(t, []string{"fast -> six (requires)", "project -> fast"}, edgeList(g), "a requirement on itself is no edge")
	assert.Equal(t, Evidence{File: "fast-1.0.0.dist-info/METADATA"}, g.Edges[0].Evidence)
}

func TestBuildNamesASecondDistInfoOfOneDistribution(t *testing.T) {
	project := fstest.MapFS{"main.py": py("six")}
	sp := sitePackages(nil,
		distInfo{name: "six", version: "1.16.0", files: []string{"six.py"}},
		distInfo{name: "six", version: "1.15.0", files: []string{"six.py"}},
	)
	sp["broken.dist-info/METADATA"] = text("Version: 1\n")
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}})
	six := node(t, g, "six")
	assert.Equal(t, "1.15.0", six.Distribution.Version, "the first .dist-info in directory order stays")
	assert.Equal(t, StateOpen, six.State)
	assert.Equal(t, "two .dist-info directories describe the distribution", six.Reason)
	require.Len(t, g.Problems, 2)
	assert.Equal(t, "broken.dist-info", g.Problems[0].Path)
	assert.Equal(t, "six-1.16.0.dist-info", g.Problems[1].Path)
	assert.Equal(t, 1, g.Stats.Distributions)
}

func TestBuildSkipsTheEnvironmentInsideTheProject(t *testing.T) {
	project := fstest.MapFS{
		"main.py":          py("requests"),
		".venv/pyvenv.cfg": text("home = /usr/bin\n"),
		".venv/lib/python3.12/site-packages/x.py": py("yaml"),
		"venv/pyvenv.cfg":                         text(""),
		"venv/lib/site-packages/y.py":             py("yaml"),
		"node_modules/pkg/z.py":                   py("yaml"),
		"build/__pycache__/w.py":                  py("yaml"),
	}
	sp := sitePackages(nil,
		distInfo{name: "requests", version: "2.31.0", files: []string{"requests/__init__.py"}},
		distInfo{name: "PyYAML", version: "6.0.1", files: []string{"yaml/__init__.py"}},
	)
	g := build(t, Input{ProjectFS: project, SitePackages: []fs.FS{sp}})
	assert.Equal(t, []string{"project -> requests"}, edgeList(g))
	assert.Equal(t, 2, g.Stats.Files)
}

func TestBuildRejectsABadInput(t *testing.T) {
	_, err := Build(context.Background(), Input{})
	assert.EqualError(t, err, "distgraph: no project")
	_, err = Build(context.Background(), Input{ProjectFS: fstest.MapFS{}, ProjectName: "std"})
	assert.EqualError(t, err, `distgraph: the project holds the reserved name "std"`)
}

func TestGraphPath(t *testing.T) {
	g := &Graph{
		Nodes: []Node{
			{Distribution: Distribution{Name: "a"}, State: StateImports, Walked: true},
			{Distribution: Distribution{Name: "b"}, State: StateImports, Walked: true},
			{Distribution: Distribution{Name: "c"}, State: StateOpen, Walked: true},
			{Distribution: Distribution{Name: "d"}, State: StateImports},
			{Distribution: Distribution{Name: "Z_z"}, State: StateImports, Walked: true},
		},
		Edges: []Edge{
			{From: Distribution{Name: "a"}, To: Distribution{Name: "b"}},
			{From: Distribution{Name: "b"}, To: Distribution{Name: "c"}},
			{From: Distribution{Name: "a"}, To: Distribution{Name: "d"}},
		},
	}
	assert.Equal(t, []string{"a", "b"}, g.Path([]string{"a"}, "b"))
	assert.Equal(t, []string{"b", "c", "Z_z"}, g.Path([]string{"b"}, "z-z"), "an open node reaches each distribution, in any spelling of its name")
	assert.Equal(t, []string{"a", "d", "Z_z"}, g.Path([]string{"a"}, "Z.Z"), "a node that Build did not walk reaches each distribution")
	assert.Equal(t, []string{"b", "c", "a"}, g.Path([]string{"b"}, "a"))
	assert.Nil(t, g.Path([]string{"Z_z"}, "a"))
	assert.Nil(t, g.Path([]string{"a"}, "a"), "a distribution is not on a path from itself")
	assert.True(t, g.Reaches([]string{"a"}, "d"))
}

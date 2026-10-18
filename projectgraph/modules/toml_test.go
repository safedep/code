package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanTOML(t *testing.T) {
	data := []byte(`
[package]
name = "tool"

[[bin]]
name = "cli"

[project]
dependencies = [
  "flask>=2",
  "pyyaml",
]
classifiers = ["a"]

[project.entry-points."flake8.extension"]
X1 = "pkg.cli:main"
scripts = { run = "pkg:run", 'stop' = 'pkg:stop' }
`)
	pairs := ScanTOML(data)
	assert.Equal(t, []TOMLPair{
		{Table: "package", Key: "name", Value: `"tool"`},
		{Table: "bin", Key: "name", Value: `"cli"`},
		{Table: "project", Key: "dependencies", Value: "[\n\"flask>=2\",\n\"pyyaml\",\n]"},
		{Table: "project", Key: "classifiers", Value: `["a"]`},
		{Table: "project.entry-points.flake8.extension", Key: "X1", Value: `"pkg.cli:main"`},
		{Table: "project.entry-points.flake8.extension", Key: "scripts", Value: `{ run = "pkg:run", 'stop' = 'pkg:stop' }`},
	}, pairs)

	name, ok := TOMLString(pairs[0].Value)
	assert.True(t, ok)
	assert.Equal(t, "tool", name)
	_, ok = TOMLString(pairs[2].Value)
	assert.False(t, ok, "a list is no string")

	inline, ok := TOMLInlineTable(pairs[5].Value)
	assert.True(t, ok)
	assert.Equal(t, []TOMLPair{{Key: "run", Value: "pkg:run"}, {Key: "stop", Value: "pkg:stop"}}, inline)
	_, ok = TOMLInlineTable(pairs[0].Value)
	assert.False(t, ok)
}

func TestCargoCrateIgnoresTheNameOfABinTable(t *testing.T) {
	assert.Equal(t, "tool", cargoCrate([]byte("[package]\nname = \"tool\"\n\n[[bin]]\nname = \"cli\"\n")))
}

func TestMatchStar(t *testing.T) {
	patterns := []string{"./lib/*", "./lib/deep/*", "./*.js", "exact"}
	cases := []struct {
		text, pattern, star string
		ok                  bool
	}{
		{"./lib/deep/x", "./lib/deep/*", "x", true},
		{"./lib/x", "./lib/*", "x", true},
		{"./a.js", "./*.js", "a", true},
		{"exact", "", "", false},
		{"./other", "", "", false},
	}
	for _, tc := range cases {
		pattern, star, ok := matchStar(patterns, tc.text)
		assert.Equal(t, tc.ok, ok, tc.text)
		assert.Equal(t, tc.pattern, pattern, tc.text)
		assert.Equal(t, tc.star, star, tc.text)
	}
}

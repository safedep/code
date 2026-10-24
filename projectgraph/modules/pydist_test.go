package modules

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPythonDistributions(t *testing.T) {
	root := fstest.MapFS{
		"PyYAML-6.0.1.dist-info/METADATA": {Data: []byte("Metadata-Version: 2.1\nName: PyYAML\nVersion: 6.0.1\nSummary: YAML\n\nThe body.\nName: not this\n")},
		"PyYAML-6.0.1.dist-info/RECORD": {Data: []byte("PyYAML-6.0.1.dist-info/METADATA,sha256=abc,1730\n" +
			"_yaml/__init__.py,sha256=abc,10\n" +
			"yaml/__init__.py,sha256=abc,13340\n" +
			"yaml/_yaml.cpython-311-x86_64-linux-gnu.so,sha256=abc,222208\n" +
			"yaml/loader.py,sha256=abc,100\n" +
			"../../bin/yamlc,sha256=abc,10\n")},
		"six-1.16.0.dist-info/METADATA":        {Data: []byte("Name: six\nVersion: 1.16.0\n")},
		"six-1.16.0.dist-info/RECORD":          {Data: []byte("six.py,sha256=abc,34549\nsix-1.16.0.dist-info/RECORD,,\n")},
		"six-1.16.0.dist-info/top_level.txt":   {Data: []byte("six\n")},
		"google_auth-2.0.0.dist-info/METADATA": {Data: []byte("Name: google-auth\nVersion: 2.0.0\n")},
		"google_auth-2.0.0.dist-info/RECORD":   {Data: []byte("google/auth/__init__.py,,\ngoogle/auth/crypt.py,,\ngoogle/oauth2/__init__.py,,\n")},
		"nometa.dist-info/RECORD":              {Data: []byte("x.py,,\n")},
		"broken.dist-info/METADATA":            {Data: []byte("Version: 1\n")},
		"notes.txt":                            {Data: []byte("")},
	}
	dists, problems := PythonDistributions(root)
	require.Len(t, problems, 1, "a .dist-info with no METADATA is no distribution, and one with no Name is a problem")
	assert.Equal(t, "broken.dist-info", problems[0].Path)
	require.Len(t, dists, 3)

	yaml := dists[0]
	assert.Equal(t, "PyYAML", yaml.Name)
	assert.Equal(t, "6.0.1", yaml.Version)
	assert.Equal(t, "pypi:PyYAML@6.0.1", yaml.Unit())
	assert.Equal(t, []string{"_yaml", "yaml"}, yaml.TopLevel, "the top-level names come from RECORD when top_level.txt is missing")
	assert.Equal(t, []string{"yaml/_yaml"}, yaml.Compiled)
	assert.NotContains(t, yaml.Files, "../../bin/yamlc", "a path that leaves the root is no file of the package")
	assert.Equal(t, []string{"_yaml", "yaml"}, yaml.Roots())

	google := dists[1]
	assert.Equal(t, []string{"google"}, google.TopLevel)
	assert.Equal(t, []string{"google/auth", "google/oauth2"}, google.Roots(), "a namespace package gives the packages under it")

	six := dists[2]
	assert.Equal(t, []string{"six"}, six.TopLevel)
	assert.Equal(t, []string{"six.py"}, six.Roots())

	assert.Equal(t, "pyyaml", NormalizePythonName("PyYAML"))
	assert.Equal(t, "typing-extensions", NormalizePythonName("typing_extensions"))
	assert.Equal(t, "zope-interface", NormalizePythonName("zope.interface"))
}

func TestPythonDistributionsWithNoRoot(t *testing.T) {
	dists, problems := PythonDistributions(nil)
	assert.Empty(t, dists)
	assert.Empty(t, problems)
}

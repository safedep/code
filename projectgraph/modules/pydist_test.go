package modules

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPythonDistributions(t *testing.T) {
	root := fstest.MapFS{
		"PyYAML-6.0.1.dist-info/METADATA": {Data: []byte("Metadata-Version: 2.1\nName: PyYAML\nVersion: 6.0.1\nSummary: YAML\n" +
			"Requires-Dist: six (>=1.0)\nRequires-Dist: typing_extensions>=4; python_version < \"3.11\"\nRequires-Dist: six\n" +
			"Requires-Dist: pytest ; extra == 'test'\nRequires-Dist: google-auth[aiohttp]\n\nThe body.\nName: not this\n")},
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
		"app-0.1.0.dist-info/METADATA":         {Data: []byte("Name: app\nVersion: 0.1.0\n")},
		"app-0.1.0.dist-info/direct_url.json":  {Data: []byte(`{"url": "file:///src/app", "dir_info": {"editable": true}}`)},
		"app-0.1.0.dist-info/RECORD":           {Data: []byte("__editable__.app-0.1.0.pth,,\n")},
		"lib-0.2.0.dist-info/METADATA":         {Data: []byte("Name: lib\nVersion: 0.2.0\n")},
		"lib-0.2.0.dist-info/direct_url.json":  {Data: []byte(`{"url": "https://example.com/lib.git", "vcs_info": {"vcs": "git"}}`)},
		"google_auth-2.0.0.dist-info/RECORD":   {Data: []byte("google/auth/__init__.py,,\ngoogle/auth/crypt.py,,\ngoogle/oauth2/__init__.py,,\n")},
		"nometa.dist-info/RECORD":              {Data: []byte("x.py,,\n")},
		"broken.dist-info/METADATA":            {Data: []byte("Version: 1\n")},
		"notes.txt":                            {Data: []byte("")},
	}
	dists, problems := PythonDistributions(root)
	require.Len(t, problems, 1, "a .dist-info with no METADATA is no distribution, and one with no Name is a problem")
	assert.Equal(t, "broken.dist-info", problems[0].Path)
	require.Len(t, dists, 5)

	yaml := dists[0]
	assert.Equal(t, "PyYAML", yaml.Name)
	assert.Equal(t, "6.0.1", yaml.Version)
	assert.Equal(t, "pypi:PyYAML@6.0.1", yaml.Unit())
	assert.Equal(t, []string{"_yaml", "yaml"}, yaml.TopLevel, "the top-level names come from RECORD when top_level.txt is missing")
	assert.Equal(t, []string{"yaml/_yaml"}, yaml.Compiled)
	assert.NotContains(t, yaml.Files, "../../bin/yamlc", "a path that leaves the root is no file of the package")
	assert.Equal(t, []string{"_yaml", "yaml"}, yaml.Roots())
	assert.Equal(t, []string{"six", "typing_extensions", "google-auth"}, yaml.Requires, "a requirement with an extra marker is no requirement of a plain install")

	assert.True(t, dists[1].Editable, "app is an editable install")
	assert.False(t, dists[3].Editable, "an install from a git URL is no editable install")
	assert.False(t, yaml.Editable)

	google := dists[2]
	assert.Equal(t, []string{"google"}, google.TopLevel)
	assert.Equal(t, []string{"google/auth", "google/oauth2"}, google.Roots(), "a namespace package gives the packages under it")

	six := dists[4]
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

func TestPythonTestPath(t *testing.T) {
	for p, want := range map[string]bool{
		"yaml/loader.py":                false,
		"django/test/client.py":         false,
		"numpy/testing/__init__.py":     false,
		"requests/tests/test_api.py":    true,
		"pkg/test_util.py":              true,
		"pkg/util_test.py":              true,
		"pkg/conftest.py":               true,
		"pkg/__pycache__/mod.py":        true,
		"six-1.16.0.dist-info/METADATA": true,
		"pkg-1.0.data/scripts/run.py":   true,
	} {
		assert.Equal(t, want, PythonTestPath(p), p)
	}
}

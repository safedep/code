package fileindex

import (
	"testing"

	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
)

func TestKeys(t *testing.T) {
	key := "src/a.py//Parser//parse"
	assert.Equal(t, key, Join("src/a.py", "Parser", "parse"))
	assert.Equal(t, "src/a.py", Join("src/a.py"))
	assert.Equal(t, "src/a.py", Unit(key))
	assert.Equal(t, []string{"Parser", "parse"}, Segments(key))
	assert.Nil(t, Segments("src/a.py"))
	assert.Equal(t, "parse", Last(key))
	assert.Equal(t, "src/a.py", Last("src/a.py"))
	assert.Equal(t, "src/a.py//Parser", Parent(key))
	assert.Equal(t, "src/a.py", Parent("src/a.py"))
	assert.True(t, HasKeyPrefix(key, "src/a.py//Parser"))
	assert.True(t, HasKeyPrefix(key, key))
	assert.False(t, HasKeyPrefix(key, "src/a.py//Pars"))
}

func TestKeysOfDependencyCode(t *testing.T) {
	key := "npm:lodash@4.17.21//lib/template.js//template//inner"
	assert.Equal(t, "npm:lodash@4.17.21//lib/template.js", Unit(key))
	assert.Equal(t, []string{"template", "inner"}, Segments(key))
	assert.Nil(t, Segments("npm:lodash@4.17.21//lib/template.js"))
	assert.Equal(t, "inner", Last(key))
	assert.Equal(t, "npm:lodash@4.17.21//lib/template.js", Last("npm:lodash@4.17.21//lib/template.js"))
	assert.Equal(t, "npm:lodash@4.17.21//lib/template.js//template", Parent(key))
	assert.Equal(t, "npm:lodash@4.17.21//lib/template.js", Parent("npm:lodash@4.17.21//lib/template.js//template"))
	assert.Equal(t, "npm:lodash@4.17.21//lib/template.js", Parent("npm:lodash@4.17.21//lib/template.js"))
	assert.Equal(t, "npm:lodash@4.17.21", PackageOf(key))
	assert.Equal(t, "npm:lodash@4.17.21", PackageOf("npm:lodash@4.17.21"))
	assert.Empty(t, PackageOf("src/a.py//Parser"))
	assert.Empty(t, PackageOf("node:fs//readFile"))

	unit, path := SplitUnit("pypi:requests@2.31.0//requests/api.py")
	assert.Equal(t, "pypi:requests@2.31.0", unit)
	assert.Equal(t, "requests/api.py", path)
	assert.Equal(t, "pypi:requests@2.31.0//requests/api.py", UnitPath(unit, path))
	unit, path = SplitUnit("requests/api.py")
	assert.Empty(t, unit)
	assert.Equal(t, "requests/api.py", path)
	assert.Equal(t, "requests/api.py", UnitPath("", path))
}

func TestParseUnit(t *testing.T) {
	tests := []struct {
		unit string
		want PackageUnit
		bad  bool
	}{
		{unit: "npm:lodash@4.17.21", want: PackageUnit{Ecosystem: "npm", Name: "lodash", Version: "4.17.21"}},
		{unit: "npm:@babel/core@7.24.0", want: PackageUnit{Ecosystem: "npm", Name: "@babel/core", Version: "7.24.0"}},
		{unit: "go:github.com/google/uuid@v1.6.0", want: PackageUnit{Ecosystem: "go", Name: "github.com/google/uuid", Version: "v1.6.0"}},
		{unit: "maven:org.yaml:snakeyaml@2.2", want: PackageUnit{Ecosystem: "maven", Name: "org.yaml:snakeyaml", Version: "2.2"}},
		{unit: "lodash@4.17.21", bad: true},
		{unit: "npm:lodash", bad: true},
		{unit: "npm:@4.17.21", bad: true},
		{unit: "npm:lodash@", bad: true},
		{unit: "NPM:lodash@1", bad: true},
		{unit: "global::System@1", bad: true},
		{unit: "npm:a//b@1", bad: true},
		{unit: "npm:a b@1", bad: true},
		{unit: "npm:a@1/2", bad: true},
		{unit: "node:fs", bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			u, err := ParseUnit(tt.unit)
			assert.Equal(t, !tt.bad, IsPackageUnit(tt.unit))
			if tt.bad {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, u)
			assert.Equal(t, tt.unit, u.String())
		})
	}
}

func TestWithoutTypeArguments(t *testing.T) {
	for in, want := range map[string]string{
		"List<string>":                    "List",
		"Map<K, V>.Entry":                 "Map.Entry",
		"serde_json::from_str::<Value>":   "serde_json::from_str::",
		"Hmac::<Sha256>::new_from_slice":  "Hmac::::new_from_slice",
		"Dictionary<string, List<int>>.X": "Dictionary.X",
		"GetRequiredService<IChat>":       "GetRequiredService",
		"plain::path":                     "plain::path",
	} {
		assert.Equal(t, want, WithoutTypeArguments(in), in)
	}
}

func TestNamespace(t *testing.T) {
	assert.Equal(t, "yaml//loader", Namespace(langcode.Python, "yaml.loader"))
	assert.Equal(t, "fs//promises", Namespace(langcode.Javascript, "fs/promises"))
	assert.Equal(t, ".//utils", Namespace(langcode.Typescript, "./utils"))
	assert.Equal(t, "java//util//List", Namespace(langcode.Java, "java.util.List"))
	assert.Equal(t, "net//http", Namespace(langcode.Go, "net/http"))
	assert.Equal(t, "System//IO", Namespace(langcode.CSharp, "System.IO"))
	assert.Equal(t, "App//Services//Loader", Namespace(langcode.PHP, `\App\Services\Loader`))
	assert.Equal(t, "crate//loader", Namespace(langcode.Rust, "crate::loader"))
	assert.Equal(t, "net//http", Namespace(langcode.Ruby, "net/http"))
	assert.Equal(t, "Net//HTTP", Namespace(langcode.Ruby, "Net::HTTP"))
}

func TestAnonymous(t *testing.T) {
	assert.Equal(t, "<anonymous:12>", Anonymous(12, 1))
	assert.Equal(t, "<anonymous:12#2>", Anonymous(12, 2))
	assert.True(t, IsAnonymous(Anonymous(3, 1)))
	assert.False(t, IsAnonymous("handler"))
}

func TestJoinNonEmptyAndCompareBool(t *testing.T) {
	assert.Equal(t, "lib", JoinNonEmpty("lib", ""))
	assert.Equal(t, "lib//load", JoinNonEmpty("lib", "load"))
	assert.Equal(t, -1, CompareBool(false, true))
	assert.Equal(t, 0, CompareBool(true, true))
	assert.Equal(t, 1, CompareBool(true, false))
}

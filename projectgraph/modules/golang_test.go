package modules

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGolangResolve(t *testing.T) {
	paths := []string{
		"main.go",
		"util/util.go",
		"vendor/github.com/x/y/y.go",
		"util/testdata/fixture.go",
		"tools/gen/gen.go",
		"broken/b.go",
	}
	root := fstest.MapFS{
		"go.mod":        {Data: []byte("module example.com/app\n\ngo 1.22\n")},
		"tools/go.mod":  {Data: []byte("module example.com/tools\n")},
		"broken/go.mod": {Data: []byte("go 1.22\n")},
	}
	dirs := []string{"", "util", "vendor/github.com/x/y", "util/testdata", "tools/gen", "broken"}
	g := NewGolang(NewFiles(paths), root, dirs)

	tests := []struct {
		name, spec, want string
		ok               bool
	}{
		{name: "the module root is a package", spec: "example.com/app", want: "", ok: true},
		{name: "a directory below the module", spec: "example.com/app/util", want: "util", ok: true},
		{name: "a nested module owns its directories", spec: "example.com/tools/gen", want: "tools/gen", ok: true},
		{name: "a nested module is not under the outer path", spec: "example.com/app/tools/gen", ok: false},
		{name: "vendor stays external", spec: "github.com/x/y", ok: false},
		{name: "vendor has no path of the module", spec: "example.com/app/vendor/github.com/x/y", ok: false},
		{name: "testdata is no package", spec: "example.com/app/util/testdata", ok: false},
		{name: "the standard library is external", spec: "fmt", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := g.Resolve(tt.spec)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}

	assert.True(t, g.FirstParty("example.com/app/missing"))
	assert.True(t, g.FirstParty("example.com/tools"))
	assert.False(t, g.FirstParty("example.com/application"))
	assert.False(t, g.FirstParty("fmt"))

	require.Len(t, g.Problems(), 1)
	assert.Equal(t, "broken/go.mod", g.Problems()[0].Path)
}

func TestGolangWithNoModule(t *testing.T) {
	g := NewGolang(NewFiles([]string{"pkg/util/u.go"}), fstest.MapFS{}, []string{"pkg/util"})

	_, ok := g.Resolve("github.com/me/proj/pkg/util")
	assert.False(t, ok, "no go.mod gives no import path")
	assert.True(t, g.FirstParty("github.com/me/proj/pkg/util"), "the path ends with a package directory")
	assert.False(t, g.FirstParty("github.com/me/proj/pkg/utility"))
	assert.Empty(t, g.Problems())
}

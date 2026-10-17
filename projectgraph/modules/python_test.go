package modules

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestPythonResolve(t *testing.T) {
	files := NewFiles([]string{
		"app/__init__.py",
		"app/main.py",
		"app/util.py",
		"app/util/__init__.py",
		"app/sub/deep.py",
		"ns/part/mod.py",
		"svc/src/svc/core.py",
		"svc/src/app/main.py",
		"vendor/lib.py",
	})
	root := fstest.MapFS{"svc/pyproject.toml": &fstest.MapFile{Data: []byte("[project]\n")}}
	p := NewPython(files, root, []string{"vendor", "../outside"})

	assert.Equal(t, []string{"", "svc", "svc/src", "vendor"}, p.Roots())

	tests := []struct {
		name     string
		importer string
		module   string
		want     string
		ok       bool
	}{
		{name: "a package wins over a module", importer: "app/main.py", module: "app.util", want: "app/util/__init__.py", ok: true},
		{name: "one dot is the package", importer: "app/main.py", module: ".util", want: "app/util/__init__.py", ok: true},
		{name: "a lone dot is the package itself", importer: "app/main.py", module: ".", want: "app/__init__.py", ok: true},
		{name: "two dots go up", importer: "app/sub/deep.py", module: "..main", want: "app/main.py", ok: true},
		{name: "dots above the root fail", importer: "app/main.py", module: "...x", ok: false},
		{name: "a bare directory is a namespace package", importer: "app/main.py", module: "ns.part", want: "ns/part/", ok: true},
		{name: "a pyproject directory and its src are roots", importer: "svc/src/svc/core.py", module: "svc.core", want: "svc/src/svc/core.py", ok: true},
		{name: "the roots of the importer come first", importer: "svc/src/app/main.py", module: "app.main", want: "svc/src/app/main.py", ok: true},
		{name: "other roots come after", importer: "vendor/lib.py", module: "app.main", want: "app/main.py", ok: true},
		{name: "a sys.path root", importer: "app/main.py", module: "lib", want: "vendor/lib.py", ok: true},
		{name: "an unknown module", importer: "app/main.py", module: "yaml", ok: false},
		{name: "an empty module", importer: "app/main.py", module: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := p.Resolve(tt.importer, tt.module)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPythonSubmodule(t *testing.T) {
	p := NewPython(NewFiles([]string{"pkg/__init__.py", "pkg/a.py", "ns/b.py"}), nil, nil)

	got, ok := p.Submodule("pkg/__init__.py", "a")
	assert.True(t, ok)
	assert.Equal(t, "pkg/a.py", got)

	got, ok = p.Submodule("ns/", "b")
	assert.True(t, ok)
	assert.Equal(t, "ns/b.py", got)

	_, ok = p.Submodule("pkg/a.py", "x")
	assert.False(t, ok, "a module file has no submodule")
}

func TestJoin(t *testing.T) {
	got, ok := Join("a/b", "../c")
	assert.True(t, ok)
	assert.Equal(t, "a/c", got)

	got, ok = Join("", ".")
	assert.True(t, ok)
	assert.Empty(t, got)

	_, ok = Join("a", "../..")
	assert.False(t, ok)
}

func TestPythonImporterDirectoryIsTheLastRoot(t *testing.T) {
	p := NewPython(NewFiles([]string{"scripts/main.py", "scripts/helper.py", "helper.py"}), nil, nil)

	got, ok := p.Resolve("scripts/main.py", "helper")
	assert.True(t, ok)
	assert.Equal(t, "helper.py", got, "a root of the project comes first")

	p = NewPython(NewFiles([]string{"scripts/main.py", "scripts/util.py"}), nil, nil)
	got, ok = p.Resolve("scripts/main.py", "util")
	assert.True(t, ok)
	assert.Equal(t, "scripts/util.py", got)
}

func TestFilesHasName(t *testing.T) {
	f := NewFiles([]string{"src/app/conf/defaults.py", "lib/yaml_util.js"})
	for _, name := range []string{"src", "app", "conf", "defaults", "lib", "yaml_util"} {
		assert.True(t, f.HasName(name), name)
	}
	for _, name := range []string{"yaml", "defaults.py", ""} {
		assert.False(t, f.HasName(name), name)
	}
}

func TestPythonRootPackage(t *testing.T) {
	p := NewPython(NewFiles([]string{"__init__.py", "impl.py", "sub/__init__.py"}), nil, nil)
	p.SetRootPackage("outer")
	assert.Equal(t, "outer", p.RootPackage())

	tests := []struct {
		module string
		want   string
		ok     bool
	}{
		{module: "outer", want: "__init__.py", ok: true},
		{module: "outer.impl", want: "impl.py", ok: true},
		{module: "outer.sub", want: "sub/__init__.py", ok: true},
		{module: "outer.none"},
		{module: "other"},
	}
	for _, tt := range tests {
		got, ok := p.Absolute("", tt.module)
		assert.Equal(t, tt.ok, ok, tt.module)
		assert.Equal(t, tt.want, got, tt.module)
	}
	assert.True(t, p.FirstParty("outer.impl"))
	assert.False(t, p.FirstParty("other"))
}

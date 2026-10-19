package projectgraph

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/safedep/code/fileindex"
	"github.com/safedep/code/langcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rustUse is src/main.rs of the crate app with one use and a call of the
// name that the use binds.
func rustUse(module, alias string) *fileindex.Index {
	const p = "src/main.rs"
	ns := fileindex.Namespace(langcode.Rust, module)
	return &fileindex.Index{
		Version:  fileindex.Version,
		Path:     p,
		Language: langcode.Rust,
		Imports:  []fileindex.Import{{Line: 1, Module: module, Alias: alias, Kind: fileindex.ImportStatic, Namespace: ns}},
		Definitions: []fileindex.Definition{
			def(fileindex.DefModule, p, 1),
			def(fileindex.DefFunction, fileindex.Join(p, "main"), 2),
		},
		Calls: []fileindex.Call{{Caller: fileindex.Join(p, "main"), Line: 2, Column: 13, Callee: alias, Targets: []string{ns}, Method: alias, Kind: fileindex.CallCall}},
	}
}

func TestBuildRustUseOfItsOwnBinding(t *testing.T) {
	root := fstest.MapFS{"Cargo.toml": {Data: []byte("[package]\nname = \"app\"\nversion = \"0.1.0\"\n")}}
	tests := []struct {
		name   string
		module string
		alias  string
		edge   string
	}{
		{"use smallvec::smallvec", "smallvec::smallvec", "smallvec", "src/main.rs//main -> smallvec//smallvec direct rust.external"},
		{"use a::a", "a::a", "a", "src/main.rs//main -> a//a direct rust.external"},
		{"use a::a::a", "a::a::a", "a", "src/main.rs//main -> a//a//a direct rust.external"},
		// The module defines no a, so the program does not compile. The
		// test checks that the link step ends.
		{"use self::a as a", "self::a", "a", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := Build(context.Background(), []*fileindex.Index{rustUse(tt.module, tt.alias)}, Options{Root: root})
			require.NoError(t, err)
			if tt.edge != "" {
				assert.Contains(t, edgeStrings(g), tt.edge)
			}
		})
	}
}

// TestBuildRustWildcardUseOfTheClassInItsImpl links an impl of a trait for
// an enum whose method imports the variants of the enum with a wildcard,
// as use self::Component::* in impl Display for Component. The base
// Display can resolve through that wildcard, which leads back to the same
// class.
func TestBuildRustWildcardUseOfTheClassInItsImpl(t *testing.T) {
	const p = "src/main.rs"
	class := fileindex.Join(p, "Component")
	ix := &fileindex.Index{
		Version:  fileindex.Version,
		Path:     p,
		Language: langcode.Rust,
		Imports: []fileindex.Import{
			{Line: 1, Module: "std::fmt::Display", Alias: "Display", Kind: fileindex.ImportStatic, Namespace: "std//fmt//Display"},
			{Line: 9, Module: "self::Component", Wildcard: true, Kind: fileindex.ImportStatic, Namespace: "self//Component"},
		},
		Definitions: []fileindex.Definition{
			def(fileindex.DefModule, p, 1),
			def(fileindex.DefClass, class, 7),
			def(fileindex.DefMethod, fileindex.Join(class, "fmt"), 8),
		},
		Classes: []fileindex.Class{{Key: class, Bases: []string{"Display"}, Methods: []string{fileindex.Join(class, "fmt")}}},
	}
	root := fstest.MapFS{"Cargo.toml": {Data: []byte("[package]\nname = \"app\"\nversion = \"0.1.0\"\n")}}
	g, err := Build(context.Background(), []*fileindex.Index{ix}, Options{Root: root})
	require.NoError(t, err)
	assert.Equal(t, NodeClass, g.Nodes[class].Kind)
}

func TestRustHelperProgram(t *testing.T) {
	for _, tt := range []struct {
		path string
		want bool
	}{
		{path: "build.rs", want: true},
		{path: "kit/build.rs", want: true},
		{path: "examples/demo.rs", want: true},
		{path: "kit/examples/demo/main.rs", want: true},
		{path: "benches/speed.rs", want: true},
		{path: "src/main.rs", want: false},
		{path: "src/bin/tool.rs", want: false},
		{path: "src/examples.rs", want: false},
	} {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, rustHelperProgram(tt.path))
		})
	}
}

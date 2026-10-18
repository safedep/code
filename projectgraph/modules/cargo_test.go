package modules

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCargoReadsTheCratesOfTheManifests(t *testing.T) {
	root := fstest.MapFS{
		"Cargo.toml":               {Data: []byte("[workspace]\nmembers = [\"crates/*\"]\n")},
		"crates/core/Cargo.toml":   {Data: []byte("[package]\nname = \"core-lib\"\nversion = \"0.1.0\"\n\n[dependencies]\nname = \"not-this\"\n")},
		"crates/web/Cargo.toml":    {Data: []byte("[package]\nname = \"web-app\"\n\n[lib]\nname = \"web\"\n")},
		"crates/web/src/lib.rs":    {Data: []byte("")},
		"crates/core/src/lib.rs":   {Data: []byte("")},
		"crates/core/src/a/b.rs":   {Data: []byte("")},
		"tools/script/src/main.rs": {Data: []byte("")},
	}
	c := NewCargo(NewFiles([]string{"crates/web/src/lib.rs", "crates/core/src/lib.rs", "crates/core/src/a/b.rs", "tools/script/src/main.rs"}), root)

	crate, ok := c.Crate("crates/core/src/a/b.rs")
	require.True(t, ok)
	assert.Equal(t, Crate{Dir: "crates/core", Name: "core_lib"}, crate, "a dash of the package name is an underscore in a path")
	crate, ok = c.Crate("crates/web/src/lib.rs")
	require.True(t, ok)
	assert.Equal(t, "web", crate.Name, "the name of the lib table wins")
	_, ok = c.Crate("tools/script/src/main.rs")
	assert.False(t, ok, "a workspace manifest names no crate")
	assert.True(t, c.Has("core_lib"))
	assert.False(t, c.Has("web_app"))
	assert.Empty(t, c.Problems())
}

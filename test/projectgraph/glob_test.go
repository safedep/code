package projectgraph

import (
	"context"
	"fmt"
	"os"
	"testing"

	codefs "github.com/safedep/code/fs"
	"github.com/safedep/code/plugin/indexer"
	"github.com/safedep/code/projectgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildRustKeepsNoGlobListThatACutGave links src/other.rs, which glob
// uses crate::sub::x and crate::c. sub gives x only through its own glob use
// of other, so the read of the glob uses of other asks for other again
// under the guard of a glob member. That read gets no module for
// crate::sub::x. The list of the glob modules of other that the build keeps
// must still hold c::x, so g() in run links g of src/c/x.rs through the
// glob use.
func TestBuildRustKeepsNoGlobListThatACutGave(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"Cargo.toml":   "[package]\nname = \"app\"\nversion = \"1.0.0\"\nedition = \"2021\"\n",
		"src/main.rs":  "mod c;\nmod other;\nmod sub;\n\nuse sub::*;\n\nfn main() {\n    x::g();\n    other::run();\n}\n",
		"src/c/mod.rs": "pub mod x;\n",
		"src/c/x.rs":   "pub fn g() {}\n",
		"src/other.rs": "use crate::sub::x::*;\npub use crate::c::*;\n\npub fn run() {\n    g();\n}\n",
		"src/sub.rs":   "pub use crate::other::*;\n",
	})
	fsys, err := codefs.NewLocalFileSystem(codefs.LocalFileSystemConfig{AppDirectories: []string{dir}})
	require.NoError(t, err)
	indexed := indexProject(t, fsys, indexer.Options{Languages: caseLanguages(t)})
	g, err := projectgraph.Build(context.Background(), indexed.Indexes, projectgraph.Options{Root: os.DirFS(dir), Files: indexed.Statuses(), Languages: indexed.Languages})
	require.NoError(t, err)
	var edges []string
	for _, e := range g.Edges {
		edges = append(edges, fmt.Sprintf("%s -> %s %s %s", e.From, e.To, e.Kind, e.Resolver))
	}
	assert.Contains(t, edges, "src/other.rs//run -> src/c/x.rs//g direct rust.wildcard-import")
}

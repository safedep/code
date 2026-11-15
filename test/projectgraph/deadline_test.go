package projectgraph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	codefs "github.com/safedep/code/fs"
	"github.com/safedep/code/plugin/indexer"
	"github.com/safedep/code/projectgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildStopsAtTheDeadline links ten Rust files that each glob use a
// name of every other file that no file defines. Each lookup of a name that
// a file does not define walks the glob uses of the other files, and the
// work grows with the orders of the files. The build must end soon after
// its deadline, as a scan gives the link step a budget.
func TestBuildStopsAtTheDeadline(t *testing.T) {
	const files = 10
	dir := t.TempDir()
	write := func(name, text string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644))
	}
	write("Cargo.toml", "[package]\nname = \"app\"\nversion = \"0.1.0\"\n")
	var lib strings.Builder
	for i := range files {
		fmt.Fprintf(&lib, "pub mod f%d;\n", i)
		var file strings.Builder
		for j := range files {
			if j != i {
				fmt.Fprintf(&file, "pub use crate::f%d::E::*;\n", j)
			}
		}
		file.WriteString("pub fn run() {\n    missing();\n}\n")
		write(fmt.Sprintf("src/f%d.rs", i), file.String())
	}
	write("src/lib.rs", lib.String())

	fsys, err := codefs.NewLocalFileSystem(codefs.LocalFileSystemConfig{AppDirectories: []string{dir}})
	require.NoError(t, err)
	indexed := indexProject(t, fsys, indexer.Options{Languages: caseLanguages(t)})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{Root: os.DirFS(dir), Files: indexed.Statuses(), Languages: indexed.Languages})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 10*time.Second)
}

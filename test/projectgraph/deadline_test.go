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
	"github.com/safedep/code/projectgraph/reach"
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

// TestBuildReadsEachGlobModuleOncePerName links two layouts of Rust glob
// uses. In the first, nine files each glob use every other file. In the
// second, a chain of twenty levels of two files each glob uses both files of
// the next level, and only one file of the last level defines leaf. A
// lookup that reads a module once per path through the glob uses grows
// with the number of paths, and the build did not end within minutes.
func TestBuildReadsEachGlobModuleOncePerName(t *testing.T) {
	complete := map[string]string{}
	var main strings.Builder
	for i := range 9 {
		fmt.Fprintf(&main, "mod m%d;\n", i)
		var file strings.Builder
		for j := range 9 {
			if j != i {
				fmt.Fprintf(&file, "pub use crate::m%d::*;\n", j)
			}
		}
		fmt.Fprintf(&file, "\npub fn f%d() {\n    let _ = std::fs::remove_file(\"a\");\n}\n", i)
		complete[fmt.Sprintf("src/m%d.rs", i)] = file.String()
	}
	main.WriteString("\nuse m0::*;\n\nfn main() {\n    f0();\n    gone();\n}\n")
	complete["src/main.rs"] = main.String()

	const levels = 20
	diamond := map[string]string{}
	main.Reset()
	for i := range levels {
		for _, side := range []string{"a", "b"} {
			fmt.Fprintf(&main, "mod l%d%s;\n", i, side)
			var file strings.Builder
			if i+1 < levels {
				fmt.Fprintf(&file, "pub use crate::l%da::*;\npub use crate::l%db::*;\n", i+1, i+1)
			} else if side == "a" {
				file.WriteString("pub fn leaf() {\n    let _ = std::fs::remove_file(\"a\");\n}\n")
			}
			fmt.Fprintf(&file, "\npub fn own_%d%s() {}\n", i, side)
			diamond[fmt.Sprintf("src/l%d%s.rs", i, side)] = file.String()
		}
	}
	main.WriteString("\nuse l0a::*;\n\nfn main() {\n    leaf();\n    gone();\n}\n")
	diamond["src/main.rs"] = main.String()

	for name, files := range map[string]map[string]string{"complete": complete, "diamond": diamond} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			files["Cargo.toml"] = "[package]\nname = \"app\"\nversion = \"0.1.0\"\n"
			for p, text := range files {
				require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, p), []byte(text), 0o644))
			}
			fsys, err := codefs.NewLocalFileSystem(codefs.LocalFileSystemConfig{AppDirectories: []string{dir}})
			require.NoError(t, err)
			indexed := indexProject(t, fsys, indexer.Options{Languages: caseLanguages(t)})

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{Root: os.DirFS(dir), Files: indexed.Statuses(), Languages: indexed.Languages})
			require.NoError(t, err)
			results, err := reach.Reach(ctx, g, reach.Query{Targets: targetsOf(t, "std//fs//remove_file")})
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, reach.Found, results[0].Status)
		})
	}
}

// TestBuildReadsEachGlobPathOnceInARingOfCrates links six crates of
// dependency code whose roots glob use the next crate, the last one back
// to the first, and a crate leaf that the last one also glob uses. Each
// file of the project glob uses one crate of the ring and calls n::f of
// leaf. A glob path that kept no answer under a cut read the ring once for
// each path round it, and the build did not end within minutes.
func TestBuildReadsEachGlobPathOnceInARingOfCrates(t *testing.T) {
	const crates = 6
	dir := t.TempDir()
	write := func(name, text string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644))
	}
	manifest := func(name string, deps ...string) string {
		var m strings.Builder
		fmt.Fprintf(&m, "[package]\nname = \"%s\"\nversion = \"1.0.0\"\nedition = \"2021\"\n\n[dependencies]\n", name)
		for _, d := range deps {
			fmt.Fprintf(&m, "%s = \"1.0.0\"\n", d)
		}
		return m.String()
	}
	want := expectation{Units: map[string]string{"vendor/leaf": "cargo:leaf@1.0.0"}}
	var main, deps []string
	for i := range crates {
		name, next := fmt.Sprintf("ring_%d", i), fmt.Sprintf("ring_%d", (i+1)%crates)
		want.Units["vendor/"+name] = fmt.Sprintf("cargo:%s@1.0.0", name)
		lib := "pub use " + next + "::*;\n"
		uses := []string{next}
		if i == crates-1 {
			lib += "pub use leaf::*;\n"
			uses = append(uses, "leaf")
		}
		write("vendor/"+name+"/Cargo.toml", manifest(name, uses...))
		write("vendor/"+name+"/src/lib.rs", lib)
		write(fmt.Sprintf("src/c%d.rs", i), fmt.Sprintf("use %s::*;\n\npub fn go() {\n    n::f();\n}\n", name))
		main = append(main, fmt.Sprintf("mod c%d;", i))
		deps = append(deps, name)
	}
	write("vendor/leaf/Cargo.toml", manifest("leaf"))
	write("vendor/leaf/src/lib.rs", "pub mod n;\n")
	write("vendor/leaf/src/n.rs", "pub fn f() {\n    let _ = std::fs::remove_file(\"a\");\n}\n")
	write("Cargo.toml", manifest("app", deps...))
	write("src/main.rs", strings.Join(main, "\n")+"\n\nfn main() {\n    c0::go();\n}\n")

	cfg, roots := fileSystemConfig(dir, want)
	fsys, err := codefs.NewLocalFileSystem(cfg)
	require.NoError(t, err)
	indexed := indexProject(t, fsys, indexer.Options{Languages: caseLanguages(t), Imports: true})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{Root: os.DirFS(dir), Files: indexed.Statuses(), Languages: indexed.Languages, Units: roots})
	require.NoError(t, err)
	results, err := reach.Reach(ctx, g, reach.Query{Targets: targetsOf(t, "leaf//n//f")})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, reach.Found, results[0].Status)
}

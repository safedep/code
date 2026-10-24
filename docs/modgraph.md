# Module graph

`modgraph` builds the module graph of a Go module: which module of the build
list imports which. A `go.mod` lists the modules of a build, but not the
edges between them. A caller needs the edges to tell that no code imports a
module, or that no other module can call one.

The package reads the code on the disk. It runs no `go` command, opens no
network connection and builds with `CGO_ENABLED=0`. It parses each Go file
with `go/parser` in `ImportsOnly` mode. It reads the first 64 KiB of a file,
and reads the whole file when that head does not show the end of the import
block: a token after the last import that is not `import`, and that ends
inside the head.

## The rule

The graph may hold an edge that a build does not have. It never misses an
edge that a build has. A caller can then trust the absence of a path: no
build of any platform or tag set can reach a module that the graph does not
reach.

So a module whose imports `Build` cannot read is **open**: it may import any
module, and a path that meets it reaches each module.

| State | The module | Edges |
| --- | --- | --- |
| `imports` | Has source. `Build` read each package of it that the build reaches | The imports of those packages |
| `requires` | Has no source, only its `go.mod` | Its require lines, as evidence only. A `go.mod` need not list each module that its code imports, because the main `go.mod` can give it as `// indirect`. So the node is open |
| `open` | Has nothing that `Build` can read, or an import that `Build` cannot resolve | None |

## The walk

1. `Build` reads each package of the main module, its test files too, as
   `./...` names them. It skips `testdata`, `vendor`, a directory or file
   whose name starts with `_` or `.`, and a directory with a `go.mod` of its
   own. A link to a file is a file of its package.
2. For each import, the resolver takes the modules of the build list whose
   path is a prefix of the import path, longest first. A module with source
   holds the package when its directory has an entry whose name ends in
   `.go` and that is a file or a link to a file, as the go command decides
   it. A module with no source may hold it, so it gets an edge too, and the
   test goes on to a shorter prefix.
   A path that no module holds is the standard library when its first
   element has no dot. Else its importer is open.
3. The walk reads each package that an import reaches, as `go list -deps`
   does, a `testdata` package or a package behind a link to a directory too.
   A package of a dependency that no import reaches adds nothing. Test files
   of a dependency never count.
4. A file counts unless its build constraint can never hold, with every tag
   free except `ignore`. `Build` reads the constraint as `go/build` does: a
   `//go:build` line in the leading comments decides, else each `// +build`
   line before the last blank line of the leading `//` comments. A file of
   `//go:build purego` counts, because a project can build with
   `-tags purego`. A GOOS or GOARCH suffix of the file name never drops a
   file.
5. Once a reached node is open, it can import any package of the build
   list. So each node counts as reached, and `Build` reads each directory
   of each module with source that an import can name, except `vendor` and
   a nested module. Else a module of one read package would hide the edges
   of its other packages. A module with a link to a directory is then open,
   because the walk does not follow the link. A module zip holds no links.
   This read skips a package that does not parse, or that imports a path
   that no module of the build list holds. No build of the project reads
   such a package, because `go mod tidy` lists in the main `go.mod` each
   module that a package of the build imports. In the walk from the main
   module, such a package makes its module open.

A main `go.mod` with a `go` directive below 1.17, or with none, gives
`ErrUnpruned`. Before module graph pruning, `go.mod` need not list each
module of the build.

## Use

```go
sources, err := modgraph.Discover(modgraph.DiscoverOptions{
    Dir:         projectDir,  // the directory of the main go.mod
    Work:        workFile,    // the go.work that uses it, or "" outside a workspace
    ModuleCache: moduleCache, // GOMODCACHE; the caller finds it
    LocalRoot:   scanRoot,    // a local replace directive is read only inside it
})
if err != nil {
    return err
}
g, err := modgraph.Build(ctx, modgraph.Input{
    MainFS:  os.DirFS(projectDir),
    Modules: sources,
    Store:   store, // optional cache of the packages of the module cache
})
if err != nil {
    return err
}
path := g.Path([]string{g.Main.Path}, "github.com/x/y") // nil when no path reaches it
```

`Graph.Path` and `Graph.Reaches` take a set of start modules. A path that
meets an open node, or a module that `Build` did not walk (`Node.Walked` is
false), can reach each module: the path ends at that node.

`Discover` finds each module as the go command builds it: `vendor` when
`vendor/modules.txt` lists the module at its version, then the local
directory of a `replace` directive, inside `LocalRoot` after links are
resolved, then the module cache, then the `.mod` file of the module cache.
Each read of a local replacement resolves its links again, and a link that
leads out of `LocalRoot` makes the module open.
A caller that finds the sources in another way can pass its own
`[]Source`. `Module.Path` is the path that import statements use, also for
a replaced module, and `Source.Origin` is the module that the source holds.

`Discover` parses the main `go.mod` in full, because a lax parse drops the
`replace` directives. A directive that `golang.org/x/mod` does not know
fails the parse, and the caller keeps no graph.

## Workspaces

With `DiscoverOptions.Work`, `Discover` gives the build list of the
workspace for the module in `Dir`, as the go command builds each module of
a workspace. The caller finds the `go.work` file, as the go command does from
`GOWORK` and the parents of the module directory.

- The build list is the result of minimal version selection with module
  graph pruning, as `readModGraph` of the go command builds it. The graph
  holds the requirements of each module of the workspace and of the
  `go.mod` of each such requirement. A `go.mod` below go 1.17, a module of
  the workspace too, adds the requirements of each `go.mod` that it
  requires, and so on down the chain. Then, while a module that the build
  needs requires a module below its selected version, the `go.mod` of the
  selected version joins the graph with each `go.mod` that it requires.
  Each path takes its highest version in the graph.
- A requirement on a module of the workspace at a version reads the
  `go.mod` of that version. The build uses the code of the workspace.
- Each other module of the workspace is a local source at no version. Its
  code is read inside `LocalRoot`, and `Source.Origin` holds its directory.
- A `replace` directive of `go.work` wins. Else the `replace` directive of a
  module of the workspace applies. Two modules that replace one module with
  other code make `Discover` fail, as they make the go command fail.
- The go command reads the workspace from the `vendor` directory next to
  `go.work` when the `go` directive of `go.work` is go 1.14 or later and
  the first line of `vendor/modules.txt` holds the `## workspace`
  annotation that `go work vendor` writes. Each module that `modules.txt`
  lists with a package then comes from `vendor`, at the version of that
  list. Outside a workspace, `Discover` reads no `vendor` directory that
  holds this annotation.
- `go.work.sum` and the `go.sum` of each module give the hashes.

A wrong version can hide an edge. So `Discover` fails when it cannot know
the build list: a `go.mod` that the graph needs is not on the disk, a module
of the workspace is outside `LocalRoot`, or a `go.mod` has an `exclude`
directive. The caller then keeps no graph. `go mod download` in the
workspace fetches each `go.mod` that the graph needs.

`examples/modgraph` prints the graph of a module. Run `go mod download`
first: a module with no source on the disk is open, and one open node that
the build reaches makes each module reached.

## Cache

`Input.Store` caches the imports of each package of an immutable source,
the module cache. The key is the module that the source holds
(`Source.Origin` for a replacement), its version, the directory and the
`go.sum` hash. A
`vendor` directory and a local replacement can change, so they are read on
each run. `Build` stores no entry for a package that it could not read in
full.

## Oracle test

`TestBuildHoldsEachEdgeOfGoList` runs `go list -deps -test` on this
repository for four platforms and checks that the graph holds each module
edge, or that the edge leaves an open node. `MODGRAPH_ORACLE_DIRS` adds more
projects, and a project with `vendor/modules.txt` reads its dependencies
from `vendor`. `TestBuildHoldsEachEdgeOfGoListInAWorkspace` puts this
repository in a workspace with a probe module. For a workspace, and for a
project of `MODGRAPH_ORACLE_DIRS` with a `go.work`, the test checks each
module that the workspace uses. It also checks that `Discover` gives each
version of the build list that `go list -m all` gives.
`TestDiscoverGivesTheBuildListOfTheGoCommand` serves synthetic modules
from a proxy on the disk and compares `Discover` with `go list -m all` for
workspaces where the versions of two modules differ. The test needs the `go` command and a module cache, and skips
with neither. It sets `GOPROXY=off`, so it downloads nothing.

## Not covered

- A file larger than 32 MiB whose first 64 KiB does not parse makes its
  module open.
- A file budget, 200,000 files by default, caps a run. Past it the module
  that is next is open, and `Stats.Truncated` is set.
- `Build` tests a build constraint one tag set at a time. A constraint of
  more than 16 tags counts as one that can hold. After 4,194,304 tested
  sets in one run, each constraint counts as one that can hold. So a
  dependency of many such files cannot make a run slow.

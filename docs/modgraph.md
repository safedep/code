# Module graph

`modgraph` builds the module graph of a Go module: which module of the build
list imports which. A `go.mod` lists the modules of a build, but not the
edges between them. A caller needs the edges to tell that no code imports a
module, or that no other module can call one.

The package reads the code on the disk. It runs no `go` command, opens no
network connection and builds with `CGO_ENABLED=0`. It parses each Go file
with `go/parser` in `ImportsOnly` mode, so it reads the build constraint and
the import block of a file and stops.

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

1. `Build` reads each package of the main module, its test files too. It
   skips `testdata`, `vendor`, a directory or file whose name starts with
   `_` or `.`, and a directory with a `go.mod` of its own.
2. For each import, the resolver takes the modules of the build list whose
   path is a prefix of the import path, longest first. A module with source
   holds the package when it has the directory. A module with no source may
   hold it, so it gets an edge too, and the test goes on to a shorter prefix.
   A path that no module holds is the standard library when its first
   element has no dot. Else its importer is open.
3. The walk reads each package that an import reaches, as `go list -deps`
   does. A package of a dependency that no import reaches adds nothing. Test
   files of a dependency never count.
4. A file counts unless its build constraint can never hold, with every tag
   free except `ignore`. A `//go:build` line decides, else the `// +build`
   lines. A file of `//go:build purego` counts, because a project can build
   with `-tags purego`. A GOOS or GOARCH suffix of the file name never drops
   a file.
5. Once a reached node is open, each node counts as reached, and a module
   that the walk read no package of becomes open.

A main `go.mod` with a `go` directive below 1.17, or with none, gives
`ErrUnpruned`. Before module graph pruning, `go.mod` need not list each
module of the build.

## Use

```go
sources, err := modgraph.Discover(modgraph.DiscoverOptions{
    Dir:         projectDir,  // the directory of the main go.mod
    ModuleCache: moduleCache, // GOMODCACHE; the caller finds it
    LocalRoot:   scanRoot,    // a local replace directive is read only inside it
})
g, err := modgraph.Build(ctx, modgraph.Input{
    MainFS:  os.DirFS(projectDir),
    Modules: sources,
    Store:   store, // optional cache of the packages of the module cache
})
g.Reaches(g.Main.Path, "github.com/x/y")
```

`Discover` finds each module as the go command builds it: the target of a
`replace` directive, then `vendor` when `vendor/modules.txt` lists the
module at its version, then the module cache, then the `.mod` file of the
module cache. A caller that finds the sources in another way can pass its
own `[]Source`. `Module.Path` is the path that import statements use, also
for a replaced module.

`examples/modgraph` prints the graph of a module. Run `go mod download`
first: a module with no source on the disk is open, and one open node that
the build reaches makes each module reached.

## Cache

`Input.Store` caches the imports of each package of an immutable source,
the module cache, by module, version, directory and `go.sum` hash. A
`vendor` directory and a local replacement can change, so they are read on
each run. `Build` stores no entry for a package that it could not read in
full.

## Oracle test

`TestBuildHoldsEachEdgeOfGoList` runs `go list -deps -test` on this
repository for four platforms and checks that the graph holds each module
edge, or that the edge leaves an open node. `MODGRAPH_ORACLE_DIRS` adds more
projects, and a project with `vendor/modules.txt` reads its dependencies
from `vendor`. The test needs the `go` command and a module cache, and skips
with neither. It sets `GOPROXY=off`, so it downloads nothing.

## Not covered

- `go.work` workspaces. A caller keeps no graph for a workspace.
- A file larger than 16 MiB whose first 64 KiB does not parse makes its
  module open.
- A file budget, 200,000 files by default, caps a run. Past it the module
  that is next is open, and `Stats.Truncated` is set.

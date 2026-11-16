# Project graph

The project graph is the call graph of a whole project, across its files and
into the code of its dependencies. It answers one question. Does code that runs
reach an external symbol, as `yaml//load`? The answer is `found`, `not-found`,
`uncertain` or `no-such-target`. The library never gives `not-found` when it
does not know.

This guide tells a caller what to call, what each call gives and what the
answers mean. The code, its comments and the cases under
`test/projectgraph/cases` record each rule. The `description` of each
`expect.yaml` tells in prose what its case pins.

## Steps

Each step reads only the output of the step before it. A job can run the steps
at different times and keep the output of each step in a store.

| Step | Call | Reads | Gives |
| --- | --- | --- | --- |
| Index | `indexer.New`, `Indexer.Run` | The source files, with tree-sitter | One `fileindex.Index` per file |
| Link | `projectgraph.Build` | The indexes and the manifests | A `projectgraph.Graph` |
| Roots | `rootmatch.Find` | The indexes and the manifests | The roots of the graph. `Build` calls it, so a caller does not |
| Reach | `reach.Reach` | The graph | One `reach.Result` per target |
| Summary | `summary.Build` | The indexes of one package | A `summary.Result` that holds the `summary.Summary` of the package |

Only the index step parses source, and only the index step needs CGO. The other
steps build with `CGO_ENABLED=0`. An [overlay](#overlay) is an optional step
between the link step and the reach query. It merges edges that a model
proposed for the call sites that the link step did not resolve.
`examples/projectgraph` runs all the steps from the command line.

## Index

```go
languages, err := lang.AllLanguages()
fsys, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
    AppDirectories:    []string{root},
    ImportDirectories: []string{lodashDir},
    ImportUnits:       map[string]string{lodashDir: "npm:lodash@4.17.21"},
})
ix, err := indexer.New(indexer.Options{Languages: languages, Store: memstore.New(), Imports: true})
indexed, err := ix.Run(ctx, fsys)
```

| `indexer.Options` field | Meaning |
| --- | --- |
| `Languages` | The languages to index. The run reads only the files that one of them parses |
| `Store` | A `store.Store` that keeps the indexes between runs. A nil store parses every file |
| `Plugins` | Extra plugins that run on the tree of each file |
| `MaxFileBytes` | The size limit of a file. A larger file fails. Zero selects the default of the parser |
| `Workers` | The number of files to parse at once. Zero selects `GOMAXPROCS`. The result does not depend on it |
| `Imports` | Index the files of the import directories too. Each such file needs a package unit, as `npm:lodash@4.17.21` |

`indexer.Result` holds `Indexes` in path order and the status of each file:
`indexed`, `cached`, `missing`, `failed` or `truncated`. `Statuses` gives the
statuses for the link step, and `Unit` gives the indexes and the statuses of
one unit for a summary.

When the context ends, `Run` marks each file that it did not index as
`missing` and returns the partial result with the error of the context. The
reach query reports each missing file as a `budget` uncertainty. The store
keeps the finished indexes, so the next run gets further.

### Store

A store keeps each index under a `fileindex.Key`: the path, the hash of the
bytes, `fileindex.Version`, the version of the grammar and the language. A
change of any of them misses, so a store never serves an index of old rules.
`memstore.New` keeps the indexes in memory, and `localdbstore.New` in the
SQLite file of a `dry/localdb` manager. `Prune` of a `store.Pruner` removes
what no run read after a time. The store also keeps the artifact of an
`indexer.CachedPlugin`, by the key of the file and the name and version of
the plugin.

## Link

```go
units := map[string]iofs.FS{"npm:lodash@4.17.21": os.DirFS(lodashDir)}
g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{
    Root:      os.DirFS(root),
    Files:     indexed.Statuses(),
    Languages: indexed.Languages,
    TestPaths: fileindex.DefaultTestPaths,
    Units:     units,
})
```

`Build` resolves each import, call and base class into one graph. One input
gives one JSON form. Each unit of dependency code links as a project of its
own, and a `package` edge joins an external key to the definition in a unit.

| `projectgraph.Options` field | Meaning |
| --- | --- |
| `Root` | The manifests of the project, as `pyproject.toml`. A nil root reads no manifest |
| `SysPath` | More Python import roots, relative to the project root |
| `Files` | The status of each file, from `indexer.Result.Statuses`. A file with no complete index becomes a module node with that status |
| `Languages` | The language of each file, from `indexer.Result.Languages`. With it, the query leaves out the budget of a file that the program does not run |
| `TestPaths` | Globs that mark the test files. `fileindex.DefaultTestPaths` holds the common ones |
| `Rules` | Root rules that go over the embedded ones by id |
| `Units` | The files of each unit of dependency code, by unit, from the directory that the paths of the package start at |
| `Surface` | Link an external node to each exported symbol of each unit, as a summary needs |
| `MaxEdges` | The cap of the edges. Past it the step adds no edge and records a truncation. Zero selects 5,000,000 |
| `MaxExportHops` | The cap of a walk through re-exports and bases. Zero selects 10 |

The standard libraries are units too: `GOROOT/src` as `go:std@<version>` and
the `Lib` directory of CPython as `python:std@<version>`. Without them, the
standard library is code outside the graph, which makes some targets uncertain.

## Graph basics

A `Graph` holds `Nodes`, `Edges`, `Unresolved` sites, `Roots`, `Files`,
`Truncated` limits, the manifest `Problems` and the `Stats`. A key names a node,
as `handlers.py//Parser//read` or the external key `yaml//load`.

| Node kind | Meaning |
| --- | --- |
| `module` | The top level of a file |
| `function`, `method`, `constructor`, `class` | A definition of a file |
| `interface` | A TypeScript interface. It holds no code |
| `variable` | A name that a module top level assigns. A walk goes on to its value |
| `export` | A name that a file gives on from another file, or a type alias |
| `external` | A symbol outside the project, by its key |

| Edge kind | Made by | Confidence |
| --- | --- | --- |
| `direct` | A call that resolves to one definition | `high` |
| `import` | An import, which runs the top level of the module | `high` |
| `export` | A re-exported name, to the definition that it stands for | `high` |
| `member` | A method call on a receiver of a known class | `high` |
| `assignment` | A call of a name that an assignment binds to a function | `high` |
| `package` | An external key, to the definition in a unit. It is no call | `high` |
| `cha` | A method call, to the override of each subclass | `medium` |
| `field` | A JavaScript or TypeScript member call on an unknown receiver, to each first-party function of the name | `low` |
| `name` | A call on an unknown receiver in another language, to each definition of the name | `low` |
| `wildcard` | A name that a wildcard import can bring in | `low` |
| `reference` | A use of a function, class or import as a value. The code that gets the value can call it | `low` |
| `llm` | An edge of an overlay file | `low` |

| Confidence | Meaning |
| --- | --- |
| `high` | The link step resolved the callee from the code |
| `medium` | The callee is an override that the receiver can hold |
| `low` | The callee matches by name or by a use as a value. It can run, and the code does not prove it |

An `UnresolvedSite` is a call that the link step did not resolve. `Caller`,
`Site` and `Text` say where it is. `Reason` says why, as `unknown-receiver` or
`ambiguous` (more than 20 candidates, so no edge), and
`fileindex.UnresolvedReason` documents each reason. A `Method` narrows the site
to the targets of that method name. `Truncated` lists the limits that the link
step hit. The query reports each one as a `budget` uncertainty.

## Roots

A root is a node that runs with no caller in the graph: a main function or a
main guard, an entry of a manifest, a route or a handler of a framework, or a
test. The rules in `fileindex/roots/rules.yaml` define them, and `Root.Rule`
names the rule that marked a root. A framework rule applies only when a
manifest depends on the framework or a file imports it.

A language family with no root of kind `main`, `manifest` or `framework` gets
the library-surface fallback. Each exported definition of that family is then a
root, and `Graph.Stats.RootFallback` is true. The case
`python/handler-beside-js-package` shows the fallback for one family alone.

## Sources

A source is an entry point that the caller names and no rule marks. It is the
key of a definition or of a module top level of a first-party file, and a root
of kind `query`. `reach.SourcesAndRoots`, the default, adds the sources to the
roots of the rules. `reach.SourcesOnly` searches from the sources alone, with
no root fallback. A source that names no first-party node is an error that
wraps `reach.ErrUnknownSource`, so a typo never gives `not-found`.

## Reach

```go
target, err := reach.TargetByImport(langcode.Python, "yaml", "load")
results, err := reach.Reach(ctx, g, reach.Query{Targets: []reach.Target{target}})

source, err := reach.SourceInFile("app/server.py", "main")
results, err = reach.Reach(ctx, g, reach.Query{
    Targets:    []reach.Target{target},
    Sources:    []reach.Source{source},
    SourceMode: reach.SourcesOnly,
})
```

A target is an external key, as `os//exec//Command`. A key that ends in `//*`
matches every key under it. `TargetByKey`, `TargetsByKey` and `TargetByImport`
make targets, and `SourceByKey`, `SourcesByKey` and `SourceInFile` make
sources. Each one validates the key.

| `reach.Query` field | Meaning |
| --- | --- |
| `Targets` | The targets. One search serves all of them |
| `Sources` | Entry points to search from, in addition to the roots or in place of them |
| `SourceMode` | `reach.SourcesAndRoots` or `reach.SourcesOnly`. `SourcesOnly` needs a source |
| `IncludeTests` | Search from the test roots too. Without it, a target that only a test reaches gets a `TestPath` and no `Path` |
| `MaxDepth` | The cap of the hops of the search. Zero selects 64 |

`Reach` returns one `reach.Result` per target, in the order of the query.

| Status | Meaning |
| --- | --- |
| `found` | A path from a root reaches the target. `Path` holds the shortest one |
| `found` with low confidence | The status is `found`, and a hop of `Path` has the confidence `low` |
| `not-found` | No path reaches the target, and no uncertainty concerns it |
| `uncertain` | No path reaches the target, and at least one uncertainty concerns it |
| `no-such-target` | The key names no node of the graph, and no uncertainty can name it. Check the form of the key |

With a source or `IncludeTests`, the search also enters the module top levels
that the program runs first. A hop into such a top level has the resolver
`load` (`reach.LoadResolver`).

`reach.PathString` writes a path as
`main.py:3 -> handlers.py:12 (name) -> yaml//load`, with the edge kind of each
low hop. `Result.String` writes the whole result as lines.

| Uncertainty kind | Cause |
| --- | --- |
| `unresolved-call` | An unresolved site in code that the search reached |
| `dynamic-flag` | A dynamic feature, as `eval`, in code that the search reached |
| `budget` | A missing, failed or truncated file, a limit of the link step, or a search that the depth cap or the deadline stopped. `Searched.DepthCut` and `Searched.Stopped` tell which |
| `root-fallback` | The roots are the library surface. Every result carries it, and it alone does not make a result `uncertain` |

An uncertainty concerns every target unless its `Method`, `ImportPrefix`,
`Unit`, `Callback` or `Language` field narrows it, as `reach.Scoped.Concerns`
decides. `Result.Uncertainties` lists at most three per kind, and
`Searched.Uncertainties` keeps the full count. `SetAside` lists the sites that
a rule proved cannot reach the target.

A Go call of a function value concerns no target in a search from the roots of
the rules, because that search reaches each function that code stores as a
value. In a search with a source, and in a summary, it concerns every target.

## Overlay

An overlay is a JSON file of `llm` edges for the unresolved sites of a built
graph. No code of this module calls a model. A person or a job writes the file,
and a reviewer edits the file to remove an edge. The package documentation of
`projectgraph/overlay` gives the format.

```go
f, err := overlay.ReadFile("overlay.json")
report, err := overlay.Merge(g, f)
```

`Merge` drops an edge whose callee is no node (`undefined-callee`) or whose
site is no unresolved site (`no-unresolved-site`), and `Report.Diagnostics`
lists it. The unresolved sites stay, so an overlay can make a target `found`
and never turns `uncertain` into `not-found`.

## Summary

```go
indexes, files := indexed.Unit("npm:lodash@4.17.21")
r, err := summary.Build(ctx, indexes, projectgraph.Options{
    Files:     files,
    Languages: indexed.Languages,
    Units:     map[string]iofs.FS{"npm:lodash@4.17.21": os.DirFS(lodashDir)},
})
status := r.Summary.Status("lodash//template", target)
path, ok, err := r.Path(ctx, "lodash//template", target)
```

A summary describes one package version alone. For each exported symbol it
holds the external symbols that the symbol reaches and the uncertainties of the
search. A job stores one summary per package version, and a project joins a
call of a symbol to the summary of its package. `Status` answers as a reach
query of the unit graph from the symbol would. A summary stores no path, and
`Path` runs one search to get one.

## Versions

| Constant | Covers | Carried by |
| --- | --- | --- |
| `fileindex.Version` | The index records and the rules that fill them | `fileindex.Key` |
| `projectgraph.Version` | The link rules | `Graph.Stats.Version` |
| `reach.Version` | The search rules | `Result.Searched.Version` |
| `summary.Version` | The summary format and its rules | `Summary.Version`, with `IndexVersion` and `LinkVersion` |

A store serves an index only when its whole `fileindex.Key` matches, so a new
`fileindex.Version` or a new grammar misses every older entry. A store of
summaries keys on the three versions of a `Summary`.

`test/projectgraph/version_test.go` records the hash of the graphs, of the
results and of the summaries of the cases per set of versions.
`plugin/indexer/plugin_test.go` records the hash of the golden indexes per
`fileindex.Version`. A change of an output fails these tests. Bump the version
that owns the rule, add a row, and keep the old rows.

## Known limits

- A Rust `use` inside a block binds its name for the whole file.
- A Rust `#[macro_use]` module declared after a call counts as in scope for it.
- A Rust glob re-export of a crate with no unit reads `no-such-target`.
- In Java, the order in which the link step reads the MROs of two classes
  that import each other statically can decide an answer.
- The markup of a component file, or an opaque route export, makes JavaScript
  targets `uncertain` (`javascript/component-reexport-star`).
- `err.Error()` becomes a `function-value` record when the graph has a
  function field named `Error`.
- A Go call of a field of a package with no unit, or a Rust call of a
  parameter that holds a function, can read `not-found` from a source.
- A target that the project names only in a value, as `legacy.Config{}` of a
  package with no unit, reads `no-such-target`.
- Code outside the graph gets records in Go only. The Python standard library
  with no `python:std` unit can call back with no record.
- A function that only assembly of the Go standard library calls is outside
  the callback scope (`go/function-no-body`).
- A Rust name that two `cfg` attributes declare names no type
  (`rust/alias-cfg-two`).
- Nuxt has no root rule. The SvelteKit `$lib` alias resolves to `src/lib`, and
  an import of a moved lib folder reads `uncertain`
  (`javascript/sveltekit-lib-moved`).

## More

- `examples/projectgraph` runs the steps on a directory, with units, sources
  and an overlay.
- `test/projectgraph/README.md` describes the test sets and how to add a case.
- `test/projectgraph/cases` holds one case per rule, by language.

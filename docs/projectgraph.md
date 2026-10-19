# Project graph

The project graph is the call graph of a whole project, across its files
and into the code of its dependencies. It answers one question: does code
that runs reach an external symbol, as `yaml//load`? The answer is `found`,
`not-found` or `uncertain`, and the library never gives `not-found` when it
does not know.

Five calls make the graph. Each one reads only the output of the one
before it, so a job can run them at different times and keep the output of
each in a store.

| Step | Call | Reads | Gives |
| --- | --- | --- | --- |
| Index | `indexer.New` and `Indexer.Run` | The source files, with tree-sitter | One `fileindex.Index` per file |
| Link | `projectgraph.Build` | The indexes and the manifests | A `projectgraph.Graph` |
| Roots | `rootmatch.Find` | The indexes and the manifests | The roots of the graph. `Build` calls it |
| Reach | `reach.Reach` | The graph | One `reach.Result` per target |
| Summary | `summary.Build` | The indexes of one package | A `summary.Summary` of the package |

The index step is the only one that parses source. It needs CGO. The other
steps read records and build with `CGO_ENABLED=0`.

An [overlay](#overlay) is an optional step between the link step and the
reach query. It merges edges that a model proposed for the call sites
that the link step could not resolve.

`examples/projectgraph` runs all five from the command line.

## Index

```go
ix, err := indexer.New(indexer.Options{
    Languages: languages,
    Store:     store,
    Workers:   4,
})
indexed, err := ix.Run(ctx, fsys)
```

`New` validates the options: a run needs a language, two languages
cannot share a file extension, and a size limit or a number of workers
cannot be negative. `Run` walks the file system, reads each file once and gives the tree of the
file to the index plugin. The index of a file holds what the link step
needs: the imports, the exports, the definitions, the classes with their
bases, the calls with their resolved targets, the references, the
unresolved call sites and the dynamic flags. It holds no tree-sitter node
and no source text past a short call text.

`indexer.Options` has these fields.

| Field | Meaning |
| --- | --- |
| `Languages` | The languages to index. The run reads only the files that one of them parses |
| `Store` | A `store.Store` that keeps the indexes between runs. A nil store parses every file |
| `Plugins` | Extra plugins that run on the tree of each file |
| `MaxFileBytes` | The size limit of a file. A larger file fails. Zero selects 4 MiB |
| `Workers` | The number of files to parse at once. Zero selects the number of CPUs. The result does not depend on it |
| `Imports` | Index the files of the import directories too. Each one needs a package unit, as `npm:lodash@4.17.21` |

`indexer.Result` holds the indexes in path order, the status of each file
(`indexed`, `cached`, `missing`, `failed`, `truncated`), the same for the
files of dependency code, and the report of the run. `Result.Unit` returns
the indexes and the files of one unit of dependency code.

When the context ends, `Run` stops, marks each file that it did not index
as `missing` and returns the result so far with the error of the context.
The link step takes the partial result, and the reach query reports each
missing file as a budget cause.

### Store

A `store.Store` keeps each index by `fileindex.Key`: the hash of the bytes
of the file, its path, `fileindex.Version`, the version of the grammar and
the language. A file whose bytes, path or any version changed misses, so a
store never serves an index of old rules. `memstore.New` is a store in
memory, and `localdbstore.New` is a store in the SQLite file of a
`dry/localdb` manager.

A `store.Pruner` is a store that marks each read and removes what no run
has read since a time, with `Prune`. Both stores are pruners.

### Plugin cache

A plugin needs the tree of a file, so a run with a plugin parses every
file of the languages of the plugin. A plugin that implements
`indexer.CachedPlugin` returns an artifact per file from `Analyze`, and the
store keeps it under the key of the file, the name of the plugin and its
version. On the next run the store serves the artifact and the plugin gets
it through `Restore` with no parse. A new plugin version misses every older
artifact. `Prune` removes artifacts as it removes indexes.

## Link

```go
g, err := projectgraph.Build(ctx, indexed.Indexes, projectgraph.Options{
    Root:      os.DirFS(root),
    Files:     indexed.Statuses(),
    TestPaths: fileindex.DefaultTestPaths,
    Units:     units,
})
```

`Build` resolves each import to a file of the project, each call to its
definition and each class to its bases, and links them into one graph. The
graph has nodes, edges, roots, unresolved sites and truncations, each in a
fixed order, so one input gives one JSON form. It does not depend on the
order of the indexes.

`projectgraph.Options` has these fields.

| Field | Meaning |
| --- | --- |
| `Root` | The manifests of the project, as `pyproject.toml`. A nil root reads no manifest |
| `SysPath` | More Python import roots, relative to the project root |
| `Files` | The status of each file of the index step. A file with no complete index becomes a module node with that status |
| `TestPaths` | Globs that mark the test files. `fileindex.DefaultTestPaths` holds the common ones |
| `Rules` | Root rules that go over the embedded ones by id |
| `Units` | The files of each unit of dependency code, by unit. The link step links each unit as a project of its own |
| `Surface` | Link an external node to each exported symbol of each unit, as a summary needs |
| `MaxEdges` | The cap of the edges. Past it the step adds no edge and records a truncation. Zero selects 5,000,000 |
| `MaxExportHops` | The cap of a walk through re-exports and bases. Zero selects 10 |

### Edge kinds

Each edge has a kind and a confidence. The kind says what made the edge,
and the confidence, from `projectgraph.EdgeKind.Confidence`, says how
sure the link step is that the code of the caller runs the callee. A
path shows the kind of each hop of low confidence, so a reader sees the
weak hop. The graph keeps one edge per caller, callee and site, the one
of the highest confidence.

| Kind | What makes it | Example | Confidence |
| --- | --- | --- | --- |
| `direct` | A call whose callee the link step resolves to one definition: a function by its name, a name that an import binds, or a constructor | `handlers.parse(path)` in `main.py` gives `main.py -> handlers.py//parse` | `high` |
| `import` | A module top level and the module that it imports. The import runs the top level of the module | `import handlers` gives `main.py -> handlers.py` | `high` |
| `export` | A re-exported name and the definition that it stands for, through a chain of re-exports | `from .loader import parse` in `pkg/__init__.py` gives `pkg/__init__.py//parse -> pkg/loader.py//parse` | `high` |
| `member` | A call of a method on a receiver whose class the link step knows, as `self`, an instance that a constructor made, or a value of a declared type | `self.read(path)` in `Parser.parse` gives `handlers.py//Parser//parse -> handlers.py//Parser//read` | `high` |
| `assignment` | A call of a name that an assignment binds to a function | `load = yaml.load` then `load(f)` gives `main.py -> yaml//load` | `high` |
| `package` | An external key and the definition that a unit of dependency code gives for it. It is no call. It marks where a path leaves the project and goes on into a package | `lodash//template -> npm:lodash@4.17.21//template.js//template` | `high` |
| `cha` | A call of a method on an instance whose static class has subclasses, to the override of each subclass, by class hierarchy analysis. A `reference` edge with the rule `override` joins the class to each override | `shape.area()` with `Circle(Shape)` gives `main.py//draw -> shapes.py//Circle//area` | `medium` |
| `field` | A JavaScript or TypeScript member call whose receiver the link step does not know, to each first-party method, object property or property assignment of the name, as the field-based rule of ACG | `req.parse()` gives `src/app.js//handle -> src/parsers.js//parse` | `low` |
| `name` | A call whose receiver the link step does not know, in any other language, to each first-party function, method, constructor or class of the name | `obj.parse()` in Python gives `main.py//run -> handlers.py//Parser//parse` | `low` |
| `wildcard` | A name that a wildcard import can bring in, to the definition of the name in the module, or to the external key | `from handlers import *` then `parse()` gives `main.py -> handlers.py//parse` | `low` |
| `reference` | A definition that uses a function, a method, a class or an import as a value, to that value. The code that gets the value can call it | `run(process)` with `from tasks import process, run` gives `main.py -> tasks.py//process`, and `run` can call it | `low` |
| `llm` | An edge of an overlay file. A model proposed it for an unresolved site, and no code of this module calls a model. See [Overlay](#overlay) | `main.py//dispatch -> parsers.py//parse` with the resolver `model m1, prompt v2` | `low` |

A call that the link step cannot resolve is an unresolved site with a
reason: `computed-name`, `dynamic-import`, `unknown-receiver`,
`no-definition`, `ambiguous` or `limit`. The site keeps the method name of
the call, so the reach query can tell which targets it concerns.

A call on the result of another call, as `html.NewTokenizer(r).Next()`,
needs the type of that result. When the link step does not know it, the
site is an `unknown-receiver` record with the method name, so a target of
that method is `uncertain` and not `no-such-target`.

## Overlay

An overlay is a file of `llm` edges that resolve unresolved sites of a
built graph. No code of this module calls a model. A person writes the
file, or a data-plane job asks a model for the callee of each unresolved
site and writes what it answers. A reviewer removes an edge by editing
the file.

The file is JSON with a `version`, now `overlay.Version` = `1`, and a
list of `edges`. Each edge names the unresolved site by its `caller` and
its `site`, and the `callee`, a node key that the graph holds, first
party or external. Its `kind` is `llm`, its `confidence` is `low`, and it
carries the `model`, the `prompt_version`, the `input_hash` of the prompt
input and the raw `output` of the model, so a reader can audit the edge.
The package documentation of `projectgraph/overlay` gives the full form.

```go
f, err := overlay.ReadFile("overlay.json")
report, err := overlay.Merge(g, f)
```

`overlay.ReadFile` decodes and validates the file. An unknown field, a
second object in the file, a version other than `1`, or an edge with a
confidence other than `low` is an error, so a file of another format or
a model output that holds two objects never merges in part.

`overlay.Merge` adds the edges to the graph with `Graph.AddEdge`, with
the resolver `model <model>, prompt <prompt_version>`, so a path through
the edge names them. It drops an edge whose callee is no node of the
graph (`undefined-callee`), and an edge whose caller and site match no
unresolved site (`no-unresolved-site`), and lists each one in
`Report.Diagnostics`. The unresolved sites stay in the graph. So an
overlay edge can turn a target that no path reaches into `found`, but it
never removes an uncertainty, and so it never turns `uncertain` into
`not-found`.

## Roots

`rootmatch.Find` marks the roots of the graph with the rules of
`fileindex/roots/rules.yaml`: a main function or a main guard, an entry of a
manifest, as a `package.json` bin or a `pyproject.toml` script, a route or a
handler of a framework, and the test files. A rule applies only when a
manifest of the project depends on the framework that it names, or when a
file imports it.

With no root of kind `main`, `manifest` or `framework`, every exported
definition is a root, and `Graph.Stats.RootFallback` says so. Each result
of a reach query then carries a `root-fallback` uncertainty.

## Sources

The rules decide the roots by default. An engineer who knows the entry
points of the application names them as sources of the query, and the
search starts from them. A source is a `reach.Source`: the key of a
definition or of a module top level of a first-party file.

| Constructor | Takes | Gives |
| --- | --- | --- |
| `reach.SourceByKey("app/server.py//main")` | The key, as a configuration or a flag writes it | `app/server.py//main` |
| `reach.SourceInFile("src/api.ts", "Router", "handle")` | The path of the file and the qualified name of the definition. With no name, the module top level of the file | `src/api.ts//Router//handle` |

`reach.SourcesByKey` reads a list of keys. `Query.SourceMode` says what
the sources do.

| Mode | Roots of the search |
| --- | --- |
| `reach.SourcesAndRoots` | The sources and the roots that the rules marked. It is the default |
| `reach.SourcesOnly` | The sources alone. The root fallback does not apply, so no result carries a `root-fallback` uncertainty. It needs at least one source |

A source must name a first-party node of the graph. A key of no node, an
external symbol or a definition of dependency code is an error of `Reach`
that wraps `reach.ErrUnknownSource`, one per bad source, so a typo never
gives `not-found`. A source is a root of kind `query` in
`Searched.Roots`, and a path from it starts with a hop of root kind
`query`. `reach.PathString` writes such a path as
`app/server.py//main (query) -> handlers.py:12 -> yaml//load`. All other
uncertainty rules stay.

A tool such as vet gives an engineer both forms: a list of entry points
in its configuration, and a repeatable flag, each as `path:name.name` or
as a key, and a switch between the two modes. With `SourcesAndRoots`
a named entry point adds what the rules missed. With `SourcesOnly` the
engineer owns the roots, as for a library whose rules give only the
fallback. The tool reports an unknown source as an error of the
configuration, and does not run the query.

`Reach` returns an error for `SourcesOnly` with no source, and for a mode
that is not one of the two constants. A search from no root would give
`not-found` for every target.

`examples/projectgraph` takes `-source app/server.py:main` and
`-sources-only`. It exits with code 2 on `-sources-only` with no
`-source`.

## Reach

```go
target, err := reach.TargetByImport(langcode.Python, "yaml", "load")
results, err := reach.Reach(ctx, g, reach.Query{Targets: []reach.Target{target}})
```

A `reach.Target` is an external symbol that a query asks for. Its key is
the key form of an import and the names under it: a Go import path with
`//` for `/`, as `os//exec//Command`, and a Java class under its package,
as `java//lang//Thread//sleep`. A target that ends in `//*` matches every
external key under it. Two constructors make one, and each one validates
the key, so a query never holds a key that no node can have.

| Constructor | Takes | Gives |
| --- | --- | --- |
| `reach.TargetByKey("yaml//load")` | The key, as a configuration or a flag writes it | `yaml//load` |
| `reach.TargetByImport(langcode.Go, "os/exec", "Command")` | The module as the source of the language writes it, and the names under it | `os//exec//Command` |

`reach.TargetsByKey` reads a list of keys. `Target.Key` returns the key,
and the JSON form of a target is its key. One breadth-first search from
the roots, and from the sources of the query, serves every target of a
query.

A `reach.Result` has the status of the target, the shortest path from a
root, the call sites of the target, the uncertainties of the answer and
what the search did.

| Status | Meaning |
| --- | --- |
| `found` | A path from a root reaches the target. `Path` holds the shortest one |
| `not-found` | No path reaches the target, and no uncertainty concerns it |
| `uncertain` | No path reaches the target, and an uncertainty concerns it |
| `no-such-target` | The key names no external node of the graph, and no uncertainty could name it |

`reach.Result.String` writes a result as lines, and `reach.PathString`
writes a path as `main.py:3 -> handlers.py:12 -> yaml//load`.

## Summary

```go
r, err := summary.Build(ctx, indexes, projectgraph.Options{Files: files, Units: units})
```

A summary describes one package version of dependency code on its own:
for each exported symbol, the external symbols of the dependencies of the
package that the symbol reaches, and the uncertainties of the search. A
job stores one summary per package version, and a project joins a call of
a symbol to the summary of its package.

The summary stores reach sets and no path. It computes the sets once per
strongly connected component of the graph, in reverse topological order,
so a package of many symbols costs one walk and not one search per
symbol. `Summary.Status(symbol, target)` gives the status of a
`reach.Target` as a reach query of the unit graph from the symbol would,
and `Result.Path` runs that one search when a finding needs the path.

## Versions

Each step has a version constant. A change of a rule or a field bumps it.

| Constant | What it covers | What a bump does |
| --- | --- | --- |
| `fileindex.Version` | The index records and the rules that fill them | Every stored index misses |
| `projectgraph.Version` | The link rules. `Graph.Stats.Version` carries it | A diff of two graphs shows the change |
| `reach.Version` | The search rules. `Searched.Version` carries it | A diff of two results shows the change |
| `summary.Version` | The summary format and its rules | A store of summaries keys on it, with the index and the link versions |

The version of the grammar of each language is part of `fileindex.Key`, so
a new grammar misses the store too. The tests of `plugin/indexer` and
`test/projectgraph` record the hash of the golden files per version, and a
change of a golden file fails until the version that owns the rule is
bumped.

## The honesty rule

The library never says `not-found` when it does not know. Everything that
it cannot resolve becomes an uncertainty on the answer, with the site that
causes it.

| Kind | Cause |
| --- | --- |
| `unresolved-call` | A call that the link step could not resolve, on a path from a root |
| `dynamic-flag` | A dynamic feature, as `eval` or a computed attribute, in code on a path from a root |
| `budget` | A missing, failed or truncated file, a limit of the link step, or a search that the depth cap or the deadline stopped |
| `root-fallback` | No root rule matched, so every exported symbol is a root |

An uncertainty concerns every target, unless its site says less. A call
with an unknown receiver and a method name concerns the targets with that
last segment, and the targets that any definition of that name reaches. A
computed import with a static prefix concerns the targets under the
prefix. A call into a package at no known definition concerns the targets
that the code of the package reaches. A result caps the uncertainties that
it lists at three per kind and keeps the full count in `Searched`.

A file of dependency code that the index step did not complete counts
only for a search that enters its package. A file of the project counts
for every search, because it can hold a root.

A summary keeps every uncertainty of a symbol in a table, with the targets
that its scope concerns, so `Status` applies the same rule with no graph.

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

A generated Go file of a unit, with a `// Code generated ... DO NOT EDIT.`
line or a name that starts with `tables`, can hit a limit of the index,
as a table of thousands of entries. Its index then comes from the file
with each top-level var value that holds only data blanked. Function bodies
and values with a call or a function literal stay, so the calls of a
generated file, as a `.pb.go` file, stay in the graph. The index keeps the
hash of the file. If the skeleton still hits a limit, the file stays
truncated, and reach counts it as budget.

The JavaScript language reads `.jsx` files with its grammar, which parses
JSX. It also reads the component files of Vue, Svelte and Astro (`.vue`,
`.svelte`, `.astro`). The parse reads only the `<script>` blocks of such a
file, and the Astro code between the two `---` lines at the top. Each byte
outside them becomes a space, so each import and call keeps its line and
column in the file. A file whose blocks have `lang="ts"` or
`lang="typescript"` is TypeScript, and so is every Astro file. A block
with a `src` attribute holds no code. The index of the file holds the hash
of the bytes of the file.

The index leaves out a Go file of a unit only when no Go build reads it by
default: a `_test.go` file, a file with `//go:build ignore`, and a file
whose constraint needs a tag that the go command never sets, as `purego`.
It keeps the files of every GOOS and GOARCH, so a reach answer holds for
each platform that a project builds for. `Result.Excluded` lists the files
that it leaves out.

When the context ends, `Run` stops, marks each file that it did not index
as `missing` and returns the result so far with the error of the context.
The link step takes the partial result, and the reach query reports each
missing file as a budget cause.

Before it returns, `Run` puts the indexes and the artifacts of the files that
it finished in the store. A run with the same deadline then reads them from
the store and gets further into the project. The store gets no index and no
artifact of a file that the deadline cut. An index with a deadline
truncation fails its file. A cached plugin that returns after the end of
the context can have stopped early with no error, so the store gets no
artifact from such a call. The next run parses that file again.

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

A cached plugin that reads the call graph of a file implements
`indexer.GraphPlugin`. The run builds the call graph once for the index, and
gives it to `AnalyzeGraph` with the tree. The plugin does not build the graph
again. The extra plugins run on one goroutine, so a second build would slow
the whole run. A file that the workers finished before the deadline passed
gets its full graph. With a done context, `Analyze` would build none.

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
| `Languages` | The language of each file of the index step, as `indexer.Result.Languages` gives it. The module node of a file with no index gets its language from here, so the reach query can tell a file that the program does not run |
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
`no-definition`, `ambiguous`, `limit`, `interface-receiver`,
`trait-receiver`, `no-dependency-source` or `function-value`. The site keeps the method name of the call, so the
reach query can tell which targets it concerns. A `function-value`
record concerns no target. [A call of a function value](#a-call-of-a-function-value)
gives the reason.

A site with more than 20 candidates of its method name, as `render` in a
Django project, is `ambiguous`. The site gets no edge, so the search cannot
follow what the candidates call. The site keeps its method name, because
the call can run only a method of that name. A record with a method name
concerns the targets with that last segment and the targets that a
definition of the name reaches, so it concerns what each candidate
reaches.

A call on the result of another call, as `html.NewTokenizer(r).Next()`,
needs the type of that result. When the link step does not know it, the
site is a record with the method name, so a target of that method is
`uncertain` and not `no-such-target`. A call on the result of a builtin of
the language, as `Promise.resolve(x).then(f)`, is a record too. Some
builtins give back a value that the call passes, or an element of it, as
`new(thing)` in Go, `max(t, t)` in Python or `Object.assign(p, o)` in
JavaScript. The method then runs on a value of the project or of a
dependency.

### Result types

The Go index records the type name of each result of a function, by
position, and each name that an assignment or a short variable declaration
binds to a result, as `z` in `z := html.NewTokenizer(r)` and `err` in
`n, err := html.Parse(r)`. The blank name `_` binds nothing. The link step
reads the definition of the function, in the project or in the unit of
dependency code that gives its package, and binds the type of the result
to the name. A call on the name then gets the external key of the method
of the type, as `golang.org//x//net//html//Tokenizer//Next`, and a package
edge joins the key to the definition in the unit. A chained call, as
`pkg.F().M()` or `a.B().C()`, takes the type of each result in turn.

| Result | What the call gets |
| --- | --- |
| A type of a unit, or of the project | The edge to the method of the type |
| An interface type | The `cha` edges to the implementations in the graph. An implementation in a unit gets its edge through its external key. With no implementation, the site is a record with the reason `interface-receiver` |
| A function of a package that no unit gives | A record with the reason `no-dependency-source` |
| A result with no named type, or a function that the unit does not define | A record with the reason `unknown-receiver` |
| Two definitions of the function, as in two platform files of a Go package, with two result types | A record with the reason `unknown-receiver`. The walk follows one type, and a method of the other type would be `not-found` |

Rust follows the same rules, with these additions.

- The link step links the code of a `cargo:` unit, as the crate directory
  in the cargo registry or in `vendor/`. The root module of the crate is
  the unit, so two versions of one crate stay apart. The external key of
  an item is the path that another crate writes, and an item of a private
  module that the root gives on with `pub use` gets the shortest path, as
  `smallvec//SmallVec`.
- A Cargo.toml can rename a dependency, as `helper = { package = "kit" }`.
  A path of the crate writes `helper`, and the link step reads it as the
  crate `kit` of the unit. The external node keeps the written name, as
  `helper//serve`, and the package edge goes on to the definition in the
  unit, whose external key is `kit//serve`.
- The index records the return type of each function and method, with
  `Self` as the type of its `impl`. A type drops `&`, `&mut`, `Box<..>`,
  `Rc<..>`, `Arc<..>` and its type arguments, so `&mut SmallVec<[u8; 4]>`
  is `SmallVec`. An `Option<T>` or a `Result<T, E>` result keeps `T` in
  `Returns` and its wrapper in `Wraps`. A wrapper matches by the last name
  of its path, so `std::io::Result<T>` and `anyhow::Result<T>` wrap `T`.
  `unwrap` and `expect` give `T`, and so does a `?`. The index records each
  `?` in the receiver of a method call as the name `?` in the target, as
  `dep//load//?//run` for `dep::load()?.run()` and for `t.run()` after
  `let t = dep::load()?`. A call of any other method of the wrapper, as
  `map` or `inspect`, has no type, so the site is a record with the reason
  `unknown-receiver`.
- A declared type of a parameter, a field or a local that a unit gives
  binds the receiver. An associated function, as `Type::from_vec(v)`,
  binds the type that it returns, and `clone`, `to_owned`, `default` and
  `from` on a type of a unit keep the type also when the crate derives
  them.
- `Box::new`, `Rc::new` and `Arc::new` hold their argument, so their
  value is no instance of the pointer. A method call on it runs a method of
  the argument. With no declared type, as `let w = Box::new(Html {})`, the
  call is a record with the reason `unknown-receiver`. A declared type of
  the local, as `let w: Box<dyn Write> = Box::new(Html {})`, binds the
  receiver. A declared type binds the receiver only where the index of the
  file knows no type of the value. So `let s: String = String::new()` keeps
  the target `String//shout` of `s.shout()`. The function must also bind
  the name once, as a parameter or a typed `let`, before the call. A `let`,
  a `match` arm, an `if let`, a `for` or a closure can bind the name again
  with another type. A call on such a name keeps its record, so a method of
  the real type does not read as `not-found`.
- `extern crate x;`, `extern crate x as y;`, `use x;` and `pub use x as y;`
  of a whole crate bind `x` or `y` in their module to the crate. A path
  through the binding is a path of the crate, so
  `crate::secure::rand::thread_rng()` after `extern crate rand;` in the
  module `secure` calls `rand//thread_rng`, and `crate::ffi::ErrorCode`
  after `pub use libsqlite3_sys as ffi;` names
  `libsqlite3_sys//ErrorCode`. A `use` of such a path into a crate that no
  unit gives is a record with the reason `no-dependency-source` and the
  crate as its import prefix, so only a target of that crate sees it. A
  first name that names no module that the file sees, as a child module or
  a module of the root, names a crate, also when a file of the project has
  that name, as `extern crate schannel;` in `src/imp/schannel.rs`.
- A path can start with a name that another `use` of the module binds. So
  `helpers::run` after `use crate::deep::helpers;` names the function of
  the module `deep::helpers`, and `sub::f` after `use dep::sub;` names
  `dep//sub//f`. A `use` never reads its own binding, so
  `use smallvec::smallvec;` names the crate `smallvec`.
- The index records a type alias, as `pub type Image<P> = ImageBuffer<P>;`,
  as a definition of the kind `alias`. The link step resolves the type
  that it names, as the file of the alias names it, when a walk first
  reads the alias. A type of the graph gets an `export` edge from the
  alias. A type of another crate is its external key, with no node, so a
  target of the type stays `no-such-target` and does not turn into
  `not-found`. A path, a call or a declared type through the alias then
  reads the type.
  An alias of a type that the file does not resolve, as a primitive type,
  gives the record `unknown-receiver` for a name after it.
- A trait object or a generic bound `T: Trait` of a unit gets the `cha`
  edges to the implementations in the graph, through their external keys.
  With no implementation, the site is a record with the reason
  `trait-receiver`.

The standard library is a unit too. Pass `GOROOT/src` as the unit
`go:std@<version>`, as `-deps $(go env GOROOT)/src=go:std@go1.25.1`. It gives
each import path whose first element has no dot, as `net/http`. The index
step leaves out the `cmd` module of it, which no program imports.

### Go fields, constants and variables

The Go index records each named field of a struct as a `field` under the
key of its type, as `html.go//Node//Data`, each constant as a `constant`,
and each package variable as a `variable`. A field and a constant hold no
code. A variable holds no code when the source shows that its value is no
function: its declared type is a predeclared type or a type literal that
is no function type, as `int` or `*Client`, or its value is a literal, a
composite literal, an arithmetic expression, a call of a builtin, or the
error of `errors.New` or `fmt.Errorf`. The index marks such a variable
with `data`. A variable whose value is a name, as `run` in
`var Run = run`, holds that value.

Such a definition of data has no node. A read of it, as `n.Data` or
`pgx.ErrNoRows`, gives no edge and no record, in the project and in the
walk of an external key into a unit. A name on it, as `bufPool.Get()`,
is a record with the reason `unknown-receiver`, because the walk does not
know its type.

A call of any other variable keeps its edge or its record. A variable that
holds a name gives the edge to that name, as a call of `Run` gives the
edge to `run`. A variable whose value can be a function that the index does
not know, as `var Handler = func() {...}` or `var h = makeHandler()`, gives
a record with the reason `unknown-receiver`. So a target that only such a
function reaches is `uncertain`, never `not-found`.

A struct gets the methods of the types that it embeds. A method that an
embedded interface declares, as `Close` of `ssh.Client` that embeds the
interface `Conn`, runs on the value that the field holds, as a call on a
value of the interface does. The walk of an external key into a unit that
ends at such a method, or at a method of an interface of the unit, is a
record with the reason `interface-receiver`. A type alias, as
`type Context = context.Context`, names its target as a base, so a method
of the alias is a method of the target type. An import of a package whose
path has one element, as `context`, binds its name, so the base resolves
to the external key `context//Context`, and a unit of the standard library
gives it on.

A `pypi:` unit is a distribution, as a site-packages directory or an
unpacked wheel holds it. These rules apply to it.

- The root of the unit is the directory that holds its code, so a path
  inside the unit is the path that Python imports, as `yaml/__init__.py`.
  A unit whose root is its package directory, with an `__init__.py` at the
  root, is that package.
- The link step reads the `.dist-info` directory of the package at the
  root, when there is one, and matches it by the normalized name. The
  `top_level.txt` or the `RECORD` gives the import names, so `PyYAML`
  gives `yaml` and a distribution with several top-level packages gives
  each one. Without it, the import name is the package name in lower
  case, with `_` for `-` and `.`.
- The `RECORD` lists the extension modules, as `yaml/_yaml.so`. Such a
  module has no Python source, so an import of it is a record with the
  reason `no-dependency-source`, and the record concerns every target
  where a search reaches the import.
- A definition of the unit carries the external key that an importer of
  its module writes, as `requests//api//get` for `requests/api.py//get`,
  so a target of a module of the package matches the definition also when
  only the code of the package calls it.
- The index step leaves out the test files of the unit: a file under a
  `tests` directory, a `test_*.py` or `*_test.py` file, a `conftest.py`,
  and the files of the `.dist-info`, `.egg-info` and `.data` directories
  of the wheel. A `test` or a `testing` directory stays, because
  `django.test` and `numpy.testing` are modules that programs import.

A type hint is the only declared type that Python has. The link step
trusts it as it trusts a TypeScript type, because both are erased at run
time and a wrong hint gives a wrong edge, not a crash. These rules apply.

- The index records the class name of a hint of a parameter, as `Parser`
  in `run(p: Parser)` or `run(p: Parser = None)`. `Optional[Parser]`,
  `Union[Parser, None]` and `Parser | None` give `Parser`, and so does the
  forward reference `"Parser"`. A builtin, as `int` or `str`, a container,
  as `list[Parser]`, and a union of two classes give no type.
- A return hint gives the result type of the function. A tuple hint, as
  `tuple[Parser, int]`, gives the type of each position, so `p, n =
  pair()` binds `Parser` to `p`.
- A local with a hint, as `t: Thing = make()`, has that type. A name that
  holds the result of a call, as `t = make()` or `a, b = make()`, has the
  result type of the call at its position, as a Go short variable
  declaration does. A name of the top level of a module that is no
  variable of its own, as `q` in `q, n = pair()`, has it too. A call that
  reads its value by reflection, as `importlib.import_module(name)` or
  `getattr(o, name)`, gives no type.
- A class has the fields that its body declares with a hint, as the field
  of a dataclass, and the attributes of the instance that its methods set,
  as `self.parser = Parser()` or `self.parser: Parser = p`. The first
  setting of a name wins.
- A call on a typed name is a `member` call of the method of its class,
  with no record. A hint that names a class of a dependency gives the
  external key of the method, as `dep//Thing//do`, and the class hierarchy
  analysis adds the `cha` edges to the overrides in the project and in the
  unit of the dependency. A method that `@abstractmethod` marks, with no
  override in the graph, is a record with the reason `interface-receiver`,
  as a Go interface with no implementation.
- A call on the result of a function with no return hint keeps the rules
  above the table: a record with the reason `unknown-receiver` and the
  `name` edges of the method name. A function of a dependency that no unit
  gives is a record with the reason `no-dependency-source`.

`examples/projectgraph` takes `-site-packages <dir>` and adds each
distribution of the directory as a unit, by its `.dist-info`.

The standard library of Python is a unit too. Pass the `Lib` directory of
CPython as the unit `python:std@<version>`, as
`-deps /usr/lib/python3.12=python:std@3.12.3`. A virtual environment names
the installation in the `home` line of its `pyvenv.cfg`. The unit gives
each module of the standard library that has Python source, as
`threading` or `http.server`, so a call of the library on `self`, as
`Thread.run` in `Thread.start`, reaches the override of the project with a
`cha` edge, and a path into the library goes on to its calls. A module
that only C gives, as `_thread`, stays an external key. The index step
leaves out the test suite under `test`, the IDLE, the turtle demos and
`lib2to3`, which no program imports.

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

A target also matches each function, method and constructor of a unit of
dependency code whose `Node.External` is the key, as the unexported
`go:golang.org/x/net@v0.61.0//html/render.go//render1` for
`golang.org//x//net//html//render1`. A call inside a unit goes to the
definition and not to the external key, so a target that only the code of
its unit calls is found through the definition. The call sites of a
target leave out the package edges, which join a key to its definition.
A definition of an internal package of a Go unit has an external key too,
as `google.golang.org//grpc//internal//transport//NewServerTransport`. Only
the module can import such a package, so no importer writes the key, but
an advisory names the symbol by it. An internal package gives no symbol
to a summary.

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
| `no-such-target` | The key names no external node and no definition of a unit of the graph, and no uncertainty could name it |

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
| `budget` | A missing, failed or truncated file that the program runs, a limit of the link step, or a search that the depth cap or the deadline stopped |
| `root-fallback` | No root rule matched, so every exported symbol is a root |

An uncertainty concerns every target, unless its site says less. A call
with an unknown receiver and a method name concerns the targets with that
last segment, and the targets that any definition of that name reaches. A
computed import with a static prefix concerns the targets under the
prefix. A call into a package at no known definition concerns the targets
that the code of the package reaches. Code of the Go standard library that
no unit gives concerns the targets that a Go method of an exported name
reaches, as [Code outside the graph](#code-outside-the-graph) argues. A
result caps the uncertainties that it lists at three per kind and keeps
the full count in `Searched`.

A file of dependency code that the index step did not complete counts
only for a search that enters its package. A file of the project counts
for every search, because it can hold a root.

Two kinds of incomplete file count for no search, because no search can
reach a caller in them. `Graph.Runs` decides the first.

- A file of a language that the program does not run. The languages that
  the program runs are the languages of the files that hold a root other
  than a test root, in `Graph.Stats.Languages`, and for a unit of dependency code the
  languages that its ecosystem installs, as Python for `pypi`. A Python
  project ships the JavaScript of its web pages, and the index step
  truncates a minified file of it. No Python program runs that file. A
  file of an unknown language counts, and so does every file of a graph
  with no root.
- A test file, which holds only test roots. It counts for a search with
  `Query.IncludeTests`.

An import that no file of the project or of the unit gives is an external
node, unless the module can still be first party: a relative module, or a
module whose first segment a search path root or a build step can give.
Such an import is a record with the reason `no-definition`, and it
concerns every target, because the module can hold any code. In Python
the first segment must be a module or a package under an import root or
in the directory of the importer, as the import system reads it. A file
of the name deep in a package, as a `types.py`, is no module of
`import types`, so that import names the standard library and is external.

A decorator counts as a dynamic feature, with the flag `decorator-wrap`,
only when the graph cannot name its code, as `@handlers[name]`. A
decorator that is a name, a dotted name or a call of one, as
`@app.route("/")` or `@click.option("-v")`, is a call of the scope that
defines the function, and the link step reads it. A call of the value of
a call, as `click.option("-v")(f)` in a decorator factory, is no record
of its own: the graph reaches the code that the inner call gives through
the inner call. A reflective inner call, as `getattr(o, n)()`, keeps its
record.

A summary keeps every uncertainty of a symbol in a table, with the targets
that its scope concerns, so `Status` applies the same rule with no graph.

### A call of a function value

A Go call of a function value, as `BackoffLinearWithJitter(d, j)(attempt)`
or `route.Handlers[0](c)`, is a record with the reason `function-value`.
The record concerns no target. This is sound because the search reaches
each function that such a call can run. A Go function value is one of
these:

- A function or a method that code uses as a value: an argument, the
  right side of an assignment or a declaration, a return value, an element
  of a composite literal, or the value of a send on a channel. The index
  records a reference there, and the link step adds a `reference` edge from
  the code that holds the value. The value exists only after that code
  runs, so the search reaches that code and then the function.
- A method value whose method the link step cannot name, as `h.Handle` on a
  value of an interface. The link step records the reference with the
  method name. That record concerns the targets that a method of the name
  reaches, as a call `h.Handle()` does.
- A function literal. The index gives the calls of the literal to the
  definition or the module top level that holds it. So a function that
  returns a literal reaches the calls of the literal.
- A package-level value, as a map of handlers. Its references start at the
  module top level of its file. The program loads each package that it
  imports before any code of the package runs. So a file that imports a
  package that runs code at load runs code at load too, also when the
  package is in a unit of dependency code. An import edge joins the
  importer to the module top level, and a search from `main` reaches it.
  An external key also has an import edge to each module top level of its
  package that runs code at load, so the search of a summary reaches it.
- A value that reflection gives, as `reflect.Value.Call`,
  `reflect.Value.MethodByName` or `reflect.Value.Method`. No reference
  leads to such a function. A call or a reference of the methods `Call`,
  `CallSlice`, `Method` or `MethodByName` in a file that imports `reflect`
  gets a `computed-name` record, which concerns every target. The link step
  does not always know that the receiver is a `reflect.Value`, so the rule
  takes any receiver.

A function value that code outside the graph makes runs code outside the
graph, which the search does not see for any call.

The rule does not apply to Rust. The Rust index records a function that an
argument, a `let`, an assignment, a `return`, a field initializer or an
array names. It does not record a function that the tail expression of a
block, a match arm, a shorthand field or a tuple holds, and a Rust function
returns its tail expression. So a call of a Rust function value stays a
`computed-name` record.

### Unknown code of a unit of dependency code

A failed or truncated file of a unit of dependency code counts for every
target when the search enters the unit. An unresolved site in the code of a
unit with no method name, as an import that no file of the unit gives,
counts for every target too. The closure of the edges of the unit, which
`unitReaches` reads, does not bound such code, for two reasons.

- The unknown code can call a package that no known code of the unit
  calls. A failed file has no index, so its imports are not known. The
  imports of the other files of the unit do not bound them, and neither does
  the closure of the imports of the packages of the unit, because the
  import closure reads the same files. Only the manifest of the unit, as the
  `require` list of a `go.mod`, bounds them. The link step does not record
  it.
- The project can give the unit a value of an interface, as a handler. The
  unknown code can call a method of that value, and the method can call any
  code of the program. A function value has a reference edge from the code
  of the project that holds it, so the search reaches the function without
  the unit. A value of an interface has no such edge. Known code of the unit
  records such a call with its method name, and the record concerns every
  target of the name. The unknown code can hold such a call for any name.

So a scope narrower than every target can make a target `not-found` that
the program reaches. The case `go/dependency-failed-callback` shows it:
`os/exec.LookPath` stays uncertain only through the budget of the failed
file.

### Code outside the graph

A Go call or reference of an external key whose package no unit gives
runs code that the graph does not hold. That code can call back into code
that the graph holds, and no edge shows it. As an example, `ReadMessage` of
`gorilla/websocket` passes its reader to `io.ReadAll`, and `io.ReadAll`
calls `Read` of the reader. With no unit of the standard library, the
search stops at the key `io//ReadAll`, and `messageReader.Read` reads as
`not-found`. So the link step gives each such key a record with the reason
`no-dependency-source`, and its caller is the key. The scope of the record
depends on what the code can call.

- The Go standard library imports no package outside it. So its code can
  run code of another module only through a value that it holds: a function
  value, or a method of a value. A function value has a reference edge from
  the code that makes it, so the search reaches the function without the
  standard library. See [A call of a function value](#a-call-of-a-function-value).
  A method of a value runs through an interface or through reflection. A
  type outside the standard library cannot declare a method of an
  unexported name of an interface of the standard library. It can only
  embed a type of the standard library that has the method, and that method
  is code of the standard library. Reflection gives only the methods of an
  exported name. So the record of a key of the standard library is a
  callback, as `Callback` marks it. It concerns the targets that a Go
  method of an exported name reaches, in the project and in each unit, as
  the closure of a method name does. Each key of the standard library gives
  the same scope, so a result lists the callback once, with no key.
- A package of another module can import the package of any target and
  call any function of it. So its record concerns every target. A summary
  links one unit alone, and the reader of a summary joins the summary of
  each module that the unit calls. So the link step gives this record only
  in a build with a project file.
- C code that cgo calls, as `C//f`, can call each Go function that an
  `//export` comment names. So its record concerns every target.
- A builtin runs no code of the program, except `panic`. The runtime prints
  the value of a panic that no `recover` stops with its method `Error` or
  `String`. So `panic` has a record for each of the two methods.

A target that no Go method of an exported name reaches keeps its answer.
In the case `go/stdlib-callback-scope`, only the function `Dial` calls
`handshake`, so `handshake` stays `not-found`. A search that reaches no
such key keeps every answer. With the unit `go:std@<version>`, the standard
library is code of the graph and gives no callback. A unit of each module
removes the records of the other modules.

Two gaps stay.

- A target of a package that no unit gives, which the project names only
  in a value, as the composite literal `legacy.Config{}`, has no node. The
  standard library can call a method of that value, and the method can call
  the target. The graph holds none of that code, so the target reads as
  `no-such-target`.
- The rule applies to Go only. Code of another language that no unit gives,
  as the standard library of Python with no `python:std` unit, can call a
  method of a value too, and no record shows it.

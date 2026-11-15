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
generated file, as a `.pb.go` file, stay in the graph. A var value that
names a function stays too, as a map of handlers, because a call of a
function value finds the function only through that reference. The index keeps the
hash of the file. If the skeleton still hits a limit, the file stays
truncated, and reach counts it as budget.

The JavaScript language reads `.jsx` files with its grammar, which parses
JSX. It also reads the component files of Vue, Svelte and Astro (`.vue`,
`.svelte`, `.astro`). The parse reads only the `<script>` blocks of such a
file, and the Astro code between the two `---` lines at the top. Each byte
outside them becomes a space, so each import and call keeps its line and
column in the file. A file whose blocks have `lang="ts"` or
`lang="typescript"` is TypeScript, and so is every Astro file. The index
of the file holds the hash of the bytes of the file. The indexer picks the
language and the code of a file once, and the parse reads them.

A `<script>` element with a `src` attribute holds no code. The index
records it as an import of the file that it loads, as
`import './logic.js'` for `<script src="./logic.js">`, so the search
reaches the code of that file.

The parse does not read the rest of a component file, the markup: the
template of a Vue file, the markup of a Svelte file and the body of an
Astro file. The markup can call each name that the script of the file
imports or defines, as `danger` in `@click="danger(item)"`, and a function
that only the markup names, as `onClick` in `@click="onClick"`, has no
caller in the graph. The markup can also call a name that the script does
not bind: a component or a property that the app registers for all
components, and a name that the framework imports with no import
statement, as Nuxt does. Vue calls the functions of the options object of
a component, as `mounted` and the functions of `methods`, and no code of
the project calls them, also in a file with no template.

So the index gives each component file one record with the reason
`markup`, at the first byte outside the code that is no space. The record
has no method name. A narrower scope, as the targets that the names of the
script reach, would miss a name that the app registers elsewhere, and a
target that only such a name reaches would read `not-found`. The markup
runs only code of the JavaScript family, so the reach query scopes the
record by its language with `Uncertainty.Language`. It concerns a target
under a module that a file of the family imports or gives on from outside
the project, as `jsonschema` for `import { validate } from 'jsonschema'`
or `plug` for `export * from 'plug'`, which `Graph.Imports` holds by
family. The case `javascript/component-reexport-star` shows it. The key
forms of npm and of Go coincide, so an import of `os` in JavaScript also
concerns a Go target under `os`. It also concerns a target that a definition of the
family is, or that an edge from a file of the family leads to, and a
target with no node. A JavaScript project of
component files thus reads `uncertain` for each JavaScript target that it
does not find. A target that only the Python code of a project calls does
not, as in the case `python/component-in-frontend`. A span and a summary
still count the record for every target.

A JSX element renders the component that its name holds, so the name is a
reference, as a function value is. `<Child />` gives a `reference` edge
to `Child`, and `<table.Row />` to `table.Row`. A lower case name, as
`div`, is a tag of the host, which JSX gives as a string, so it gives no
edge. A function in a JSX expression, as `handle` in
`<button onClick={handle}>`, is a reference too. This holds in `.jsx`,
`.tsx` and `.js` files.

A spread attribute, as `<button {...handlers}>`, makes each property of
the object a prop of the element, and React can call each function of it.
When the object is a definition of the file, as `const handlers = {
onClick() {} }`, the element gets a reference to each function that the
definition holds, as `handlers.onClick`. The index cannot name the
functions of any other object, as a parameter, the result of a call or an
import. A function of such an object can have no other reference, so the
spread gets a `computed-name` record, which concerns every target. The
cases `javascript/jsx-spread-attributes` and
`javascript/jsx-spread-unknown-object` show each answer.

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
`trait-receiver`, `no-dependency-source`, `function-value` or `markup`. The site keeps the method name of the call, so the
reach query can tell which targets it concerns. A `function-value`
record concerns no target in a search from the roots that the rules
mark, and every target in a search with a source and in a summary.
[A call of a function value](#a-call-of-a-function-value) gives the
reason.

A site with more than 20 candidates of its method name, as `render` in a
Django project, is `ambiguous`. The site gets no edge, so the search cannot
follow what the candidates call. A record with a method name concerns the
targets with that last segment and the targets that a definition of the
name reaches, so it concerns what each candidate reaches. A candidate can
also run code that no edge leads to, through an unresolved site of its
own. So the record also concerns what each uncertainty of the code that a
candidate reaches concerns, as [The honesty rule](#the-honesty-rule) says.

An ambiguous site keeps its method name only where a call of the name can
run nothing but a method of the name or a function value that a
`reference` edge leads to. The profile of the language says where.

- Go. A call runs a method of the name or a function value, as a field of
  a function type. A function value has a `reference` edge from the code
  that stores it, as [A call of a function value](#a-call-of-a-function-value)
  argues. So the site keeps its name, and the search from the roots
  reaches the stored function through that edge. A search with a source
  and a summary may not reach the code that stores the value, so there the
  site concerns every target. The case `go/name-ambiguous` shows each
  answer.
- Rust. A call on a receiver, as `h.handle(s)`, runs only a method of the
  name, because a field that holds a function needs the form `(h.f)(s)`.
  So the site keeps its name, as in the case `rust/name-ambiguous-method`.
  A call with no receiver, as `handle(s)` of a parameter, can run any
  function value, and the Rust index does not record each one. So that
  site concerns every target, as in the case `rust/name-ambiguous-param`.
  The reach query does not read the language of a site, so a search with a
  source and a summary count a Rust site for every target too.
- Python, JavaScript, TypeScript, Ruby and PHP. A call of a name can run
  code that no definition of the name holds: an object with `__call__` in a
  class attribute, a value that `__getattr__` gives, a function that a
  module binds to the name by an import, or code of a module that no unit
  gives. So an ambiguous site keeps no method name and concerns every
  target. The cases `python/name-ambiguous-callable`,
  `python/name-ambiguous-getattr`, `python/name-ambiguous-module` and
  `python/name-ambiguous-nounit` show each form.

Python runs some methods of an instance with no call of their name. A
call of the instance runs `__call__`. A read of an attribute runs
`__getattribute__`, `__getattr__` for a name that the class does not
define, and `__get__` of a descriptor in a class attribute. The code that
calls or reads the instance often has an unknown receiver, or is code
outside the graph. So a `reference` edge with the rule `implicit` joins
each edge that can make an instance of a Python class to the method of
each of these names that the instance runs, from the class or from a base
in its MRO. A call of the class makes an instance. So does code that gets
the class as a value, as `functools.partial(C)`, `map(C, xs)` or
`field(default_factory=C)`, and `raise C`. So does a call of the first
parameter of a classmethod, as `cls()` or `klass()`, also from a function
that the method defines, which holds the class of the method or a
subclass. The index records that parameter as `Definition.ClassParam`. A
call of `self.__class__` or `type(self)` makes an instance of the class of
the method or of a subclass too. The edge can reach the class
through an export hop or the `package` edge of a unit. The code that
makes the instance holds it, so a search from the roots reaches the
method, as it reaches a stored function value. A class that code only
tests a value against or reads, as in `isinstance(x, Proxy)`,
`issubclass`, `typing.cast(Proxy, x)`, `hasattr(Proxy, "x")` or
`getattr`, also through a conditional, makes no instance.
The index marks such a reference with `Check`, and it gets no edge. A
call of a class also runs `__call__` of its metaclass, as `Meta` in
`class Made(metaclass=Meta)`, which `Class.Metaclass` records. A call with
an unknown receiver keeps its record with the method name. The target is
then found. The edge proves only that an instance can exist, and not that
code calls it. The cases `python/unknown-receiver-callable`,
`python/unknown-receiver-getattr`, `python/instance-methods-no-instance`
and `python/instance-made-by-reference`
show it. A function that code binds to a module attribute, as
`hooks.handler = wipe`, has a `reference` edge from that code, and the
assignment has the flag `monkey-patch`, so it needs no rule of its own.

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

The index records the typed parameters and locals of a Go function with no
extent, and the link step reads the type of the first one of a name. An
inner block can bind the name again, as `x` in
`x := local{}; { x := remote{}; x.run() }`, and a function literal can
take a parameter of the name. So the index checks each call and each
method value on a typed name. The binding of the name in scope there must
be the binding whose type the link step reads, or a binding of the same
type. A binding in scope with a type of its own, as `x := remote{}` in the
block, gives the call the target of a typed local under an alias, as
`x#120`, so the link step reads that type. Otherwise the site is a record
with the reason `unknown-receiver` and the method name, and the method of
the real type gets a `name` edge. The index checks the first name of the
receiver too, because the call graph can keep a raw target, as `x//run` for
a parameter `x` of a function literal. A receiver that binds no local at
the call names the import, as `url` in `url, err := url.Parse(raw)`, whose
scope starts after the declaration, and the call keeps its target. A local
that shadows an import, as `log` in `for _, log := range ls { log.printf()
}`, gets the target of the local in place of the target of the import. A
short variable declaration that assigns a name of its own block, as `err`
in `n, err := f()` after `m, err := g()`, binds no new name.

An assignment can type a variable that the function binds once, as `r` in
`r := o.r; if r == nil { r = &remote{} }`. The link step reads that type
only when it is the type of the variable. A declared type must be that
type, and with no declared type each write must give it, as the field `r`
of the struct `opts` of the file does for a parameter `o opts`, and not for
a local that shadows the parameter. A write of an element of a map or a
slice gives the element type that the source declares for it, as
`call, dup := p.calls[key]` with a field `calls map[string]*remote`, or a
parameter, a local or a variable of the file of a map or slice type. A
write that constructs or asserts a type, as `&remote{}`, `new(remote)` or
`x.(*remote)`, gives that type. A variable that can hold an
interface, as `r` of a field `r runner`, a range variable or `l := ls[0]`,
can hold another type, so its call is a record. The case
`go/typed-local-shadow` shows each form.

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
- A `use` path names a module or an item, so the link step reads all of
  the path of `use dep::top` as a module. The unit gives the function `top`
  for that path, and a call of `top()` gets the package edge to it, as a
  call of `dep::top()` does.
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
  the target `String//shout` of `s.shout()`.
- A declared type holds at a call only when the function binds the name
  once, as a parameter or a typed `let`, before the call. The index checks
  this for a receiver that the call graph does not type and for a target
  that goes through a declared type, as `run//t//inspect` for `t.inspect()`
  in `fn run(t: Local)`. The call graph reads a parameter and a `let` of
  one name, and no other binder. A `match` arm, an `if let`, a `for`, a
  closure, another pattern, a `static`, a `const` or a macro can bind the
  name again with another type. The hygiene of Rust keeps a `let` of a
  macro body apart from the names of the caller, so a macro binds only a
  name that its call passes as a token. A macro of std binds no such name,
  except `thread_local!`, which makes a `static`. A `macro_rules!` or a
  `use` of the file can give the name of a macro of std to a macro that
  binds a name, as in the case `rust/macro-std-name`. So a call on a name that such a binder binds
  before the call is a record with the reason `unknown-receiver`, and so is
  a call whose declared type does not hold. The call graph binds a name to
  the value of its first `let`, so a call after a second `let` of the name
  with another type is a record too, as in the case
  `rust/shadowed-let-type`. A `let` counts once its statement ends, and
  `let`s of one type keep the target, as `c.send()` after
  `let c = c.with_retry();` in the case `rust/let-same-type`. Two `let`s
  that call one function of one type hold one type, also when the type is
  in another file, as in the case `rust/let-same-constructor`. A method of the
  real type then reads as `uncertain` and not as `not-found`.
- The link step resolves a type that a parameter, a `let`, a field or the
  result of a function declares at the root of the file, or as a struct,
  an enum, a union or a trait of an inline module that holds it. A name of
  a call path in an inline module also names such a type of the module
  first. So the index keeps a declared type only when no scope that holds
  it declares the first name of the type in another way. Such a scope is a
  block or an inline module with an item, a `use` or an `extern crate` of
  the name, or a function, an `impl`, a trait or a type with a type
  parameter of the name. A glob `use` can declare any name, except
  `use super::*` in a module of the root, which reads the names of the
  root. `self` and `super` in an inline module name other modules. So in
  `fn run() { type Local = dep::Thing<u8>; let t: Local = dep::new_thing(); t.inspect(); }`
  with a `struct Local` at the root, `t.inspect()` takes no method of the
  root `Local`. The call keeps the target that the call graph gives, or a
  record with the reason `unknown-receiver`. A type parameter with a bound,
  as `S` in `fn run<S: Store>(s: S)`, names its bound. A `use` that
  declares the name gives the path of the `use` in place of the type, as
  `crate::store::Item` for `Item` after `use crate::store::Item;` in an
  inline module, as in the case `rust/use-type-result`. The case
  `rust/inline-module-types` shows the types of an inline module, and the
  case `rust/generic-field-type` a type parameter of a struct and of an
  `impl`.
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
  `use smallvec::smallvec;` names the crate `smallvec`. A binding can
  name a module, so `use util::helpers;` after `use crate::deep;` and
  `use deep::util;` names the module `deep::util::helpers`, as in the case
  `rust/use-chain-module`. A glob `use` brings such a binding of its
  module too, as in the case `rust/glob-use-binding`.
- A `use` in an inline module reads `self` and `super` from that module.
  The index keeps the uses of a file in one scope, so it writes such a
  path from the module of the file, as `self::client::tls::conn` for
  `use self::tls::conn;` in `mod client`, as in the cases
  `rust/use-self-inline-module` and `rust/use-super-inline-module`.
- The call graph finds the first name of a path in the scopes that hold
  the call, and reads no `use` and no type parameter. A `use` in a block
  or in an inline module can bind the name again, and the index then gives
  the call the path of that `use`, as in the case `rust/use-shadows-module`.
  A method call whose target the call graph finds as a method of a type of
  an outer scope gets the path of a `use` of the type name the same way,
  as `x.save()` after `let x = Item::new();` in the case
  `rust/use-type-receiver`. A glob `use` between the call and that scope
  can bring the name too, unless it reads `super`, `self` or a crate of
  the toolchain, so the call is a record, as in the case
  `rust/glob-shadows-outer-type`.
  A type parameter of the name names a type that the caller gives, so the
  call is a record with the reason `unknown-receiver`, as in the case
  `rust/generic-shadows-type`.
- A glob `use` brings each name of its module into the module of the
  file. So `helpers::cleanup` after `use super::*;` in
  `src/deep/worker.rs` names the function of the module `deep::helpers`,
  as in the cases `rust/glob-use-module` and `rust/glob-use-crate-module`.
  Rust reads a name of a glob `use` after a child module and a `use` of
  the name, and before a crate, and so does the link step. A first name
  that a glob `use` brings as a module never reads as another crate. The
  path resolves in the project, or the `use` of the path keeps a record
  with the reason `no-definition`. So a target under it reads `found` or
  `uncertain`, and not `not-found`. A glob `use` in an inline module, as
  `use super::*;` in `mod tests`, names its modules from the inline
  module, so the link step does not read it for the module of the file.
  Read from the module of the file, `super` names a sibling module, and a
  path of a crate with the name of that sibling would lose its edge to the
  crate, as the case `rust/glob-use-inline-module` shows. The path of a
  glob `use` reads no other glob `use`.
- A glob `use` of a module of another crate, as `use dep::html::*;`,
  brings a name when a unit gives the crate and the name in that module.
  So `render()` names `dep//html//render`, as in the case `rust/glob-use`,
  and `html::render()` after `use dep::*;` names it too, and not a crate
  `html`, as in the case `rust/glob-use-dependency-module`. A glob `use`
  of a crate that no unit gives can bring any name, so the path resolves
  as if the glob `use` were not there. The unit can bring the name from a
  third crate by a `use` or a glob `use`, or a child module by a glob
  `use` of its own crate, as `text` after `pub use inner::*;` and `leaf`
  after `pub use crate::sub::*;` in the case `rust/glob-use-reexport`. Two
  crates can bring each other's names, so a cycle guard cuts the read, and
  the link step keeps no provision of a unit that a cut read gives, as the
  case `rust/glob-use-crate-cycle` shows. A local closure or a parameter can
  bind a single name, so such a call keeps its record too. A glob `use`
  brings only what another crate sees: a `pub` item, a child module, or the
  binding of a `pub use`, as in the case `rust/glob-use-private-item`. An
  item that is `pub(crate)`, `pub(super)` or `pub(in ..)`, and a `use` that
  is not `pub`, stay out, as in the case `rust/glob-use-crate-visibility`.
  The index marks an item as exported only when it is `pub`, and marks each
  `pub use`.
  A glob `use` of the root of the file brings no name into an inline module
  with uses of its own, as in the case `rust/glob-use-inline-scope`.
- `#[macro_export]` puts a macro at the root of its crate, from any module
  of the crate. So `dep::wipe` names the macro `wipe` of a private module
  of the unit, as in the case `rust/macro-export-module`. A call of a
  single name that no item binds, as `render_now!()` after
  `#[macro_use] extern crate dep;`, gets the edge to such a macro of a
  dependency of the crate, as in the case `rust/macro-import`. The index
  does not tell a call of a macro from a call of a function value, so the
  call keeps its record. A `macro_rules!` of the name in the crate of the
  file, as one of a module with `#[macro_use]`, shadows such a macro, so
  the call keeps its record alone, as in the case
  `rust/macro-local-shadows-dependency`. A function or a method of the name
  does not, as in the case `rust/macro-dependency-function-name`. The index
  marks a `macro_rules!` with the decorator `macro_rules!`. A macro reads `$crate::sink` as
  `crate::sink` of the crate of the macro. A call of a metavariable, as
  `$f()`, is a record with the reason `computed-name`, as in the case
  `rust/macro-metavariable-call`.
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
  Only a `cfg` attribute lets a scope declare a name more than once, as a
  struct under one `cfg` and an alias under the other, or two aliases of
  other types. The index does not know which `cfg` the build sets, so such
  an alias names no type. A call through the name, as `Client::new` or
  `c.send()` on a parameter of the type `Client`, then gets the record
  `unknown-receiver` and a `name` edge to the method of each declaration.
  It does not take the method of one declaration only, which would turn a
  target of the other declaration into `not-found`.
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
know its type. A call of it, as `s.cfg.Remove("x")` for a field of a
function type, runs the function value that it holds. No edge names that
function, so the call is a record with the reason `function-value`, as
[A call of a function value](#a-call-of-a-function-value) says. A search
from a source that does not reach the code that stores the value keeps the
target `uncertain`. The case `go/source-field-func` shows it.

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

### Rust glob lookups

Rust reads a name of a glob `use` before a crate, and rejects a name that
two glob `use`s bring. A glob `use` of an inline module stays out of the
module of the file. Modules and crates can bring each other with glob
`use`s, in a diamond or in a cycle. A unit can bring a name by a glob
`use` of a third crate, so the read of the unit can ask for the same path
again. A walk of each path through such `use`s grows with the number of
paths.

So the link step reads each glob lookup once: the glob path of a file and
a first name, or the glob member of a module and a name. The outermost
lookup reads each lookup once, and a lookup that it reads again gives its
first answer. A read of a lookup in progress is a cut. An answer stays for
the build when each cut under it hit the lookup or a lookup under it, as
the low link of Tarjan tells, and when no other guard cut it. Such a cut
leaves out no path, because the lookup in progress reads that path itself.
`providers` keeps no answer that a cut gave.

A visited answer is part of the lookup that read it. When that lookup
stays for the build, a visited answer under it that found its item stays
too. The kept answers give each later read the same way through a cycle
of crates. Any other visited answer under it goes. A later read with
another depth would take its low for its own, and the answer can lack an
item that the lookup found after it. When the lookup does not stay, the
low of each visited answer under it falls to the low of the lookup,
because they lack what it lacks.

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
definition is a root, and `Graph.Stats.RootFallback` says so. The link
step decides it per language family. A family with no such root of its
own gets the library-surface roots, whatever another family has, as a
Python Lambda handler next to a JavaScript package with a `main`. The
case `python/handler-beside-js-package` shows it. Each result
of a reach query then carries a `root-fallback` uncertainty. A root of a
rule with `keeps_fallback` does not count here. Such a rule names files
that a framework loads, but not the entry that loads them.

A framework loads a component file of Vue, Svelte or Astro in ways that no
static import shows: a route table with `() => import('./views/X.vue')`,
an alias of the bundler as `@/`, `import.meta.glob`, or a file-based
router, as the `pages` of Nuxt. So the rule `node.component` marks each
component file as a root, with no `requires`. The rule has
`skips_tests`, so it skips a component under a test path, which runs only
under a test, and it matches the name of a file, not of a directory, as
the case `javascript/component-test-path` shows. A route rule does not
skip a test path, because a route can have one, as `app/api/test`. The
cases `typescript/next-route-test-dir` and `javascript/sveltekit-test-dir`
show it. A component file holds a `markup` record, so this root makes each
JavaScript target that the search does not find read `uncertain`. A static import
of a component file does that too in most such projects. The rule keeps
the fallback on, because a Vite app starts at `index.html`, which no rule
reads. The rules `node.sveltekit.route` and `node.astro.page` mark the
route files of SvelteKit and the pages of Astro, as `+page.ts`,
`+server.ts` and `src/pages/api/data.ts`, with their exports. The Astro
rule also marks the middleware, the actions, the content config and
`astro.config.*`. They keep
the fallback on too, because a framework can load a file by a convention
that a rule does not list. Nuxt gets no rule of its own. It imports the
code of `composables` and `utils` with no import statement, so no list of
paths is complete. The cases `javascript/vue-route-dynamic-import`,
`javascript/vue-alias-import`, `javascript/vue-import-meta-glob`,
`javascript/nuxt-pages`, `javascript/sveltekit-routes`,
`javascript/astro-pages` and `javascript/vue-vite-fallback` show each form.

A path rule also marks each function under an exported object of its
files, as `default` and `save` of `export const actions = {...}` in a
`+page.server.ts`. The framework calls such a function by its name, and no
edge leads from the object to it. A destructured export, as
`export const { GET } = handlers`, gives an export record whose target is
the member that the name takes, as `handlers//GET`, also through a nested
pattern and a rename. A name of an array pattern or a rest element takes
the object that holds it. The case `javascript/sveltekit-destructured-export`
shows each form.
A default of a pattern, as `fb` in `{ PUT = fb } = h`, is a second export
record of the name, and a path rule marks each record that resolves. An
export of a name that holds another name, as `export const GET = handler`,
gives that name first. An exported name whose value is no literal, as the
result of a call in `export const actions = makeActions()` or of
`export default NextAuth({...})`, is `Opaque` in the index. A value with no
call, `new`, `await` or read of a member, as `60 * 60` or
`'x' as const`, is a literal, and the index looks through `satisfies`,
`as`, `!` and parentheses. The framework calls the functions of an opaque
value, and no edge names them. So a path rule and the manifest rule give
the export a `computed-name` record in place of a root, and so does an
export whose name no record resolves, as GET of
`export const { GET } = handlers` for an imported `handlers`. The record
has the scope of the markup record. It concerns the targets of the
JavaScript family and of the modules that the file imports. The cases `typescript/next-route-literals`,
`typescript/next-route-opaque` and `javascript/library-opaque-export`
show it. The index records a function that an argument of a call
in an export statement names, as `first` in
`export const handle = sequence(first)`, as a reference. The case
`javascript/sveltekit-opaque-exports` shows each form.

The resolver maps the alias `$lib` of SvelteKit to `src/lib` of the nearest package that depends on `@sveltejs/kit`. It
does not read `svelte.config.js`, which can move the directory. So an
import of `$lib` that names no file is first party, and it is a record
with the reason `no-definition`. The cases `javascript/sveltekit-actions-lib`
and `javascript/sveltekit-lib-moved` show both.

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

A search from a source also enters the load code of each package whose
code it reaches, because the program runs that code before the source. The
caller of a source can also run other code before it that no root
reaches, and that code can store a function value. So in a search with a
source, in either mode, a call of a function value and an ambiguous call
concern every target. See
[A call of a function value](#a-call-of-a-function-value).

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
call of a Rust conversion runs a method of another name, as `into` runs the
`from` of an impl of `From`. So the record concerns the method names that
the profile of Rust gives for the name too. The reach query does not read
the language of a site, so a call of the name in another language concerns
them too. The cases `rust/into-ambiguous` and `rust/into-dependency` show
it. A
computed import with a static prefix concerns the targets under the
prefix. A call into a package at no known definition concerns the targets
that the code of the package reaches. Code of the Go standard library that
no unit gives concerns the targets that a Go method of an exported name
reaches, as [Code outside the graph](#code-outside-the-graph) argues. A
result caps the uncertainties that it lists at three per kind and keeps
the full count in `Searched`.

The code of such a scope can hold uncertainties of its own: an unresolved
site, a dynamic flag, a truncation, or a file of a unit that the index
step did not complete. That code can run code that no edge leads to. So
the scope also concerns what each of them concerns, as the search of a
query counts them, and the scope of a method name, a unit or a callback
that one of them names goes on into that code too. Without that, a target
that only such code reaches reads as `not-found`. The cases
`rust/name-ambiguous-method` for a method name,
`python/dependency-dark-unit-record` for a unit and
`go/stdlib-callback-record` for a callback show it. A summary keeps the
same scope: `reach.Scoped` holds the method names and the import prefixes
of those uncertainties, and `Every` marks one that concerns every target.

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
In a search from the roots that the rules mark, the record concerns no
target. This is sound because that search reaches each function that such
a call can run. A Go function value is one of these:

- A function or a method that code uses as a value: an argument, the
  operand of a conversion, the right side of an assignment or a
  declaration, a return value, an element of a composite literal, the
  value of a send on a channel, or the expression of a range clause, as
  `walk` in `for x := range walk`. A range over a function calls it, and the
  function calls the body of the loop. The index records a reference there, and
  the link step adds a `reference` edge from the code that holds the value.
  The value exists only after that code runs, so the search reaches that
  code and then the function. The value is a name, as a function of
  another file of the package, a member chain, a method expression, as
  `(*T).M`, a method value on a call, a composite literal, a type assertion,
  `new` or a dereference, as `f().M`, or a generic instantiation, as `F[int]`.
  Each of them can be in parentheses or behind `&`. The index keeps a
  predeclared name, as `clear`, as a name too. A declaration of another
  file of the package can shadow it, and the index of one file does not
  see that file. The link step resolves the name in the package, or drops
  it when no declaration binds it. The case `go/predeclared-shadow` shows
  it. A parameter or a local of a declared type holds a value of the
  type, so its name gives no reference. The index takes the name as that
  local only where the declaration is in scope. So a typed local of a
  closed block does not hide a function of the package with the same name.
  The case `go/typed-local-block` shows it.
- A method value whose method the index or the link step cannot name, as
  `ts[0].Close` or `h.Handle` on a value of an interface. The index or the
  link step records the reference with the method name. That record
  concerns the targets that a method of the name reaches, as a call
  `h.Handle()` does.
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
  package that runs code at load.
- A value that reflection gives, as `reflect.Value.Call`,
  `reflect.Value.MethodByName` or `reflect.Value.Method`. No reference
  leads to such a function. A call or a reference of the methods `Call`,
  `CallSlice`, `Method` or `MethodByName` in a file that imports `reflect`
  gets a `computed-name` record, which concerns every target. The link step
  does not always know that the receiver is a `reflect.Value`, so the rule
  takes any receiver. Each other form of `reflect` gives back a value that
  code stored with a reference, or a method that these methods give. So
  `Value.Interface().(func())` needs no record of its own. `MakeFunc` makes
  a function that calls the function of its argument, and the argument is
  a reference.
- A value that a plugin gives, as `sym.(func())()` after `plugin.Open`
  and `Plugin.Lookup`. The plugin is code of another build. `Open` runs
  its init, and `Lookup` gives its functions and variables. No reference
  of the graph names them, also when a unit gives the standard library. So
  a call or a reference of `Open` or `Lookup` in a file that imports
  `plugin` gets a `computed-name` record too, in both modes. The cases
  `go/plugin-lookup` and `go/plugin-lookup-std` show it.

No other package of the standard library gives a function that no
reference names. A callback that the standard library stores, as the
function of `syscall.NewCallback`, `sync.OnceFunc` or `runtime.SetFinalizer`,
is an argument, so a reference names it. `runtime.FuncForPC` gives no
function that code can call. A conversion with `unsafe` of a pointer to a
function value needs that value first, and a reference names it.

A function value that code outside the graph makes, as the result of
`sync.OnceFunc(g)`, runs code outside the graph. A call of that code makes
the value, and the call has the record that
[Code outside the graph](#code-outside-the-graph) gives. The search
reaches that record where it reaches the code that makes the value.

The argument holds only for a search that starts from the roots that the
rules mark. Those roots are the entries of the program: `main` with the
module top level of each file of its package, a test root, or, with the
root fallback, each module top level and each exported symbol. Code that
stores a function value runs only after an entry, so the search reaches
it. An import edge joins a module top level to each module top level of a
package that it imports and that runs code at load, so the search also
reaches the load code of each package that the program loads.

A search with a source does not have the argument, in either source mode.
The caller of a source can run code before the source that no root
reaches, and that code can store a function value: the init of a package
that only `main` imports and that fills a registry, or a `setup` that
`main` calls before `serve`. With `SourcesAndRoots` the source is an entry
that the rules missed, so its caller can be code that the rules missed
too. So in a search with a source, a `function-value` record concerns
every target, as a `computed-name` record does. So does an ambiguous site,
because a Go call of a name can run a function value in a field. The case
`go/source-function-value` holds both forms of the store, and
`go/name-ambiguous` holds a field.

A summary does not have the argument either. A summary is a search from
one symbol of a unit, and a search of any kind reads it: a search from the
roots or from a source, of a project that the summary does not see. The
value can come from that project, from another unit, or from a package of
the unit that the symbol does not import, as a registry that the init of
another package fills. So a summary counts a `function-value` record and
an ambiguous site for every target. `summarytest` checks a summary against
a reach query of the unit graph from the symbol as a root of kind `query`,
which has the same rule.

A query with a source, or with `IncludeTests`, adds a load step, and the
search of a test path adds it too. The program runs the load code of a
package before the source or the test, and no path from the source or the
test root leads to it. Each node that the search reaches enters the module
top level of each file of its load group, as `Graph.LoadGroup` gives it.
The load group is the package of the file in a language whose program
loads a whole package before any code of the package runs, as Go. In
another language it is the file alone, as a Python or a JavaScript module,
which runs its top level before any of its functions can run. A test file
is in the load group only of the code of a test. The span of a method
name, a unit or a callback takes the same step, so the code that a
candidate of a method name holds in its package counts too. A hop of the
step has the resolver `load`, and `reach.PathString` writes it as
`main.go (load)`.

The step keeps two answers sound. A call in the load code, as `var x =
dep.Init()`, runs before the source, so the search finds its target. A
value that the load code stores in a variable or an attribute is reached
through its reference, also where the call of the value is a record with a
name, as `o.hook()` in the case `python/source-module-load`. The step only
adds nodes to a search, so it turns no answer into `not-found`. It costs
precision. A source that calls nothing still reaches the load code of its
package, as `idle` does in the case `go/source-package-load`. A search from
the roots alone does not take the step, so its results do not change. The
summary of a unit does not take it either. A summary answers for a search
from one symbol of a unit, and the import edges of an external key give
that search the load code of its Go package. The cases
`go/source-package-load`, `python/source-module-load` and
`javascript/source-module-load` hold a value in the load code and call it
from a source.

A call of a parameter, a local or a range variable that holds a function,
as `f(x)`, has a record with the name `f`, which concerns what a
definition of the name reaches. A local of a declared function type, as
`g` in `var g F = h`, has no record of its own. The function value has
another name, so in Go the link step also gives each such site a
`function-value` record. It does the same for a call of a member whose
receiver it does not know, as `h.fn()`, when a Go field of the graph has the
name and can hold a function. The index marks a field of a type of data, as
`Name string`, with `Data`, and such a field does not count. In a search
from the roots, the `reference` edge of the stored function covers the
call, and the `function-value` record concerns no target. In a search with
a source and in a summary, it concerns every target.

A local that the function binds once, to a function literal, and never
assigns again, as `f` in `f := func() {...}; f()`, runs only the literal.
The index gives the calls of the literal to the function, so the call of
`f` is no call of the index and has no record. A range, an assignment in a
range or a receive, or an address of `f` can give it another function, so
its call keeps its records.

The index also gives a `function-value` record to code that calls a
function value with no call record of its own: a call of a field of an
anonymous struct, as `s.act()`, a call of a dereference of a value or a
receive, as `(*p)()` or `(<-ch)()`, and a call of a local whose declared
type is a function type of another package, as `h` in
`var h http.HandlerFunc`. A conversion, as `(*T)(p)`, dereferences a type
and calls nothing. The fields of an anonymous struct and of a struct type
that a function declares have no definition of their own type, so the index
gives each such field a definition under the key `struct` of its file, as
`anon.go//struct//exec`, and a call of such a field in another file is a
`function-value` record too. A
record of a method value that a reference reads, as `g := h.fn`, has no
receiver, so the link step does not take it for a call of a field. The
case `go/source-local-func` shows each form.

A range over a function calls it, as `seq(yield)` does. So a range over a
value that the source does not show to be data gets a `function-value`
record: a parameter of a function type of another file, as `seq` in
`func consume(seq Seq) { for range seq {} }`, a local that a field gives,
or an element of a slice of functions. A literal, a slice of a value, a
value of a type of data, an element of a map or a slice of a declared type
of data, and a local whose declaration gives such a value need no record.

The index gives the link step what it needs to decide the other ranges,
with the reason `range-value`, and the link step never keeps that reason.

- A member, as `n.Attr`. The record has no function-value record when a
  reference at the site names a function or data, as `t.All` or `n.Attr`
  of a typed `t` or `n`, or when each field of the name holds data and no
  method has the name. A method of the name makes it a record with the
  method name, as the range can call a method value, and a field of the
  name that can hold a function makes it a `function-value` record.
- A value of a named type, as `l List` with `type List []string` in
  another file. The index marks a named type whose values hold no function
  with `ValueData`.
- The result of a call, as `names()` or `strings.Split(s, ",")`. The index
  marks a function whose first result holds no function with `ValueData`.
  Code outside the graph has its own record, which covers a function that
  it makes.

The case `go/range-over-func` shows each form.

Two gaps stay. In each of them, a search with a source can read
`not-found`.

- A field of a type of a package that no unit gives can hold a function,
  and no definition names that field. So a call of it keeps only the
  record with the name.
- In Rust, a call of a parameter or a local keeps only the record with the
  name.

Some records cost precision only. A range over a value of a named type of a
package that no unit gives, as `http.Header`, or over an element of it,
keeps its record. So does a call of a local that a call into such a
package gives, as `cancel` of `context.WithTimeout`.

The rule does not apply to Rust. The Rust index records a function that an
argument, a `let`, an assignment, a `return`, a field initializer or an
array names. It does not record a function that the tail expression of a
block, a match arm, a shorthand field or a tuple holds, and a Rust function
returns its tail expression. So a call of a Rust function value in an
expression, as `v[0](x)`, stays a `computed-name` record.

### Unknown code of a unit of dependency code

A failed or truncated file of a unit of dependency code counts for every
target when the search enters the unit. An unresolved site in the code of a
unit with no method name, as an import that no file of the unit gives,
counts for every target too. The closure of the edges of the unit, which
the scope of a unit reads, does not bound such code, for two reasons.

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
  standard library. A function of a plugin has no such edge, so a call of
  `plugin.Open` or `Lookup` concerns every target. See
  [A call of a function value](#a-call-of-a-function-value).
  A method of a value runs through an interface or through reflection. A
  type outside the standard library cannot declare a method of an
  unexported name of an interface of the standard library. It can only
  embed a type of the standard library that has the method, and that method
  is code of the standard library. Reflection gives only the methods of an
  exported name. So the record of a key of the standard library is a
  callback, as `Callback` marks it. It concerns the targets that a Go
  method of an exported name reaches, in the project and in each unit, as
  the scope of a method name does, and what the uncertainties of that code
  concern. Each key of the standard library gives
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

A Go function with no body runs code that the graph does not hold too: a
stub of assembly, as `func Sum(xs []int) int` with a `sum_amd64.s` file,
or a function that `//go:linkname` binds to a function of another package.
The index gives such a function a record with the reason
`no-dependency-source`. When a package of the graph holds the function of
a linkname, as `main.aliased` or `app/internal/impl.secret`, the link step
joins the stub to it with an edge in place of the record. Otherwise it
gives the record a scope.

- A linkname into the standard library, or a stub of assembly in a unit of
  the standard library, runs code of the standard library. So its record is
  a callback, as above. A path with no dot names the standard library only
  when no package of the graph gives it.
- The assembly of a package of a unit calls code of its package and of the
  runtime. So its record concerns what the code of its unit reaches.
- A linkname into another package and the assembly of the project can call
  any function. So their records concern every target.

The case `go/function-no-body` shows each form. A function of the standard
library that only its assembly calls, as a function of the runtime, can
call a target, and the callback does not concern that target.

A Go call of a method on a value whose type the link step does not know is
a record with the method name, as `srv.ListenAndServe()` on a
package-level variable, on an element of a slice, on a range variable, on a
parameter of an interface type, or after a dot import of `net/http`. Such a
call can run a method of a type of a package that no unit gives, and no
edge leads to that code. So the link step gives the caller of each such
record the record of that code too.

- When no unit gives the standard library, the caller gets a callback
  record. Code of the standard library can give a value of its type to any
  code, as an `error`, so the rule does not ask for an import.
- When a Go file imports a package of another module that no unit gives,
  in a build with a project file, the caller gets a record that concerns
  every target. Only code that imports the module, or code of the module,
  makes a value of its type, so such an import names the module.
- A record of the reason `no-dependency-source` names its package, and
  that package has its own record. It gets no other record.

The cases `go/stdlib-callback-method-record` and
`go/dependency-no-unit-method` show it.

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
  method of a value too, and a call of a method on a value of an unknown
  type can run such code. No record shows it.

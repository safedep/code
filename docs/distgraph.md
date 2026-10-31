# Distribution graph

`distgraph` builds the distribution graph of a Python project: which
installed distribution of its environment imports which. A lockfile lists
the distributions of an environment, but not the edges between them. A
caller needs the edges to tell that no code imports a distribution, or
that no other distribution can call one.

The package reads the code on the disk: the Python files of the project
and of each distribution in the site-packages directories that the caller
names. It runs no Python, opens no network connection and reads no byte
code. It parses each file with the tree-sitter grammar of Python and reads
the import statements and the calls of `importlib.import_module` and
`__import__`. It is the analog of [`modgraph`](modgraph.md) for Python.

## The rule

The graph may hold an edge that a run of the program does not have. It
never misses an edge that a run has. A caller can then trust the absence
of a path: no run of the program can reach a distribution that the graph
does not reach.

So a distribution whose imports `Build` cannot read in full is **open**:
it may import any distribution, and a path that meets it reaches each
distribution.

| State | The distribution | Edges |
| --- | --- | --- |
| `imports` | `Build` read each Python file of it | The imports of those files |
| `requires` | Has no Python file, as a distribution of one extension module | Its `Requires-Dist` lines, as evidence only. A requirement is no import, and an import needs no requirement. So the node is open |
| `open` | Has a file that `Build` could not read in full, or an import that `Build` cannot resolve | The imports that `Build` read, and its `Requires-Dist` lines as evidence |

A distribution is open when:

- it has an extension module, a `.so`, a `.pyd` or a `.dylib` file. Its
  code can import any module. `Input.TrustCompiled` keeps such a node
  closed, for a caller that reads the graph for a path into a distribution
  and not for the absence of one. Most environments hold such a
  distribution, as numpy, pydantic-core or cryptography;
- a file of it imports a computed name, as `importlib.import_module(name)`
  or `__import__(f"pkg.{sub}")`. An import of a plain string, as
  `importlib.import_module("yaml")`, is an import of that module;
- a file of it imports a module that no distribution of the site-packages
  directories, no module of the project and no module of the standard
  library holds. The module can be in a path that a `.pth` file or
  `PYTHONPATH` adds, or in a system site-packages that the caller did not
  name. An import inside a `try` statement whose `except` clause catches
  `ImportError`, `ModuleNotFoundError`, `Exception` or every exception
  opens nothing: the program runs without the module. An optional import
  outside such a guard, as in a `contrib` module, opens its distribution;
- a file of it does not parse;
- the file budget ran out, `Input.MaxFiles` or 200,000 files;
- two `.dist-info` directories describe it, as a stale install next to the
  current one. `Build` keeps the first in directory order, and cannot tell
  which code the program runs.

`Node.Reason` names the first cause, with the file and the line.

## The walk

1. `Build` reads the `.dist-info` directories of each site-packages
   directory with `modules.PythonDistributions`: the name and the version
   of `METADATA`, the import names of `top_level.txt` or `RECORD`, the
   extension modules of `RECORD` and the `Requires-Dist` lines with no
   `extra` marker. A `.dist-info` that it cannot read is a problem of the
   graph, not a node.
2. It reads each Python file of the project, its test files too, except
   under a hidden directory, a `__pycache__`, a `node_modules`, a
   `site-packages` or a virtual environment, which holds a `pyvenv.cfg`.
   An edge that only a test file gives is `TestOnly`. The top-level names
   of the project are the modules and the packages at its root and under
   `src`.
3. For each import, the first segment of the module path decides. A name
   of the project gives no edge. A top-level name of a distribution gives
   an edge to it. A name that two distributions share, as `google`, goes
   to the distribution whose `RECORD` lists a file under the longest
   prefix of the module path that any of them holds, as
   `google/cloud/storage/__init__.py` for `google.cloud.storage`, else to
   each one. A module of the standard library, by the table of
   `sys.stdlib_module_names`, gives an edge to the node `std`. A relative
   import stays inside its distribution.
4. The walk reads each distribution that an import reaches. The files of a
   distribution are the files under its top-level names, except its test
   files, which no program imports. A distribution that no import reaches
   adds nothing, and `Node.Walked` is false for it.
5. Once a reached node is open, it can import any distribution. So each
   node counts as reached, and `Build` reads each distribution.

## Use

```go
g, err := distgraph.Build(ctx, distgraph.Input{
    ProjectFS:    os.DirFS(projectDir),
    ProjectName:  "app",
    SitePackages: []fs.FS{os.DirFS(sitePackagesDir)},
})
if err != nil {
    return err
}
path := g.Path([]string{"app"}, "urllib3") // nil when no path reaches it
```

`Graph.Path` and `Graph.Reaches` take a set of start distributions, in any
spelling that PEP 503 normalizes to the name of the node. A path that
meets an open node, or a distribution that `Build` did not walk, can reach
each distribution: the path ends at that node.

The caller finds the site-packages directories. A virtual environment has
one under `lib/python3.X/site-packages`, and `pyvenv.cfg` says whether the
system site-packages count too. `examples/distgraph` takes each one as
`-site-packages`. Install the project first, as `pip install -e .` or
`uv sync`: an import of a distribution that is not installed opens its
importer, and one open node that the project reaches makes each
distribution reached.

## What the graph does not read

- Byte code, `.pth` files and `PYTHONPATH`. A module that only they give is
  an import that nothing holds.
- Imports under `if TYPE_CHECKING:`. They give an edge, which a run does
  not have. The rule allows the edge.
- The version of Python. The table of the standard library is the union of
  the modules of CPython 3.13 and of the modules that 3.12 and 3.13
  removed, so a program on an older Python resolves them too.

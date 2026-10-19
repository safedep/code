# Project graph tests

This directory tests the index step, the link step and the reach query of
`projectgraph` on whole projects. It has three parts.

| Part | Files | Runs in |
| --- | --- | --- |
| Hand-written cases | `cases/<language>/<case>/`, `cases_test.go`, `receivers_test.go` (the forms of a method receiver), `workers_test.go` (one output for any number of workers), `version_test.go` (the hash of the graphs and the summaries per version) | `go test ./...` |
| Public sets | `public/`, `public_test.go`, `govulncheck_test.go`, `methods_test.go` | `go test -tags projectgraph` |
| Benchmark | `bench_test.go`, `synth_test.go`, `synth_languages_test.go` (the C#, PHP, Ruby and Rust generators) | `go test -tags projectgraph -bench .` |

`helpers_test.go` holds the index and link helpers that every part uses.
`report.md` holds the numbers of the last run that a person committed.

`version_test.go` records the hash of the graphs of every case for each
pair of `fileindex.Version` and `projectgraph.Version`, the hash of the
reach results of every case for each triple with `reach.Version`, and the
hash of the golden summaries for each triple with `summary.Version`. A change of a
graph or a summary fails the test until the Version that owns the rule is
bumped and a row is added. The job `projectgraph-sets` of
`.github/workflows/ci.yml` runs the committed public sets on every pull
request and fails under the gates of `scripts/check_projectgraph_gates.sh`:
`precision-high-medium` of pycg-micro at or over 0.95, and every govulncheck
stack found in order. The weekly benchmark job fetches SWARM-JS and gates
it at 0.80.

## Add a hand-written case

1. Make the directory `cases/<language>/<case>/project/` and put two or more
   source files in it. Add a manifest, as `package.json` or `pyproject.toml`,
   when the case needs a root from it.
2. Write `cases/<language>/<case>/expect.yaml`. `TestCases` reads these keys.

   | Key | What the test checks |
   | --- | --- |
   | `source` | The source of the labels, as `safedep:hand`. The test fails when it is empty |
   | `description` | Free text |
   | `units` | The package unit of each directory of dependency code in the project, as `node_modules/lodash: npm:lodash@4.17.21`. The index step reads these directories as import directories, and the link step links each unit as a project of its own |
   | `import_dirs` | The import directories that hold the `units`, as `venv/site-packages`. Without it, each directory of `units` is its own import directory |
   | `test_paths` | Globs that mark test files, as `tests` or `*_test.py` |
   | `max_file_bytes` | The size limit of a file for the index step. A larger file fails |
   | `failed` | The whole set of files that the index step fails |
   | `problems` | The whole set of problems of the link step, with `path` and `text`, as a manifest that it could not read or a unit that no import names |
   | `roots` | The whole set of roots, with `key`, `kind` and `rule`, and `site` when the case checks where a rule found the root |
   | `edges` | A subset of the edges, with `from`, `to`, `kind`, `confidence` and `site` |
   | `exact_edges` | `true` makes `edges` the whole set, so a spurious edge fails the case. Use it for a small case |
   | `no_edges` | Pairs of `from` and `to` that no edge of the graph may join. A case that is not exact needs one for each result that is found, so a spurious edge on its path fails the case |
   | `unresolved` | The whole set of unresolved sites, with `caller`, `site`, `reason` and `method` |
   | `truncated` | The whole set of truncation records of the link step, with `kind`, `key` and `site` |
   | `dropped` | The whole set of `overlay.json` edges that the merge drops, with `site`, `callee` and `reason` |
   | `results` | The reach result of each `target`, with `status`, `path`, `call_sites` and the uncertainties. An `llm` hop of a path has its `resolver`, as `model m1, prompt v2`. The first hop has its `root` kind, as `query`, when the case checks who chose the start of the path. `sources` names the sources of the query, as keys, and `sources_only` searches from them alone. `error` is the text of the error that the query must give, as for an unknown source, in place of a result |

   A site is `path:line`. A key is a symbol key, as `pkg/loader.py//parse` or
   `yaml//load`.
3. Optional. Put an `overlay.json` next to `expect.yaml`. The test merges it
   into the graph after the link step, as `projectgraph/overlay` reads it.
   The package documentation of `projectgraph/overlay` gives the format.
4. A case with `units` has the summary of each unit under `summaries/`, one
   JSON file per unit, as `summaries/npm_lodash_4.17.21.json`. The test
   compares the summary of each unit with its file. Run the test with
   `-update` to write the files, and read each one before you commit it.
5. Run `go test ./test/projectgraph/ -run TestCases/<language>/<case> -count=1 -v`.
   A failure prints the difference between the graph and `expect.yaml`.

Write the expectation from the source, not from the output of the graph. A
case that fails shows a defect or a gap in the link step.

## Run the public sets

```bash
go test -tags projectgraph -run TestPublicSets -count=1 -v ./test/projectgraph/
```

The test logs one block of numbers per set. Set `PROJECTGRAPH_PUBLIC_REPORT`
to a file path to write the block to that file.

| Set | Language | Where it comes from |
| --- | --- | --- |
| `pycg-micro` | Python | `public/pycg-micro/`, a copy of the PyCG micro-benchmark under Apache-2.0. `NOTICE` names the commit |
| `swarm-js` | JavaScript | The SWARM-JS snippets of SWARM-CG. SWARM-CG has no license, so this repository has no copy |
| `govulncheck` | Go | `public/govulncheck/`, the source modules and the JSON test scripts of govulncheck under BSD-3-Clause. `NOTICE` names the commit |

To score SWARM-JS, fetch it at the pinned commit and point the test at it:

```bash
export PROJECTGRAPH_SWARM_JS=$(bash scripts/fetch_swarm_js.sh "$(mktemp -d)")
go test -tags projectgraph -run TestPublicSets -count=1 -v ./test/projectgraph/
```

Each case of a set is a directory with a `callgraph.json`, which maps each
caller name to its callee names. The runner indexes and links the case, then
maps each call edge of the graph to a pair of names:

- `pkg/mod.py//Class//method` becomes `pkg.mod.Class.method`. A package file
  (`__init__.py`, `index.js`) becomes the name of its directory.
- An external key that no file of the case imports becomes a builtin, as
  `<builtin>.len`.
- An edge to a class goes on to the constructor that the link step hangs off
  the class, as `main.A.__init__`. A class with no constructor makes no pair.
- An edge to a re-exported name goes on through the export hops.
- `import` and `export` edges make no pair, because they are not calls.
- A function with an anonymous name, as `<anonymous:12>`, stands for the
  scope that defines it, because the sets number such functions in another
  order. The edge from the scope to the function makes no pair.

The govulncheck set has call stacks and no `callgraph.json`, so
`TestGovulncheckTraces` scores it on its own. For each stack it asks the
reach query for the first function of a dependency on the stack and
compares the path with the functions of the module on the stack. Set
`PROJECTGRAPH_GOVULNCHECK_REPORT` to a file path to write its numbers.

```bash
go test -tags projectgraph -run TestGovulncheckTraces -count=1 -v ./test/projectgraph/
```

Set `PROJECTGRAPH_PUBLIC_DUMP` to a case name, as `classes/self_call`, or to
`all`, to log the edges, the extra pairs and the missed pairs of the case.

## Run the method measure

`methods_test.go` measures how often a target of a vulnerable function or
method of a dependency ends uncertain on real projects.
`public/methods/corpus.txt` lists the projects and the dependencies, and
`public/methods/advisories.json` holds their advisories from the Go vulndb
and from RustSec through OSV. The code is not in the repository, so fetch
it first:

```bash
export PROJECTGRAPH_METHOD_CORPUS=$(bash scripts/fetch_method_corpus.sh "$(mktemp -d)")
go test -tags projectgraph -run TestMethodTargets -count=1 -v ./test/projectgraph/
```

The test logs the count of each answer per language and per symbol kind,
and the same per project. Set `PROJECTGRAPH_METHOD_REPORT` to a file path
to write the block to that file. `report.md` says how the corpus was
chosen and what each cause means.

`receivers_test.go` holds one row per form of the receiver of a method
call, for Go and for Rust, against the small dependency units under
`testdata/receivers`. It runs in `go test ./...`. A row that the link step
does not answer yet names the issue that fixes it and its answer before
that fix, so the test stays green until the fix changes the answer.

## Run the benchmark

```bash
go test -tags projectgraph -run XXX -bench . -benchtime 1x ./test/projectgraph/
```

`synth_test.go` generates one project per language in `b.TempDir()` from a
template with a fixed seed. A project has packages of 25 modules, a barrel
file and a test file per package, an entry file and a manifest. A module
imports one to three other modules and one sibling, calls two or three
external packages, and has a class with four methods and four helper
functions. Six modules in 100 have a dynamic feature: a computed
attribute, a computed import or `eval`. The 200 functions of the 20 external
packages of a language are the reach targets.

`PROJECTGRAPH_BENCH_FILES` sets the project sizes, as `1000,10000`. The
default is `1000`. `PROJECTGRAPH_BENCH_WORKERS` lists the workers of the
cold index step, as `1,4`, the default. The other index steps use the last
one. A run at 50,000 files sets `4` and leaves out the serial step. A run of
one step selects it by name, as
`-bench 'BenchmarkProjectGraph/python/files=10000/link$'`. The scheduled job
`.github/workflows/projectgraph-bench.yml` runs every Monday at 1,000 files
and on manual dispatch with any size. It writes the numbers to the job
summary.

| Step | What it measures |
| --- | --- |
| `index-cold/workers=1` | The index step with no store and one worker. Every file is parsed |
| `index-cold/workers=N` | The same with N workers. The speed-up is the time of `workers=1` over this time |
| `index-cold-store` | The index step with an empty `localdbstore` and the parallel workers. Every file is parsed and stored |
| `index-warm` | The index step with a store that holds every index. No file is parsed |
| `link` | `projectgraph.Build` on the indexes |
| `reach` | `reach.Reach` with the 200 targets |
| `summary` | `summary.Summarize` of the project read as one unit of dependency code, for the languages whose dependency code the link step links. It reports the `symbols` of the unit and the `reaches` of all of them |

Each step reports these metrics next to `ns/op`.

| Metric | Meaning |
| --- | --- |
| `s/1k-files` | Seconds per 1,000 files |
| `base-heap-MiB` | The heap before the step, after a GC. It holds the indexes and the graph that the step reads |
| `peak-heap-MiB` | The highest heap that a sample every 5 ms saw during the step |
| `index-KiB/file` | The JSON size of one index, as the store keeps it |
| `retained-KiB/file` | The heap that the result of the index step holds after a GC, per file |
| `retained-B/edge` | The heap that the graph holds after a GC, per edge |
| `edges`, `roots`, `unresolved` | The size of the graph. `unresolved` counts the distinct unresolved call sites |
| `unresolved-rate` | The unresolved call sites over all call sites of the indexes |
| `targets`, `found` | The reach targets and the targets with a path |

The memory that a step uses is `peak-heap-MiB` minus `base-heap-MiB`.

`TestDegradesAtHalfTheColdTime` and `TestSynthGivesOneOutputForAnyNumberOfWorkers`
run with the tag and no `-bench`. The first gives the index step half of
its cold time and checks the `missing` files and the `budget` causes. The
second compares the output of one worker and four workers byte for byte.

## Read the report

`report.md` has one section per part. The metrics of the public sets are
these.

| Metric | Definition |
| --- | --- |
| `exact-cases` | The share of cases whose pairs are the pairs of `callgraph.json` |
| `precision` | The share of graph pairs that `callgraph.json` holds |
| `precision-high-medium` | The same, for the pairs with a `high` or `medium` edge |
| `recall` | The share of `callgraph.json` pairs that the graph holds at any confidence |
| `unresolved-rate` | The share of call sites that stay unresolved after the link step |
| `kind=<kind>` | The precision of the pairs with an edge of that kind, and the share of the `callgraph.json` pairs that an edge of that kind finds |
| `callee=<class>` | The recall of the pairs whose callee is in the project, external or a builtin |
| `category=<category>` | The recall of the cases of one directory of the set, as `classes` |

The design gates a language on a precision at or above 0.95 for `high` and
`medium` edges. The report says where a language is below a gate and why.

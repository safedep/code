# Project graph tests

This directory tests the index step, the link step and the reach query of
`projectgraph` on whole projects. It has three parts.

| Part | Files | Runs in |
| --- | --- | --- |
| Hand-written cases | `cases/<language>/<case>/`, `cases_test.go` | `go test ./...` |
| Public sets | `public/`, `public_test.go` | `go test -tags projectgraph` |
| Benchmark | `bench_test.go`, `synth_test.go` | `go test -tags projectgraph -bench .` |

`report.md` holds the numbers of the last run that a person committed.

## Add a hand-written case

1. Make the directory `cases/<language>/<case>/project/` and put two or more
   source files in it. Add a manifest, as `package.json` or `pyproject.toml`,
   when the case needs a root from it.
2. Write `cases/<language>/<case>/expect.yaml`. `TestCases` reads these keys.

   | Key | What the test checks |
   | --- | --- |
   | `source` | The source of the labels, as `safedep:hand`. The test fails when it is empty |
   | `description` | Free text |
   | `test_paths` | Globs that mark test files, as `tests` or `*_test.py` |
   | `max_file_bytes` | The size limit of a file for the index step. A larger file fails |
   | `failed` | The whole set of files that the index step fails |
   | `roots` | The whole set of roots, with `key`, `kind` and `rule` |
   | `edges` | A subset of the edges, with `from`, `to`, `kind`, `confidence` and `site` |
   | `no_edges` | Pairs of `from` and `to` that no edge of the graph may join |
   | `unresolved` | The whole set of unresolved sites, with `caller`, `site`, `reason` and `method` |
   | `results` | The reach result of each `target`, with `status`, `path`, `call_sites` and the uncertainties |

   A site is `path:line`. A key is a symbol key, as `pkg/loader.py//parse` or
   `yaml//load`.
3. Run `go test ./test/projectgraph/ -run TestCases/<language>/<case> -count=1 -v`.
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

Set `PROJECTGRAPH_PUBLIC_DUMP` to a case name, as `classes/self_call`, or to
`all`, to log the edges, the extra pairs and the missed pairs of the case.

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
default is `1000`. The scheduled job `.github/workflows/projectgraph-bench.yml`
runs every Monday at 1,000 files and on manual dispatch with any size. It
writes the numbers to the job summary.

| Step | What it measures |
| --- | --- |
| `index-cold` | The index step with no store. Every file is parsed |
| `index-cold-store` | The index step with an empty `localdbstore`. Every file is parsed and stored |
| `index-warm` | The index step with a store that holds every index. No file is parsed |
| `link` | `projectgraph.Build` on the indexes |
| `reach` | `reach.Reach` with the 200 targets |

Each step reports these metrics next to `ns/op`.

| Metric | Meaning |
| --- | --- |
| `s/1k-files` | Seconds per 1,000 files |
| `base-heap-MiB` | The heap before the step, after a GC. It holds the indexes and the graph that the step reads |
| `peak-heap-MiB` | The highest heap that a sample every 5 ms saw during the step |
| `index-KiB/file` | The JSON size of one index, as the store keeps it |
| `edges`, `roots`, `unresolved` | The size of the graph. `unresolved` counts the distinct unresolved call sites |
| `unresolved-rate` | The unresolved call sites over all call sites of the indexes |
| `targets`, `found` | The reach targets and the targets with a path |

The memory that a step uses is `peak-heap-MiB` minus `base-heap-MiB`.

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
| `kind=<kind>` | The precision of the pairs with an edge of that kind |
| `callee=<class>` | The recall of the pairs whose callee is in the project, external or a builtin |
| `category=<category>` | The recall of the cases of one directory of the set, as `classes` |

The design gates a language on a precision at or above 0.95 for `high` and
`medium` edges. The report says where a language is below a gate and why.

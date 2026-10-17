# Project graph report

Two runs of the benchmark and the public sets, both on 2026-10-17 on one
machine. "Before" is the commit that adds this file (1848f7c). "After" is
the commit that adds this second run, after the fixes of the adversarial
review: the `name` and `reference` edges, the records for bare names and
for imports that can be first-party, `super`, module variables, decorator
closures and the budget rule for incomplete files. `README.md` says how to
run them and what each metric means. The run of 10,000 files is from
before only.

## Machine

| Item | Value |
| --- | --- |
| Cores | 4 (`nproc`) |
| CPU | `Intel(R) Xeon(R) Processor @ 2.10GHz` (`model name` in `/proc/cpuinfo`, a virtual machine) |
| Memory | 15 GiB |
| OS | Linux 6.18, amd64 |
| Go | go1.25.1 linux/amd64 |

The index step is serial today. The parallel executor comes in step 32. So
every cold number below uses one core of the four, and the user time of a
run is about its wall time.

## Per language

Precision and recall come from the public sets. TypeScript has no public set
yet, so it has no precision or recall. The synthetic unresolved rate comes
from the generated project of 1,000 files. Times are per 1,000 files on the
project of 1,000 files. Memory is the peak heap minus the heap before the
step.

| Language | Run | Set | Precision, high and medium | Recall | Unresolved rate, set | Unresolved rate, synthetic | Index cold | Index warm | Link | Reach, 200 targets | Memory cold / warm / link / reach |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Python | before | pycg-micro | 0.735 (139/189) | 0.542 (143/264) | 0.204 (51/250) | 0.159 | 4.22 s | 0.31 s | 0.25 s | 31 ms | 71 / 42 / 41 / 6 MiB |
| Python | after | pycg-micro | 0.940 (142/151) | 0.697 (184/264) | 0.344 (86/250) | 0.159 | 4.03 s | 0.30 s | 0.33 s | 33 ms | 77 / 43 / 50 / 6 MiB |
| JavaScript | before | swarm-js | 0.618 (152/246) | 0.526 (152/289) | 0.189 (58/307) | 0.228 | 4.37 s | 0.34 s | 0.24 s | 32 ms | 80 / 43 / 35 / 7 MiB |
| JavaScript | after | swarm-js | 0.821 (156/190) | 0.706 (204/289) | 0.358 (110/307) | 0.263 | 4.48 s | 0.26 s | 0.28 s | 36 ms | 77 / 41 / 43 / 8 MiB |
| TypeScript | before | none | n/a | n/a | n/a | 0.125 | 7.02 s | 0.27 s | 0.20 s | 31 ms | 66 / 32 / 34 / 5 MiB |
| TypeScript | after | none | n/a | n/a | n/a | 0.125 | 6.94 s | 0.28 s | 0.23 s | 24 ms | 66 / 35 / 35 / 5 MiB |
| Python | step 27 | pycg-micro | 0.954 (145/152) | 0.708 (187/264) | 0.340 (85/250) | 0.159 | 4.15 s | 0.33 s | 0.40 s | 37 ms | 81 / 45 / 56 / 6 MiB |
| JavaScript | step 27 | swarm-js | 0.821 (156/190) | 0.706 (204/289) | 0.358 (110/307) | 0.263 | 4.80 s | 0.29 s | 0.35 s | 29 ms | 73 / 41 / 49 / 8 MiB |
| TypeScript | step 27 | none | n/a | n/a | n/a | 0.094 | 7.38 s | 0.33 s | 0.34 s | 27 ms | 68 / 34 / 49 / 4 MiB |
| Go | step 29 | govulncheck | n/a | 1.000 (9/9) stacks | n/a | 0.082 | 3.78 s | 0.30 s | 0.32 s | 31 ms | 89 / 42 / 52 / 4 MiB |

Python is now at 0.940, just under the precision gate of the design (0.95
for `high` and `medium` edges). After step 27 it is 0.954, and its recall is
0.708. JavaScript stays at 0.821 and 0.706.

The Go row comes from step 29. The govulncheck set gives call stacks and
no call graph, so it has no precision of edges. Its recall is the share of
the stacks that the reach query finds, and the section on the public sets
has the other numbers. The synthetic Go project of 1,000 files has 32,015
edges and 2,860 unresolved sites, and the reach query finds 199 of 200
targets. Most unresolved sites are calls on a loop variable of an interface
type, as `item.Notify(data)`, which the template has in every method.

The step 27 rows come from one more run of the benchmark at 1,000 files.
The declared types of TypeScript narrow the calls on a typed field, as
`this.store.fetch(x)`, so the synthetic TypeScript project has 2,860 and
not 3,781 unresolved sites and 31,486 and not 38,695 edges. It now finds
199 of 200 targets. The one that it does not find is uncertain, because a
call on the module of a computed import is ambiguous. Its path of before
went through a `name` edge from `this.store.decodeToken(x)` to a function
of another module, and the declared type of `store` shows that the path is
false. The link step is 0.07 to 0.11 s slower per 1,000 files. JavaScript is at 0.821. The section on the
public sets names the causes. The unresolved rate of the sets went up,
because a call of a bare name with no binding is now a record and not an
external edge.

## Benchmark

### The generated projects

`synth_test.go` writes each project from a template with the seed 20261017.
A source file has about 85 lines and 2.3 KiB. The index of a file is 8.9 KiB
(TypeScript) to 10.9 KiB (JavaScript) as JSON. The 200 reach targets are the
functions of 20 external packages. At 1,000 files the reach query finds 200
targets in Python and JavaScript and 199 in TypeScript.

The synthetic unresolved rate depends on the template. Most of its
unresolved sites are calls on a field or a loop variable, as
`self.items.append(x)`, `this.store.fetch(x)` and `item.notify(x)`. The link
step had no field types before step 27, so these stayed unresolved by design.

### 1,000 files

`-benchtime 1x`, one run per step.

After:

| Language | Step | Time | Base heap | Peak heap | Graph |
| --- | --- | --- | --- | --- | --- |
| Python | index-cold | 4.03 s | 27.2 MiB | 104.6 MiB | |
| Python | index-cold-store | 4.14 s | 27.2 MiB | 103.7 MiB | |
| Python | index-warm | 0.30 s | 27.2 MiB | 70.7 MiB | |
| Python | link | 0.33 s | 27.2 MiB | 76.9 MiB | 48,566 edges, 113 roots, 6,585 unresolved |
| Python | reach | 33 ms | 10.7 MiB | 17.1 MiB | 200 of 200 found |
| JavaScript | index-cold | 4.48 s | 25.3 MiB | 102.6 MiB | |
| JavaScript | index-cold-store | 4.84 s | 25.3 MiB | 103.3 MiB | |
| JavaScript | index-warm | 0.26 s | 25.3 MiB | 66.2 MiB | |
| JavaScript | link | 0.28 s | 25.3 MiB | 68.0 MiB | 38,307 edges, 39 roots, 9,443 unresolved |
| JavaScript | reach | 36 ms | 9.9 MiB | 17.6 MiB | 200 of 200 found |
| TypeScript | index-cold | 6.94 s | 21.1 MiB | 87.5 MiB | |
| TypeScript | index-cold-store | 6.84 s | 21.1 MiB | 83.1 MiB | |
| TypeScript | index-warm | 0.28 s | 21.1 MiB | 56.1 MiB | |
| TypeScript | link | 0.23 s | 21.1 MiB | 56.4 MiB | 38,695 edges, 39 roots, 3,781 unresolved |
| TypeScript | reach | 24 ms | 9.1 MiB | 13.8 MiB | 200 of 200 found |

The index of a file is now 10.4 KiB (Python and JavaScript) and 9.1 KiB
(TypeScript) as JSON. The graph has 7,948 more edges in Python, 2,850 in
JavaScript and 8,185 in TypeScript, most of them `name` and `reference`
edges, and the link step is 15% to 32% slower. The cold index step is
inside the noise of one run. A back-to-back run of the JavaScript cold
step gave 4.23 s before and 4.58 s after. TypeScript now finds 200 of 200
targets.

Before:

| Language | Step | Time | Base heap | Peak heap | Graph |
| --- | --- | --- | --- | --- | --- |
| Python | index-cold | 4.22 s | 25.6 MiB | 96.6 MiB | |
| Python | index-cold-store | 4.10 s | 25.6 MiB | 92.9 MiB | |
| Python | index-warm | 0.31 s | 25.6 MiB | 67.4 MiB | |
| Python | link | 0.25 s | 25.6 MiB | 66.4 MiB | 40,618 edges, 113 roots, 6,585 unresolved |
| Python | reach | 31 ms | 9.6 MiB | 15.4 MiB | 200 of 200 found |
| JavaScript | index-cold | 4.37 s | 26.3 MiB | 105.9 MiB | |
| JavaScript | index-cold-store | 4.67 s | 26.3 MiB | 107.3 MiB | |
| JavaScript | index-warm | 0.34 s | 26.3 MiB | 69.5 MiB | |
| JavaScript | link | 0.24 s | 26.3 MiB | 61.7 MiB | 35,457 edges, 39 roots, 9,332 unresolved |
| JavaScript | reach | 32 ms | 9.6 MiB | 16.4 MiB | 200 of 200 found |
| TypeScript | index-cold | 7.02 s | 19.6 MiB | 85.7 MiB | |
| TypeScript | index-cold-store | 7.27 s | 19.6 MiB | 81.8 MiB | |
| TypeScript | index-warm | 0.27 s | 19.6 MiB | 51.4 MiB | |
| TypeScript | link | 0.20 s | 19.6 MiB | 53.3 MiB | 30,510 edges, 39 roots, 3,781 unresolved |
| TypeScript | reach | 31 ms | 8.1 MiB | 12.7 MiB | 199 of 200 found |

### 10,000 files

Before only. One run of `PROJECTGRAPH_BENCH_FILES=10000`. The whole run took 9 min 12 s,
so the scheduled job keeps 1,000 files as its default.

| Language | Step | Time | Base heap | Peak heap | Graph |
| --- | --- | --- | --- | --- | --- |
| Python | index-cold | 45.8 s | 246 MiB | 979 MiB | |
| Python | index-cold-store | 42.3 s | 246 MiB | 968 MiB | |
| Python | index-warm | 3.33 s | 246 MiB | 633 MiB | |
| Python | link | 4.17 s | 246 MiB | 616 MiB | 407,311 edges, 1,115 roots, 65,687 unresolved |
| Python | reach | 0.43 s | 86 MiB | 135 MiB | 200 of 200 found |
| JavaScript | index-cold | 46.9 s | 253 MiB | 1,057 MiB | |
| JavaScript | index-cold-store | 49.9 s | 253 MiB | 1,053 MiB | |
| JavaScript | index-warm | 3.33 s | 253 MiB | 646 MiB | |
| JavaScript | link | 3.64 s | 253 MiB | 556 MiB | 355,104 edges, 373 roots, 93,242 unresolved |
| JavaScript | reach | 0.41 s | 86 MiB | 143 MiB | 200 of 200 found |
| TypeScript | index-cold | 71.0 s | 186 MiB | 787 MiB | |
| TypeScript | index-cold-store | 73.1 s | 186 MiB | 789 MiB | |
| TypeScript | index-warm | 2.82 s | 186 MiB | 477 MiB | |
| TypeScript | link | 3.09 s | 186 MiB | 481 MiB | 305,084 edges, 373 roots, 37,702 unresolved |
| TypeScript | reach | 0.37 s | 71 MiB | 105 MiB | 200 of 200 found |

Python index-cold-store is faster than index-cold at 10,000 files. A single
run has a noise of about 8%. The store costs less than that.

### Against the budget table

The budget table of the architecture asset, section 7, gives targets on 4
cores. The measured index step uses one core. The table holds the numbers
from before. After, the link step of 1,000 files is 0.23 to 0.33 s, at the
target of 0.3 s, and the other steps at 1,000 files keep their place
against the targets.

| Step | Target, 1,000 | Measured, 1,000 (Python / JavaScript / TypeScript) | Target, 10,000 | Measured, 10,000 | Inside the target |
| --- | --- | --- | --- | --- | --- |
| Index, cold | 3 s | 4.2 / 4.4 / 7.0 s | 25 s | 45.8 / 46.9 / 71.0 s | No. At 4 workers with a linear speed-up, 1.1 / 1.1 / 1.8 s and 11 / 12 / 18 s |
| Index, warm | 0.2 s | 0.31 / 0.34 / 0.27 s | 1.5 s | 3.3 / 3.3 / 2.8 s | No |
| Link | 0.3 s | 0.25 / 0.24 / 0.20 s | 2 s | 4.2 / 3.6 / 3.1 s | At 1,000 only |
| Reach, 200 targets | 50 ms | 31 / 32 / 31 ms | 0.5 s | 0.43 / 0.41 / 0.37 s | Yes |
| Memory, index cold | 50 MiB per worker | 66 to 80 MiB, one worker | | 601 to 804 MiB | No |
| Memory, index records | about 4 KiB per file | 8.9 to 10.9 KiB per file as JSON, 30 to 40 KiB per file in memory | | | No |
| Memory, link | about 1 KiB per 10 edges | about 9 KiB per 10 edges (370 MiB for 407,311 Python edges) | | 296 to 370 MiB | No |

What the numbers say:

- The cold index step costs 4.2 to 7.0 s per 1,000 files on one core. With
  a linear speed-up on 4 workers it is inside the target. The parallel
  executor of step 32 must show that.
- The cold index step holds every index of the run in `Result.Indexes`, so
  its memory grows with the project and not with the number of workers. At
  10,000 files it is 600 to 800 MiB.
- An index in memory is 3 to 4 times its JSON size. The warm step is 10.7
  times slower at 10 times the files, so it is linear. It decodes about
  10 KiB of JSON per file.
- The link step is not linear. It is 15 to 17 times slower at 10 times the
  files. Linear growth from 10,000 files gives 21 s for Python at 50,000
  files. The cause of the extra growth is not known yet.
- The reach query is inside its target at both sizes.
- The link memory is about 0.9 KiB per edge, which is 9 times the estimate.
  At 50,000 files the graph of this template would be about 2 GiB.

## Public sets

| Set | Language | Source | Cases | Files | License |
| --- | --- | --- | --- | --- | --- |
| pycg-micro | Python | github.com/vitsalis/PyCG `micro-benchmark/snippets` at 8d5dc408 | 119 | 160 | Apache-2.0, committed in `public/pycg-micro/` |
| swarm-js | JavaScript | github.com/secure-software-engineering/SWARM-CG `benchmarks/javascript/swarm_js` at 5929c0a3 | 126 | 171 | None. Fetched by `scripts/fetch_swarm_js.sh`, not committed |
| govulncheck | Go | go.googlesource.com/vuln `cmd/govulncheck/testdata` at e672cbe8 | 5 modules, 9 stacks | 10 | BSD-3-Clause, committed in `public/govulncheck/` |

The design names 126 PyCG cases. That is the `pycg_extended` set of SWARM-CG,
which has no license. The PyCG repository itself has 119 cases.

### govulncheck, step 29

`TestGovulncheckTraces` reads the JSON output of the five test scripts of
govulncheck in source mode. Each finding at the level of a symbol has a
stack from the entry of the program to the vulnerable symbol. The runner
takes the first function of a dependency on the stack, asks the reach
query for it, and compares the path with the functions of the module on the
stack. It also asks for each vulnerable symbol that govulncheck reports as
not called.

| Metric | Value |
| --- | --- |
| Stacks that the reach query finds | 1.000 (9/9) |
| Paths that end at the call site of the stack | 1.000 (7/7). The script of the standard library leaves out the lines |
| Paths with the functions of the stack, in order | 1.000 (9/9) |
| Symbols that govulncheck does not reach and the graph finds | 0.000 (0/63) |

At the commit before step 29 the same runner gives 0.333 (3/9) found and
0.000 (0/9) stacks. The other six were uncertain, because every import of
a Go file was a record that can reach any target, and `go.main` matched
nothing. Two stacks needed fixes of the per-file graph: a call on a composite literal, as
`gjson.Result{}.Get("")`, and a call of a generic function with type
arguments, as `work[string]("golang")`, which the grammar parses as a
conversion. The set is small, and its stacks go from `main` to a
dependency in at most three first-party calls. It does not measure the
`cha` edges of interfaces.

### Totals

"Step 27" is the run after the `cha` and `field` edges and the declared
types of plan step 27, on 2026-10-17. The benchmark did not run again.

| Metric | pycg-micro before | pycg-micro after | pycg-micro step 27 | swarm-js before | swarm-js after | swarm-js step 27 |
| --- | --- | --- | --- | --- | --- | --- |
| Cases with the exact pairs | 0.378 (45/119) | 0.445 (53/119) | 0.454 (54/119) | 0.262 (33/126) | 0.333 (42/126) | 0.333 (42/126) |
| Precision | 0.741 (143/193) | 0.694 (184/265) | 0.703 (187/266) | 0.618 (152/246) | 0.664 (204/307) | 0.664 (204/307) |
| Precision, high and medium | 0.735 (139/189) | 0.940 (142/151) | 0.954 (145/152) | 0.618 (152/246) | 0.821 (156/190) | 0.821 (156/190) |
| Recall | 0.542 (143/264) | 0.697 (184/264) | 0.708 (187/264) | 0.526 (152/289) | 0.706 (204/289) | 0.706 (204/289) |
| Unresolved rate | 0.204 (51/250) | 0.344 (86/250) | 0.340 (85/250) | 0.189 (58/307) | 0.358 (110/307) | 0.358 (110/307) |

### Precision per edge kind

| Kind | pycg-micro before | pycg-micro after | pycg-micro step 27 | swarm-js before | swarm-js after | swarm-js step 27 |
| --- | --- | --- | --- | --- | --- | --- |
| `direct` | 0.678 (99/146) | 0.934 (99/106) | 0.934 (99/106) | 0.569 (112/197) | 0.810 (115/142) | 0.810 (115/142) |
| `member` | 0.938 (30/32) | 0.943 (33/35) | 1.000 (36/36) | 0.929 (26/28) | 0.931 (27/29) | 0.931 (27/29) |
| `assignment` | 0.923 (12/13) | 1.000 (12/12) | 1.000 (12/12) | 0.667 (14/21) | 0.737 (14/19) | 0.737 (14/19) |
| `wildcard` | 1.000 (4/4) | 1.000 (4/4) | 1.000 (4/4) | none | none | none |
| `cha` | none | none | none | none | none | none |
| `field` | none | none | none | none | none | 0.429 (3/7) |
| `name` | none | 0.400 (2/5) | 0.400 (2/5) | none | 0.375 (3/8) | none |
| `reference` | none | 0.405 (47/116) | 0.405 (47/116) | none | 0.470 (62/132) | 0.470 (62/132) |

A `reference` edge goes from the definition that holds the value, as
`main` in `run(func)`. The sets name the function that calls the value, as
`run`, so most `reference` pairs count as false. They are `low`, so the
gate does not count them.

### Recall per edge kind

The share of the `callgraph.json` pairs that an edge of the kind finds. A
pair that edges of two kinds find counts for both. The first run did not
measure it.

| Kind | pycg-micro after | pycg-micro step 27 | swarm-js after | swarm-js step 27 |
| --- | --- | --- | --- | --- |
| `direct` | 0.375 (99/264) | 0.375 (99/264) | 0.398 (115/289) | 0.398 (115/289) |
| `member` | 0.125 (33/264) | 0.136 (36/264) | 0.093 (27/289) | 0.093 (27/289) |
| `assignment` | 0.045 (12/264) | 0.045 (12/264) | 0.048 (14/289) | 0.048 (14/289) |
| `wildcard` | 0.015 (4/264) | 0.015 (4/264) | none | none |
| `cha` | none | none | none | none |
| `field` | none | none | none | 0.010 (3/289) |
| `name` | 0.008 (2/264) | 0.008 (2/264) | 0.010 (3/289) | none |
| `reference` | 0.178 (47/264) | 0.178 (47/264) | 0.215 (62/289) | 0.215 (62/289) |

### What step 27 changed

- Neither set makes a `cha` edge. Their class cases make each instance by
  a call of the class, as `b = B()`, and such a receiver gets no `cha`
  edge. The calls on `self` and `this` in the sets reach a method that no
  subclass overrides, or an attribute that no class defines as a method.
  So the sets do not measure `cha`. The hand-written `subclass-override`
  and `interface` cases do, and each `cha` edge there is true.
- In swarm-js the 8 `name` edges of member calls are now 7 `field`
  edges. The edge to a plain function that no property holds is gone, and
  the 3 true pairs stay. A JavaScript call of a bare name keeps the `name`
  edge, and swarm-js has none.
- The method lookup now reads the MRO, and a call on `self` whose method
  a base in another file defines now resolves. The two false `member`
  pairs of pycg-micro are gone, and three true pairs are new. `classes`,
  `external` and `mro` each find one more pair.
- The high and medium precision of Python is now 0.954, over the gate of
  0.95. JavaScript stays at 0.821.

### Recall per callee

| Callee | pycg-micro before | pycg-micro after | swarm-js before | swarm-js after |
| --- | --- | --- | --- | --- |
| In the project | 0.531 (129/243) | 0.700 (170/243) | 0.529 (147/278) | 0.716 (199/278) |
| External | 0.667 (6/9) | 0.667 (6/9) | none | none |
| Builtin | 0.667 (8/12) | 0.667 (8/12) | 0.455 (5/11) | 0.455 (5/11) |

### Recall per category

| Category | pycg-micro before | pycg-micro after | swarm-js before | swarm-js after |
| --- | --- | --- | --- | --- |
| args | 0.500 (7/14) | 0.500 (7/14) | 0.500 (11/22) | 0.636 (14/22) |
| arrays | | | 0.412 (7/17) | 0.824 (14/17) |
| arrow_functions | | | 0.357 (5/14) | 0.357 (5/14) |
| assignments | 0.133 (2/15) | 1.000 (15/15) | 0.263 (5/19) | 1.000 (19/19) |
| builtins | 0.300 (3/10) | 0.600 (6/10) | 0.077 (1/13) | 0.385 (5/13) |
| classes | 0.750 (39/52) | 0.788 (41/52) | 0.720 (36/50) | 0.820 (41/50) |
| decorators | 0.591 (13/22) | 0.636 (14/22) | 0.455 (10/22) | 0.545 (12/22) |
| dicts | 0.263 (5/19) | 0.684 (13/19) | | |
| direct_calls | 0.400 (4/10) | 0.500 (5/10) | 0.607 (17/28) | 0.714 (20/28) |
| dynamic | 0.000 (0/2) | 0.000 (0/2) | 0.000 (0/2) | 0.000 (0/2) |
| exceptions | 0.000 (0/3) | 0.333 (1/3) | 0.667 (2/3) | 0.667 (2/3) |
| external | 0.727 (8/11) | 0.727 (8/11) | | |
| functions | 1.000 (4/4) | 1.000 (4/4) | 1.000 (4/4) | 1.000 (4/4) |
| generators | 0.444 (8/18) | 0.444 (8/18) | 0.429 (6/14) | 0.429 (6/14) |
| imports | 0.929 (13/14) | 1.000 (14/14) | 0.958 (23/24) | 1.000 (24/24) |
| inheritance | | | 0.600 (6/10) | 0.800 (8/10) |
| kwargs | 0.300 (3/10) | 0.500 (5/10) | 0.333 (3/9) | 0.556 (5/9) |
| lambdas | 0.357 (5/14) | 0.357 (5/14) | | |
| lists | 0.500 (8/16) | 0.938 (15/16) | | |
| mixins | | | 0.429 (3/7) | 0.571 (4/7) |
| mro | 0.722 (13/18) | 0.833 (15/18) | | |
| objects | | | 0.263 (5/19) | 0.684 (13/19) |
| returns | 0.667 (8/12) | 0.667 (8/12) | 0.667 (8/12) | 0.667 (8/12) |

### Why the numbers are low, after

- Python has 9 false pairs with a `high` or `medium` edge. 3 are calls of
  a builtin, as `str.join` and `eval`, which the sets name by the type of
  the receiver. The other 6 are single cases, as a decorator call and a
  member of an external base.
- JavaScript has 34 such pairs. 21 are calls of a builtin, as
  `console.log`, `Object.assign`, `Array.map` and `Error`, which swarm-js
  leaves out of the expected pairs or names by type. 11 are calls of a
  variable that holds a function, and 2 are member calls.
- Recall is still low where a function moves through a lambda, a
  generator or an arrow function. A function that a call returns is a
  `reference` from the function that returns it, and the sets want the
  pair from the caller of the result.
- The `name` and `reference` edges raise the recall and keep the gate
  precision, because they are `low`. Their own precision is 0.38 to 0.47.

### Why the numbers were low, before

- Most false pairs are calls of a local variable or a parameter, as `a()`
  after `a = func`. The link step makes a `direct` edge from such a call to
  an external node with the bare name `a`. These are 41 of the 50 false
  pairs of pycg-micro and 55 of the 96 of swarm-js. With no such edge, the
  precision would be 0.941 (143/152) for Python and 0.796 (152/191) for
  JavaScript. The fix belongs in the link step. A bare name that the file
  binds as a local or a parameter is not an external call.
- Recall is low where a function value moves through data: assignments,
  dicts, lists, keyword arguments, lambdas, returned functions and objects.
  The per-file assignment graph does not follow a function through a
  container, a parameter or a return value. Imports (0.93 and 0.96) and
  classes (0.75 and 0.72) are the strong parts.
- swarm-js names an anonymous function by its position, as `main.<arrow1>`.
  The graph names it by the variable that holds it. The runner does not map
  one to the other, so these pairs count as missed and, where the graph
  names the variable, as false.
- A builtin method on a value, as `<**PyStr**>.split` or `<builtin>.Array.map`,
  needs the type of the receiver. The graph names the call by its text, as
  `str.split`, so it does not match.

Venkatesh et al. report 95.0% exact edge matches for PyCG on the 126 Python
cases and 82.2% for Jelly on swarm-js. This report did not run PyCG or Jelly,
and its exact-cases metric counts whole cases, so the numbers do not compare
one to one. The recall gate of the design needs a run of PyCG and Jelly on
the same cases.

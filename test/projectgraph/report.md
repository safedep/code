# Project graph report

The first run of the benchmark and the public sets, on 2026-10-17, at the
commit that adds this file. `README.md` says how to run them and what each
metric means.

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

| Language | Set | Precision, high and medium | Recall | Unresolved rate, set | Unresolved rate, synthetic | Index cold | Index warm | Link | Reach, 200 targets | Memory cold / warm / link / reach |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Python | pycg-micro | 0.735 (139/189) | 0.542 (143/264) | 0.204 (51/250) | 0.159 | 4.22 s | 0.31 s | 0.25 s | 31 ms | 71 / 42 / 41 / 6 MiB |
| JavaScript | swarm-js | 0.618 (152/246) | 0.526 (152/289) | 0.189 (58/307) | 0.228 | 4.37 s | 0.34 s | 0.24 s | 32 ms | 80 / 43 / 35 / 7 MiB |
| TypeScript | none | n/a | n/a | n/a | 0.125 | 7.02 s | 0.27 s | 0.20 s | 31 ms | 66 / 32 / 34 / 5 MiB |

Both languages with a set are below the precision gate of the design (0.95
for `high` and `medium` edges). The section on the public sets names the
cause.

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
step has no field types before step 27, so these stay unresolved by design.

### 1,000 files

`-benchtime 1x`, one run per step.

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

One run of `PROJECTGRAPH_BENCH_FILES=10000`. The whole run took 9 min 12 s,
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
cores. The measured index step uses one core.

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

The design names 126 PyCG cases. That is the `pycg_extended` set of SWARM-CG,
which has no license. The PyCG repository itself has 119 cases.

### Totals

| Metric | pycg-micro | swarm-js |
| --- | --- | --- |
| Cases with the exact pairs | 0.378 (45/119) | 0.262 (33/126) |
| Precision | 0.741 (143/193) | 0.618 (152/246) |
| Precision, high and medium | 0.735 (139/189) | 0.618 (152/246) |
| Recall | 0.542 (143/264) | 0.526 (152/289) |
| Unresolved rate | 0.204 (51/250) | 0.189 (58/307) |

### Precision per edge kind

| Kind | pycg-micro | swarm-js |
| --- | --- | --- |
| `direct` | 0.678 (99/146) | 0.569 (112/197) |
| `member` | 0.938 (30/32) | 0.929 (26/28) |
| `assignment` | 0.923 (12/13) | 0.667 (14/21) |
| `wildcard` | 1.000 (4/4) | none |

The sets make no `cha` or `field` edge, because the link step has no such
edge before step 27.

### Recall per callee

| Callee | pycg-micro | swarm-js |
| --- | --- | --- |
| In the project | 0.531 (129/243) | 0.529 (147/278) |
| External | 0.667 (6/9) | none |
| Builtin | 0.667 (8/12) | 0.455 (5/11) |

### Recall per category

| Category | pycg-micro | swarm-js |
| --- | --- | --- |
| args | 0.500 (7/14) | 0.500 (11/22) |
| arrays | | 0.412 (7/17) |
| arrow_functions | | 0.357 (5/14) |
| assignments | 0.133 (2/15) | 0.263 (5/19) |
| builtins | 0.300 (3/10) | 0.077 (1/13) |
| classes | 0.750 (39/52) | 0.720 (36/50) |
| decorators | 0.591 (13/22) | 0.455 (10/22) |
| dicts | 0.263 (5/19) | |
| direct_calls | 0.400 (4/10) | 0.607 (17/28) |
| dynamic | 0.000 (0/2) | 0.000 (0/2) |
| exceptions | 0.000 (0/3) | 0.667 (2/3) |
| external | 0.727 (8/11) | |
| functions | 1.000 (4/4) | 1.000 (4/4) |
| generators | 0.444 (8/18) | 0.429 (6/14) |
| imports | 0.929 (13/14) | 0.958 (23/24) |
| inheritance | | 0.600 (6/10) |
| kwargs | 0.300 (3/10) | 0.333 (3/9) |
| lambdas | 0.357 (5/14) | |
| lists | 0.500 (8/16) | |
| mixins | | 0.429 (3/7) |
| mro | 0.722 (13/18) | |
| objects | | 0.263 (5/19) |
| returns | 0.667 (8/12) | 0.667 (8/12) |

### Why the numbers are low

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

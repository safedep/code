# Project graph report

Two runs of the benchmark and the public sets, both on 2026-10-17 on one
machine. "Before" is the commit that adds this file (1848f7c). "After" is
the commit that adds this second run, after the fixes of the adversarial
review: the `name` and `reference` edges, the records for bare names and
for imports that can be first-party, `super`, module variables, decorator
closures and the budget rule for incomplete files. `README.md` says how to
run them and what each metric means. The run of 10,000 files is from
before only.

Steps 32 and 33 add one more run, on 2026-10-17, at 1,000, 10,000 and
50,000 files per language, with the parallel index step. Its numbers are
the "step 33" rows and the section "Steps 32 and 33". The earlier rows stay
as history.

The review fixes of steps 27 to 30 add one more run, on 2026-10-17, at
1,000 files per language with the default workers, and of the public
sets. Its numbers are the section "Review fixes of steps 27 to 30".

## Machine

| Item | Value |
| --- | --- |
| Cores | 4 (`nproc`) |
| CPU | `Intel(R) Xeon(R) Processor @ 2.10GHz` (`model name` in `/proc/cpuinfo`, a virtual machine) |
| Memory | 15 GiB |
| OS | Linux 6.18, amd64 |
| Go | go1.25.1 linux/amd64 |

Before step 32 the index step was serial. So every cold number before the
step 33 rows uses one core of the four, and the user time of a run is
about its wall time. The step 33 rows use 4 workers.

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
| Java | step 30 | none | n/a | n/a | n/a | 0.148 | 2.76 s | 0.33 s | 0.46 s | 34 ms | 61 / 38 / 57 / 5 MiB |
| Python | step 33 | pycg-micro | 0.954 (145/152) | 0.708 (187/264) | 0.340 (85/250) | 0.159 | 1.57 s | 0.37 s | 0.36 s | 28 ms | 79 / 41 / 41 / 6 MiB |
| JavaScript | step 33 | swarm-js | 0.821 (156/190) | 0.706 (204/289) | 0.358 (110/307) | 0.263 | 1.77 s | 0.42 s | 0.30 s | 31 ms | 80 / 36 / 38 / 7 MiB |
| Python | review fixes | pycg-micro | 0.954 (145/152) | 0.708 (187/264) | 0.340 (85/250) | 0.159 | 1.48 s | n/a | 0.28 s | n/a | n/a |
| JavaScript | review fixes | swarm-js | 0.828 (159/192) | 0.716 (207/289) | 0.345 (106/307) | 0.263 | 1.88 s | n/a | 0.32 s | n/a | n/a |
| TypeScript | step 33 | none | n/a | n/a | n/a | 0.094 | 2.31 s | 0.31 s | 0.27 s | 20 ms | 72 / 33 / 38 / 4 MiB |
| Go | step 33 | govulncheck | n/a | 1.000 (9/9) stacks | n/a | 0.082 | 1.38 s | 0.38 s | 0.21 s | 22 ms | 102 / 43 / 36 / 4 MiB |
| Java | step 33 | none | n/a | n/a | n/a | 0.148 | 1.01 s | 0.39 s | 0.28 s | 25 ms | 76 / 37 / 37 / 5 MiB |
| Kotlin | step 35 | none | n/a | n/a | n/a | 0.283 | 1.27 s | 0.46 s | 0.48 s | 32 ms | 94 / 45 / 45 / 8 MiB |
| C# | step 36 | none | n/a | n/a | n/a | 0.330 | 1.28 s | 0.51 s | 0.92 s | 75 ms | 100 / 49 / 69 / 12 MiB |
| PHP | step 36 | none | n/a | n/a | n/a | 0.093 | 1.02 s | 0.30 s | 0.19 s | 17 ms | 85 / 34 / 36 / 4 MiB |
| Ruby | step 36 | none | n/a | n/a | n/a | 0.459 | 1.12 s | 0.56 s | 0.94 s | 53 ms | 90 / 52 / 48 / 11 MiB |
| Rust | step 36 | none | n/a | n/a | n/a | 0.308 | 1.37 s | 0.32 s | 0.30 s | 27 ms | 80 / 39 / 40 / 6 MiB |

Python is now at 0.940, just under the precision gate of the design (0.95
for `high` and `medium` edges). After step 27 it is 0.954, and its recall is
0.708. JavaScript stays at 0.821 and 0.706 until the review fixes of steps
27 to 30, which take it to 0.828 and 0.716. The "review fixes" rows come
from the run of that section, which measured the cold index step and the
link step only.

The step 33 rows come from the run of steps 32 and 33. The index cold step
uses 4 workers, and its memory holds the 4 parsers and the trees in
flight. The link step does not change its output in step 33, so the
precision, the recall and the unresolved rates stay.

The Go row comes from step 29. The govulncheck set gives call stacks and
no call graph, so it has no precision of edges. Its recall is the share of
the stacks that the reach query finds, and the section on the public sets
has the other numbers. The synthetic Go project of 1,000 files has 32,015
edges and 2,860 unresolved sites, and the reach query finds 199 of 200
targets. Most unresolved sites are calls on a loop variable of an interface
type, as `item.Notify(data)`, which the template has in every method.

The Java row comes from step 30. No public set with a license that allows
a copy ships Java call graphs, so Java has no precision or recall yet. The
synthetic Java project of 1,000 files has 37,534 edges and 4,745
unresolved sites, and the reach query finds 200 of 200 targets. Most
unresolved sites are the records of the calls on a loop variable of an
interface type, as `item.onEvent(data)`, because a class of a dependency
can implement the interface. The others are calls on the value of a call,
as `invoke` in `getClass().getMethod(...).invoke(...)`.

The Kotlin row comes from step 35, with 4 workers. No public set with a
license ships Kotlin call graphs, so Kotlin has no precision or recall. The
synthetic Kotlin project of 1,000 files has the shape of the Java project,
with the static members in a companion object, the listener calls in a
lambda and an extension function per module. It has 32,953 edges and 9,341
unresolved sites, and the reach query finds 200 of 200 targets. The
unresolved rate is higher than the rate of Java, because a call on the
value of a call, as `toString()` in `result.toString().tag00()`, and a call
on a list of the standard library, as `items.forEach`, are records. The
link step is 0.2 s slower per 1,000 files than Java, because each call
with a receiver looks for an extension function of its name.

The C#, PHP, Ruby and Rust rows come from step 36, with 4 workers. No
public set with a license ships call graphs of these languages, so they
have no precision or recall. Each synthetic project of 1,000 files has the
shape of the Java project, and the reach query finds 200 of 200 targets in
each one.

- C# has one namespace per package, a lambda in `items.ForEach` and a
  static `Package` class per namespace. It has 88,864 edges and 11,166
  unresolved sites. It has the most edges, because each method that is not
  static has an edge to its class, and the link step looks for an
  extension method of the name of each call on a receiver that runs no
  method of the project.
- PHP has one namespace per package under a PSR-4 prefix of composer.json
  and a script under bin. It has 32,930 edges and 2,811 unresolved sites,
  the lowest unresolved rate of the four.
- Ruby has one module per package under lib, a require_relative per
  dependency and a script under exe. It has 53,354 edges and 18,563
  unresolved sites. A call on an instance variable, as `@store.m(data)`,
  and on a block parameter, as `item.on_event(data)`, has no known class,
  so it is a record with name edges. The link step was 35 s at first,
  because every file reopens the module Bench, and each lookup of a method
  read the 1,000 aliases of the module. The members of a group of aliases
  are now one table, and the link step is 0.94 s.
- Rust has one module directory per package, whose mod.rs declares its
  modules, a test file per package under tests, and a `Vec<Box<dyn
  Listener>>` per module. It has 29,635 edges and 11,150 unresolved sites.
  Most records are the calls on a value of a call of std, as `unwrap` and
  `to_string`.

Step 36 also keys a narrowed call by its site and its callee. A chain of
calls, as `a.b().c()`, has one site. Before step 36, the narrowed call of
`b` dropped the record of the call of `c`. The synthetic projects of the
earlier languages have the same edges and unresolved sites after the
change.

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

## Review fixes of steps 27 to 30

"Before" is 209b8ad, the last commit of step 33. "After" is the commit
that adds this section. Both ran back to back on the machine below, with
`-benchtime 1x` and one run per step.

### 1,000 files

| Language | Step | Before | After |
| --- | --- | --- | --- |
| Python | index-cold, 4 workers | 1.53 s | 1.48 s |
| Python | link | 0.33 s, 48,566 edges, 113 roots | 0.28 s, 48,566 edges, 114 roots |
| Python | reach | 200 of 200 found | 200 of 200 found |
| JavaScript | index-cold, 4 workers | 1.68 s | 1.88 s |
| JavaScript | link | 0.30 s, 38,462 edges, 39 roots | 0.32 s, 41,348 edges, 150 roots |
| JavaScript | reach | 200 of 200 found | 200 of 200 found |
| TypeScript | index-cold, 4 workers | 2.41 s | 2.48 s |
| TypeScript | link | 0.30 s, 31,486 edges, 39 roots | 0.31 s, 31,598 edges, 150 roots |
| TypeScript | reach | 199 of 200 found | 199 of 200 found |
| Go | index-cold, 4 workers | 1.38 s | 1.50 s |
| Go | link | 0.21 s, 32,015 edges, 76 roots | 0.23 s, 32,015 edges, 76 roots |
| Go | reach | 199 of 200 found | 199 of 200 found |
| Java | index-cold, 4 workers | 1.08 s | 1.11 s |
| Java | link | 0.28 s, 37,534 edges, 75 roots | 0.32 s, 28,284 edges, 75 roots |
| Java | reach | 200 of 200 found | 200 of 200 found |

The serial cold step is within 9% of before for each language, faster
for three of them, so the changes of the 4-worker step are the noise of
one run. The index of a JavaScript file is 0.9 KiB larger as JSON, 12.3
KiB, because of the definitions of the function literals. The other
languages change by less than 0.1 KiB. JavaScript and
TypeScript have about 110 more roots: a function literal that a test file
passes to describe or test is now a definition, so it is a test root.
JavaScript has 2,886 more edges, one per function literal of an argument.
Java has 9,250 fewer edges. Before, each method of a class with an
instance field initializer had a class-init edge to the class. Now only a
class with static code has them, and they go to `<clinit>`.

### Public sets

| Metric | pycg-micro before | pycg-micro after | swarm-js before | swarm-js after |
| --- | --- | --- | --- | --- |
| Cases with the exact pairs | 0.454 (54/119) | 0.454 (54/119) | 0.333 (42/126) | 0.341 (43/126) |
| Precision | 0.703 (187/266) | 0.700 (187/267) | 0.664 (204/307) | 0.670 (207/309) |
| Precision, high and medium | 0.954 (145/152) | 0.954 (145/152) | 0.821 (156/190) | 0.828 (159/192) |
| Recall | 0.708 (187/264) | 0.708 (187/264) | 0.706 (204/289) | 0.716 (207/289) |
| Unresolved rate | 0.340 (85/250) | 0.340 (85/250) | 0.352 (108/307) | 0.345 (106/307) |

- An arrow function in an argument no longer takes the name of the
  declarator above it, so the callback of `const a = list.filter(x => f())`
  is no function `a`. Two `arrays` pairs and one `direct` pair are new.
- A class expression in a class field is a class with the name of the
  field, so `new A.B()` reaches the constructor of `B`. `exceptions` finds
  3 of 3. The `super()` of that class makes one more pair to the builtin
  `Error`, which swarm-js does not list.
- The one new `reference` pair of pycg-micro is an `override` edge.
- The runner maps a function with an anonymous name to the scope that
  defines it, so the function literals of arguments change no pair.

govulncheck stays at 9 of 9 stacks found, 7 of 7 call sites and 9 of 9
stacks in order. The graph makes no call to the symbols that govulncheck
reports as not called, as the govulncheck section counts them.

### The language profiles

The language profiles of projectgraph change no output. The JSON of the
index step and the link step for the 93 hand-written cases, and for a
generated project of 3,000 files per language, has the same sha256 before
and after the change.

| Input | sha256 |
| --- | --- |
| 93 cases | `bbde0029f4876000e6f541362ba97a176c3587676b100f95005c7392d1353c65` |
| Python, 3,000 files | `8894d2d8cea3b808f6b049eedcbcffa224c2829a52f5d84aec9d2d05b9f1f567` |
| JavaScript, 3,000 files | `082794a806da95629ae91e16620293709410550ded7e6d05c72f22fa9eae6618` |
| TypeScript, 3,000 files | `9788a26829c6bd67813b988f3dc32fec36035588f0af1987c78a307d8f726023` |
| Go, 3,000 files | `8ca2bf14a7b76946a96ff307e226509467670bad3ae11c58b7de7cee63decca9` |
| Java, 3,000 files | `2ee9d26b74c1d35cb7ff10248c125637ab207032f866193018015abbee4c8f36` |

## Steps 32 and 33

One run of `PROJECTGRAPH_BENCH_FILES=1000,10000` with the default workers
(1 and 4), one run of `PROJECTGRAPH_BENCH_FILES=50000` with
`PROJECTGRAPH_BENCH_WORKERS=4`, and one run of the `link` step at the three
sizes after the last fix of the link step. The run at 50,000 files took
34 min for the five languages and fit in the 6.7 GB of free disk, because
each language removes its project and its stores. It leaves out the
serial cold step, which would add about 20 min. The index-cold-store step
costs about 8 min of the 34, so a run without it fits in 30 min.

### The parallel index step

`indexer.Options.Workers` sets the workers. Each worker owns a parser.
The goroutine of `Run` takes the outcomes in path order and runs the extra
plugins, so the output of 1 and 4 workers is the same byte for byte. Tests
compare it on the hand-written cases, on a project of two batches, and on
a synthetic project of 600 files per language.

| Language | 1,000, 1 worker | 1,000, 4 workers | Speed-up | 10,000, 1 worker | 10,000, 4 workers | Speed-up | 50,000, 4 workers |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Python | 4.38 s | 1.57 s | 2.79 | 45.7 s | 16.2 s | 2.81 | 90.4 s (1.81 s per 1,000) |
| JavaScript | 4.92 s | 1.77 s | 2.78 | 50.4 s | 17.5 s | 2.87 | 94.1 s (1.88 s per 1,000) |
| TypeScript | 6.99 s | 2.31 s | 3.03 | 74.8 s | 25.0 s | 2.99 | 129.2 s (2.58 s per 1,000) |
| Go | 3.81 s | 1.38 s | 2.76 | 39.0 s | 14.9 s | 2.62 | 79.1 s (1.58 s per 1,000) |
| Java | 2.88 s | 1.01 s | 2.85 | 27.4 s | 10.5 s | 2.62 | 56.3 s (1.13 s per 1,000) |

The speed-up is 2.6 to 3.0 on 4 cores, over the gate of 2. It is not 4,
for these reasons:

- The store, the result and the report have no lock. Only the goroutine of
  `Run` changes them, so no lock waits.
- The calls into tree-sitter through cgo take about 46% of the CPU samples
  of a parallel run, and each call enters and leaves the Go scheduler. The
  callgraph build and the projection read each node through a cgo call.
- The GC takes about 10%. With `GOGC=400` the Python step at 1,000 files
  takes 1.31 s and not 1.46 s, for twice the peak heap.
- The read of the files and the batches of the store run on one goroutine
  between the batches.

The per-file cost at 50,000 files is 3 to 12% over the cost at 10,000
files. The heap of the benchmark holds the indexes and the graph of the
warm store, about 1 GiB, so each GC marks more.

A parse that the deadline cuts is `missing`, with a parse failure in the
report. The fix also resets the sitter parser after a cut parse. Before, a
parser that a deadline cut went on with the old input at its next parse.

### The link step

A CPU profile of the link step at 1,000 and 10,000 Python files
(`-cpuprofile` on `BenchmarkProjectGraph/python/files=10000/link`) showed
these costs:

- The field and name candidates of each unresolved site came from a scan
  of every definition of the method name in the project, then the cap of
  20 dropped the site as ambiguous. A common name has a definition in most
  files, so the scan grew with the square of the project. It was 28 times
  slower at 10 times the files. The scan now stops past the cap.
- `javaFirstParty` scanned every package of the project at each import,
  and the Go resolver with no `go.mod` scanned every package directory.
  Both grew with the square of the project. The Java link step at 50,000
  files took 42.7 s, and now takes 23.2 s. Both now look up the prefixes
  or the suffixes of the name.
- The graph kept each edge three times, as a set of `Edge` values of 128
  bytes, a map from a slot of 64 bytes to an `Edge`, and a set of caller
  and callee pairs. The addition and the sort of the edges took about 38%
  of the step.
  The edge set now interns the strings of the edges and keys a slot by 24
  bytes of ids. It makes the `Edge` values once, at the end, sorted by the
  rank of each string.
- The three call rules of JavaScript each resolved every call again. The
  root finder now resolves a call once.

| Language | Link before, 1,000 | after | Link before, 10,000 | after | after, 50,000 | Growth before, 1,000 to 10,000 | after | after, 10,000 to 50,000 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Python | 0.61 s | 0.36 s | 8.50 s | 4.85 s | 34.8 s | 13.9 | 13.5 | 7.2 |
| JavaScript | 0.55 s | 0.30 s | 8.14 s | 4.31 s | 25.1 s | 14.8 | 14.4 | 5.8 |
| TypeScript | 0.53 s | 0.27 s | 6.81 s | 3.35 s | 23.9 s | 12.8 | 12.4 | 7.1 |
| Go | 0.29 s | 0.21 s | 4.58 s | 2.83 s | 20.7 s | 15.8 | 13.5 | 7.3 |
| Java | 0.41 s | 0.28 s | 6.21 s | 3.53 s | 23.2 s | 15.1 | 12.6 | 6.6 |

"Before" is the commit of step 32 (048565a). The link step is now 1.6 to
2.0 times faster at 10,000 files. It still grows faster than the files, about 13 times for 10
times the files and 7 times for 5 times the files. The number of edges
grows about as the files do. No step of the profile is quadratic now. Each
part of the step is 12 to 20 times slower at 10 times the files, and the
probe of the Go map is 24 times slower. A run with `GOGC=off` grows by the
same factor, so the cause is not the GC. A map of 400,000 or 2,000,000
keys does not fit in the cache of the CPU, so each lookup reads the main
memory. The sorts of the edges and the unresolved sites are n log n. A
further cut needs fewer map lookups per call site, and that is a change of
the resolution code.

### Memory

| Measure | Before (step 32) | After | Where |
| --- | --- | --- | --- |
| Index in memory, Python / JavaScript | 24.2 / 23.1 KiB per file | 14.5 / 13.4 KiB per file | `retained-KiB/file` of index-warm at 10,000 files |
| Index in memory, TypeScript / Go / Java | | 12.3 / 14.1 / 12.2 KiB per file | the same |
| Index as JSON | 10.0 to 13.4 KiB per file | the same | `index-KiB/file` |
| Graph, Python / JavaScript | 200 / 230 bytes per edge | 181 / 213 bytes per edge | `retained-B/edge` at 10,000 files |
| Graph, TypeScript / Go / Java | 200 / 198 / 196 bytes per edge | 183 / 184 / 180 bytes per edge | the same |
| Link peak, Python / JavaScript | 1.70 / 2.33 KiB per edge | 1.16 / 1.19 KiB per edge | peak minus base over the edges at 10,000 files |
| Link peak, TypeScript / Go / Java | 1.97 / 1.46 / 1.29 KiB per edge | 1.14 / 1.10 / 0.99 KiB per edge | the same |

The `retained` metrics are new in step 33. They measure the heap that the
result of a step holds after two GCs, so they leave out the garbage that
the peak counts. The report of step 21 named 30 to 40 KiB per index in
memory and 0.9 KiB per edge. Those were peaks, with garbage.

- An index in memory was twice its JSON size. A JSON decode gives each
  caller key, target and kind of an index its own string, and an append
  leaves up to half of a slice free. The indexer now interns the strings of
  the indexes of a run and fits each record slice to its length. An index
  in memory is now 0.9 to 1.3 times its JSON size. The rest is the size of
  the records: a `Call` is 144 bytes and a `Definition` 248 bytes, with a
  string of 16 bytes for each kind. A smaller record needs a change of the
  types of `fileindex`. The interning makes the warm step 0.1 to 0.4 s
  slower at 10,000 files.
- A graph edge is 128 bytes in `Graph.Edges`, and the node map and the
  unresolved sites hold the rest. The public `Edge` keeps its fields, so
  the graph keeps that size.
- The cold index step still grows with the project, because
  `Result.Indexes` holds every index for the link step. The result is 12 to
  15 MiB per 1,000 files. The peak of the step is higher, because the GC
  lets the heap grow to twice the live heap.

### 10,000 files

The link rows come from the run after the last fix of the link step.

| Language | Step | Time | Base heap | Peak heap | Graph |
| --- | --- | --- | --- | --- | --- |
| Python | index-cold, workers 1 | 45.7 s | 213 MiB | 865 MiB |  |
| Python | index-cold, workers 4 | 16.2 s | 213 MiB | 982 MiB |  |
| Python | index-cold-store | 18.1 s | 213 MiB | 936 MiB |  |
| Python | index-warm | 4.05 s | 213 MiB | 578 MiB |  |
| Python | link | 4.85 s | 213 MiB | 675 MiB | 407,554 edges, 1,115 roots, 65,687 unresolved |
| Python | reach | 0.45 s | 76 MiB | 128 MiB | 200 of 200 found |
| JavaScript | index-cold, workers 1 | 50.4 s | 193 MiB | 804 MiB |  |
| JavaScript | index-cold, workers 4 | 17.5 s | 193 MiB | 964 MiB |  |
| JavaScript | index-cold-store | 19.8 s | 193 MiB | 859 MiB |  |
| JavaScript | index-warm | 3.91 s | 193 MiB | 526 MiB |  |
| JavaScript | link | 4.31 s | 193 MiB | 546 MiB | 302,727 edges, 373 roots, 94,355 unresolved |
| JavaScript | reach | 0.30 s | 68 MiB | 124 MiB | 200 of 200 found |
| TypeScript | index-cold, workers 1 | 74.8 s | 176 MiB | 770 MiB |  |
| TypeScript | index-cold, workers 4 | 25.0 s | 176 MiB | 894 MiB |  |
| TypeScript | index-cold-store | 26.1 s | 176 MiB | 848 MiB |  |
| TypeScript | index-warm | 3.37 s | 176 MiB | 474 MiB |  |
| TypeScript | link | 3.35 s | 176 MiB | 528 MiB | 314,336 edges, 373 roots, 28,450 unresolved |
| TypeScript | reach | 0.28 s | 61 MiB | 94 MiB | 200 of 200 found |
| Go | index-cold, workers 1 | 39.0 s | 195 MiB | 882 MiB |  |
| Go | index-cold, workers 4 | 14.9 s | 195 MiB | 1,156 MiB |  |
| Go | index-cold-store | 16.7 s | 195 MiB | 1,035 MiB |  |
| Go | index-warm | 4.32 s | 195 MiB | 530 MiB |  |
| Go | link | 2.83 s | 195 MiB | 540 MiB | 320,487 edges, 744 roots, 28,522 unresolved |
| Go | reach | 0.31 s | 62 MiB | 98 MiB | 200 of 200 found |
| Java | index-cold, workers 1 | 27.4 s | 184 MiB | 751 MiB |  |
| Java | index-cold, workers 4 | 10.5 s | 184 MiB | 916 MiB |  |
| Java | index-cold-store | 12.1 s | 184 MiB | 845 MiB |  |
| Java | index-warm | 4.13 s | 184 MiB | 522 MiB |  |
| Java | link | 3.53 s | 184 MiB | 546 MiB | 375,828 edges, 743 roots, 47,497 unresolved |
| Java | reach | 0.37 s | 76 MiB | 117 MiB | 200 of 200 found |

### 50,000 files

`PROJECTGRAPH_BENCH_WORKERS=4`. The link row comes from the run after the
last fix of the link step.

| Language | Step | Time | Base heap | Peak heap | Graph |
| --- | --- | --- | --- | --- | --- |
| Python | index-cold, 4 workers | 90.4 s | 1,051 MiB | 4,788 MiB | |
| Python | index-cold-store | 98.3 s | 1,051 MiB | 4,585 MiB | |
| Python | index-warm | 20.9 s | 1,051 MiB | 2,805 MiB | |
| Python | link | 34.8 s | 1,051 MiB | 3,210 MiB | 2,037,238 edges, 5,558 roots, 328,832 unresolved |
| Python | reach | 2.57 s | 370 MiB | 615 MiB | 200 of 200 found |
| JavaScript | index-cold, 4 workers | 94.1 s | 954 MiB | 4,856 MiB | |
| JavaScript | index-cold-store | 105.7 s | 954 MiB | 4,478 MiB | |
| JavaScript | index-warm | 20.3 s | 954 MiB | 2,653 MiB | |
| JavaScript | link | 25.1 s | 953 MiB | 2,966 MiB | 1,514,084 edges, 1,854 roots, 471,968 unresolved |
| JavaScript | reach | 2.58 s | 329 MiB | 606 MiB | 200 of 200 found |
| TypeScript | index-cold, 4 workers | 129.2 s | 868 MiB | 4,573 MiB | |
| TypeScript | index-cold-store | 140.4 s | 868 MiB | 4,206 MiB | |
| TypeScript | index-warm | 18.5 s | 868 MiB | 2,366 MiB | |
| TypeScript | link | 23.9 s | 868 MiB | 2,779 MiB | 1,571,302 edges, 1,854 roots, 142,381 unresolved |
| TypeScript | reach | 2.49 s | 295 MiB | 460 MiB | 200 of 200 found |
| Go | index-cold, 4 workers | 79.1 s | 960 MiB | 5,559 MiB | |
| Go | index-cold-store | 91.8 s | 960 MiB | 5,210 MiB | |
| Go | index-warm | 20.3 s | 960 MiB | 2,650 MiB | |
| Go | link | 20.7 s | 960 MiB | 2,661 MiB | 1,604,044 edges, 3,706 roots, 142,585 unresolved |
| Go | reach | 2.31 s | 301 MiB | 473 MiB | 200 of 200 found |
| Java | index-cold, 4 workers | 56.3 s | 908 MiB | 4,930 MiB | |
| Java | index-cold-store | 67.6 s | 908 MiB | 4,378 MiB | |
| Java | index-warm | 21.1 s | 908 MiB | 2,674 MiB | |
| Java | link | 23.2 s | 908 MiB | 2,739 MiB | 1,880,322 edges, 3,705 roots, 237,682 unresolved |
| Java | reach | 3.28 s | 366 MiB | 569 MiB | 200 of 200 found |

The base heap of the benchmark holds the warm indexes and a graph, about
1 GiB. The cold peak of 4 to 5.5 GiB is that base, the new indexes, and
the garbage that the GC lets grow to twice the live heap. A caller that
holds only one result needs about half of it.

### Degradation at half of the cold time

`TestDegradesAtHalfTheColdTime` indexes 1,000 files per language with 4
workers, then indexes them again with a deadline of half of that time. It
links the partial result and asks the reach query for the 200 targets.

| Language | Cold time | Indexed at half | Missing at half |
| --- | --- | --- | --- |
| Python | 1.47 s | 465 | 535 |
| JavaScript | 1.89 s | 555 | 445 |
| TypeScript | 2.82 s | 613 | 387 |
| Go | 1.49 s | 453 | 547 |
| Java | 1.06 s | 518 | 482 |

Every file is `indexed` or `missing`, and no file fails. Each target that
the partial graph does not find is `uncertain` with a `budget` cause, and
no target is `not-found`. A partial graph can find a path that the whole
graph does not. A site with more than 20 field candidates in the whole
project is ambiguous, and with fewer files it has field edges. So the test
does not compare the found targets with the whole graph.

## Summary step

One run on 2026-10-17, after the final review, of the `summary` step at
1,000 files with 4 workers. The step reads the generated project as one
unit of dependency code and summarizes every exported symbol. The index of
the graph is built once per summary since the review fix. Before the fix
a summary of 3,000 symbols of one file took 26 s in the unit test of
`projectgraph/summary`, and it takes 0.47 s after.

| Language | Symbols | Reaches | Time | Peak heap |
| --- | --- | --- | --- | --- |
| Python | 16,730 | 2,193,241 | 197.5 s | 12,271 MiB |
| JavaScript | 9,474 | 1,487,550 | 104.7 s | 8,932 MiB |
| TypeScript | 9,503 | 1,460,529 | 45.1 s | 5,131 MiB |
| Go | 11,137 | 1,460,695 | 45.3 s | 5,353 MiB |

The generated project calls two or three external packages from every
module and imports one to three other modules, so most symbols reach most
of the 200 external functions. The step searched the graph once per
symbol and kept a path per reached symbol, so its time and its heap grew
with the symbols times what they reach.

### Reach sets per component

Round C of the final fixes stores reach sets and no path. The summary
computes the sets once per strongly connected component of the unit
graph, in reverse topological order, so each component unions the sets of
its successors once, and a component whose set is the set of one
successor shares its storage. Two symbols that reach one set share the
reached slice. A caller that needs a path asks `Result.Path` for one
search. The uncertainties of a symbol are indexes into one table, with the
targets that each scope concerns, so `Summary.Status` gives the answer of
a reach query with no graph. `summary.Version` is 2.

Two runs of the `summary` step at 1,000 files with 4 workers on
2026-10-18, before and after the change, on the machine of this report.
The reach counts are the same, so the sets are the same.

| Language | Symbols | Reaches | Time before | Time after | Peak heap before | Peak heap after |
| --- | --- | --- | --- | --- | --- | --- |
| Python | 16,730 | 2,193,241 | 182.5 s | 5.9 s | 11,843 MiB | 435 MiB |
| JavaScript | 9,474 | 1,487,550 | 128.7 s | 3.2 s | 8,716 MiB | 338 MiB |
| TypeScript | 9,503 | 1,460,529 | 44.9 s | 2.1 s | 4,997 MiB | 165 MiB |
| Go | 11,137 | 1,460,695 | 48.7 s | 2.5 s | 4,978 MiB | 180 MiB |

Python is under the target of 10 s and 1 GiB. Most of the time after the
change goes to the 2.2 million reached keys of the output, which the
format keeps as strings.

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
cores. Step 33 measures every step on 4 workers at the three sizes. Each
cell is the lowest and the highest number of the five languages.

| Step | Target, 1,000 | Measured, 1,000 | Target, 10,000 | Measured, 10,000 | Target, 50,000 | Measured, 50,000 | Inside the target |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Index, cold | 3 s | 1.0 to 2.3 s | 25 s | 10.5 to 25.0 s | 120 s | 56 to 129 s | Yes, but TypeScript at 50,000 |
| Index, warm | 0.2 s | 0.31 to 0.42 s | 1.5 s | 3.4 to 4.3 s | 8 s | 18.5 to 21.1 s | No |
| Link | 0.3 s | 0.21 to 0.36 s | 2 s | 2.8 to 4.9 s | 12 s | 20.7 to 34.8 s | At 1,000 only, but Python |
| Reach, 200 targets | 50 ms | 20 to 31 ms | 0.5 s | 0.28 to 0.45 s | 3 s | 2.3 to 3.3 s | Yes, but Java at 50,000 |
| Memory, index cold | 50 MiB per worker, 4 workers | 72 to 102 MiB | | 718 to 961 MiB | | 3.6 to 4.5 GiB, over a base of 1 GiB | At 1,000 only |
| Memory, index records | about 4 KiB per file | 12.2 to 14.5 KiB per file in memory, 10.0 to 13.4 KiB as JSON | | the same | | the same | No |
| Memory, link | about 1 KiB per 10 edges, under 1 GiB at 50,000 | 165 to 186 bytes per edge held, 0.9 to 1.2 KiB per edge at the peak | | 180 to 213 bytes held, 1.0 to 1.2 KiB peak | | 176 to 208 bytes held (270 to 345 MiB), 1.0 to 1.4 KiB peak (1.7 to 2.1 GiB) | The graph, yes. The peak, no |

What the numbers of step 33 say:

- The cold index step is inside its target at 1,000 and 10,000 files. At
  50,000 files TypeScript takes 129 s for a target of 120 s.
- The warm step decodes about 11 KiB of JSON per file on one goroutine,
  at about 0.4 s per 1,000 files. A decode on the workers would cut it.
- The link step costs about 0.2 to 0.7 s per 1,000 files, and the cost per
  file grows with the project. The section on the link step names the
  cause.
- The graph that the link step returns is under 1 GiB at 50,000 files.
  The peak of the step is 6 to 7 times the graph.
- The cold index memory grows with the project, because the result holds
  every index.

Before step 32, the measured index step used one core. This table holds
the numbers from before. After, the link step of 1,000 files is 0.23 to 0.33 s, at the
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

What the numbers said before step 32:

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
| Distinct symbols that govulncheck reports as not called, in a package that the module calls into, and the graph finds | 0.000 (0/43) |

The last row counts each symbol once, and only in a package that a file
of the module calls into, because a symbol of any other package cannot be
a node of the graph. Before this round the row counted 63 mentions of
symbols over every package of the OSV entries. A found symbol here would
be a call that the link step resolves to the wrong name.

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

## Method targets

This section measures how often a target of a vulnerable function or
method of a dependency ends uncertain on real projects. `README.md` says
how to run it. The "before" run is from the commit that adds this
section, on 2026-10-19, on the machine above.

### The corpus

`public/methods/corpus.txt` lists 10 Go projects and 10 Rust projects.
Each one depends on a module or a crate with function or method symbols
in an advisory. `public/methods/advisories.json` holds the advisories of
these dependencies: the Go vulndb (`vuln.go.dev`) and RustSec through
OSV, as of 2026-10-19.

| Language | Projects | Dependencies with an advisory |
| --- | --- | --- |
| Go | bluemonday v1.0.16, goquery v1.8.0, colly v2.1.0, hey v0.1.4, melody v1.1.1, scany v2.0.0, fiber v2.40.0, sftp v1.13.5, go-grpc-middleware v1.3.0, go-rabbitmq v0.12.1 | golang.org/x/net, golang.org/x/crypto, golang.org/x/text, golang.org/x/sys, golang.org/x/oauth2, github.com/antchfx/xpath, github.com/antchfx/xmlquery, google.golang.org/protobuf, google.golang.org/grpc, github.com/gorilla/websocket, github.com/jackc/pgx/v5, github.com/valyala/fasthttp, github.com/sirupsen/logrus, github.com/rabbitmq/amqp091-go |
| Rust | postgres 0.19.7, parking_lot 0.11.0, native-tls 0.2.10, cookie 0.14.4, imageproc 0.21.0, cssparser 0.27.2, selectors 0.22.0, tokio-postgres 0.7.7, lettre_email 0.9.4, rusqlite_migration 0.5.0 | tokio, tokio-postgres, postgres-protocol, lock_api, openssl, time, rand, image, rulinalg, smallvec, lettre, rusqlite |

A Go dependency has the version that the go.mod of the project names. A
Rust dependency has the newest version that the requirement of the
Cargo.toml admits and that an advisory affects, as a lock file at a
vulnerable version would pin it. The code of each such dependency is a
unit of the link step. Every symbol of an advisory that affects the
version of a dependency is a target. A Go advisory with no symbol list
covers its whole package and gives no target. A Rust name with type
arguments is no key and gives no target. A Rust name whose last segment
starts with an upper case letter is a type, and the table counts it as
`other`.

The test runs no network call and the corpus is not in the repository,
so a run needs `scripts/fetch_method_corpus.sh` first. The real Go
standard library is not in the corpus, so the measure has no target of
the standard library.

### What the columns mean

A target ends as `found`, `not-found`, `no-such-target` or `uncertain`.
An uncertain target has one cause. When the search from the roots
reaches an unresolved call whose method name is the last segment of the
target, the cause is the reason of that call, and `unknown-receiver`
wins over another reason. Else the cause is the first uncertainty of the
result, as `budget`, `ambiguous` or `computed-name`. A library project
has no main, so every exported symbol is a root.

### Results

The "before" rows come from the commit of FOU-542, before any fix of this
measure. The "FOU-543" rows come from the commit of FOU-543, which also
holds the fixes of FOU-545, FOU-546 and FOU-547 before it. The "FOU-544"
rows come from the commit of FOU-544. The Rust rows of FOU-543 and the Go
function row of FOU-544 do not change, so the table leaves them out.

| Language | Kind | Run | Targets | found | not-found | no-such-target | unknown-receiver | no-definition | interface-receiver | no-dependency-source | budget | ambiguous | computed-name |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Go | function | before | 256 | 13 | 0 | 7 | 3 | 1 | 0 | 0 | 145 | 53 | 34 |
| Go | function | FOU-543 | 256 | 13 | 0 | 7 | 3 | 0 | 0 | 3 | 39 | 132 | 59 |
| Go | method | before | 1298 | 29 | 2 | 15 | 294 | 119 | 0 | 0 | 576 | 158 | 105 |
| Go | method | FOU-543 | 1298 | 30 | 2 | 15 | 319 | 112 | 3 | 2 | 123 | 459 | 233 |
| Go | method | FOU-544 | 1298 | 30 | 2 | 15 | 320 | 112 | 3 | 2 | 123 | 459 | 232 |
| Rust | function | before | 13 | 0 | 0 | 10 | 3 | 0 | 0 | 0 | 0 | 0 | 0 |
| Rust | function | FOU-544 | 13 | 1 | 0 | 5 | 0 | 5 | 0 | 0 | 0 | 2 | 0 |
| Rust | method | before | 38 | 0 | 1 | 19 | 13 | 5 | 0 | 0 | 0 | 0 | 0 |
| Rust | method | FOU-544 | 38 | 0 | 0 | 7 | 11 | 11 | 0 | 0 | 0 | 9 | 0 |
| Rust | other | before | 5 | 0 | 0 | 5 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| Rust | other | FOU-544 | 5 | 0 | 0 | 5 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

| Language | Run | Method targets that end as unknown-receiver |
| --- | --- | --- |
| Go | before | 0.227 (294/1298) |
| Go | FOU-543 | 0.246 (319/1298) |
| Go | FOU-544 | 0.247 (320/1298) |
| Rust | before | 0.342 (13/38) |
| Rust | FOU-544 | 0.289 (11/38) |

Before:

- Go: `budget` is the largest cause. The index of each
  `idna/tables*.go` of x/net hits a limit, so the file is truncated. Each
  `gen.go` of x/net has `//go:build ignore` and `package main`, so the
  link step records a truncation for it. A `_test.go` file of x/crypto is
  truncated too. The budget of a file of a unit concerns every target
  once the search enters the unit. hey has 47 truncations and scany 55.
- Go: an `ambiguous` or a `computed-name` call concerns every target.
  These calls come from the code of a dependency unit, as pgx and
  amqp091-go, and from the project, as fiber and go-grpc-middleware.
- Rust: the link step does not link the code of a `cargo:` unit, so the
  graph holds no edge into the crate. A target that no project file names
  is `no-such-target`.

After FOU-543:

- Go: no project has a truncation of the link step. The graphs of the 10
  Go projects hold 5,286 files, 68% of the 7,810 of before, because the
  index step leaves out the test files and the files that no Go build
  reads by default. It keeps the files of every platform. `budget`
  falls from 576 to 123 method targets. The rest is scany: two
  `tables.go` files of x/text are over the size limit of the index step,
  so they fail.
- Go: with less budget, the first uncertainty of a result is more often
  an `ambiguous` or a `computed-name` call of the code of a unit, which
  concerns every target. So these columns grow, and the share of
  `unknown-receiver` grows a little. The search reaches more code of the
  units through the new edges, and more call sites with an unknown
  receiver.
- Go: `found` moves from 29 to 30 method targets. Most symbols of an
  advisory are methods that the code of the dependency calls inside its
  own module, as the methods of the HTTP/2 server of x/net. A call inside
  a unit goes to the definition of the method and not to its external key,
  so the search reaches the definition and never meets the target, which
  is the external key. The target reads as uncertain when an unresolved
  call concerns it, and as not-found when a call of the project that no
  root reaches names it. FOU-555 fixes the match. The receiver forms of FOU-543 make the
  calls of the project find the method, as the receiver table and the
  cases show. They do not change how a target matches code inside a unit.
- The measure has no target of the standard library. The Go vulndb has
  the most method symbols for it, and FOU-543 adds `GOROOT/src` as a unit,
  but the corpus pins no Go release.

After FOU-544:

- Rust: the link step links the code of each crate. The 10 Rust graphs
  hold 1,389 files, against 193 of before, and no project has a problem
  of a crate that the link step does not link.
- Rust: `no-such-target` falls from 19 to 7 method targets and from 10 to
  5 function targets, because the graph now holds the keys of the crates.
  lettre_email finds one function target. The other targets end
  uncertain, with an unknown receiver, a call of a name with no
  definition, or an `ambiguous` call that the search reaches. As for Go,
  a target that only the code of its own crate calls reads as uncertain,
  because a call inside a unit goes to the definition and not to the
  external key.
- Rust: the five `other` targets of parking_lot name types of lock_api,
  as `lock_api::MappedMutexGuard`. A type has no call, so they stay
  `no-such-target`.
- Go: the rows move by one target between two causes, because a result
  type of an external walk is now always its external key.

After the review fixes of FOU-544 and FOU-547, the measure gives the same
counts as the FOU-544 rows. The index now keeps the files of every
platform and the bodies of generated files, which changes the graphs and
not the first uncertainty of any result.

The `unknown-receiver` share counts a target when any reached site has an
unresolved receiver with the method name of the target, also a site
inside a unit. More reached code of the units gives more name matches on
`Next`, `Read` or `ServeHTTP`. So the share rises as the search reaches
more code, and it is not a measure of the receiver forms. The receiver
table of FOU-542 measures those, and every row of it is found.

After FOU-555:

| Language | Kind | Targets | found | not-found | no-such-target | unknown-receiver | no-definition | interface-receiver | no-dependency-source | budget | ambiguous | computed-name |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Go | function | 256 | 51 | 0 | 0 | 3 | 5 | 0 | 2 | 39 | 106 | 50 |
| Go | method | 1298 | 331 | 0 | 0 | 154 | 71 | 2 | 2 | 109 | 416 | 213 |
| Rust | function | 13 | 4 | 0 | 0 | 0 | 9 | 0 | 0 | 0 | 0 | 0 |
| Rust | method | 38 | 16 | 0 | 0 | 0 | 13 | 0 | 0 | 9 | 0 | 0 |
| Rust | other | 5 | 0 | 0 | 5 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

- A target now matches each definition of a unit whose external key is the
  target, so a target that only the code of its unit calls is found
  through the definition. `found` moves from 30 to 331 Go method targets,
  from 13 to 51 Go function targets, and from 0 to 16 Rust method targets.
  The share of Go method targets that end as `unknown-receiver` falls from
  0.247 to 0.119, and no target is `no-such-target` any more for Go.
- The match first gave 8 Rust targets as `not-found`. cookie calls
  `rand::thread_rng` from its public `Key::generate`, but its `build.rs`
  was the only root, so the library-surface roots were off. A build
  script, an example or a benchmark of a Rust crate is now a helper
  program. Its root stays, and it does not turn off the library-surface
  roots. No target of the measure is `not-found` now.
- cookie reaches `rand::thread_rng` through the code of rand, not through
  its own call `crate::secure::rand::thread_rng()`. The link step does not
  read `extern crate rand;` in a submodule. The path is longer, and the
  answer is the same.


After the fixes for the review of PR 53, the measure gives the same
numbers. No project of the corpus renames a Cargo dependency, and no
function of a dependency has two platform twins with two result types.

### Python

The Python rows of the corpus come from the first step of the Python
plan, on 2026-10-24, on the machine above, at the commit that adds them.
No fix of the Python link step is in this run. The numbers are the
baseline that the later steps of the plan must move.

| Project | Dependencies with an advisory |
| --- | --- |
| mkdocs 1.1.2 | PyYAML 5.3.1, Jinja2 2.11.3, tornado 6.3.1 |
| httpie 3.2.1 | requests 2.28.2, urllib3 1.26.14, idna 3.6 |
| requests-toolbelt 0.9.1 | requests 2.30.0, urllib3 1.26.16 |
| flask-cors 3.0.10 | Flask 2.2.4, Werkzeug 2.2.2 |
| fabric 2.7.1 | paramiko 2.9.2 |
| imageio 2.9.0 | Pillow 9.2.0 |
| pyopenssl 23.1.0 | cryptography 40.0.2 |
| python-docx 0.8.11 | lxml 4.9.0 |
| django-filter 23.1 | Django 4.2.0 |
| django-debug-toolbar 3.8.1 | Django 4.1.7, sqlparse 0.4.3 |

A Python dependency has a version that the requirement of the project
admits and that an advisory affects, as the Rust rows do. OSV lists no
symbols for a PyPI advisory, so `scripts/pypi_advisories.py` holds the
symbols of 72 advisories of the 14 packages by hand, as the text of each
advisory names them, with the ranges from OSV. The code of a dependency is
its wheel, unpacked, so the unit has the layout of site-packages. A
dependency whose code is compiled, as lxml, has only its `.so` files and
its Python wrappers in the unit.

The receiver table of `receivers_test.go` has 18 Python rows. 2 rows give
their answer today: a call on an instance from a constructor is `found`,
and a call on an untyped parameter is `uncertain/unknown-receiver`. The
other 16 wait on the type-hint step of the plan. 8 of them are
`uncertain/unknown-receiver` today, 2 are `uncertain/no-definition`, and 6
are `no-such-target`: a call on a variable that holds the result of an
external call, as `t = dep.new_thing()` then `t.do()`, gets the false key
`dep//new_thing//do` and no record, so the method of the real type is
`no-such-target`. A Python definition of a unit has no external key either,
because the Python profile has no `symbol` rule, so a target that only the
code of its unit calls cannot match a definition.

| Language | Kind | Run | Targets | found | not-found | no-such-target | unknown-receiver | no-definition | dynamic-flag | budget |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Python | function | every language | 44 | 5 | 0 | 0 | 3 | 1 | 6 | 29 |
| Python | method | every language | 63 | 0 | 0 | 0 | 27 | 5 | 13 | 18 |
| Python | function | python alone | 44 | 5 | 0 | 0 | 3 | 13 | 14 | 9 |
| Python | method | python alone | 63 | 0 | 0 | 0 | 27 | 14 | 17 | 5 |

| Language | Run | Method targets that end as unknown-receiver |
| --- | --- | --- |
| Python | every language | 0.429 (27/63) |
| Python | python alone | 0.429 (27/63) |

The "every language" run indexes each language, as the Go and Rust rows
do. The "python alone" run sets `PROJECTGRAPH_METHOD_LANGUAGES=python`.
The 10 Python graphs hold 3,170 files in the first run and 2,956 in the
second.

- No target is `not-found` and no target is `no-such-target`. Every
  search meets an uncertainty that concerns every target, so the measure
  cannot yet tell a target that the code reaches from one that it does
  not. The 5 found function targets are `yaml.load` in mkdocs,
  `requests.utils.get_netrc_auth` and `idna.encode` in httpie, and
  `sqlparse.parse` and `sqlparse.format` in django-debug-toolbar, each a
  call of the project on an import.
- `budget` is the largest cause of the first run, and most of it is
  JavaScript. A Python project ships the JavaScript of its web pages, and
  the index step truncates a minified file: `jquery-1.10.2.min.js`,
  `bootstrap.min.js` and `lunr.js` in the themes of mkdocs, and
  `jquery.js`, `jquery.min.js` and `select2.full.min.js` in the admin of
  the Django unit. A truncated file of the project concerns every search,
  and one of a unit concerns every search that enters the unit, so 32 of
  the 32 Django targets of django-filter end as `budget`. No Python program
  runs that JavaScript. The run of `python` alone has 14 budget targets
  left: `_tifffile.py` of imageio, a module of 10,182 lines and 360 KiB
  over a limit of the index, and the test files `tests/test_ssl.py` of pyopenssl and
  `tests/opc/unitdata/rels.py` and `tests/oxml/test_xmlchemy.py` of
  python-docx. The test paths mark them as test files, and a truncated
  test file still concerns every search. A Werkzeug file,
  `werkzeug/serving.py`, is truncated too.
- `dynamic-flag` is the next cause, and `decorator-wrap` is most of it.
  The index flags a decorator that is not a bare name, as
  `@click.option(...)`, `@app.route(...)` and `@functools.wraps(f)`,
  because the decorator can wrap the function in code that the graph does
  not see. Every click, Flask or Django project has such a decorator on
  the path from its roots, so the flag concerns every target of the
  project. The other flags are `monkey-patch` in httpie, python-docx and
  the Django unit, `reflection` in fabric and `computed-attribute` in
  httpie. The `computed-name` records of `click.option(...)` come from the
  same decorators: the call of a call result has no name.
- `no-definition` is the first cause when no JavaScript is in the graph: a
  call of a name that no file defines, as `http.client` in httpie and
  `.packages.six.moves.http_client` in urllib3, both a module of the
  standard library or a vendored copy that the graph has no unit for. The
  Python standard library has no unit in this run.
- `unknown-receiver` decides 27 of the 63 method targets, and the share is
  the same in both runs. The receiver forms that the rows name are the
  cause: a typed parameter, a typed attribute, a function result, and a
  call on a value that a `for` loop or a comprehension binds.
- fabric has no root of its own. It is a library, so every exported symbol
  is a root, and the `root-fallback` uncertainty is on each result.
- lxml gives no target of its own kind. Its `iterwalk`, `canonicalize` and
  `iterparse` are compiled, and the unit holds no Python definition for
  them, so each one ends in the uncertainty of the project.

The numbers say what the plan must change first. The type-hint step moves
`unknown-receiver`. Before it can show, a Python project needs three
rules of the index and the link step: a JavaScript file that a Python
project or a pypi unit ships for a web page is no code of the program, a
truncated test file concerns only a search that counts the tests, and a
decorator that a framework rule knows, as a route or a command, wraps the
function in code that the rule already reads.

#### After the four rules of the budget, the decorators and the imports

The commit after the baseline adds four rules. `docs/projectgraph.md`
holds each one.

1. An incomplete file counts for a search only when the program runs its
   language: a root other than a test root is in a file of that language,
   or the ecosystem of its unit installs that language. `Graph.Runs`
   decides it, from `Node.Language` and `Stats.Languages`.
2. An incomplete test file counts only for a search that includes the
   tests.
3. A decorator is a dynamic feature only when the graph cannot name its
   code, as `@handlers[name]`. A decorator that is a name, a dotted name
   or a call of one is a call of the scope that defines the function. A
   call of the value of a call, as `click.option("-v")(f)`, is no record
   of its own, unless the inner call reflects.
4. A Python import whose first segment is no module under an import root
   or in the directory of the importer is external, not a record. Before,
   `import types` in cryptography was a record because a `types.py` is
   deep in a package of it, and `from collections import defaultdict` in
   Django for the same reason. Each such record concerned every target.

The run of the two languages gives the same numbers as the run of
`python` alone now, so the table has one row per kind.

| Language | Kind | Run | Targets | found | not-found | no-such-target | unknown-receiver | no-definition | ambiguous | computed-name | dynamic-flag | budget |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Python | function | four rules | 44 | 5 | 0 | 0 | 3 | 2 | 14 | 8 | 11 | 1 |
| Python | method | four rules | 63 | 0 | 0 | 0 | 27 | 6 | 14 | 4 | 8 | 4 |

- `budget` falls from 47 to 5 targets, all of imageio: `_tifffile.py` is a
  module of 10,182 lines that the index truncates, and it is Python code
  that the program runs.
- `dynamic-flag` falls from 19 to 19 targets in the first run and from 31
  to 19 in the run of `python` alone. What is left is real: `getattr` on a
  computed name in flask-cors, fabric and pyopenssl, and the monkey patch
  of python-docx.
- `no-definition` falls from 31 to 8 in the run of `python` alone. The
  rest is `.packages.six.moves.http_client` in urllib3, a module that six
  makes at run time, and three imports of imageio.
- `ambiguous` and `computed-name` are the first cause now that nothing
  hides them. Django has `template.render` on an unknown receiver with
  more than 20 candidate methods, and a definition named `render` reaches
  most of Django, so the record concerns every target of the corpus that
  Django code reaches. mkdocs has `config['plugins'].run_event`, a call on
  a subscript, which computes its name.
- `found` stays at 5 and `unknown-receiver` at 27 of 63 methods. The
  rules remove uncertainty that hid these causes. They add no path, as
  the type-hint step will.
- pycg-micro keeps its precision of 0.954 and its recall of 0.708. Its
  unresolved rate falls from 0.340 to 0.312, because an import whose first
  segment names a file deep in a case is external now. govulncheck stays
  at 9 of 9 stacks.
- The receiver table does not change. The 16 Python rows still wait on
  the type-hint step.

#### After the distribution reader, the compiled-module record and the external keys

The commit after the four rules adds the rest of step 2 of the plan.

1. The index step leaves out the test files and the wheel metadata of a
   pypi unit, as it leaves out the test files of a Go unit.
2. The link step reads the `.dist-info` at the root of a pypi unit: the
   import names of `top_level.txt` or `RECORD`, and the extension modules
   of `RECORD`. An import of an extension module is a record with the
   reason `no-dependency-source`, because the graph cannot read its code.
3. A definition of a pypi unit carries the external key that an importer
   of its module writes, as `requests//api//get`, through the `symbol`
   rule that Go and Rust had. A target that only the code of its package
   calls matches the definition.
4. `outcome` of the measure and of the receiver rows now says `found/low`
   for a path with a hop of low confidence, as a `name` edge or a
   `reference` edge of the override rule, and `found` only for a path
   that the calls and the declared types prove.

| Language | Kind | Run | Targets | found | found/low | not-found | no-such-target | unknown-receiver | no-definition | ambiguous | computed-name | dynamic-flag | budget |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Python | function | step 2 | 44 | 1 | 18 | 0 | 0 | 0 | 0 | 10 | 4 | 10 | 1 |
| Python | method | step 2 | 63 | 0 | 36 | 0 | 0 | 4 | 1 | 11 | 2 | 6 | 3 |

| Language | Run | Method targets that end as unknown-receiver |
| --- | --- | --- |
| Python | step 2 | 0.063 (4/63) |

- The external keys move 54 targets from uncertain to `found/low`. As for
  Go after FOU-555, most symbols of an advisory are methods that the code
  of the dependency calls inside its own package, and the match on the
  definition finds them. Each of these paths has a `name` edge or a
  `reference` edge. The `name` edges come from calls on unknown receivers
  inside the unit, as `self.request.GET.get` in Django, which the type
  hints of step 3 and the instance attributes of the unit must narrow
  before a path reads as `found`.
- `unknown-receiver` falls from 27 to 4 method targets, because a reached
  record with the method name of the target no longer decides the answer
  when a path reaches the definition. The 4 that stay are requests
  methods in httpie and requests-toolbelt whose paths the search does not
  find.
- The 10 Python receiver rows that reach their method today reach it as
  `found/low`: the override rule joins the class `Thing` to its `do`, so
  the constructor call in the unit reaches `do`. The rows keep their gap,
  because they want the member edge of the declared type.
- `no-such-target` stays at 0, and `not-found` at 0. No project of the
  corpus has a dependency it never imports.
- pycg-micro keeps 0.954 precision and 0.708 recall, with the unresolved
  rate at 0.312. govulncheck stays at 9 of 9 stacks.

#### After the type hints

The commit after step 2 adds step 3 of the plan: the index reads the
type hints of Python, and the link step trusts them as it trusts
TypeScript types. `docs/projectgraph.md` holds the rules.

1. The index records the class of a hint of a parameter, with
   `Optional`, `Union` and `| None` unwrapped, the return type and the
   tuple positions of a function, an annotated local, a name that holds
   the result of a call, a typed name of the top level of a module, and
   the fields of a class from its annotations and from the attributes
   that its methods set.
2. A call on a typed name gets the key of the name in its scope as its
   target, as the Go index does, so the link step reads the type. A call
   that the call graph bound to a method of a class of the file keeps
   that target. A result of a reflective call, as `import_module`, has
   no type.
3. A hint that names a dependency class gives the external key of the
   method. The class hierarchy analysis adds the overrides in the project
   and in the unit, and an abstract method with no override is a record
   with the reason `interface-receiver`.

| Language | Kind | Run | Targets | found | found/low | not-found | no-such-target | unknown-receiver | no-definition | ambiguous | computed-name | dynamic-flag | budget |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Python | function | step 3 | 44 | 2 | 17 | 0 | 0 | 0 | 0 | 10 | 4 | 10 | 1 |
| Python | method | step 3 | 63 | 0 | 35 | 0 | 0 | 4 | 1 | 12 | 2 | 6 | 3 |

| Language | Run | Method targets that end as unknown-receiver |
| --- | --- | --- |
| Python | step 3 | 0.063 (4/63) |

- All 16 Python receiver rows that waited on this step give their wanted
  answer: 14 are `found` with a `member` edge of high confidence or a
  `cha` edge of medium confidence, the abstract base with no
  implementation is `uncertain/interface-receiver`, and the external
  result with no dependency code is `uncertain/no-dependency-source`.
  The rows have no gap left.
- The corpus moves two targets. `requests.utils.get_netrc_auth` in httpie
  goes from `found/low` to `found`. `MultiPartParser.parse` in
  django-filter goes from `found/low` to `uncertain/ambiguous`: the path
  before went through `parser.parse()` in `django/template/base.py`,
  where the call graph did not bind `parser` to the `Parser(...)` call
  over several lines, so a `name` edge joined the site to every `parse`
  method of Django. The hint-free assignment now types `parser` as the
  template `Parser`, and the false path is gone.
- The other numbers do not move, and the reason is the corpus, not the
  rules. The paths of the Python targets run through requests, Django
  4.2 and urllib3 1.26, and that code has no type hints on the receivers
  that the paths cross: `conn.urlopen(...)` in `requests/adapters.py`
  binds `conn` to the result of an untyped method, and Django reads
  `self.request` with no annotation. A project of the corpus that types
  its own code, as httpie, calls these dependencies through their
  untyped API. The receiver rows and the six cases of this step prove the
  mechanism, and the corpus shows how much of the Python ecosystem of
  2023 the mechanism can read: little, until a dependency ships hints.
- The 4 `unknown-receiver` method targets of httpie and requests-toolbelt
  are `urlopen` of urllib3, which requests calls on `conn`, a result of
  `self.get_connection_with_tls_context(...)` with no hint.
- pycg-micro keeps 0.954 precision and 0.708 recall, with the unresolved
  rate at 0.312. Its cases have no type hints. govulncheck stays at 9 of 9
  stacks.
- `PROJECTGRAPH_METHOD_DETAIL` now writes one line per target with its
  outcome and the sites of its path or of its uncertainties, so the next
  step can read the cause of each number.

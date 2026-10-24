# Code
SafeDep Code Analysis Framework

[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/safedep/code/badge)](https://scorecard.dev/viewer/?uri=github.com/safedep/code)

> [!WARNING]
> Code Analysis Framework (CAF) is under active development. Contracts and APIs are subject to change.

See [docs/ADR.md](./docs/ADR.md) for architectural decisions.

See [docs/detection.md](./docs/detection.md) for the APIs that security detections use.

See [docs/projectgraph.md](./docs/projectgraph.md) for the project graph: the call graph of a
whole project across its files and into its dependencies, with the index, link, roots, reach and
summary steps.

See [docs/modgraph.md](./docs/modgraph.md) for the module graph of a Go module: which module
imports which, from the code on the disk, with no Go toolchain.

See [docs/distgraph.md](./docs/distgraph.md) for the distribution graph of a Python project: which
installed distribution imports which, from the code on the disk, with no Python.

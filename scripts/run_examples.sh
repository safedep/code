#!/bin/bash
set -euo pipefail

go run examples/walker/main.go -dir ./examples/fixtures
go run examples/resolver/main.go -dir ./examples/fixtures
go run examples/plugin/main.go -dir ./examples/fixtures
go -C examples/astdb build ./...
go run ./examples/projectgraph -dir test/projectgraph/cases/python/package-init-reexport/project -target yaml//load

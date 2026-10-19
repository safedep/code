#!/bin/bash
# Fetch the projects and the dependencies of the method measure into the
# directory of the first argument. Go modules come from the module proxy
# into <dir>/gomod, and crates from crates.io into <dir>/crates. The list
# is test/projectgraph/public/methods/corpus.txt. Point
# PROJECTGRAPH_METHOD_CORPUS at the directory to measure them.
set -euo pipefail

dir="${1:?usage: fetch_method_corpus.sh <dir>}"
list="$(dirname "$0")/../test/projectgraph/public/methods/corpus.txt"
mkdir -p "$dir/gomod" "$dir/crates"
dir="$(cd "$dir" && pwd)"

grep -v '^#' "$list" | while read -r language _ project deps; do
	for item in $project $deps; do
		case "$language" in
		go)
			GOFLAGS=-modcacherw GOMODCACHE="$dir/gomod" go mod download "$item"
			;;
		rust)
			name="${item%@*}"
			version="${item#*@}"
			if [ ! -d "$dir/crates/$name-$version" ]; then
				curl -sSfL "https://static.crates.io/crates/$name/$name-$version.crate" | tar -xz -C "$dir/crates"
			fi
			;;
		esac
	done
done
echo "$dir"

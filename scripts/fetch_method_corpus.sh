#!/bin/bash
# Fetch the projects and the dependencies of the method measure into the
# directory of the first argument. Go modules come from the module proxy
# into <dir>/gomod, crates from crates.io into <dir>/crates, and PyPI
# packages from pypi.org into <dir>/pypi/<name>-<version>. Each Go project
# also gets the src tree of the Go release of the go line of its go.mod
# into <dir>/gostd/<release>/src. The file
# <dir>/gostd/releases.txt gives the release of each Go project. A dependency
# comes as a wheel, unpacked, so its directory has the layout of
# site-packages with the .dist-info of the package. A project comes as its
# sdist, so its directory has the manifests. The list is
# test/projectgraph/public/methods/corpus.txt. Point
# PROJECTGRAPH_METHOD_CORPUS at the directory to measure them.
set -euo pipefail

dir="${1:?usage: fetch_method_corpus.sh <dir>}"
list="$(cd "$(dirname "$0")/.." && pwd)/test/projectgraph/public/methods/corpus.txt"
mkdir -p "$dir/gomod" "$dir/crates" "$dir/pypi" "$dir/gostd"
dir="$(cd "$dir" && pwd)"
: >"$dir/gostd/releases.txt"

# The go commands fill the module cache of the corpus. They run outside of
# any module, with the local toolchain and no workspace, so no go.mod or
# go.work of the caller changes what they fetch.
cd "$dir"
export GOMODCACHE="$dir/gomod" GOFLAGS=-modcacherw GOTOOLCHAIN=local GOWORK=off

# pypi_url prints the download URL of a package version: the sdist for
# the project, and for a dependency a pure wheel, else a manylinux x86_64
# wheel, else any wheel, else the sdist.
pypi_url() {
	local name="$1" version="$2" kind="$3"
	curl -sSfL "https://pypi.org/pypi/$name/$version/json" | jq -r --arg kind "$kind" '
		.urls as $urls
		| ($urls | map(select(.packagetype == "sdist")) | .[0].url) as $sdist
		| ($urls | map(select(.packagetype == "bdist_wheel"))) as $wheels
		| if $kind == "project" then $sdist
		  else (
			($wheels | map(select(.filename | test("none-any[.]whl$"))) | .[0].url)
			// ($wheels | map(select(.filename | test("manylinux.*x86_64"))) | sort_by(.filename) | reverse | .[0].url)
			// ($wheels | .[0].url)
			// $sdist
		  ) end'
}

# pypi_fetch unpacks a package version into <dir>/pypi/<name>-<version>.
pypi_fetch() {
	local item="$1" kind="$2"
	local name="${item%@*}"
	local version="${item#*@}"
	local out="$dir/pypi/$name-$version"
	if [ -d "$out" ]; then
		return
	fi
	local url
	url="$(pypi_url "$name" "$version" "$kind")"
	if [ -z "$url" ] || [ "$url" = "null" ]; then
		echo "no file of $item on pypi.org" >&2
		exit 1
	fi
	local tmp
	tmp="$(mktemp -d)"
	curl -sSfL "$url" -o "$tmp/package"
	mkdir -p "$out"
	case "$url" in
	*.whl)
		unzip -q "$tmp/package" -d "$out"
		;;
	*.zip)
		unzip -q "$tmp/package" -d "$tmp/src"
		mv "$tmp"/src/*/* "$out"
		;;
	*)
		tar -xzf "$tmp/package" -C "$out" --strip-components=1
		;;
	esac
	rm -rf "$tmp"
}

# go_release prints the Go release of the go line of a go.mod, as go1.16
# for go 1.16 and go1.21.0 for go 1.21. The go command reads a go.mod with
# no go line as go 1.16.
go_release() {
	local version minor
	version="$(awk '$1 == "go" { print $2; exit }' "$1")"
	version="${version:-1.16}"
	minor="${version#1.}"
	if [ "$minor" = "${minor%%.*}" ] && [ "$minor" -ge 21 ]; then
		version="$version.0"
	fi
	echo "go$version"
}

# go_project fetches the src tree of the Go release of a Go project, and
# records the release.
go_project() {
	local project="$1" src release tmp
	src="$(go mod download -json "$project" | jq -r .Dir)"
	release="$(go_release "$src/go.mod")"
	echo "$project $release" >>"$dir/gostd/releases.txt"
	if [ ! -d "$dir/gostd/$release" ]; then
		tmp="$(mktemp -d)"
		curl -sSfL "https://go.dev/dl/$release.src.tar.gz" | tar -xz -C "$tmp" go/src
		mv "$tmp/go" "$dir/gostd/$release"
		rm -rf "$tmp"
	fi
}

grep -v '^#' "$list" | while read -r language _ project deps; do
	if [ "$language" = python ]; then
		pypi_fetch "$project" project
		for item in $deps; do
			pypi_fetch "$item" dependency
		done
		continue
	fi
	if [ "$language" = go ]; then
		go_project "$project"
	fi
	for item in $project $deps; do
		case "$language" in
		go)
			go mod download "$item"
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

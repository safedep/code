#!/bin/bash
# Fetch the SWARM-JS snippets of SWARM-CG at a pinned commit into the
# directory of the first argument. SWARM-CG has no license, so the
# repository does not hold a copy. Point PROJECTGRAPH_SWARM_JS at
# <dir>/benchmarks/javascript/swarm_js to score them.
set -euo pipefail

dir="${1:?usage: fetch_swarm_js.sh <dir>}"
commit="5929c0a36a9c8e8c6c7539dc412336cdf6c09e58"

git init --quiet "$dir"
git -C "$dir" fetch --quiet --depth 1 https://github.com/secure-software-engineering/SWARM-CG.git "$commit"
git -C "$dir" checkout --quiet FETCH_HEAD
echo "$dir/benchmarks/javascript/swarm_js"

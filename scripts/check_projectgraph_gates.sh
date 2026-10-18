#!/bin/bash
# Check the gates of the public sets of the project graph.
#
#   check_projectgraph_gates.sh PUBLIC_REPORT GOVULNCHECK_REPORT [SET=MIN ...]
#
# PUBLIC_REPORT is the file that PROJECTGRAPH_PUBLIC_REPORT names, and
# GOVULNCHECK_REPORT the file that PROJECTGRAPH_GOVULNCHECK_REPORT names.
# Each SET=MIN fails the check when precision-high-medium of the set is
# under MIN, as pycg-micro=0.95. The govulncheck report must find every
# stack with its functions in order.
set -euo pipefail

public="${1:?usage: check_projectgraph_gates.sh PUBLIC_REPORT GOVULNCHECK_REPORT [SET=MIN ...]}"
govulncheck="${2:?usage: check_projectgraph_gates.sh PUBLIC_REPORT GOVULNCHECK_REPORT [SET=MIN ...]}"
shift 2
status=0

for gate in "$@"; do
  set_name="${gate%%=*}"
  min="${gate#*=}"
  value=$(awk -v set="set=$set_name" '
    $1 == set { in_set = 1; next }
    /^set=/ { in_set = 0 }
    in_set && /precision-high-medium=/ { sub(/.*precision-high-medium=/, ""); print $1; exit }
  ' "$public")
  if [ -z "$value" ]; then
    echo "no precision-high-medium of $set_name in $public"
    status=1
  elif awk -v v="$value" -v m="$min" 'BEGIN { exit !(v + 0 < m + 0) }'; then
    echo "precision-high-medium of $set_name is $value, under the gate $min"
    status=1
  else
    echo "precision-high-medium of $set_name is $value, at or over the gate $min"
  fi
done

for metric in found stack; do
  value=$(awk -v key="$metric=" 'index($1, key) == 1 { sub(key, "", $1); print $1; exit }' "$govulncheck")
  if [ "$value" != "1.000" ]; then
    echo "govulncheck $metric is ${value:-missing}, want 1.000"
    status=1
  else
    echo "govulncheck $metric is 1.000"
  fi
done

exit $status

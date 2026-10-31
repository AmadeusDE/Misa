#!/usr/bin/env bash
# Native Windows gate: runs go test on every package marked native in packages.txt, once per tag set.
# Every package runs even after a failure so one run reports them all; the exit status is non-zero if any failed.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
export CGO_ENABLED=0
mapfile -t pkgs < <(awk '!/^#/ && $2 == "native" {print $1}' "$here/packages.txt")
fail=0
for tags in 'nough noweb' 'nough noweb nogui'; do
  for p in "${pkgs[@]}"; do
    echo "::group::go test -tags '$tags' ./$p"
    go test -count=1 -tags "$tags" "./$p" || { echo "FAILED: $p [$tags]"; fail=1; }
    echo "::endgroup::"
  done
done
exit $fail

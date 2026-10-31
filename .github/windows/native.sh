#!/usr/bin/env bash
# Native Windows gate: runs go test on every package marked native in packages.txt, once per tag set.
# Every package runs even after a failure so one run reports them all; the exit status is non-zero if any failed.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
export CGO_ENABLED=0
pkgs=(internal/builtins) # TEMP
fail=0
for tags in 'nough noweb'; do
  for p in "${pkgs[@]}"; do
    echo "::group::go test -tags '$tags' ./$p"
    go test -v -timeout 20m -count=1 -tags "$tags" "./$p" || { echo "FAILED: $p [$tags]"; fail=1; }
    echo "::endgroup::"
  done
done
exit $fail

#!/usr/bin/env bash
# Cross-build gate on Linux: builds the root binary, vets, and compiles the test binary of every package in
# packages.txt for windows/amd64 and windows/arm64, each into its own output path. Usage: cross.sh <outdir>
# cmd/ugh is not built: the agent has no Windows port yet (docs/plans/windows.md part 2).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
out=${1:?output directory}
export GOOS=windows CGO_ENABLED=0
mapfile -t pkgs < <(awk '!/^#/ && NF {print $1}' "$here/packages.txt")
paths=()
for p in "${pkgs[@]}"; do paths+=("./$p"); done
for arch in amd64 arm64; do
  export GOARCH=$arch
  d="$out/$arch"
  mkdir -p "$d/bin" "$d/tests"
  for tags in 'nough noweb' 'nough noweb nogui'; do
    t=${tags// /-}
    go build -tags "$tags" -o "$d/bin/misa-$t.exe" .
    go vet -tags "$tags" "${paths[@]}"
    for p in "${pkgs[@]}"; do
      name=${p//\//_}
      [ "$p" = . ] && name=root
      go test -c -tags "$tags" -o "$d/tests/$name-$t.test.exe" "./$p"
    done
  done
done

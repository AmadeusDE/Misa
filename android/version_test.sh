#!/usr/bin/env bash
# version.sh arithmetic. Git is not consulted when the overrides are set.
set -euo pipefail
cd "$(dirname "$0")/.."
v() { MISA_VERSION_NAME=$1 MISA_VERSION_COUNT=${2:-} android/version.sh "$3" "${4:-}"; }

[[ $(v v1.2.3 "" name) == v1.2.3 ]]
[[ $(v v1.2.3 "" base) == 1002003 ]]
[[ $(v v1.2.3 "" code universal) == 10020030 ]]
[[ $(v v1.2.3 "" code arm64-v8a) == 10020031 ]]
[[ $(v v1.2.3 "" code x86_64) == 10020032 ]]
[[ $(v v0.0.1 "" code arm64) == 11 ]]
[[ $(v v1.2.3-4-gabcdef 40 code x86_64) == 402 ]]
[[ $(v d568d5d 16 code universal) == 160 ]]
echo "version.sh ok"

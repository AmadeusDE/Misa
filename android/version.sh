#!/usr/bin/env bash
# versionName and versionCode for the APK. Gradle and CI both call this.
#
#   version.sh name
#   version.sh base
#   version.sh code [arm64-v8a|x86_64|universal]
#
# versionName is `git describe --tags --always`.
# An exact vMAJOR.MINOR.PATCH tag uses MAJOR*1000000 + MINOR*1000 + PATCH.
# Any other revision uses the commit count. versionCode is that base times
# 10 plus an ABI digit (universal 0, arm64-v8a 1, x86_64 2) so split APKs
# of one revision stay ordered and a newer revision sorts above all of them.
#
# MISA_VERSION_NAME and MISA_VERSION_COUNT override git for tests.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
mode=${1:-}
abi=${2:-universal}

if [[ -n ${MISA_VERSION_NAME:-} ]]; then
	name=$MISA_VERSION_NAME
else
	name=$(git -C "$root" describe --tags --always 2>/dev/null || echo dev)
fi

base=0
if [[ $name =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
	base=$((BASH_REMATCH[1] * 1000000 + BASH_REMATCH[2] * 1000 + BASH_REMATCH[3]))
elif [[ -n ${MISA_VERSION_COUNT:-} ]]; then
	base=$MISA_VERSION_COUNT
else
	base=$(git -C "$root" rev-list --count HEAD 2>/dev/null || echo 1)
fi

digit=0
case $abi in
"" | universal) digit=0 ;;
arm64-v8a | arm64 | aarch64) digit=1 ;;
x86_64 | amd64 | x86-64) digit=2 ;;
*)
	echo "unknown abi: $abi" >&2
	exit 1
	;;
esac

case $mode in
name) printf '%s\n' "$name" ;;
base) printf '%s\n' "$base" ;;
code) printf '%s\n' $((base * 10 + digit)) ;;
*)
	echo "usage: version.sh name|base|code [abi]" >&2
	exit 1
	;;
esac

#!/usr/bin/env bash
# android-emulator-runner runs each script: line with /bin/sh -c (dash).
# The loop lives here so one bash invocation owns set -e and the adb steps.
# Test binaries read testdata relative to the package directory, the same
# paths `go test` uses in the repo.
set -euo pipefail

stage="$RUNNER_TEMP/misa-device-tree"
rm -rf "$stage"
mkdir -p \
	"$stage/internal/term" \
	"$stage/internal/builtins" \
	"$stage/internal/gui" \
	"$stage/internal/sshclient" \
	"$stage/tests/uutils/testdata/tests/fixtures"
cp "$RUNNER_TEMP/android-tests/term.test" "$stage/internal/term/"
cp "$RUNNER_TEMP/android-tests/builtins.test" "$stage/internal/builtins/"
cp "$RUNNER_TEMP/android-tests/gui.test" "$stage/internal/gui/"
cp "$RUNNER_TEMP/android-tests/sshclient.test" "$stage/internal/sshclient/"
cp -a internal/term/testdata "$stage/internal/term/testdata"
cp -a tests/uutils/testdata/tests/fixtures/sum "$stage/tests/uutils/testdata/tests/fixtures/sum"
cp -a tests/uutils/testdata/tests/fixtures/b2sum "$stage/tests/uutils/testdata/tests/fixtures/b2sum"

adb shell 'mkdir -p /data/local/tmp/misa'
adb push "$stage/." /data/local/tmp/misa/
adb shell 'chmod 755 /data/local/tmp/misa/internal/term/term.test /data/local/tmp/misa/internal/builtins/builtins.test /data/local/tmp/misa/internal/gui/gui.test /data/local/tmp/misa/internal/sshclient/sshclient.test'

run() {
	dir=$1
	bin=$2
	echo "=== $bin ==="
	adb shell "cd /data/local/tmp/misa/$dir && PATH=/system/bin:/system/xbin:\$PATH TMPDIR=/data/local/tmp HOME=/data/local/tmp ./$bin -test.count=1 -test.timeout 15m"
}
run internal/term term.test
run internal/builtins builtins.test
run internal/gui gui.test
run internal/sshclient sshclient.test

#!/usr/bin/env bash
# android-emulator-runner runs each script: line with /bin/sh -c (dash).
# The loop lives here so one bash invocation owns set -e and the adb steps.
set -euo pipefail

adb shell 'mkdir -p /data/local/tmp/misa-tests'
adb push "$RUNNER_TEMP/android-tests/." /data/local/tmp/misa-tests/
adb shell 'chmod 755 /data/local/tmp/misa-tests/*.test'
for t in term builtins gui sshclient; do
	echo "=== $t ==="
	adb shell "cd /data/local/tmp/misa-tests && TMPDIR=/data/local/tmp HOME=/data/local/tmp ./$t.test -test.count=1 -test.timeout 15m"
done

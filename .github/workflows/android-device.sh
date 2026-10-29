#!/usr/bin/env bash
# android-emulator-runner runs each script: line with /bin/sh -c (dash).
# The loop lives here so one bash invocation owns set -e and the adb steps.
# Test binaries read testdata relative to the package directory, the same
# paths `go test` uses in the repo. ExtraKeys.java is staged at that same
# relative path so guiandroid.test can check the mux button ids.
set -euo pipefail

stage="$RUNNER_TEMP/misa-device-tree"
rm -rf "$stage"
mkdir -p \
	"$stage/internal/term" \
	"$stage/internal/builtins" \
	"$stage/internal/gui" \
	"$stage/internal/gui/android" \
	"$stage/internal/sshclient" \
	"$stage/android/app/src/main/java/app/misa" \
	"$stage/tests/uutils/testdata/tests/fixtures"
cp "$RUNNER_TEMP/android-tests/term.test" "$stage/internal/term/"
cp "$RUNNER_TEMP/android-tests/builtins.test" "$stage/internal/builtins/"
cp "$RUNNER_TEMP/android-tests/gui.test" "$stage/internal/gui/"
cp "$RUNNER_TEMP/android-tests/guiandroid.test" "$stage/internal/gui/android/"
cp "$RUNNER_TEMP/android-tests/sshclient.test" "$stage/internal/sshclient/"
cp android/app/src/main/java/app/misa/ExtraKeys.java "$stage/android/app/src/main/java/app/misa/"
cp -a internal/term/testdata "$stage/internal/term/testdata"
cp -a tests/uutils/testdata/tests/fixtures/sum "$stage/tests/uutils/testdata/tests/fixtures/sum"
cp -a tests/uutils/testdata/tests/fixtures/b2sum "$stage/tests/uutils/testdata/tests/fixtures/b2sum"

adb shell 'mkdir -p /data/local/tmp/misa'
adb push "$stage/." /data/local/tmp/misa/
adb shell 'chmod 755 /data/local/tmp/misa/internal/term/term.test /data/local/tmp/misa/internal/builtins/builtins.test /data/local/tmp/misa/internal/gui/gui.test /data/local/tmp/misa/internal/gui/android/guiandroid.test /data/local/tmp/misa/internal/sshclient/sshclient.test'

run() {
	dir=$1
	bin=$2
	echo "=== $bin ==="
	adb shell "cd /data/local/tmp/misa/$dir && PATH=/system/bin:/system/xbin:\$PATH TMPDIR=/data/local/tmp HOME=/data/local/tmp ./$bin -test.count=1 -test.timeout 15m"
}
run internal/term term.test
run internal/builtins builtins.test
run internal/gui gui.test
run internal/gui/android guiandroid.test
run internal/sshclient sshclient.test

echo "=== apk ==="
shopt -s nullglob
apks=(android/app/build/outputs/apk/debug/*x86_64*.apk)
if ((${#apks[@]} == 0)); then
	apks=(android/app/build/outputs/apk/debug/*universal*.apk)
fi
if ((${#apks[@]} == 0)); then
	apks=(android/app/build/outputs/apk/debug/*.apk)
fi
if ((${#apks[@]} == 0)); then
	echo "no debug APK to install" >&2
	exit 1
fi
adb install -r -t "${apks[0]}"
# API 26 logd sometimes returns "failed to clear the 'main' log" while busy.
# set -e would abort the install check; a leftover buffer is still scanned below.
adb logcat -c || true
adb shell am start -n app.misa/.MisaActivity
sleep 5
# exec-out keeps the PNG intact; `adb shell screencap` can mangle it with CR.
adb exec-out screencap -p > misa-apk-screen.png || true
adb logcat -d -v threadtime > "$RUNNER_TEMP/misa-apk-logcat.txt"
if ! adb shell pidof app.misa >/dev/null; then
	echo "app.misa is not running" >&2
	tail -n 80 "$RUNNER_TEMP/misa-apk-logcat.txt" >&2 || true
	exit 1
fi
pid=$(adb shell pidof app.misa | tr -d '\r' | awk '{print $1}')
adb logcat -d -v threadtime --pid="$pid" > "$RUNNER_TEMP/misa-apk-pid.txt" || true
if ! awk '
	index(FILENAME, "pid") && /panic:/ { bad = 1 }
	index(FILENAME, "pid") && /fatal error: all goroutines/ { bad = 1 }
	index(FILENAME, "logcat") && /FATAL EXCEPTION/ { blk = 1; next }
	index(FILENAME, "logcat") && blk && /Process: app\.misa/ { bad = 1 }
	END { exit bad ? 1 : 0 }
' "$RUNNER_TEMP/misa-apk-logcat.txt" "$RUNNER_TEMP/misa-apk-pid.txt"; then
	echo "app.misa crashed" >&2
	tail -n 120 "$RUNNER_TEMP/misa-apk-logcat.txt" >&2 || true
	exit 1
fi
echo "apk is up"

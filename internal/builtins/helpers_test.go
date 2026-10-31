package builtins

import (
	"runtime"
	"testing"
)

// exeSuffix is the extension a test executable needs to be found on PATH.
var exeSuffix = map[bool]string{true: ".exe", false: ""}[runtime.GOOS == "windows"]

// unameSysname is what uname prints as the kernel name on the host.
var unameSysname = map[bool]string{true: "Windows_NT", false: "Linux"}[runtime.GOOS == "windows"]

// skipOnWindows skips a test that asserts Unix-only behaviour, with the reason.
func skipOnWindows(t *testing.T, reason string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip(reason)
	}
}

// diffCaseStride thins the permutation tests that start a patch process per
// case, which costs about 15 ms each on Windows.
var diffCaseStride = map[bool]int{true: 7, false: 1}[runtime.GOOS == "windows"]

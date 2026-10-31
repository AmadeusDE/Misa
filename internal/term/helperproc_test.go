package term

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"
)

const helperEnv = "MISA_TERM_TEST_HELPER"

// helperArgv returns argv that re-executes the test binary as a tiny portable program
// (no sh, no echo), so the same test runs on Unix and Windows.
func helperArgv(t *testing.T, mode string, args ...string) []string {
	t.Helper()
	t.Setenv(helperEnv, mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{exe, "-test.run=^TestHelperProcess$", "--"}, args...)
}

// skipOnWindows marks a test whose behaviour exists only with a Unix PTY, shell or signals.
func skipOnWindows(t *testing.T, reason string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("windows: " + reason)
	}
}

// TestHelperProcess is the child of helperArgv: modes echo, status, sleep.
func TestHelperProcess(t *testing.T) {
	mode := os.Getenv(helperEnv)
	if mode == "" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	switch mode {
	case "echo":
		fmt.Print(args[0])
		os.Exit(0)
	case "status":
		fmt.Print(args[0])
		n, _ := strconv.Atoi(args[1])
		os.Exit(n)
	case "sleep":
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}
	os.Exit(2)
}

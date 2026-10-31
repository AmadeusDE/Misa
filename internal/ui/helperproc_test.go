package ui

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const helperEnv = "MISA_UI_TEST_HELPER"

// helperArgv re-executes the test binary as a tiny portable program (no sh, no seq),
// so a terminal-backed test runs the same on Unix and Windows.
func helperArgv(t *testing.T, mode string, args ...string) []string {
	t.Helper()
	t.Setenv(helperEnv, mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{exe, "-test.run=^TestHelperProcess$", "--"}, args...)
}

// skipOnWindows marks a test whose behaviour exists only with Unix jobs, signals or a shell.
func skipOnWindows(t *testing.T, reason string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("windows: " + reason)
	}
}

// TestHelperProcess is the child of helperArgv: modes status, readline, flood.
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
	case "status":
		fmt.Print(args[0])
		n, _ := strconv.Atoi(args[1])
		os.Exit(n)
	case "readline":
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		fmt.Printf("got %s\n", strings.TrimRight(line, "\r\n"))
		os.Exit(0)
	case "flood":
		w := bufio.NewWriter(os.Stdout)
		fmt.Fprint(w, "\x1b[?1049h")
		n, _ := strconv.Atoi(args[0])
		for i := 1; i <= n; i++ {
			fmt.Fprintln(w, i)
		}
		w.Flush()
		os.Exit(0)
	}
	os.Exit(2)
}

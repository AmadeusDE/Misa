package vi

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireRunnerShell(t *testing.T) {
	t.Helper()
	shell := shellExecutable()
	if filepath.IsAbs(shell) {
		if _, err := os.Stat(shell); err != nil {
			t.Skipf("shell unavailable: %v", err)
		}
	} else if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("shell unavailable: %v", err)
	}
}

func TestShellRunnerStreamsEnvironmentAndDirectory(t *testing.T) {
	requireRunnerShell(t)
	dir := t.TempDir()
	result := runShell(context.Background(), "printf '%s\\n' \"$T03_TEST\"; pwd; while IFS= read -r line; do printf '%s\\n' \"$line\"; done; printf err >&2; exit 7", []byte("input\n"), dir, append(os.Environ(), "T03_TEST=fixture"))
	if result.err == nil || result.exitCode != 7 {
		t.Fatalf("exit: %+v", result)
	}
	if result.stdout != "fixture\n"+dir+"\ninput\n" || result.stderr != "err" {
		t.Fatalf("streams: %+v", result)
	}
}

func TestShellRunnerCancellation(t *testing.T) {
	requireRunnerShell(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := runShell(ctx, ":", nil, t.TempDir(), os.Environ())
	if result.err == nil || !strings.Contains(result.err.Error(), "cancel") {
		t.Fatalf("expected cancellation: %+v", result)
	}
}

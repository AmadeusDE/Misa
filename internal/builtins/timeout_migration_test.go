//go:build unix

package builtins

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Exercise external execution without registering a multicall helper.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "--timeout-no-helper" {
		os.Exit(runTimeout(&Context{Args: []string{"2", hostBin("sh"), "-c", "exit 17"}, Stdout: os.Stdout, Stderr: os.Stderr}))
	}
}

func TestTimeoutExternalWithoutHelper(t *testing.T) {
	cmd := exec.Command(os.Args[0], "--timeout-no-helper")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 17 {
		t.Fatalf("external command without helper: %v", err)
	}
}

func TestTimeoutBuiltinDispatch(t *testing.T) {
	code, out, stderr := timeoutTestRun(t, t.TempDir(), []string{"PATH="}, "2", "echo", "builtin")
	if code != 0 || out != "builtin\n" {
		t.Fatalf("code=%d out=%q stderr=%q", code, out, stderr)
	}
	code, _, stderr = timeoutTestRun(t, t.TempDir(), []string{"PATH="}, ".05", "sleep", "5")
	if code != 124 {
		t.Fatalf("builtin timeout: code=%d stderr=%q", code, stderr)
	}
}

func TestTimeoutCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "deadline"}[deadline], func(t *testing.T) {
			c, cancel := context.WithCancel(context.Background())
			if deadline {
				c, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			}
			defer cancel()
			var stderr bytes.Buffer
			done := make(chan int, 1)
			go func() {
				done <- runTimeout(&Context{Ctx: c, Dir: t.TempDir(), Env: withTimeoutPath(nil), Args: []string{"10", hostBin("sh"), "-c", "sleep 5"}, Stderr: &stderr})
			}()
			if !deadline {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}
			select {
			case code := <-done:
				want := 130
				if deadline {
					want = 124
				}
				if code != want {
					t.Fatalf("code=%d want=%d stderr=%q", code, want, stderr.String())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("cancellation did not stop timeout")
			}
		})
	}
}

func TestTimeoutStandaloneInterrupt(t *testing.T) {
	if testing.Short() {
		t.Skip("builds standalone binary")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	binary := filepath.Join(t.TempDir(), "misa")
	build := exec.Command("go", "build", "-o", binary, "../..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			dir := t.TempDir()
			ready := filepath.Join(dir, "ready")
			survived := filepath.Join(dir, "survived")
			script := `echo ready > "$1"; sleep 1; echo survived > "$2"`
			cmd := exec.Command(binary, "timeout", "10", hostBin("sh"), "-c", script, "sh", ready, survived)
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Kill()
			until := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				if time.Now().After(until) {
					t.Fatalf("child not ready: %s", output.String())
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				e, ok := err.(*exec.ExitError)
				if !ok || (e.ExitCode() != 130 && e.ExitCode() != 128+int(sig)) {
					t.Fatalf("exit %v output %q", err, output.String())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("standalone did not exit")
			}
			time.Sleep(1100 * time.Millisecond)
			if b, err := os.ReadFile(survived); err == nil {
				t.Fatalf("child survived: %s", strings.TrimSpace(string(b)))
			}
		})
	}
}

func TestTimeoutForegroundLeavesDescendant(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "survived")
	// This shell starts a child before it is signaled; -f must not signal the
	// child's inherited group. The child exits after writing the marker.
	script := `(sleep .25; echo survived > "$1") & wait`
	code, _, stderr := timeoutTestRun(t, dir, nil, "-f", ".1", hostBin("sh"), "-c", script, "sh", marker)
	if code != 124 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	until := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		if time.Now().After(until) {
			t.Fatal("foreground timeout killed descendant")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

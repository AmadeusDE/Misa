//go:build unix

package builtinexec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == proceffect.Flag {
		os.Exit(ProcessEffect(os.Args[2:]))
	}
	if err := proceffect.EnableSelfHelper(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func fixtureLink(t *testing.T, name string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, name); err != nil {
		t.Fatal(err)
	}
}

func TestLauncherResolutionMatrix(t *testing.T) {
	dir := t.TempDir()
	fixtureLink(t, filepath.Join(dir, "poison"))
	fixtureLink(t, filepath.Join(dir, "ls"))
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	fixtureLink(t, filepath.Join(dir, "bin", "fixture"))
	if err := os.Mkdir(filepath.Join(dir, "directory"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blocked"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "executed")
	if err := os.WriteFile(filepath.Join(dir, "text"), []byte("echo unsafe > "+marker+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, path string
		env        []string
		args       []string
		status     int
		output     string
	}{
		{"unset PATH ignores cwd", "poison", []string{"BUILTINEXEC_HELPER=argv0"}, []string{"-test.run=TestHelperProcess"}, 127, ""},
		{"empty PATH is cwd", "poison", []string{"PATH=", "BUILTINEXEC_HELPER=argv0"}, []string{"-test.run=TestHelperProcess"}, 0, "poison"},
		{"relative PATH", "fixture", []string{"PATH=bin", "BUILTINEXEC_HELPER=argv0"}, []string{"-test.run=TestHelperProcess"}, 0, "fixture"},
		{"directory", "directory", []string{"PATH="}, nil, 126, ""},
		{"nonexecutable", "blocked", []string{"PATH="}, nil, 126, ""},
		{"ENOEXEC", "text", []string{"PATH="}, nil, 126, ""},
		{"explicit builtin basename", "./ls", []string{"BUILTINEXEC_HELPER=argv0"}, []string{"-test.run=TestHelperProcess"}, 0, "./ls"},
		{"builtin beats cwd", "ls", []string{"PATH=", "BUILTINEXEC_HELPER=argv0"}, []string{"--version"}, 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			got := Run(context.Background(), tc.path, &builtins.Context{Dir: dir, Env: tc.env, Args: tc.args, Stdout: &out, Stderr: &stderr})
			if got != tc.status {
				t.Errorf("status %d want %d; stderr %q", got, tc.status, stderr.String())
			}
			if tc.output != "" && out.String() != tc.output {
				t.Errorf("output %q want %q", out.String(), tc.output)
			}
			if tc.name == "builtin beats cwd" && (out.String() == "ls" || !strings.Contains(strings.ToLower(out.String()), "ls")) {
				t.Errorf("builtin output %q", out.String())
			}
		})
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("ENOEXEC shell fallback ran: %v", err)
	}
}

func TestLauncherSignalStatus(t *testing.T) {
	shell := "/bin/sh"
	if _, err := os.Stat(shell); err != nil {
		t.Skip(err)
	}
	var stderr bytes.Buffer
	got := Run(context.Background(), shell, &builtins.Context{Args: []string{"-c", "kill -TERM $$"}, Env: []string{}, Stderr: &stderr})
	if got != 128+int(syscall.SIGTERM) {
		t.Fatalf("status %d stderr %q", got, stderr.String())
	}
}

func TestLauncherCancellationKillsGroupAndReaps(t *testing.T) {
	shell := "/bin/sh"
	if _, err := os.Stat(shell); err != nil {
		t.Skip(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		done <- Run(ctx, shell, &builtins.Context{Args: []string{"-c", "sleep 30 & echo $$ $!; wait"}, Env: []string{"PATH=/bin:/usr/bin"}, Stdout: w, Stderr: io.Discard})
	}()
	lines := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(r).ReadString('\n'); lines <- line }()
	var pid, grandchild int
	select {
	case line := <-lines:
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("handshake %q", line)
		}
		pid, err = strconv.Atoi(fields[0])
		if err != nil {
			t.Fatal(err)
		}
		grandchild, err = strconv.Atoi(fields[1])
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no child handshake")
	}
	cancel()
	select {
	case status := <-done:
		if status != 130 {
			t.Errorf("status %d want 130", status)
		}
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(grandchild, syscall.SIGKILL)
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatal("Run did not return")
	}
	var ws syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil); err != syscall.ECHILD {
		t.Errorf("direct child not reaped: %v", err)
	}
	w.Close()
	eof := make(chan error, 1)
	go func() { _, e := io.Copy(io.Discard, r); eof <- e }()
	select {
	case e := <-eof:
		if e != nil {
			t.Error(e)
		}
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(grandchild, syscall.SIGKILL)
		t.Error("descendant retained stdout after cancellation")
	}
}

func TestLauncherNestedWrappers(t *testing.T) {
	var out, stderr bytes.Buffer
	got := Run(context.Background(), "env", &builtins.Context{Dir: t.TempDir(), Env: []string{"X=outer"}, Args: []string{"-i", "X=inner", "nice", "-n", "1", "timeout", "5", "printenv", "X"}, Stdin: strings.NewReader("input"), Stdout: &out, Stderr: &stderr})
	if got != 0 || out.String() != "inner\n" {
		t.Fatalf("status %d stdout %q stderr %q", got, out.String(), stderr.String())
	}
}

func TestLauncherConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var out, stderr bytes.Buffer
			value := strconv.Itoa(i)
			got := Run(context.Background(), "env", &builtins.Context{Dir: t.TempDir(), Env: []string{}, Args: []string{"-i", "X=" + value, "printenv", "X"}, Stdout: &out, Stderr: &stderr})
			if got != 0 || out.String() != value+"\n" {
				t.Errorf("%d: status %d output %q stderr %q", i, got, out.String(), stderr.String())
			}
		}(i)
	}
	wg.Wait()
}

func TestLauncherNestedExternalContext(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	args := []string{"-i", "BUILTINEXEC_HELPER=1", "VALUE=nested", "nice", "-n", "1", "timeout", "5", exe, "-test.run=TestHelperProcess"}
	got := Run(context.Background(), "env", &builtins.Context{Dir: dir, Env: []string{"VALUE=outer"}, Args: args, Stdin: strings.NewReader("input"), Stdout: &out, Stderr: &stderr})
	if got != 7 || out.String() != "input|nested|"+dir || stderr.String() != "separate" {
		t.Fatalf("status %d stdout %q stderr %q", got, out.String(), stderr.String())
	}
}

func TestLauncherNestedCancellation(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		done <- Run(ctx, "env", &builtins.Context{Dir: t.TempDir(), Env: []string{}, Args: []string{"-i", "BUILTINEXEC_HELPER=1", "BUILTINEXEC_WAIT=1", "nice", "-n", "1", "timeout", "5", exe, "-test.run=TestHelperProcess"}, Stdout: w, Stderr: io.Discard})
	}()
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(r).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if line != "ready\n" {
			t.Fatalf("handshake %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no child handshake")
	}
	cancel()
	select {
	case status := <-done:
		if status != 130 {
			t.Errorf("status %d want 130", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nested Run did not return")
	}
}

// The shell and its sleep child belong to timeout's separately owned group,
// nested inside nice's helper. Both retain stdout until they exit. Pipe EOF
// detects live descendants without assuming that adopted grandchildren have
// already been reaped by the system's init process.
func TestLauncherNiceTimeoutCancelsDescendantGroup(t *testing.T) {
	shell := "/bin/sh"
	if _, err := os.Stat(shell); err != nil {
		t.Skip(err)
	}
	if _, err := os.Stat("/bin/sleep"); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		done <- Run(ctx, "nice", &builtins.Context{Dir: dir, Env: []string{"PATH=/bin:/usr/bin"}, Args: []string{"-n", "1", "timeout", "30", shell, "-c", "/bin/sleep 30 & echo $$ $!; wait"}, Stdout: w, Stderr: io.Discard})
	}()
	lines := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(r).ReadString('\n'); lines <- line }()
	var shellPID, sleepPID int
	select {
	case line := <-lines:
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("handshake %q", line)
		}
		shellPID, err = strconv.Atoi(fields[0])
		if err != nil {
			t.Fatal(err)
		}
		sleepPID, err = strconv.Atoi(fields[1])
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no nested child handshake")
	}
	cancel()
	select {
	case status := <-done:
		if status != 130 {
			t.Errorf("status %d, want 130", status)
		}
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(sleepPID, syscall.SIGKILL)
		_ = syscall.Kill(shellPID, syscall.SIGKILL)
		t.Fatal("nested Run did not return")
	}
	w.Close()
	eof := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, r); eof <- err }()
	select {
	case err := <-eof:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(sleepPID, syscall.SIGKILL)
		_ = syscall.Kill(shellPID, syscall.SIGKILL)
		t.Error("nested shell or grandchild retained stdout after cancellation")
	}
}

package builtins

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/proceffect"
)

func TestNiceChildIsolation(t *testing.T) {
	before, err := proceffect.Niceness()
	if err != nil {
		t.Skip(err)
	}
	var out, stderr bytes.Buffer
	called := false
	ctx := &Context{Args: []string{"-n", "5", "nice"}, Stdout: &out, Stderr: &stderr, Exec: func(context.Context, string, *Context) int { called = true; return 99 }}
	status := Run(context.Background(), "nice", ctx)
	after, err := proceffect.Niceness()
	if err != nil {
		t.Fatal(err)
	}
	if status != 0 || called || strings.TrimSpace(out.String()) != fmtInt(clampNice(before+5)) || after != before {
		t.Fatalf("status=%d called=%v output=%q stderr=%q parent=%d -> %d", status, called, out.String(), stderr.String(), before, after)
	}
}

func fmtInt(n int) string { return strconv.Itoa(n) }
func clampNice(n int) int {
	if n > 19 {
		return 19
	}
	return n
}

func TestNiceChildInvocation(t *testing.T) {
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	before, err := proceffect.Niceness()
	if err != nil {
		t.Skip(err)
	}
	ctx := &Context{Dir: dir, Args: []string{"-n", "5", helperPath(t), "--nice-test-helper", "inspect"}, Env: []string{"VALUE=child"}, Stdin: strings.NewReader("input"), Stdout: &out, Stderr: &stderr}
	if status := Run(context.Background(), "nice", ctx); status != 0 || out.String() != fmt.Sprintf("%s|child|%d|input", dir, clampNice(before+5)) || stderr.String() != "error" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, out.String(), stderr.String())
	}

	// An explicit path is never reinterpreted as a builtin, even if its basename is one.
	out.Reset()
	stderr.Reset()
	ctx.Args = []string{"-n", "0", filepath.Join(dir, "echo")}
	if status := Run(context.Background(), "nice", ctx); status != 127 {
		t.Fatalf("missing explicit path status=%d stderr=%q", status, stderr.String())
	}
	f := filepath.Join(dir, "noexec")
	if err := os.WriteFile(f, []byte("not executable"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx.Args = []string{"-n", "0", f}
	if status := Run(context.Background(), "nice", ctx); status != 126 {
		t.Fatalf("non-executable status=%d stderr=%q", status, stderr.String())
	}
}

func TestNiceCancellation(t *testing.T) {
	c, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	var stderr bytes.Buffer
	done := make(chan int, 1)
	helper := helperPath(t)
	go func() {
		done <- Run(c, "nice", &Context{Args: []string{"-n", "5", helper, "--nice-test-helper", "sleep-parent"}, Stdout: writer, Stderr: &stderr})
	}()
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(reader).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("unexpected handshake %q", line)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child did not start")
	}
	cancel()
	select {
	case status := <-done:
		if status != 130 {
			t.Fatalf("status=%d stderr=%q", status, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child did not exit after cancellation")
	}
	writer.Close()
	eof := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, reader); eof <- err }()
	select {
	case err := <-eof:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("descendant still holds stdout after cancellation")
	}
}

func TestNiceConcurrentInvocations(t *testing.T) {
	before, err := proceffect.Niceness()
	if err != nil {
		t.Skip(err)
	}
	mask, err := proceffect.Umask()
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	helper := helperPath(t)
	envBefore := os.Environ()
	done := make(chan struct{})
	var monitor sync.WaitGroup
	monitor.Add(1)
	go func() {
		defer monitor.Done()
		for {
			select {
			case <-done:
				return
			default:
				m, err := proceffect.Umask()
				if err != nil || m != mask {
					t.Errorf("concurrent umask %o: %v", m, err)
					return
				}
			}
		}
	}()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			var out, errout bytes.Buffer
			status := Run(context.Background(), "nice", &Context{Stdout: &out, Stderr: &errout})
			if status != 0 || strings.TrimSpace(out.String()) != strconv.Itoa(before) {
				t.Errorf("other pane nice status=%d output=%q stderr=%q", status, out.String(), errout.String())
				return
			}
		}
	}()
	for i := 0; i < 12; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			adj := i % 6
			var out, stderr bytes.Buffer
			c := &Context{Dir: dir, Env: []string{fmt.Sprintf("VALUE=%d", i)}, Args: []string{"-n", strconv.Itoa(adj), helper, "--nice-test-helper", "concurrent", fmt.Sprintf("file%d", i)}, Stdout: &out, Stderr: &stderr}
			status := Run(context.Background(), "nice", c)
			want := fmt.Sprintf("%04o\n%d\n%d\n", mask, i, clampNice(before+adj))
			if status != 0 || out.String() != want {
				t.Errorf("%d status=%d out=%q want=%q err=%q", i, status, out.String(), want, stderr.String())
			}
			info, err := os.Stat(filepath.Join(dir, fmt.Sprintf("file%d", i)))
			if err != nil {
				t.Error(err)
			} else if info.Mode().Perm() != os.FileMode(0666&^mask) {
				t.Errorf("mode=%o mask=%o", info.Mode().Perm(), mask)
			}
		}()
	}
	wg.Wait()
	close(done)
	monitor.Wait()
	if !reflect.DeepEqual(envBefore, os.Environ()) {
		t.Error("process environment changed")
	}
	after, err := proceffect.Niceness()
	if err != nil || before != after {
		t.Errorf("priority %d -> %d: %v", before, after, err)
	}
	afterMask, err := proceffect.Umask()
	if err != nil || mask != afterMask {
		t.Errorf("umask %o -> %o: %v", mask, afterMask, err)
	}
}

func TestNiceStatuses(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want int
	}{
		{[]string{"-n"}, 125},
		{[]string{"-n", "0", helperPath(t), "--nice-test-helper", "exit"}, 37},
		{[]string{"-n", "0", helperPath(t), "--nice-test-helper", "signal"}, 143},
	} {
		var out, stderr bytes.Buffer
		status := Run(context.Background(), "nice", &Context{Args: tc.args, Stdout: &out, Stderr: &stderr})
		if status != tc.want {
			t.Errorf("%q: status=%d want=%d stderr=%q", tc.args, status, tc.want, stderr.String())
		}
	}
}

func helperPath(t *testing.T) string {
	t.Helper()
	p, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNiceNegativeAdjustmentWarning(t *testing.T) {
	before, err := proceffect.Niceness()
	if err != nil {
		t.Skip(err)
	}
	var out, stderr bytes.Buffer
	status := Run(context.Background(), "nice", &Context{Args: []string{"-n", "-1", "nice"}, Stdout: &out, Stderr: &stderr})
	if status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	got, err := strconv.Atoi(strings.TrimSpace(out.String()))
	if err != nil {
		t.Fatal(err)
	}
	if got == before && before > -20 && !strings.Contains(stderr.String(), "nice: warning: setpriority:") {
		t.Fatalf("missing priority warning: %q", stderr.String())
	}
}

func TestNiceUnavailableHost(t *testing.T) {
	cmd := exec.Command(helperPath(t), "--nice-no-isolation-helper")
	output, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 125 || string(output) != "nice: process isolation unavailable in this host\n" {
		t.Fatalf("unavailable host: err=%v output=%q", err, output)
	}
}

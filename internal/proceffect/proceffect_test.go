//go:build unix

package proceffect

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Android exposes shell tools under /system/bin, not /bin or /usr/bin.
func testShellPath() string {
	if runtime.GOOS == "android" {
		return "/system/bin"
	}
	return "/bin:/usr/bin"
}
func testShell() string {
	if runtime.GOOS == "android" {
		return "/system/bin/sh"
	}
	return "/bin/sh"
}

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--unregistered-test" {
		before, err := Niceness()
		if err != nil {
			os.Exit(2)
		}
		var stderr bytes.Buffer
		status := Run(context.Background(), Request{Adjustment: 1, Argv: []string{"inspect"}}, Invocation{Stderr: &stderr})
		after, err := Niceness()
		_, fallback := umaskChild()
		if Available() || status != 125 || stderr.String() != "process isolation unavailable\n" || before != after || err != nil || !errors.Is(fallback, ErrUnavailable) {
			os.Exit(3)
		}
		os.Exit(0)
	}
	if err := EnableSelfHelper(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == Flag {
		os.Exit(Handle(os.Args[2:], func(a []string) int {
			switch a[0] {
			case "inspect":
				n, _ := Niceness()
				d, _ := os.Getwd()
				fmt.Printf("%d|%s|%s", n, d, os.Getenv("ONLY"))
				return 7
			case "signal":
				_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
				time.Sleep(time.Second)
				return 1
			case "nested":
				return Run(context.Background(), Request{Argv: []string{"sh", "-c", `(touch "$1"; sleep 1; touch "$2") & wait`, "sh", a[1], a[2]}}, Invocation{Env: []string{"PATH=" + testShellPath()}})
			case "sleep":
				time.Sleep(time.Minute)
				return 0
			default:
				return Exec(a)
			}
		}))
	}
	os.Exit(m.Run())
}
func TestRunIsolation(t *testing.T) {
	before, err := Niceness()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	status := Run(context.Background(), Request{Adjustment: 2, Argv: []string{"inspect"}}, Invocation{Dir: dir, Env: []string{"ONLY=exact"}, Stdout: &out, Stderr: &stderr})
	if status != 7 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	parts := strings.Split(out.String(), "|")
	n, e := strconv.Atoi(parts[0])
	if e != nil || n != min(19, before+2) || len(parts) != 3 || parts[1] != dir || parts[2] != "exact" {
		t.Fatalf("result %q", out.String())
	}
	after, _ := Niceness()
	if before != after {
		t.Fatal("parent priority changed")
	}
}
func TestUmask(t *testing.T) {
	a, e := Umask()
	if e != nil {
		t.Fatal(e)
	}
	b, e := Umask()
	if e != nil || a != b {
		t.Fatalf("%o %o %v", a, b, e)
	}
}
func TestRunStatus(t *testing.T) {
	if s := Run(context.Background(), Request{Argv: []string{"signal"}}, Invocation{}); s != 143 {
		t.Fatalf("signal: %d", s)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if s := Run(ctx, Request{Argv: []string{"sleep"}}, Invocation{}); s != 124 {
		t.Fatalf("cancel: %d", s)
	}
	if s := Run(context.Background(), Request{Argv: []string{"not-an-executable"}}, Invocation{Env: []string{"PATH="}}); s != 127 {
		t.Fatalf("missing: %d", s)
	}
}
func TestExecPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hello")
	if err := os.WriteFile(p, []byte("#!"+testShell()+"\nprintf found"), 0755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	s := Run(context.Background(), Request{Argv: []string{"hello"}}, Invocation{Env: []string{"PATH=" + dir}, Stdout: &out})
	if s != 0 || out.String() != "found" {
		t.Fatalf("%d %q", s, out.String())
	}
}

func TestCancellationKillsGroup(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	survived := filepath.Join(dir, "survived")
	// The descendant announces readiness before cancellation and would write a
	// second marker if it escaped the owned process group.
	script := `(touch "$1"; sleep 1; touch "$2") & wait`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		done <- Run(ctx, Request{Argv: []string{"sh", "-c", script, "sh", ready, survived}}, Invocation{Env: []string{"PATH=" + testShellPath()}})
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("descendant did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if s := <-done; s != 130 {
		t.Fatalf("status %d", s)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, err := os.Stat(survived); !os.IsNotExist(err) {
		t.Fatalf("descendant survived: %v", err)
	}
}

func BenchmarkRun(b *testing.B) {
	for b.Loop() {
		if Run(context.Background(), Request{Argv: []string{"inspect"}}, Invocation{}) != 7 {
			b.Fatal("child failed")
		}
	}
}

type slowWriter struct{ bytes.Buffer }

func (w *slowWriter) Write(p []byte) (int, error) {
	time.Sleep(300 * time.Millisecond)
	return w.Buffer.Write(p)
}
func TestSlowOutputNotTruncated(t *testing.T) {
	var w slowWriter
	if s := Run(context.Background(), Request{Argv: []string{"sh", "-c", "printf complete"}}, Invocation{Env: []string{"PATH=" + testShellPath()}, Stdout: &w}); s != 0 || w.String() != "complete" {
		t.Fatalf("%d %q", s, w.String())
	}
}
func TestEmptyPathSearchesCWD(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "local"), []byte("#!"+testShell()+"\nexit 0"), 0755); e != nil {
		t.Fatal(e)
	}
	if s := Run(context.Background(), Request{Argv: []string{"local"}}, Invocation{Dir: dir, Env: []string{"PATH="}}); s != 0 {
		t.Fatal(s)
	}
}

func TestNestedCancellationKillsGroup(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	survived := filepath.Join(dir, "survived")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- Run(ctx, Request{Argv: []string{"nested", ready, survived}}, Invocation{}) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("descendant did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if s := <-done; s != 130 {
		t.Fatal(s)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, err := os.Stat(survived); !os.IsNotExist(err) {
		t.Fatalf("descendant survived: %v", err)
	}
}

func TestUmaskChildFallback(t *testing.T) {
	before, err := Umask()
	if err != nil {
		t.Fatal(err)
	}
	child, err := umaskChild()
	if err != nil {
		t.Fatal(err)
	}
	after, err := Umask()
	if err != nil || before != after || child != before {
		t.Fatalf("before=%o child=%o after=%o err=%v", before, child, after, err)
	}
}

func TestUnregisteredHost(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "--unregistered-test")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
}

func TestReniceTargets(t *testing.T) {
	// The helper's own pid is a valid target: the child reniced it instead of
	// the parent, so the assertion is about status and diagnostics only.
	before, err := Niceness()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	status := Renice(context.Background(), ReniceRequest{Priority: 1, Which: syscall.PRIO_PROCESS, IDs: []int{1 << 22}},
		Invocation{Stderr: &stderr})
	if status != 1 || !strings.Contains(stderr.String(), "renice: setpriority 4194304: ") {
		t.Fatalf("status %d stderr %q", status, stderr.String())
	}
	if after, err := Niceness(); err != nil || after != before {
		t.Fatalf("parent priority %d -> %d (%v)", before, after, err)
	}
}

func TestIgnoreHupSurvivesExec(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer
	status := Run(context.Background(), Request{
		DispatchOnly: true, IgnoreHup: true,
		Argv: []string{"sh", "-c", `kill -HUP $$; echo alive`},
	}, Invocation{Dir: dir, Env: []string{"PATH=" + testShellPath()}, Stderr: &stderr})
	if status != 0 {
		t.Fatalf("status %d stderr %q", status, stderr.String())
	}
	// Without IgnoreHup the same command dies from the default disposition.
	stderr.Reset()
	status = Run(context.Background(), Request{
		DispatchOnly: true,
		Argv:         []string{"sh", "-c", `kill -HUP $$; echo alive`},
	}, Invocation{Dir: dir, Env: []string{"PATH=" + testShellPath()}, Stderr: &stderr})
	if status != 128+1 {
		t.Fatalf("control status %d stderr %q", status, stderr.String())
	}
}

package session

import (
	"bytes"
	"context"
	"errors"
	"io"

	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/sh/expand"
)

func TestStateAndIsolation(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	cwdBefore, _ := os.Getwd()
	envBefore := os.Environ()
	var out bytes.Buffer
	s, err := New(Options{Dir: dir, Env: []string{"BASE=yes"}, Stdout: &out})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	run := func(src string) Result {
		t.Helper()
		r := s.Run(context.Background(), src)
		if r.Err != nil {
			t.Fatalf("%s: %v", src, r.Err)
		}
		return r
	}
	run("x=hello; f() { printf '%s:%s' \"$x\" \"$BASE\"; }; export x; cd child")
	run("f; false")
	run("printf ':%s' \"$?\"")
	if out.String() != "hello:yes:1" {
		t.Fatal(out.String())
	}
	if s.Dir() != child {
		t.Fatal(s.Dir())
	}
	if s.Vars()["x"].Str != "hello" || !s.Vars()["x"].Exported {
		t.Fatal(s.Vars())
	}
	if !strings.Contains(strings.Join(s.Env(), "\n"), "x=hello") {
		t.Fatal(s.Env())
	}
	s2, _ := New(Options{Dir: dir, Env: []string{}})
	defer s2.Close()
	if r := s2.Run(context.Background(), "f"); r.Status != 127 {
		t.Fatalf("function leaked: %+v", r)
	}
	if s2.Dir() != dir || s2.Vars()["x"].IsSet() {
		t.Fatal("state leaked")
	}
	cwd, _ := os.Getwd()
	if cwd != cwdBefore || !slices.Equal(os.Environ(), envBefore) {
		t.Fatal("process changed")
	}
}
func TestParseAndExit(t *testing.T) {
	s, _ := New(Options{Env: []string{}})
	defer s.Close()
	s.Run(context.Background(), "x=before")
	r := s.Run(context.Background(), "x=after; if")
	var se *SyntaxErr
	if r.Status != 2 || !errors.As(r.Err, &se) || r.ParseError != se || se.Line == 0 || se.Column == 0 {
		t.Fatalf("%+v", r)
	}
	if s.Vars()["x"].Str != "before" {
		t.Fatal(s.Vars())
	}
	r = s.Run(context.Background(), "exit 7")
	if !r.Exited || r.Status != 7 || r.Err != nil {
		t.Fatalf("%+v", r)
	}
	if r = s.Run(context.Background(), "true"); !errors.Is(r.Err, ErrExited) {
		t.Fatalf("%+v", r)
	}
}
func TestCancelClose(t *testing.T) {
	started := make(chan struct{})
	s, _ := New(Options{Exec: func(ctx context.Context, i Invocation) int { close(started); <-ctx.Done(); return 1 }})
	done := make(chan Result)
	go func() { done <- s.Run(context.Background(), "block") }()
	<-started
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	r := <-done
	if r.Status != 130 {
		t.Fatalf("%+v", r)
	}
	if s.Close() != nil {
		t.Fatal("close")
	}
	if !errors.Is(s.Run(context.Background(), "true").Err, ErrExited) {
		t.Fatal("not sealed")
	}
}
func TestCancelRecoveryAndExec(t *testing.T) {
	var seen Invocation
	s, _ := New(Options{Env: []string{"A=B"}, Exec: func(ctx context.Context, i Invocation) int { seen = i; <-ctx.Done(); return 3 }})
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	r := s.Run(ctx, "waiter 'a b'")
	if r.Status != 124 || !errors.Is(r.Err, context.DeadlineExceeded) {
		t.Fatalf("%+v", r)
	}
	if seen.Name != "waiter" || len(seen.Args) != 1 || seen.Args[0] != "a b" || !strings.Contains(strings.Join(seen.Env, "\n"), "A=B") {
		t.Fatalf("%+v", seen)
	}
	if r = s.Run(context.Background(), "true"); r.Status != 0 || r.Err != nil {
		t.Fatalf("%+v", r)
	}
}
func TestSnapshotsAndDefaultExec(t *testing.T) {
	s, _ := New(Options{Env: []string{"A=B"}})
	defer s.Close()
	s.Run(context.Background(), "v=one")
	v := s.Vars()
	delete(v, "v")
	e := s.Env()
	e[0] = "BAD=x"
	if s.Vars()["v"].Str != "one" || strings.Contains(strings.Join(s.Env(), " "), "BAD=x") {
		t.Fatal("snapshot alias")
	}
	if r := s.Run(context.Background(), "unknown-command"); r.Status != 127 || r.Err != nil {
		t.Fatalf("%+v", r)
	}
}
func TestDependencies(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", ".")
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "bubbletea") {
		t.Fatal("bubbletea dependency")
	}
}

func TestStatusAcrossSubmissions(t *testing.T) {
	var out bytes.Buffer
	s, _ := New(Options{Env: []string{}, Stdout: &out, Exec: func(ctx context.Context, _ Invocation) int { <-ctx.Done(); return 1 }})
	defer s.Close()
	s.Run(context.Background(), "false")
	s.Run(context.Background(), "")
	s.Run(context.Background(), "if")
	s.Run(context.Background(), "printf '%s' \"$?\"")
	if out.String() != "1" {
		t.Fatalf("status after empty/parse: %q", out.String())
	}
	out.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx, "anything")
	s.Run(context.Background(), "printf '%s' \"$?\"")
	if out.String() != "130" {
		t.Fatalf("cancel status %q", out.String())
	}
}

func TestEnvironmentAndUnset(t *testing.T) {
	t.Setenv("MISA_SESSION_SNAPSHOT_TEST", "original")
	s, _ := New(Options{})
	defer s.Close()
	empty, _ := New(Options{Env: []string{}})
	defer empty.Close()
	if !strings.Contains(strings.Join(s.Env(), "\n"), "MISA_SESSION_SNAPSHOT_TEST=original") || strings.Contains(strings.Join(empty.Env(), "\n"), "MISA_SESSION_SNAPSHOT_TEST=") {
		t.Fatal("nil/empty env")
	}
	t.Setenv("MISA_SESSION_SNAPSHOT_TEST", "changed")
	if s.Vars()["MISA_SESSION_SNAPSHOT_TEST"].Str != "original" {
		t.Fatal("not snapshotted")
	}
	s.Run(context.Background(), "private=value; export PUBLIC=value; unset MISA_SESSION_SNAPSHOT_TEST")
	e := strings.Join(s.Env(), "\n")
	if strings.Contains(e, "private=") || strings.Contains(e, "MISA_SESSION_SNAPSHOT_TEST=") || !strings.Contains(e, "PUBLIC=value") {
		t.Fatal(e)
	}
	if r := empty.Run(context.Background(), "f"); r.Status != 127 {
		t.Fatalf("function leaked: %+v", r)
	}
}

type trackedReader struct{ closed bool }

func (r *trackedReader) Read([]byte) (int, error) { return 0, io.EOF }
func (r *trackedReader) Close() error             { r.closed = true; return nil }
func TestStreamOwnershipAndDeepSnapshot(t *testing.T) {
	in := &trackedReader{}
	s, err := New(Options{Stdin: in})
	if err != nil {
		t.Fatal(err)
	}
	s.runner.Vars["array"] = expand.Variable{Set: true, Kind: expand.Indexed, List: []string{"before"}, Map: map[string]string{"k": "before"}}
	snapshot := s.Vars()
	v := snapshot["array"]
	v.List[0] = "changed"
	v.Map["k"] = "changed"
	if got := s.Vars()["array"]; got.List[0] != "before" || got.Map["k"] != "before" {
		t.Fatal(got)
	}
	s.Close()
	if in.closed {
		t.Fatal("caller input closed")
	}
}

func TestInvalidDirDoesNotReadInput(t *testing.T) {
	r := &readCount{}
	_, err := New(Options{Dir: filepath.Join(t.TempDir(), "missing"), Stdin: r})
	if err == nil {
		t.Fatal("expected error")
	}
	if r.read {
		t.Fatal("input consumed")
	}
}

type readCount struct{ read bool }

func (r *readCount) Read([]byte) (int, error) { r.read = true; return 0, io.EOF }

func TestInitialVarsAndDeadlineStatus(t *testing.T) {
	var out bytes.Buffer
	s, err := New(Options{Env: []string{}, Stdout: &out, Exec: func(ctx context.Context, _ Invocation) int { <-ctx.Done(); return 0 }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Vars()["PWD"].Str != s.Dir() {
		t.Fatal("missing initial PWD")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	r := s.Run(ctx, "blocked")
	if r.Status != 124 {
		t.Fatal(r)
	}
	r = s.Run(context.Background(), "printf '%s' \"$?\"")
	if r.Status != 0 || out.String() != "124" {
		t.Fatalf("%+v %q", r, out.String())
	}
}

func TestNilInputIsEmpty(t *testing.T) {
	var stderr bytes.Buffer
	s, err := New(Options{Env: []string{}, Stderr: &stderr})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := s.Run(context.Background(), "read value")
	if r.Status != 1 || r.Err != nil || stderr.Len() != 0 {
		t.Fatalf("empty read: %+v, stderr %q", r, stderr.String())
	}
}

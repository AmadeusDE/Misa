package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/tools"
	"github.com/AmadeusDE/Misa/internal/ugh/worktree"
)

func start(t *testing.T) Machine {
	t.Helper()
	m, err := Local{}.Start(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func args(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestToolsRunAtTheFarEnd(t *testing.T) {
	m := start(t)
	dir := t.TempDir()
	ctx := context.Background()
	if _, err := m.RunTool(ctx, "write", dir, args(map[string]string{"path": "a.txt", "content": "hello\n"})); err != nil {
		t.Fatal(err)
	}
	out, err := m.RunTool(ctx, "bash", dir, args(map[string]string{"command": "cat a.txt; echo err >&2"}))
	if err != nil || !strings.Contains(out, "hello") {
		t.Fatalf("bash = %q, %v", out, err)
	}
	if _, err := m.RunTool(ctx, "read", dir, args(map[string]string{"path": "missing"})); err == nil {
		t.Fatal("a tool's error must come back as an error")
	}
	if _, err := m.RunTool(ctx, "nope", dir, nil); err == nil {
		t.Fatal("unknown tool")
	}
}

func TestProgressScratchAndBlobCrossTheStream(t *testing.T) {
	m := start(t)
	dir, scratch, blobs := t.TempDir(), filepath.Join(t.TempDir(), "s"), t.TempDir()
	var tails []string
	ctx := tools.WithProgress(context.Background(), func(s string) { tails = append(tails, s) })
	ctx = tools.WithScratch(ctx, scratch)
	ctx = tools.WithBlob(ctx, tools.Blob{Dir: blobs, Alias: "o1"})
	out, err := m.RunTool(ctx, "bash", dir, args(map[string]string{"command": `echo "$UGH_SCRATCH"; for i in 1 2 3; do echo tick $i; sleep 0.3; done; seq 1 20000`}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, scratch) {
		t.Fatalf("scratch not passed: %.200q", out)
	}
	if len(tails) == 0 {
		t.Fatal("no progress came back")
	}
	full, err := os.ReadFile(filepath.Join(blobs, "o1.txt"))
	if err != nil || !strings.Contains(string(full), "\n19999\n") {
		t.Fatalf("blob not saved here: %v", err)
	}
}

func TestCancellingACallStopsItThere(t *testing.T) {
	m := start(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	began := time.Now()
	_, err := m.RunTool(ctx, "bash", t.TempDir(), args(map[string]string{"command": "sleep 30"}))
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(began) > 5*time.Second {
		t.Fatalf("err = %v after %s", err, time.Since(began))
	}
	if out, err := m.RunTool(context.Background(), "bash", t.TempDir(), args(map[string]string{"command": "echo still"})); err != nil || !strings.Contains(out, "still") {
		t.Fatalf("machine unusable after a cancel: %q %v", out, err)
	}
}

func TestGitRunsAtTheFarEnd(t *testing.T) {
	m := start(t)
	dir := t.TempDir()
	ctx := worktree.WithGit(context.Background(), m.Git)
	if _, _, err := worktree.Git(ctx, dir, nil, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	common, err := worktree.FindGitCommonDir(ctx, dir)
	if err != nil || common != filepath.Join(dir, ".git") {
		t.Fatalf("common = %q, %v", common, err)
	}
	if _, stderr, err := worktree.Git(ctx, dir, nil, "rev-parse", "HEAD"); err == nil || len(stderr) == 0 {
		t.Fatalf("a failing git must return its stderr: %q %v", stderr, err)
	}
}

func TestALostStreamFailsCalls(t *testing.T) {
	r, w := io.Pipe()
	c := NewClient(strings.NewReader(""), w)
	go io.Copy(io.Discard, r)
	<-c.Done()
	if _, err := c.RunTool(context.Background(), "bash", "/", nil); !errors.Is(err, ErrLost) {
		t.Fatalf("err = %v", err)
	}
}

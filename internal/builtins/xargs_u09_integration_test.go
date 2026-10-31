package builtins

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestXargsOversizedEnvironment(t *testing.T) {
	for _, envSize := range []int{128 * 1024, 128*1024 - len("worker") - 2} {
		// The second environment leaves room for the fixed command, but not
		// even the first input item. Neither case may start a child.
		var stderr bytes.Buffer
		called := false
		inv := &Context{Args: []string{"worker"}, Env: []string{"BIG=" + strings.Repeat("x", envSize-4)}, Stdin: strings.NewReader("item"), Stdout: io.Discard, Stderr: &stderr, Exec: func(context.Context, string, *Context) int { called = true; return 0 }}
		code := Run(context.Background(), "xargs", inv)
		if code != 1 || stderr.String() != "xargs: argument list too long\n" || called {
			t.Fatalf("envSize=%d status=%d stderr=%q called=%v", envSize, code, stderr.String(), called)
		}
	}
}

func TestFindPrint0Xargs0Builtins(t *testing.T) {
	dir := t.TempDir()
	names := []string{"space name", "line\nbreak", "-leading", "quote'\"name", `back\slash`}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content:"+name+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var paths, findErr bytes.Buffer
	find := &Context{Dir: dir, Env: []string{}, Args: []string{".", "-type", "f", "-print0"}, Stdin: strings.NewReader(""), Stdout: &paths, Stderr: &findErr}
	if code := Run(context.Background(), "find", find); code != 0 {
		t.Fatalf("find status=%d stderr=%q", code, findErr.String())
	}
	// Run a real builtin through xargs; -n1 preserves each NUL-delimited path.
	var output, stderr bytes.Buffer
	inv := &Context{Dir: dir, Env: []string{}, Args: []string{"-0", "-n1", "cat", "--"}, Stdin: bytes.NewReader(paths.Bytes()), Stdout: &output, Stderr: &stderr}
	if code := Run(context.Background(), "xargs", inv); code != 0 {
		t.Fatalf("xargs status=%d stderr=%q", code, stderr.String())
	}
	// Contents contain newlines themselves, so compare whole records in find's sorted order.
	sort.Strings(names)
	var want strings.Builder
	for _, name := range names {
		want.WriteString("content:" + name + "\n")
	}
	if !reflect.DeepEqual(output.String(), want.String()) {
		t.Fatalf("output=%q want=%q paths=%q", output.String(), want.String(), paths.String())
	}
}

func TestFindPrint0Xargs0ParallelPipe(t *testing.T) {
	dir := t.TempDir()
	names := []string{"space name", "line\nbreak", "-leading", "quote'\"name", `back\slash`}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	findDone := make(chan int, 1)
	var findErr bytes.Buffer
	go func() {
		code := Run(context.Background(), "find", &Context{Dir: dir, Env: []string{}, Args: []string{".", "-type", "f", "-print0"}, Stdin: strings.NewReader(""), Stdout: w, Stderr: &findErr})
		_ = w.Close()
		findDone <- code
	}()
	var mu sync.Mutex
	var got []string
	var stderr bytes.Buffer
	inv := &Context{Dir: dir, Env: []string{}, Args: []string{"-0", "-P2", "-n1", "worker"}, Stdin: r, Stdout: io.Discard, Stderr: &stderr}
	inv.Exec = func(_ context.Context, _ string, c *Context) int {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, c.Args...)
		return 0
	}
	code := Run(context.Background(), "xargs", inv)
	// Closing the reader also unblocks find if xargs rejects an option early.
	_ = r.Close()
	findCode := xargsWait(t, findDone)
	if code != 0 || findCode != 0 {
		t.Fatalf("xargs=%d find=%d xargs stderr=%q find stderr=%q", code, findCode, stderr.String(), findErr.String())
	}
	want := make([]string, len(names))
	for i, name := range names {
		want[i] = "./" + name
	}
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths=%q want=%q", got, want)
	}
}

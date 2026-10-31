package builtins

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFindExecDirAndBatchBoundary(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	var calls []string
	ctx := &Context{Dir: dir, Env: []string{"PATH=/bin"}, Exec: func(_ context.Context, cmd string, sub *Context) int {
		calls = append(calls, sub.Dir+":"+strings.Join(sub.Args, ","))
		return 0
	}}
	p := findParser{ctx: ctx, args: []string{"-execdir", "external", "{}", "+"}}
	node, err := p.parse()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a/one", "a/two", "b/three"} {
		node(&findEntry{ctx: ctx, path: n, full: filepath.Join(dir, n)})
	}
	if p.flushExec() {
		t.Fatal("unexpected failure")
	}
	want := []string{filepath.Join(dir, "a") + ":./one,./two", filepath.Join(dir, "b") + ":./three"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls %#v want %#v", calls, want)
	}
}

func TestFindExecdirBatchRejectsSwappedRootFileParent(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	launches := 0
	ctx := &Context{Dir: dir, Env: []string{"PATH=/bin"}, Stderr: &stderr, Args: []string{"sub/file", "-execdir", "capture", "{}", "+", ",", "-exec", "swap", ";"}, Exec: func(_ context.Context, cmd string, _ *Context) int {
		if cmd == "capture" {
			launches++
			return 0
		}
		if err := os.Rename(sub, filepath.Join(dir, "saved")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, sub); err != nil {
			t.Fatal(err)
		}
		return 0
	}}
	if status := Run(context.Background(), "find", ctx); status == 0 || launches != 0 {
		t.Fatalf("status %d launches %d stderr %q", status, launches, stderr.String())
	}
}

func TestFindExecdirFlushBeforeDepthDirectoryAction(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "tree")
	sub := filepath.Join(tree, "sub")
	if err := os.MkdirAll(sub, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var calls int
	var stderr strings.Builder
	ctx := &Context{Dir: dir, Env: []string{"PATH=/bin"}, Stderr: &stderr, Args: []string{"tree", "-type", "f", "-execdir", "capture", "{}", "+", "-o", "-delete"}, Exec: func(_ context.Context, _ string, subctx *Context) int {
		calls++
		if subctx.Dir != sub || !reflect.DeepEqual(subctx.Args, []string{"./file"}) {
			t.Errorf("dir %q args %q", subctx.Dir, subctx.Args)
		}
		if _, err := os.Stat(subctx.Dir); err != nil {
			t.Errorf("directory unavailable before batch: %v", err)
		}
		// Simulate capture consuming the file, so -delete can remove its parent.
		if err := os.Remove(filepath.Join(subctx.Dir, "file")); err != nil {
			t.Errorf("remove: %v", err)
		}
		return 0
	}}
	status := Run(context.Background(), "find", ctx)
	if status != 0 || calls != 1 {
		t.Fatalf("status %d calls %d stderr %q", status, calls, stderr.String())
	}
	if _, err := os.Stat(tree); !os.IsNotExist(err) {
		t.Fatalf("tree remains: %v", err)
	}
}

func TestFindExecdirFlushMultipleNestedBatches(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"tree/a/deep", "tree/b"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "f"), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var events []string
	var stderr strings.Builder
	ctx := &Context{Dir: dir, Env: []string{"PATH=/bin"}, Stderr: &stderr, Args: []string{"tree", "-type", "f", "(", "-execdir", "one", "{}", "+", ",", "-execdir", "two", "{}", "+", ")", "-o", "-delete"}, Exec: func(_ context.Context, cmd string, sub *Context) int {
		events = append(events, filepath.Base(sub.Dir)+":"+cmd)
		if cmd == "two" {
			if err := os.Remove(filepath.Join(sub.Dir, sub.Args[0])); err != nil {
				t.Error(err)
				return 1
			}
		}
		return 0
	}}
	if status := Run(context.Background(), "find", ctx); status != 0 {
		t.Fatalf("status %d stderr %q", status, stderr.String())
	}
	if want := []string{"deep:one", "deep:two", "b:one", "b:two"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events %q want %q", events, want)
	}
}

func TestFindExecdirFlushBeforeMindepthSkip(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"tree/a", "tree/z/check"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "tree/a/file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var events []string
	ctx := &Context{Dir: dir, Env: []string{"PATH=/bin"}, Args: []string{"tree", "-depth", "-mindepth", "2", "(", "-type", "f", "-execdir", "batch", "{}", "+", ")", "-o", "(", "-type", "d", "-exec", "check", ";", ")"}, Exec: func(_ context.Context, cmd string, _ *Context) int { events = append(events, cmd); return 0 }}
	if status := Run(context.Background(), "find", ctx); status != 0 || !reflect.DeepEqual(events, []string{"batch", "check"}) {
		t.Fatalf("status %d events %q", status, events)
	}
}

func TestFindExecdirRootLexicalNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ root, dir, name string }{
		{".", dir, "./."}, {"./", dir, "./."}, {"sub/.", filepath.Join(dir, "sub"), "./."}, {"sub/", dir, "./sub"}, {"..", dir, "./.."}, {"/", "/", "./."},
	} {
		t.Run(tc.root, func(t *testing.T) {
			var gotDir string
			var gotArgs []string
			ctx := &Context{Dir: dir, Env: []string{"PATH=/bin"}, Args: []string{tc.root, "-maxdepth", "0", "-execdir", "capture", "{}", ";"}, Exec: func(_ context.Context, _ string, sub *Context) int { gotDir = sub.Dir; gotArgs = sub.Args; return 0 }}
			if status := Run(context.Background(), "find", ctx); status != 0 || gotDir != tc.dir || !reflect.DeepEqual(gotArgs, []string{tc.name}) {
				t.Fatalf("status %d dir %q args %q want %q %q", status, gotDir, gotArgs, tc.dir, tc.name)
			}
		})
	}
}

func TestFindExecdirLiteralSymlinkDotDot(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(outside, "victim"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "victim", "file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "sub"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	var gotDir string
	var gotArgs []string
	ctx := &Context{Dir: dir, Env: []string{"PATH=/bin"}, Args: []string{"link/../victim/file", "-execdir", "capture", "{}", ";"}, Exec: func(_ context.Context, _ string, sub *Context) int { gotDir = sub.Dir; gotArgs = sub.Args; return 0 }}
	if status := Run(context.Background(), "find", ctx); status != 0 || gotDir != filepath.Join(outside, "victim") || !reflect.DeepEqual(gotArgs, []string{"./file"}) {
		t.Fatalf("status %d cwd %q args %q", status, gotDir, gotArgs)
	}
}

func TestFindExecdirBatchValidatesEveryEntry(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	reject := false
	var stderr strings.Builder
	ctx := &Context{Dir: dir, Env: []string{"PATH=/bin"}, Stderr: &stderr, Exec: func(context.Context, string, *Context) int { calls++; return 0 }}
	p := findParser{ctx: ctx, args: []string{"-execdir", "capture", "{}", "+"}}
	node, err := p.parse()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"first", "second"} {
		name := n
		node(&findEntry{ctx: ctx, path: n, full: filepath.Join(dir, n), validate: func() error {
			if name == "second" && calls == 0 && reject {
				return errors.New("changed")
			}
			return nil
		}})
	}
	reject = true
	if !p.flushExec() || calls != 0 {
		t.Fatalf("calls %d stderr %q", calls, stderr.String())
	}
}

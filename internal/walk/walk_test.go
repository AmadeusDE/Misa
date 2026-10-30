package walk

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func tree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"a/b", "c"} {
		os.MkdirAll(filepath.Join(dir, d), 0o755)
	}
	for _, f := range []string{"a/x", "a/b/y", "c/z", "top", "with space", "-dash", "new\nline"} {
		os.WriteFile(filepath.Join(dir, f), nil, 0o644)
	}
	return dir
}

func collect(t *testing.T, root string, opt Options) []string {
	t.Helper()
	var got []string
	err := Walk(context.Background(), root, opt, func(e Entry) error {
		if e.Err != nil {
			got = append(got, "ERR "+e.Path)
			return nil
		}
		got = append(got, e.Path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestWalkOrderAndNames(t *testing.T) {
	dir := tree(t)
	got := collect(t, ".", Options{Base: dir})
	want := []string{".", "./-dash", "./a", "./a/b", "./a/b/y", "./a/x", "./c", "./c/z", "./new\nline", "./top", "./with space"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestWalkEmptyRootNamesRelative(t *testing.T) {
	dir := tree(t)
	got := collect(t, "", Options{Base: filepath.Join(dir, "a")})
	want := []string{"", "b", "b/y", "x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWalkDepthAndInfo(t *testing.T) {
	dir := tree(t)
	Walk(context.Background(), "a", Options{Base: dir}, func(e Entry) error {
		want := map[string]int{"a": 0, "a/b": 1, "a/x": 1, "a/b/y": 2}[e.Path]
		if e.Depth != want {
			t.Errorf("%s depth %d, want %d", e.Path, e.Depth, want)
		}
		if (e.Path == "a/b") != (e.Info.IsDir() && e.Path != "a") {
			t.Errorf("%s IsDir = %v", e.Path, e.Info.IsDir())
		}
		return nil
	})
}

func TestWalkSkipDir(t *testing.T) {
	dir := tree(t)
	var got []string
	Walk(context.Background(), "a", Options{Base: dir}, func(e Entry) error {
		got = append(got, e.Path)
		if e.Path == "a/b" {
			return SkipDir
		}
		return nil
	})
	if want := []string{"a", "a/b", "a/x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWalkStopsOnError(t *testing.T) {
	dir := tree(t)
	stop := errors.New("stop")
	err := Walk(context.Background(), "a", Options{Base: dir}, func(e Entry) error {
		if e.Path == "a/b" {
			return stop
		}
		return nil
	})
	if err != stop {
		t.Errorf("err = %v, want stop", err)
	}
}

func TestWalkSymlinks(t *testing.T) {
	dir := tree(t)
	os.Symlink("../c", filepath.Join(dir, "a", "linkdir"))
	os.Symlink("nowhere", filepath.Join(dir, "a", "dangling"))
	os.Symlink("a", filepath.Join(dir, "rootlink"))

	got := collect(t, "a", Options{Base: dir})
	want := []string{"a", "a/b", "a/b/y", "a/dangling", "a/linkdir", "a/x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("default: got %q, want %q", got, want)
	}
	Walk(context.Background(), "a", Options{Base: dir}, func(e Entry) error {
		if e.Path == "a/linkdir" && e.Info.Mode()&os.ModeSymlink == 0 {
			t.Error("unfollowed link should report the link itself")
		}
		return nil
	})

	got = collect(t, "a", Options{Base: dir, FollowLinks: true})
	want = []string{"a", "a/b", "a/b/y", "ERR a/dangling", "a/linkdir", "a/linkdir/z", "a/x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("follow: got %q, want %q", got, want)
	}

	if got := collect(t, "rootlink", Options{Base: dir}); len(got) != 1 {
		t.Errorf("root link without FollowRoot: got %q", got)
	}
	if got := collect(t, "rootlink", Options{Base: dir, FollowRoot: true}); len(got) != 6 {
		t.Errorf("root link with FollowRoot: got %q", got)
	}
}

func TestWalkLoop(t *testing.T) {
	dir := tree(t)
	os.Symlink("..", filepath.Join(dir, "a", "b", "up"))
	var loops []string
	Walk(context.Background(), "a", Options{Base: dir, FollowLinks: true}, func(e Entry) error {
		if errors.Is(e.Err, ErrLoop) {
			loops = append(loops, e.Path)
		}
		return nil
	})
	if want := []string{"a/b/up"}; !reflect.DeepEqual(loops, want) {
		t.Errorf("loops = %q, want %q", loops, want)
	}
}

func TestWalkUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	dir := tree(t)
	locked := filepath.Join(dir, "a", "b")
	os.Chmod(locked, 0)
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	got := collect(t, "a", Options{Base: dir})
	want := []string{"a", "a/b", "ERR a/b", "a/x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWalkMissingRoot(t *testing.T) {
	got := collect(t, "nope", Options{Base: t.TempDir()})
	if want := []string{"ERR nope"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWalkCancel(t *testing.T) {
	dir := tree(t)
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	err := Walk(ctx, ".", Options{Base: dir}, func(e Entry) error {
		n++
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || n != 1 {
		t.Errorf("err = %v after %d entries, want context.Canceled after 1", err, n)
	}
}

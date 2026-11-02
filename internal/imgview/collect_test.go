package imgview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func collectFile(t *testing.T, root, name string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCollectNaturalAndMixed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	for _, n := range []string{"img10.png", "IMG3.PNG", "img2.png", "img1.png", "x.txt", "photo.JpG"} {
		collectFile(t, dir, n)
	}
	a := collectFile(t, root, "a.png")
	weird := filepath.Join(root, "weird.ext")
	got, err := CollectPaths([]string{a, dir, weird}, CollectOptions{})
	want := []string{a, filepath.Join(dir, "img1.png"), filepath.Join(dir, "img2.png"), filepath.Join(dir, "IMG3.PNG"), filepath.Join(dir, "img10.png"), filepath.Join(dir, "photo.JpG"), weird}
	if err != nil || !reflect.DeepEqual(got.Paths, want) {
		t.Fatalf("got %+v, %v; want %v", got, err, want)
	}
	missing, err := CollectPaths([]string{filepath.Join(root, "missing")}, CollectOptions{})
	if err != nil || len(missing.Paths) != 1 {
		t.Fatalf("missing: %+v %v", missing, err)
	}
}

func TestCollectHiddenRecursiveDepth(t *testing.T) {
	root := t.TempDir()
	a := collectFile(t, root, "a.png")
	z := collectFile(t, root, "z.png")
	b := collectFile(t, root, "sub/b.png")
	c := collectFile(t, root, "sub/deep/c.png")
	hidden := collectFile(t, root, ".hidden.png")
	inHidden := collectFile(t, root, ".dir/inside.png")
	cases := []struct {
		name string
		opts CollectOptions
		want []string
	}{
		{"flat", CollectOptions{}, []string{a, z}},
		{"recursive", CollectOptions{Recursive: true}, []string{a, b, c, z}},
		{"depth1", CollectOptions{Recursive: true, MaxDepth: 1}, []string{a, z}},
		{"depth2", CollectOptions{Recursive: true, MaxDepth: 2}, []string{a, b, z}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CollectPaths([]string{root}, tc.opts)
			if err != nil || !reflect.DeepEqual(got.Paths, tc.want) {
				t.Fatalf("got %+v %v want %v", got, err, tc.want)
			}
		})
	}
	got, err := CollectPaths([]string{hidden, filepath.Join(root, ".dir")}, CollectOptions{})
	if err != nil || !reflect.DeepEqual(got.Paths, []string{hidden, inHidden}) {
		t.Fatalf("hidden: %+v %v", got, err)
	}
}

func TestCollectSymlinks(t *testing.T) {
	root := t.TempDir()
	target := collectFile(t, root, "sub/a.png")
	for _, link := range []struct{ old, new string }{{"sub", "dirlink"}, {"..", "sub/loop"}, {target, "linked.png"}} {
		if err := os.Symlink(link.old, filepath.Join(root, link.new)); err != nil {
			t.Skipf("symlink: %v", err)
		}
	}
	got, err := CollectPaths([]string{root}, CollectOptions{Recursive: true})
	want := []string{filepath.Join(root, "linked.png"), target}
	if err != nil || !reflect.DeepEqual(got.Paths, want) {
		t.Fatalf("got %+v %v want %v", got, err, want)
	}
}

func TestCollectLimitsAndEmpty(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"a.png", "b.png", "c.png"} {
		collectFile(t, root, n)
	}
	for _, opts := range []CollectOptions{{MaxFiles: 2}, {MaxEntriesScanned: 2}} {
		explicit := filepath.Join(root, "missing")
		got, err := CollectPaths([]string{root, explicit}, opts)
		if err != nil || len(got.Paths) != 3 || got.Paths[2] != explicit || !strings.Contains(got.Warning, "limit reached") {
			t.Fatalf("opts %+v: %+v %v", opts, got, err)
		}
	}
	_, err := CollectPaths([]string{t.TempDir()}, CollectOptions{})
	if !errors.Is(err, ErrNoImages) {
		t.Fatalf("empty: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = CollectPathsContext(ctx, []string{root}, CollectOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestCollectWarningText(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"a.png", "b.png", "c.png"} {
		collectFile(t, root, n)
	}
	got, err := CollectPaths([]string{root}, CollectOptions{MaxFiles: 2})
	if err != nil || len(got.Paths) != 2 || got.Warning != "showing first 2 images; limit reached" {
		t.Fatalf("got %+v %v", got, err)
	}
	if got, err := CollectPaths([]string{root}, CollectOptions{}); err != nil || got.Warning != "" {
		t.Fatalf("unexpected warning: %+v %v", got, err)
	}
}

func TestImageName(t *testing.T) {
	for _, ext := range imageExtensions {
		if !isImageName("x" + strings.ToUpper(ext)) {
			t.Errorf("extension %s", ext)
		}
	}
	for _, n := range []string{"x.txt", "x.png.txt", ""} {
		if isImageName(n) {
			t.Errorf("accepted %q", n)
		}
	}
}

func TestCollectDepthWarningAndNaturalZeros(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"img01.png", "img1.png", "img001.png", "nested/x.png"} {
		collectFile(t, root, name)
	}
	got, err := CollectPaths([]string{root}, CollectOptions{Recursive: true, MaxDepth: 1})
	want := []string{filepath.Join(root, "img1.png"), filepath.Join(root, "img01.png"), filepath.Join(root, "img001.png")}
	if err != nil || !reflect.DeepEqual(got.Paths, want) || !strings.Contains(got.Warning, "limit reached") {
		t.Fatalf("got %+v %v, want %v and warning", got, err, want)
	}
}

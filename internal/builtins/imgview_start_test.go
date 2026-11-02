package builtins

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestImgviewStartFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "a.png"), filepath.Join(dir, "b; c.png")}
	for _, start := range []string{"b; c.png", paths[1], filepath.Join("sub", "..", "b; c.png")} {
		v, err := ImgviewViewer([]string{"--start", start, "--", "a.png", "b; c.png"}, dir)
		if err != nil {
			t.Fatalf("start %q: %v", start, err)
		}
		if v.Index() != 1 || !reflect.DeepEqual(v.Paths(), paths) {
			t.Fatalf("start %q: index=%d paths=%v", start, v.Index(), v.Paths())
		}
	}
}

func TestImgviewStartDirectoryKeepsOrder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	for _, name := range []string{"img10.png", "img2.png", "img1.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err := ImgviewViewer([]string{"--start", "img10.png", "."}, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "img1.png"), filepath.Join(dir, "img2.png"), filepath.Join(dir, "img10.png")}
	if v.Index() != 2 || !reflect.DeepEqual(v.Paths(), want) {
		t.Fatalf("index=%d paths=%v, want natural directory order %v", v.Index(), v.Paths(), want)
	}
}

func TestImgviewStartErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--start"}, "requires"},
		{[]string{"--start=", "a.png"}, "start file"},
		{[]string{"--start", "b.png", "a.png"}, "not in the image collection"},
	} {
		_, err := ImgviewViewer(tt.args, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: got %v, want %q", tt.args, err, tt.want)
		}
	}
}

func TestImgviewBrowserCollectionNotice(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MISA_IMGVIEW_LIMITED", "1")
	for _, embedded := range []string{"0", "1"} {
		t.Setenv("MISA_IMGVIEW_EMBEDDED", embedded)
		v, err := ImgviewViewer([]string{"a.png"}, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		v.Resize(160, 4)
		if got := strings.Contains(v.View(), "collection truncated"); got != (embedded == "1") {
			t.Fatalf("embedded=%s: limit notice present=%v", embedded, got)
		}
	}
}

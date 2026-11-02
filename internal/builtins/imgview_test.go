package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImgviewArguments(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	for _, tt := range []struct {
		args []string
		want string
	}{
		{nil, "FILE"},
		{[]string{"--renderer=bad", "a"}, "unknown renderer"},
		{[]string{"--renderer"}, "requires"},
		{[]string{"."}, "directories"},
		{[]string{"--unknown", "a"}, "unknown"},
	} {
		_, err := ImgviewViewer(tt.args, dir)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: got %v, want %q", tt.args, err, tt.want)
		}
	}
	for _, args := range [][]string{{"missing.png"}, {"--renderer=kitty", "a"}, {"--thumbnail", "--renderer=halfblock", "a", "b"}, {"--", "-name.png"}} {
		if _, err := ImgviewViewer(args, dir); err != nil {
			t.Errorf("%q: %v", args, err)
		}
	}
}

func TestImgviewHelpVersionAndErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, tt := range []struct {
		args   []string
		status int
		output string
	}{
		{[]string{"--help"}, 0, "--thumbnail"},
		{[]string{"--version"}, 0, "imgview"},
		{nil, 1, "FILE"},
	} {
		var out, stderr bytes.Buffer
		ctx := &Context{CmdName: "imgview", Args: tt.args, Stdout: &out, Stderr: &stderr}
		code := runImgview(ctx)
		if code != tt.status || !strings.Contains(out.String()+stderr.String(), tt.output) {
			t.Errorf("%v: code %d stdout %q stderr %q", tt.args, code, out.String(), stderr.String())
		}
	}
	if _, ok := Lookup("imgview"); !ok {
		t.Fatal("imgview not registered")
	}
}

func TestImgviewConfigAndOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "misa", "imageviewer.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("initial_mode: thumbnails\nrenderer: kitty\n")
	configured, err := ImgviewViewer([]string{"a.png"}, dir)
	if err != nil {
		t.Fatalf("configured renderer: %v", err)
	}
	configured.Resize(100, 4)
	if !strings.Contains(configured.View(), "kitty unavailable; using halfblock") {
		t.Fatal("configured kitty fallback warning missing")
	}
	viewer, err := ImgviewViewer([]string{"--renderer=halfblock", "a.png"}, dir)
	if err != nil || !viewer.Thumbnail() {
		t.Fatalf("CLI renderer override / configured thumbnail: %v", err)
	}
	viewer.Resize(100, 4)
	if strings.Contains(viewer.View(), "kitty unavailable") {
		t.Fatal("override retained kitty warning")
	}
	viewer, err = ImgviewViewer([]string{"--renderer=kitty", "a.png"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	viewer.Resize(100, 4)
	if !strings.Contains(viewer.View(), "kitty unavailable; using halfblock") {
		t.Fatal("CLI kitty fallback warning missing")
	}
	write("initial_mode: single\nrenderer: halfblock\n")
	viewer, err = ImgviewViewer([]string{"a.png"}, dir)
	if err != nil || viewer.Thumbnail() {
		t.Fatalf("configured single mode: %v", err)
	}
	viewer, err = ImgviewViewer([]string{"--thumbnail", "--renderer=auto", "a.png"}, dir)
	if err != nil || !viewer.Thumbnail() {
		t.Fatalf("CLI thumbnail override: %v", err)
	}
}

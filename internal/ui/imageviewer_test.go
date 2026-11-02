package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/imgview"
)

func TestImageViewerWords(t *testing.T) {
	for _, tt := range []struct {
		line  string
		words []string
		ok    bool
	}{
		{`imgview --thumbnail 'a b.png' "c|d.png"`, []string{"imgview", "--thumbnail", "a b.png", "c|d.png"}, true},
		{`imgview a\ b.png "a\z.png" ''`, []string{"imgview", "a b.png", `a\z.png`, ""}, true},
		{`imgview "$HOME/a"`, nil, false},
		{`imgview a | cat`, nil, false},
		{`imgview 'unfinished`, nil, false},
		{`imgview *.png`, nil, false},
	} {
		got, ok := imageViewerWords(tt.line)
		if ok != tt.ok || !reflect.DeepEqual(got, tt.words) {
			t.Errorf("%q: got %#v %v", tt.line, got, ok)
		}
	}
}

func TestImageViewerDispatch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(t.TempDir())
	m.width, m.height = 80, 24
	m.cmdInput.SetValue(`imgview 'a b.png'`)
	if ok, _ := m.handleBuiltinCmd(`imgview 'a b.png'`); !ok || m.procViewer == nil {
		t.Fatal("did not enter image viewer")
	}
	if m.cmdInput.Value() != "" {
		t.Error("input not cleared")
	}
	for _, line := range []string{"imgview --help", "imgview x | cat"} {
		other := New(t.TempDir())
		if ok, _ := other.handleBuiltinCmd(line); ok {
			t.Errorf("%q should fall through", line)
		}
	}
	other := New(t.TempDir())
	if ok, _ := other.handleBuiltinCmd("imgview --renderer=kitty 'x y'"); !ok {
		t.Fatal("kitty did not dispatch")
	}
	assertKittyFallback(t, other)
	bad := New(t.TempDir())
	if ok, _ := bad.handleBuiltinCmd("imgview --renderer=bad x"); !ok || bad.lastExitCode != 1 || !bufferHas(bad, "unknown renderer") {
		t.Fatal("invalid arguments should show an in-process diagnostic")
	}
}

func TestImageViewerDispatchConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "misa", "imageviewer.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("initial_mode: thumbnails\nrenderer: kitty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(t.TempDir())
	if ok, _ := m.handleBuiltinCmd("imgview --renderer=halfblock a.png"); !ok {
		t.Fatal("did not dispatch")
	}
	v, ok := m.procViewer.(*imgview.Viewer)
	if !ok || !v.Thumbnail() {
		t.Fatal("configured thumbnail mode not applied in UI")
	}
	other := New(t.TempDir())
	if ok, _ := other.handleBuiltinCmd("imgview a.png"); !ok {
		t.Fatal("configured renderer did not dispatch")
	}
	assertKittyFallback(t, other)
}

func assertKittyFallback(t *testing.T, m *Model) {
	t.Helper()
	v, ok := m.procViewer.(*imgview.Viewer)
	if !ok {
		t.Fatal("expected image viewer")
	}
	v.Resize(100, 4)
	if !strings.Contains(v.View(), "kitty unavailable; using halfblock") {
		t.Fatal("fallback warning missing")
	}
}

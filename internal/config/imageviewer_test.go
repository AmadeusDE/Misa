package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func imageConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "misa", "imageviewer.yaml")
}

func TestImageViewerDefaultsAndExplicitFalse(t *testing.T) {
	path := imageConfigDir(t)
	if got, want := LoadImageViewer(), DefaultImageViewer(); got.Preview != want.Preview || got.AnimationFPS != want.AnimationFPS || !got.AnimationAutoplay {
		t.Fatalf("defaults: %+v", got)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("preview: false\nanimation_autoplay: false\npreview_delay_ms: 0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := LoadImageViewer()
	if got.Preview || got.AnimationAutoplay || got.PreviewDelayMS != 0 || got.Renderer != "auto" {
		t.Fatalf("partial config: %+v", got)
	}
}

func TestImageViewerValidation(t *testing.T) {
	c := DefaultImageViewer()
	c.Renderer, c.Fit, c.OpenMode, c.InitialMode, c.AnimationLoops = "bad", "bad", "bad", "bad", "bad"
	c.ThumbnailSize, c.CacheMiB, c.AnimationCacheMiB = -1, 2, 100
	c.MaxPixels, c.AnimationFPS = 999999999, 0
	warnings := c.Validate()
	if len(warnings) != 9 {
		t.Fatalf("warnings = %v", warnings)
	}
	if c.Renderer != "auto" || c.Fit != "shrink" || c.OpenMode != "builtin" || c.InitialMode != "single" || c.AnimationLoops != "file" || c.ThumbnailSize != 32 || c.AnimationCacheMiB != 2 || c.MaxPixels != 32000000 || c.AnimationFPS != 1 {
		t.Fatalf("validated: %+v", c)
	}
	if got := c.Validate(); len(got) != 0 {
		t.Fatalf("not idempotent: %v", got)
	}
}

func TestImageViewerCollectionBounds(t *testing.T) {
	c := DefaultImageViewer()
	if c.MaxImages != 10000 || c.MaxDirectoryDepth != 32 || c.MaxDirectoryEntries != 200000 {
		t.Fatalf("defaults: %+v", c)
	}
	c.MaxImages, c.MaxDirectoryDepth, c.MaxDirectoryEntries = 0, 0, 0
	if warnings := c.Validate(); len(warnings) != 3 || c.MaxImages != 1 || c.MaxDirectoryDepth != 1 || c.MaxDirectoryEntries != 1 {
		t.Fatalf("clamps: %v %+v", warnings, c)
	}
	path := imageConfigDir(t)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("max_images: 5\nmax_directory_depth: 2\nmax_directory_entries: 7\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := LoadImageViewer()
	if got.MaxImages != 5 || got.MaxDirectoryDepth != 2 || got.MaxDirectoryEntries != 7 {
		t.Fatalf("yaml: %+v", got)
	}
}

func TestImageViewerSaveLoadAndMalformed(t *testing.T) {
	path := imageConfigDir(t)
	c := DefaultImageViewer()
	c.ExternalCommand = []string{"viewer", "{file}"}
	c.Preview = false
	if err := SaveImageViewer(c); err != nil {
		t.Fatal(err)
	}
	got := LoadImageViewer()
	if got.Preview || strings.Join(got.ExternalCommand, " ") != "viewer {file}" {
		t.Fatalf("round trip: %+v", got)
	}
	if err := os.WriteFile(path, []byte("preview: [\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := LoadImageViewer(); !got.Preview || got.Renderer != "auto" {
		t.Fatalf("malformed fallback: %+v", got)
	}
}

func TestImageViewerIgnoresCWD(t *testing.T) {
	imageConfigDir(t)
	t.Chdir(t.TempDir())
	if err := os.WriteFile("imageviewer.yaml", []byte("renderer: kitty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := LoadImageViewer().Renderer; got != "auto" {
		t.Fatalf("loaded from cwd: %s", got)
	}
}

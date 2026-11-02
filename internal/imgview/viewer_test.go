package imgview

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/charmbracelet/x/ansi"
	"image"
	"strings"
	"testing"
)

func key(s string) tea.KeyPressMsg { r := []rune(s); return tea.KeyPressMsg{Code: r[0], Text: s} }
func TestCollectionKeys(t *testing.T) {
	v, _ := New([]string{"one", "two", "three"}, false, "auto")
	v.SetSize(40, 12)
	v.Update(key("m"))
	v.Update(key("n"))
	v.Update(key("m"))
	v.Update(key("P"))
	if v.Index() != 0 || v.MarkedCount() != 2 {
		t.Fatalf("index %d marks %d", v.Index(), v.MarkedCount())
	}
	v.Update(key("M"))
	if v.MarkedCount() != 1 {
		t.Fatal(v.MarkedCount())
	}
	v.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if v.MarkedCount() != 0 {
		t.Fatal("clear")
	}
	v.Update(key("9"))
	v.Update(key("n"))
	if v.Index() != 2 {
		t.Fatal(v.Index())
	}
	v.Update(key("g"))
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !v.Thumbnail() {
		t.Fatal("grid")
	}
}
func TestLayoutAndTransform(t *testing.T) {
	v, _ := New([]string{"one"}, false, "auto")
	v.SetSize(20, 11)
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 40, 20)), nil)
	s := v.Scene(1, 1)
	if len(s.Placements) != 1 {
		t.Fatal(s)
	}
	if got := s.Placements[0].Dest; got != image.Rect(0, 5, 20, 15) {
		t.Fatal(got)
	}
	v.Update(key(">"))
	s = v.Scene(1, 1)
	if got := s.Placements[0].Raster.Pixels.Bounds().Size(); got != image.Pt(20, 40) {
		t.Fatal(got)
	}
	v.Update(key("+"))
	v.Update(key("l"))
	v.Update(key("n"))
	if v.Index() != 0 {
		t.Fatal(v.Index())
	}
}
func TestEmptyAndUnsafeFilename(t *testing.T) {
	v, _ := New(nil, false, "auto")
	v.SetSize(1, 1)
	v.Update(key("m"))
	_ = v.View()
	if len(v.Scene(1, 1).Placements) != 0 {
		t.Fatal("empty")
	}
	v, _ = New([]string{"bad\x1b[31m\nname"}, false, "auto")
	v.SetSize(80, 10)
	for _, r := range ansi.Strip(v.View()) {
		if r == '\x1b' {
			t.Fatal("escape in filename")
		}
	}
}

func TestGridMouseAndWheel(t *testing.T) {
	v, _ := New([]string{"a", "b", "c"}, true, "auto")
	v.Resize(48, 20)
	v.Update(tea.MouseClickMsg{X: 17, Y: 2, Button: tea.MouseLeft})
	if v.Index() != 1 || !v.Thumbnail() {
		t.Fatal(v.Index(), v.Thumbnail())
	}
	v.Update(tea.MouseClickMsg{X: 17, Y: 2, Button: tea.MouseLeft})
	if v.Thumbnail() {
		t.Fatal("second click should open")
	}
	v.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if v.Index() != 2 {
		t.Fatal(v.Index())
	}
}
func TestDecodeFailureDoesNotPanic(t *testing.T) {
	v, _ := New([]string{"x"}, false, "auto")
	v.Resize(30, 5)
	v.Update(viewerLoaded{index: 0, err: fmt.Errorf("bad")})
	if !strings.Contains(ansi.Strip(v.View()), "bad") {
		t.Fatal(v.View())
	}
}
func TestGridSelectedNeedsSecondClick(t *testing.T) {
	v, _ := New([]string{"a"}, true, "auto")
	v.Resize(20, 10)
	click := tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft}
	v.Update(click)
	if !v.Thumbnail() {
		t.Fatal("first click opened")
	}
	v.Update(click)
	if v.Thumbnail() {
		t.Fatal("second click did not open")
	}
}

func TestConfiguredLimitsAndPresentation(t *testing.T) {
	cfg := config.DefaultImageViewer()
	cfg.CacheMiB, cfg.MaxFileMiB, cfg.MaxPixels = 1, 1, 123
	cfg.Fit, cfg.InitialMode, cfg.ThumbnailSize = "actual", "thumbnails", 256
	v, err := NewConfigured([]string{"one", "two"}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !v.grid || v.fit != fitActual || v.thumb != 32 || v.limits.MaxPixels != 123 || v.limits.MaxEncodedBytes != 1<<20 {
		t.Fatalf("config not applied: %+v", v)
	}
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 12, 12)), nil)
	if v.items[0].err != ErrLimit {
		t.Fatalf("expected pixel limit, got %v", v.items[0].err)
	}
	v.selectIndex(1)
	if v.fit != fitActual {
		t.Fatal("selection lost configured fit")
	}
}

func TestViewerCacheBudgetAndReload(t *testing.T) {
	cfg := config.DefaultImageViewer()
	cfg.CacheMiB = 1
	v, _ := NewConfigured([]string{"one", "two"}, false, "auto", cfg)
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 400, 400)), nil)
	v.SetImage(1, image.NewNRGBA(image.Rect(0, 0, 400, 400)), nil)
	if v.cache.Bytes() > 1<<20 || v.pixels(0) != nil || v.pixels(1) == nil {
		t.Fatal("cache did not evict LRU")
	}
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 600, 600)), nil)
	if v.items[0].err == nil || v.cache.Bytes() > 1<<20 {
		t.Fatal("oversized image admitted")
	}
	v.Update(key("r"))
	if v.items[0].err != nil || v.items[0].loaded {
		t.Fatal("reload did not clear error")
	}
}

func TestViewerStaleLoadAndTransformGeneration(t *testing.T) {
	v, _ := New([]string{"one"}, false, "auto")
	v.Resize(20, 10)
	cmd := v.Init()
	if cmd == nil {
		t.Fatal("no load")
	}
	version := v.items[0].version
	v.Update(key("r"))
	v.Update(viewerLoaded{index: 0, version: version, decoded: Decoded{Pixels: image.NewNRGBA(image.Rect(0, 0, 2, 2))}})
	if v.pixels(0) != nil {
		t.Fatal("stale load published")
	}
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 2, 2)), nil)
	before := v.Scene(1, 1).Placements[0].Raster.Generation
	v.Update(key(">"))
	if v.Scene(1, 1).Placements[0].Raster.Generation == before {
		t.Fatal("transform did not change generation")
	}
}

func TestKittyFallbackWarning(t *testing.T) {
	for _, renderer := range []string{"kitty", "auto", "halfblock"} {
		v, err := New([]string{"one", "two"}, false, renderer)
		if err != nil {
			t.Fatal(err)
		}
		v.Resize(80, 4)
		for _, index := range []int{0, 1} {
			v.selectIndex(index)
			got := v.View()
			if strings.Contains(got, "\x1b_G") {
				t.Errorf("renderer %s emitted kitty graphics escape", renderer)
			}
			if strings.Contains(got, "kitty unavailable; using halfblock") != (renderer == "kitty") {
				t.Errorf("renderer %s index %d: %q", renderer, index, got)
			}
		}
	}
	if _, err := New(nil, false, "bad"); err == nil {
		t.Fatal("invalid renderer accepted")
	}
}

package gui

import (
	"image"
	"testing"
)

func embeddedFonts(t testing.TB) *fontSet {
	t.Helper()
	lib := newFontLibrary()
	lib.fcMatch = false
	lib.dirsSet = true
	fs, err := newFontSet(lib, "", 12, 1)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestEmbeddedFontGrid(t *testing.T) {
	fs := embeddedFonts(t)
	if fs.cellW < 5 || fs.cellH <= fs.cellW || fs.ascent >= fs.cellH {
		t.Fatalf("cell %dx%d ascent %d", fs.cellW, fs.cellH, fs.ascent)
	}
	if fs.underline <= fs.ascent-1 || fs.underline+fs.lineWidth > fs.cellH {
		t.Errorf("underline at %d (+%d) outside descent of %d", fs.underline, fs.lineWidth, fs.cellH)
	}
	for s, f := range fs.styles {
		if f == nil {
			t.Errorf("style %d missing", s)
		}
	}
}

func TestGlyphsSitInTheirCell(t *testing.T) {
	fs := embeddedFonts(t)
	g := fs.glyph('A', styleRegular, 1)
	if g == nil {
		t.Fatal("no glyph for A")
	}
	cell := image.Rect(0, 0, fs.cellW, fs.cellH)
	if !g.mask.Rect.In(cell) {
		t.Errorf("A mask %v outside cell %v", g.mask.Rect, cell)
	}
	if fs.glyph(' ', styleRegular, 1) != nil {
		t.Error("space drew a glyph")
	}
	if box := fs.glyph('─', styleBold, 1); box == nil || box.mask.Rect != cell {
		t.Error("box drawing not procedural")
	}
	if fs.glyph('A', styleRegular, 1) != g {
		t.Error("glyph not cached")
	}
}

func TestIsFontPath(t *testing.T) {
	for spec, want := range map[string]bool{"/usr/share/fonts/x.ttf": true, "Iosevka.TTC": true, "JetBrains Mono": false, "": false} {
		if got := isFontPath(spec); got != want {
			t.Errorf("isFontPath(%q) = %v", spec, got)
		}
	}
}

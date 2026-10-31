package gui

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/gomono"

	"github.com/AmadeusDE/Misa/internal/chess"
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

// TestChessGlyphsWithoutFontconfig covers Android: no fc-match and a system
// mono font (here Go Mono) with no chess symbols. The pieces the chess
// variants draw must still come out of the embedded chess font.
func TestChessGlyphsWithoutFontconfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DroidSansMono.ttf"), gomono.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	lib := newFontLibrary()
	lib.fcMatch = false
	lib.dirsSet = true
	lib.dirs = []string{dir}
	fs, err := newFontSet(lib, "", 12, 1)
	if err != nil {
		t.Fatal(err)
	}
	if fs.styles[styleRegular].file == lib.embedded(styleRegular) {
		t.Fatal("system mono font was not used")
	}
	if lib.embedded(styleRegular).has('♚') {
		t.Fatal("Go Mono has chess symbols now; this test no longer covers the fallback")
	}
	for r := rune(0x2654); r <= 0x265f; r++ {
		for _, s := range []style{styleRegular, styleBold} {
			sf := fs.faceFor(r, s)
			if sf == nil || sf.file != lib.symbols() {
				t.Fatalf("%c (style %d) not from the embedded chess font", r, s)
			}
			for _, span := range []int{1, 2} {
				g := fs.glyph(r, s, span)
				if g == nil || g.mask.Rect.Empty() {
					t.Fatalf("%c (style %d, span %d) drew nothing", r, s, span)
				}
				var ink int
				for _, a := range g.mask.Pix {
					ink += int(a)
				}
				if ink == 0 {
					t.Fatalf("%c (style %d, span %d) has an empty mask", r, s, span)
				}
			}
		}
	}
}

// TestVariantGlyphsRender checks every glyph a builtin chess variant uses
// renders with only the embedded fonts. The variants come from the chess
// package's embedded files, so this also runs on a device.
func TestVariantGlyphsRender(t *testing.T) {
	fs := embeddedFonts(t)
	vs, errs := chess.Builtins()
	if len(errs) > 0 || len(vs) == 0 {
		t.Fatalf("builtin variants: %v", errs)
	}
	seen := map[rune]bool{}
	for _, v := range vs {
		for _, p := range v.Pieces {
			for _, r := range p.Glyph {
				if seen[r] {
					continue
				}
				seen[r] = true
				if g := fs.glyph(r, styleRegular, 1); g == nil {
					t.Errorf("%s: %s glyph %c (U+%04X) does not render", v.Name, p.Name, r, r)
				}
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no glyphs found in the builtin variants")
	}
}

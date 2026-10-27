package gui

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/term"
)

func TestPaletteDefaultsAndOverrides(t *testing.T) {
	p := newPalette(config.GUIConfig{Background: "#000", Palette: []string{"", "#ff0000"}})
	if p.bg != (rgb{0, 0, 0}) {
		t.Errorf("bg = %v", p.bg)
	}
	if p.colors[1] != (rgb{255, 0, 0}) {
		t.Errorf("color 1 = %v", p.colors[1])
	}
	if p.colors[0] != parseHex(gruvboxDark[0], rgb{}) {
		t.Errorf("empty palette entry did not keep the default: %v", p.colors[0])
	}
	if p.colors[16] != (rgb{0, 0, 0}) || p.colors[231] != (rgb{255, 255, 255}) || p.colors[232] != (rgb{8, 8, 8}) {
		t.Errorf("cube/gray = %v %v %v", p.colors[16], p.colors[231], p.colors[232])
	}
}

func TestCellColors(t *testing.T) {
	p := newPalette(config.GUIConfig{Foreground: "#ffffff", Background: "#000000"})
	c := term.Cell{FG: term.RGBColor(1, 2, 3), HasFG: true, Reverse: true}
	fg, bg, def := p.cellColors(&c)
	if fg != (rgb{0, 0, 0}) || bg != (rgb{1, 2, 3}) || def {
		t.Errorf("reverse = %v on %v", fg, bg)
	}
	c = term.Cell{FG: term.IndexedColor(196), HasFG: true}
	if fg, _, def := p.cellColors(&c); fg != (rgb{255, 0, 0}) || !def {
		t.Errorf("indexed 196 = %v", fg)
	}
	c = term.Cell{Dim: true}
	if fg, _, _ := p.cellColors(&c); fg == p.fg || fg == p.bg {
		t.Errorf("dim fg = %v", fg)
	}
}

func TestOpacity(t *testing.T) {
	half, over, solid := 0.5, 3.0, 1.0
	for _, tt := range []struct {
		in   *float64
		want uint8
	}{{nil, 140}, {&half, 128}, {&over, 255}, {&solid, 255}} {
		if got := newPalette(config.GUIConfig{Opacity: tt.in}).alpha; got != tt.want {
			t.Errorf("opacity %v = %d, want %d", tt.in, got, tt.want)
		}
	}
}

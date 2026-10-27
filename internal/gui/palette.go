package gui

import (
	"math"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/term"
)

type rgb struct{ r, g, b uint8 }

// gruvboxDark matches Misa's default UI colors.
var gruvboxDark = [16]string{
	"#282828", "#cc241d", "#98971a", "#d79921", "#458588", "#b16286", "#689d6a", "#a89984",
	"#928374", "#fb4934", "#b8bb26", "#fabd2f", "#83a598", "#d3869b", "#8ec07c", "#ebdbb2",
}

type palette struct {
	fg, bg, cursor rgb
	colors         [256]rgb
	alpha          uint8 // opacity of the default background
}

func newPalette(cfg config.GUIConfig) palette {
	var p palette
	for i, def := range gruvboxDark {
		p.colors[i] = parseHex(def, rgb{})
		if i < len(cfg.Palette) {
			p.colors[i] = parseHex(cfg.Palette[i], p.colors[i])
		}
	}
	levels := [6]uint8{0, 95, 135, 175, 215, 255}
	for i := 0; i < 216; i++ {
		p.colors[16+i] = rgb{levels[i/36], levels[i/6%6], levels[i%6]}
	}
	for i := 0; i < 24; i++ {
		v := uint8(8 + 10*i)
		p.colors[232+i] = rgb{v, v, v}
	}
	p.fg = parseHex(cfg.Foreground, p.colors[15])
	p.bg = parseHex(cfg.Background, p.colors[0])
	p.cursor = parseHex(cfg.Cursor, p.fg)
	p.alpha = opacity(cfg.Opacity)
	return p
}

const defaultOpacity = 0.55

func opacity(o *float64) uint8 {
	v := defaultOpacity
	if o != nil {
		v = *o
	}
	return uint8(math.Round(max(0, min(1, v)) * 255))
}

func (p *palette) translucent() bool { return p.alpha < 0xff }

func parseHex(s string, def rgb) rgb {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if len(s) != 6 || err != nil {
		return def
	}
	return rgb{uint8(v >> 16), uint8(v >> 8), uint8(v)}
}

func (p *palette) color(c term.Color, set bool, def rgb) rgb {
	if !set {
		return def
	}
	switch c.Kind {
	case term.ColorIndexed:
		return p.colors[c.Index]
	case term.ColorRGB:
		return rgb{c.R, c.G, c.B}
	}
	return def
}

// cellColors resolves a cell's foreground and background after reverse
// video and faint; defaultBG reports the window background, the only one
// drawn translucent.
func (p *palette) cellColors(c *term.Cell) (fg, bg rgb, defaultBG bool) {
	fg = p.color(c.FG, c.HasFG, p.fg)
	bg = p.color(c.BG, c.HasBG, p.bg)
	if c.Reverse {
		fg, bg = bg, fg
	}
	if c.Dim {
		fg = mix(bg, fg, 140)
	}
	return fg, bg, !c.HasBG && !c.Reverse
}

// mix blends a toward b by alpha/255.
func mix(a, b rgb, alpha uint8) rgb {
	m := func(x, y uint8) uint8 {
		return uint8((int(x)*(255-int(alpha)) + int(y)*int(alpha) + 127) / 255)
	}
	return rgb{m(a.r, b.r), m(a.g, b.g), m(a.b, b.b)}
}

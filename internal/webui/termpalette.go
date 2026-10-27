package webui

import (
	"github.com/AmadeusDE/Misa/internal/term"
)

type rgb struct{ r, g, b uint8 }

var gruvboxDark = [16]rgb{
	{0x28, 0x28, 0x28}, {0xcc, 0x24, 0x1d}, {0x98, 0x97, 0x1a}, {0xd7, 0x99, 0x21},
	{0x45, 0x85, 0x88}, {0xb1, 0x62, 0x86}, {0x68, 0x9d, 0x6a}, {0xa8, 0x99, 0x84},
	{0x92, 0x83, 0x74}, {0xfb, 0x49, 0x34}, {0xb8, 0xbb, 0x26}, {0xfa, 0xbd, 0x2f},
	{0x83, 0xa5, 0x98}, {0xd3, 0x86, 0x9b}, {0x8e, 0xc0, 0x7c}, {0xeb, 0xdb, 0xb2},
}

// termPalette resolves terminal colours to CSS hex.
type termPalette struct {
	fg, bg rgb
	colors [256]rgb
}

func newTermPalette() *termPalette {
	p := &termPalette{}
	copy(p.colors[:16], gruvboxDark[:])
	levels := [6]uint8{0, 95, 135, 175, 215, 255}
	for i := 0; i < 216; i++ {
		p.colors[16+i] = rgb{levels[i/36], levels[i/6%6], levels[i%6]}
	}
	for i := 0; i < 24; i++ {
		v := uint8(8 + 10*i)
		p.colors[232+i] = rgb{v, v, v}
	}
	p.fg, p.bg = p.colors[15], p.colors[0]
	return p
}

func (c rgb) hex() string {
	const digits = "0123456789abcdef"
	return string([]byte{'#', digits[c.r>>4], digits[c.r&15], digits[c.g>>4], digits[c.g&15], digits[c.b>>4], digits[c.b&15]})
}

func (p *termPalette) color(c term.Color, set bool, def rgb) rgb {
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

// resolve returns the cell's foreground and background after reverse video
// and faint, as CSS hex; a colour equal to the default is returned empty so
// the client can leave it to its stylesheet.
func (p *termPalette) resolve(c *term.Cell) (fg, bg string) {
	f := p.color(c.FG, c.HasFG, p.fg)
	b := p.color(c.BG, c.HasBG, p.bg)
	if c.Reverse {
		f, b = b, f
	}
	if c.Dim {
		f = mixRGB(b, f, 140)
	}
	if f != p.fg || c.Reverse {
		fg = f.hex()
	}
	if b != p.bg || c.Reverse {
		bg = b.hex()
	}
	return fg, bg
}

func mixRGB(a, b rgb, alpha int) rgb {
	m := func(x, y uint8) uint8 { return uint8((int(x)*(255-alpha) + int(y)*alpha + 127) / 255) }
	return rgb{m(a.r, b.r), m(a.g, b.g), m(a.b, b.b)}
}

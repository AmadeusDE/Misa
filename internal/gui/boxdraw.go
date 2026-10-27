package gui

import (
	"image"
	"math"
)

// boxLines gives U+2500–U+257F as up, right, down, left arm weights:
// 0 none, 1 light, 2 heavy, 3 double. Dashed lines draw solid; the arcs and
// diagonals (all zero) are drawn separately.
const boxLines = "" +
	"0101020210102020010102021010202001010202101020200110021001200220" +
	"0011001200210022110012002100220010011002200120021110121021101120" +
	"2120221012202220101110122011102120212012102220220111011202110212" +
	"0121012202210222110111021201120221012102220122021111111212111212" +
	"2111112121212112221111221221221212222122222122220101020210102020" +
	"0303303003100130033000130031003313003100330010033001300313103130" +
	"3330101330313033031301310333130331013303131331313333000000000000" +
	"0000000000000000000110000100001000022000020000200201102001022010"

func boxWeights(r rune) (arms [4]byte) {
	i := int(r-0x2500) * 4
	for k := range arms {
		arms[k] = boxLines[i+k] - '0'
	}
	return arms
}

// boxGlyph draws box drawing, block element and braille characters to fill
// a w×h cell exactly, so lines join across cells whatever the font. It
// returns nil for other runes.
func boxGlyph(r rune, w, h int) *image.Alpha {
	m := image.NewAlpha(image.Rect(0, 0, w, h))
	switch {
	case r >= 0x256d && r <= 0x2570:
		drawArc(m, r)
	case r >= 0x2571 && r <= 0x2573:
		drawDiagonals(m, r)
	case r >= 0x2500 && r <= 0x257f:
		drawArms(m, boxWeights(r))
	case r >= 0x2580 && r <= 0x259f:
		drawBlock(m, r)
	case r >= 0x2800 && r <= 0x28ff:
		drawBraille(m, byte(r-0x2800))
	default:
		return nil
	}
	return m
}

func lineWidth(w int) int { return max(1, (w+4)/9) }

func fillAlpha(m *image.Alpha, r image.Rectangle, a uint8) {
	r = r.Intersect(m.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			m.Pix[y*m.Stride+x] = max(m.Pix[y*m.Stride+x], a)
		}
	}
}

// band is the span of a line of thickness t centered on c.
func band(c, t int) (int, int) { return c - t/2, c - t/2 + t }

func drawArms(m *image.Alpha, arms [4]byte) {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	cx, cy := w/2, h/2
	light := lineWidth(w)
	thick := func(weight byte) int {
		switch weight {
		case 2:
			return light * 2
		case 3:
			return light * 3
		}
		return light
	}
	// reach is how far horizontal arms extend past the center to meet the
	// vertical ones (and vice versa), so corners and tees close.
	reachH := max(thick(arms[0])*int(min(arms[0], 1)), thick(arms[2])*int(min(arms[2], 1)))
	reachV := max(thick(arms[1])*int(min(arms[1], 1)), thick(arms[3])*int(min(arms[3], 1)))
	horizontal := func(weight byte, x0, x1 int) {
		t := thick(weight)
		if weight == 3 {
			for _, y := range []int{cy - light, cy + light} {
				a, b := band(y, light)
				fillAlpha(m, image.Rect(x0, a, x1, b), 255)
			}
			return
		}
		a, b := band(cy, t)
		fillAlpha(m, image.Rect(x0, a, x1, b), 255)
	}
	vertical := func(weight byte, y0, y1 int) {
		t := thick(weight)
		if weight == 3 {
			for _, x := range []int{cx - light, cx + light} {
				a, b := band(x, light)
				fillAlpha(m, image.Rect(a, y0, b, y1), 255)
			}
			return
		}
		a, b := band(cx, t)
		fillAlpha(m, image.Rect(a, y0, b, y1), 255)
	}
	lo, hi := band(cx, reachH)
	if arms[3] != 0 {
		horizontal(arms[3], 0, hi)
	}
	if arms[1] != 0 {
		horizontal(arms[1], lo, w)
	}
	lo, hi = band(cy, reachV)
	if arms[0] != 0 {
		vertical(arms[0], 0, hi)
	}
	if arms[2] != 0 {
		vertical(arms[2], lo, h)
	}
}

// drawArc draws ╭╮╯╰ as a quarter circle joining the two arm midpoints.
func drawArc(m *image.Alpha, r rune) {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	lw := lineWidth(w)
	t := float64(lw)
	x0, _ := band(w/2, lw)
	y0, _ := band(h/2, lw)
	lineX, lineY := float64(x0)+t/2, float64(y0)+t/2
	sx, sy := -1.0, -1.0
	reachX, reachY := lineX, lineY
	if r == 0x256d || r == 0x2570 {
		sx, reachX = 1, float64(w)-lineX
	}
	if r == 0x256d || r == 0x256e {
		sy, reachY = 1, float64(h)-lineY
	}
	rad := math.Min(reachX, reachY)
	ox, oy := lineX+sx*rad, lineY+sy*rad
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			var d float64
			switch {
			case (px-ox)*sx > 0:
				d = math.Abs(py - lineY)
			case (py-oy)*sy > 0:
				d = math.Abs(px - lineX)
			default:
				d = math.Abs(math.Hypot(px-ox, py-oy) - rad)
			}
			if a := t/2 + 0.5 - d; a > 0 {
				m.Pix[y*m.Stride+x] = max(m.Pix[y*m.Stride+x], uint8(math.Min(a, 1)*255))
			}
		}
	}
}

func drawDiagonals(m *image.Alpha, r rune) {
	w, h := float64(m.Rect.Dx()), float64(m.Rect.Dy())
	t := float64(lineWidth(m.Rect.Dx()))
	length := math.Hypot(w, h)
	line := func(rising bool) {
		for y := 0; y < m.Rect.Dy(); y++ {
			for x := 0; x < m.Rect.Dx(); x++ {
				px, py := float64(x)+0.5, float64(y)+0.5
				if rising {
					py = h - py
				}
				d := math.Abs(px*h-py*w) / length
				if a := t/2 + 0.5 - d; a > 0 {
					m.Pix[y*m.Stride+x] = max(m.Pix[y*m.Stride+x], uint8(math.Min(a, 1)*255))
				}
			}
		}
	}
	if r != 0x2572 {
		line(true)
	}
	if r != 0x2571 {
		line(false)
	}
}

const (
	quadUL = 1 << iota
	quadUR
	quadLL
	quadLR
)

var quadrants = [10]byte{
	quadLL, quadLR, quadUL, quadUL | quadLL | quadLR, quadUL | quadLR,
	quadUL | quadUR | quadLL, quadUL | quadUR | quadLR, quadUR, quadUR | quadLL, quadUR | quadLL | quadLR,
}

func drawBlock(m *image.Alpha, r rune) {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	eighthsH := func(n int) int { return (h*n + 4) / 8 }
	eighthsW := func(n int) int { return (w*n + 4) / 8 }
	switch {
	case r == 0x2580:
		fillAlpha(m, image.Rect(0, 0, w, h/2), 255)
	case r >= 0x2581 && r <= 0x2588:
		fillAlpha(m, image.Rect(0, h-eighthsH(int(r-0x2580)), w, h), 255)
	case r >= 0x2589 && r <= 0x258f:
		fillAlpha(m, image.Rect(0, 0, eighthsW(int(0x2590-r)), h), 255)
	case r == 0x2590:
		fillAlpha(m, image.Rect(w/2, 0, w, h), 255)
	case r >= 0x2591 && r <= 0x2593:
		fillAlpha(m, m.Rect, uint8(64*(r-0x2590)))
	case r == 0x2594:
		fillAlpha(m, image.Rect(0, 0, w, eighthsH(1)), 255)
	case r == 0x2595:
		fillAlpha(m, image.Rect(w-eighthsW(1), 0, w, h), 255)
	default:
		q := quadrants[r-0x2596]
		parts := []struct {
			bit  byte
			rect image.Rectangle
		}{
			{quadUL, image.Rect(0, 0, w/2, h/2)}, {quadUR, image.Rect(w/2, 0, w, h/2)},
			{quadLL, image.Rect(0, h/2, w/2, h)}, {quadLR, image.Rect(w/2, h/2, w, h)},
		}
		for _, p := range parts {
			if q&p.bit != 0 {
				fillAlpha(m, p.rect, 255)
			}
		}
	}
}

// brailleDots maps the eight dot bits to (column, row) in the 2×4 grid.
var brailleDots = [8][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}

func drawBraille(m *image.Alpha, bits byte) {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	size := max(1, min(w/2, h/4)*2/3)
	for i, d := range brailleDots {
		if bits&(1<<i) == 0 {
			continue
		}
		cx := (w*(2*d[0]+1) + 2) / 4
		cy := (h*(2*d[1]+1) + 4) / 8
		x0, x1 := band(cx, size)
		y0, y1 := band(cy, size)
		fillAlpha(m, image.Rect(x0, y0, x1, y1), 255)
	}
}

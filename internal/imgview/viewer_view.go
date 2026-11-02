package imgview

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/AmadeusDE/Misa/internal/imageout"
	"github.com/charmbracelet/x/ansi"
)

// View renders a portable half-block presentation; no graphics escapes or
// terminal writes occur here, so it is safe inside an ordinary Misa pane.
func (m *Viewer) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if m.help {
		return ansi.Truncate("q quit · Enter grid/image · n/p navigate · arrows pan/select\n+/- zoom · w/W/e/E fit · f actual · </> rotate · ?/| flip\nm mark · M invert · Ctrl+u clear · N/P marked · r reload · Esc close\nCtrl+a play/pause · Ctrl+n/Ctrl+p step frame · thumbs stay still", m.width, "")
	}
	lines := make([]string, max(0, m.height-1))
	renderer := imageout.NewHalfBlock(color.NRGBA{R: 32, G: 32, B: 32, A: 255})
	frame, err := renderer.Prepare(context.Background(), m.Scene(1, 1))
	if err == nil && frame.Cells != nil {
		layer := frame.Cells
		w := layer.Bounds.Dx()
		for y := 0; y < len(lines) && y < layer.Bounds.Dy(); y++ {
			var b strings.Builder
			for x := 0; x < w; x++ {
				i := y*w + x
				if i >= len(layer.Cells) {
					break
				}
				c := layer.Cells[i]
				text := c.Text
				if text == "" {
					text = " "
				}
				style := lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c.Foreground.R, c.Foreground.G, c.Foreground.B))).Background(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c.Background.R, c.Background.G, c.Background.B)))
				b.WriteString(style.Render(text))
			}
			lines[y] = b.String()
		}
	}
	if m.grid {
		start, end := m.visibleRange()
		cols := m.columns()
		rowh := m.thumb/2 + 2
		for i := start; i < end; i++ {
			row := (i - start) / cols * rowh
			if row >= len(lines) {
				break
			} // Labels are kept outside the image boxes.
			label := safeName(m.items[i].path)
			mark := " "
			if m.items[i].marked {
				mark = "*"
			}
			sel := " "
			if i == m.index {
				sel = ">"
			}
			label = ansi.Truncate(sel+mark+label, m.thumb, "")
			lines[row] = overlay(lines[row], (i-start)%cols*m.thumb, label, m.width)
		}
	}
	status := "No images"
	if len(m.items) > 0 {
		it := m.items[m.index]
		pix := m.pixels(m.index)
		mark := ""
		if it.marked {
			mark = " *"
		}
		state := ""
		if it.err != nil {
			state = " · " + safeText(it.err.Error())
		} else if !it.loaded {
			state = " · loading"
		} else if pix == nil {
			state = " · no image"
		}
		if it.animated && it.err == nil {
			state = m.animationStatus(&m.items[m.index])
			if m.grid {
				state = " · animated"
			}
		}
		size := ""
		if pix != nil {
			b := pix.Bounds()
			size = fmt.Sprintf(" · %dx%d · %.0f%%", b.Dx(), b.Dy(), m.scale()*100)
		}
		status = fmt.Sprintf("%d/%d %s%s%s%s · %d marked · halfblock", m.index+1, len(m.items), safeName(it.path), mark, state, size, m.MarkedCount())
		if m.note != "" {
			status += " · " + m.note
		}
	}
	if m.rendererWarning != "" {
		status = m.rendererWarning + " · " + status
	}
	lines = append(lines, ansi.Truncate(status, m.width, ""))
	return strings.Join(lines, "\n")
}
func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == 0x2028 || r == 0x2029 {
			return '�'
		}
		return r
	}, s)
}

// Overlay a label by replacing a cell segment. ANSI-aware cuts preserve the
// image cells before and after it.
func overlay(line string, x int, label string, width int) string {
	line = ansi.Truncate(line, width, "")
	left := ansi.Truncate(line, x, "")
	pad := max(0, x-ansi.StringWidth(left))
	right := ansi.Cut(line, x+ansi.StringWidth(label), width)
	return left + strings.Repeat(" ", pad) + label + right
}

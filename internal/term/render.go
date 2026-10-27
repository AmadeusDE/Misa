package term

import (
	"fmt"
	"image/color"
	"strings"
	"sync"
	"sync/atomic"

	"charm.land/lipgloss/v2"
)

func colorToLipgloss(c Color) color.Color {
	switch c.Kind {
	case ColorIndexed:
		return lipgloss.ANSIColor(c.Index)
	case ColorRGB:
		return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B))
	}
	return lipgloss.NoColor{}
}

func sameAttr(a, b *Cell) bool {
	return a.FG == b.FG && a.BG == b.BG &&
		a.HasFG == b.HasFG && a.HasBG == b.HasBG &&
		a.Bold == b.Bold && a.Dim == b.Dim &&
		a.Italic == b.Italic && a.Under == b.Under &&
		a.Reverse == b.Reverse && a.Strike == b.Strike &&
		a.Hidden == b.Hidden
}

func (c *Cell) styled() bool {
	return c.HasFG || c.HasBG || c.Bold || c.Dim || c.Italic ||
		c.Under || c.Reverse || c.Strike
}

func (c *Cell) style() lipgloss.Style {
	s := lipgloss.NewStyle()
	if c.HasFG {
		s = s.Foreground(colorToLipgloss(c.FG))
	}
	if c.HasBG {
		s = s.Background(colorToLipgloss(c.BG))
	}
	return s.Bold(c.Bold).Faint(c.Dim).Italic(c.Italic).Underline(c.Under).
		Strikethrough(c.Strike).Reverse(c.Reverse)
}

type styleKey struct {
	fg, bg                                    Color
	hasFG, hasBG                              bool
	bold, dim, italic, under, reverse, strike bool
}

type styleWrap struct{ prefix, suffix string }

// styleCache holds each attribute set's escape sequences, rendered once by
// lipgloss: restyling every run of every row each frame dominated render time.
// Truecolor output can name millions of sets, so past maxStyles it starts over.
var (
	styleCache sync.Map
	styleCount atomic.Int64
)

const maxStyles = 4096

const styleSentinel = "\x00"

// renderStyled is cell.style().Render(text), memoized per attribute set.
func renderStyled(c *Cell, text string) string {
	key := styleKey{c.FG, c.BG, c.HasFG, c.HasBG, c.Bold, c.Dim, c.Italic, c.Under, c.Reverse, c.Strike}
	if w, ok := styleCache.Load(key); ok {
		w := w.(styleWrap)
		return w.prefix + text + w.suffix
	}
	out := c.style().Render(styleSentinel)
	prefix, suffix, ok := strings.Cut(out, styleSentinel)
	if !ok || strings.Contains(suffix, styleSentinel) {
		return c.style().Render(text)
	}
	if _, loaded := styleCache.LoadOrStore(key, styleWrap{prefix, suffix}); !loaded && styleCount.Add(1) > maxStyles {
		styleCache.Clear()
		styleCount.Store(0)
	}
	return prefix + text + suffix
}

// writeGlyph appends the text for row[i] and reports how many cells it
// consumed. A wide character whose right half was lost renders as a space so
// every row is exactly as wide as the grid.
func writeGlyph(sb *strings.Builder, row []Cell, i int) int {
	c := &row[i]
	wide := c.Wide && i+1 < len(row) && row[i+1].cont
	switch {
	case c.cont || (c.Wide && !wide):
		sb.WriteByte(' ')
		return 1
	case c.Hidden || c.Ch == 0:
		sb.WriteByte(' ')
		if wide {
			sb.WriteByte(' ')
		}
	default:
		sb.WriteRune(c.Ch)
		sb.WriteString(c.Marks())
	}
	if wide {
		return 2
	}
	return 1
}

func renderRow(row []Cell, cols int) string {
	var sb, run strings.Builder
	c := 0
	for c < cols {
		cell := &row[c]
		run.Reset()
		end := c
		for end < cols && sameAttr(&row[end], cell) {
			end += writeGlyph(&run, row, end)
		}
		if cell.styled() {
			sb.WriteString(renderStyled(cell, run.String()))
		} else {
			sb.WriteString(run.String())
		}
		c = end
	}
	return strings.TrimRight(sb.String(), " ")
}

func (t *Terminal) Render() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var lines []string
	for r := 0; r < t.Rows; r++ {
		lines = append(lines, renderRow(t.cells[r], t.Cols))
	}
	return strings.Join(lines, "\n")
}

type Highlight struct {
	StartRow, StartCol int
	EndRow, EndCol     int
}

func (h *Highlight) contains(row, col int) bool {
	r1, c1, r2, c2 := h.StartRow, h.StartCol, h.EndRow, h.EndCol
	if r1 > r2 || (r1 == r2 && c1 > c2) {
		r1, c1, r2, c2 = r2, c2, r1, c1
	}
	if row < r1 || row > r2 {
		return false
	}
	if row == r1 && row == r2 {
		return col >= c1 && col < c2
	}
	if row == r1 {
		return col >= c1
	}
	if row == r2 {
		return col < c2
	}
	return true
}

func (t *Terminal) RenderWithCursor(showCursor bool) string {
	return t.RenderFull(showCursor, nil)
}

func (t *Terminal) RenderFull(showCursor bool, sel *Highlight) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	curR, curC := -1, -1
	if showCursor && !t.cursorHidden && t.curRow >= 0 && t.curRow < t.Rows && t.curCol >= 0 && t.curCol < t.Cols {
		curR, curC = t.curRow, t.curCol
	}
	var lines []string
	for r := 0; r < t.Rows; r++ {
		needsSpecial := r == curR || (sel != nil && sel.rowTouched(r))
		if needsSpecial {
			lines = append(lines, renderRowFull(t.cells[r], t.Cols, r, curR, curC, sel))
		} else {
			lines = append(lines, renderRow(t.cells[r], t.Cols))
		}
	}
	return strings.Join(lines, "\n")
}

func (h *Highlight) rowTouched(row int) bool {
	if h == nil {
		return false
	}
	r1, r2 := h.StartRow, h.EndRow
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	return row >= r1 && row <= r2
}

func renderRowFull(row []Cell, cols, rowIdx, curR, curC int, sel *Highlight) string {
	var sb, glyph strings.Builder
	for c := 0; c < cols; {
		cell := &row[c]
		glyph.Reset()
		n := writeGlyph(&glyph, row, c)
		isCursor := rowIdx == curR && (c == curC || (n == 2 && c+1 == curC))
		isSel := sel != nil && sel.contains(rowIdx, c)

		switch {
		case isCursor || isSel:
			s := lipgloss.NewStyle().Reverse(true).Bold(isCursor)
			if cell.HasFG {
				s = s.Foreground(colorToLipgloss(cell.FG))
			}
			if cell.HasBG {
				s = s.Background(colorToLipgloss(cell.BG))
			}
			sb.WriteString(s.Render(glyph.String()))
		case cell.styled():
			sb.WriteString(renderStyled(cell, glyph.String()))
		default:
			sb.WriteString(glyph.String())
		}
		c += n
	}
	return strings.TrimRight(sb.String(), " ")
}

// Lines renders scrollback plus the normal screen, even if the program left
// the alternate screen active when it exited.
func (t *Terminal) Lines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := make([]string, 0, len(t.scrollback)+t.Rows)
	for _, row := range t.scrollback {
		lines = append(lines, renderRow(row, len(row)))
	}
	for _, row := range t.mainCells() {
		lines = append(lines, renderRow(row, t.Cols))
	}
	return trimBlankTail(lines)
}

func trimBlankTail(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (t *Terminal) LinesWithCursor(showCursor bool, sel *Highlight) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	curR, curC := -1, -1
	if showCursor && !t.cursorHidden && t.curRow >= 0 && t.curRow < t.Rows && t.curCol >= 0 && t.curCol < t.Cols {
		curR = len(t.scrollback) + t.curRow
		curC = t.curCol
	}
	var lines []string
	for i, row := range t.scrollback {
		if i == curR || (sel != nil && sel.rowTouched(i)) {
			lines = append(lines, renderRowFull(row, len(row), i, curR, curC, sel))
		} else {
			lines = append(lines, renderRow(row, len(row)))
		}
	}
	sbLen := len(t.scrollback)
	for r := 0; r < t.Rows; r++ {
		absR := sbLen + r
		if absR == curR || (sel != nil && sel.rowTouched(absR)) {
			lines = append(lines, renderRowFull(t.cells[r], t.Cols, absR, curR, curC, sel))
		} else {
			lines = append(lines, renderRow(t.cells[r], t.Cols))
		}
	}
	lines = trimBlankTail(lines)
	for len(lines) <= curR {
		lines = append(lines, "")
	}
	return lines
}

func plainRow(row []Cell, cols int) string {
	var sb strings.Builder
	for c := 0; c < cols; {
		c += writeGlyph(&sb, row, c)
	}
	return strings.TrimRight(sb.String(), " ")
}

func (t *Terminal) PlainLines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var lines []string
	for _, row := range t.scrollback {
		lines = append(lines, plainRow(row, len(row)))
	}
	for r := 0; r < t.Rows; r++ {
		lines = append(lines, plainRow(t.cells[r], t.Cols))
	}
	return trimBlankTail(lines)
}

func (t *Terminal) PlainScreen() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := make([]string, t.Rows)
	for r := 0; r < t.Rows; r++ {
		lines[r] = plainRow(t.cells[r], t.Cols)
	}
	return lines
}

func (t *Terminal) ScrollbackLen() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.scrollback)
}

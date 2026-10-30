package mines

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	faceWidth   = 4
	chromeLines = 5 // header, two border rows, status, help
	help        = "space open  f flag  n new  1-3 level  c custom  q quit"
	// A board larger than the terminal scrolls. The viewport needs at least
	// this many cells in each direction (or the whole board, when the board is
	// smaller than that) to be worth drawing.
	minViewCols = 3
	minViewRows = 3
)

var (
	hiddenBg   = lipgloss.Color("#3c3836")
	hiddenSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7c6f64")).Background(hiddenBg)
	flagSt     = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Background(hiddenBg).Bold(true)
	mineSt     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2")).Bold(true)
	explodedSt = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbf1c7")).Background(lipgloss.Color("#cc241d")).Bold(true)
	cursorSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#fabd2f")).Bold(true)
	counterSt  = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Background(lipgloss.Color("#1d2021")).Bold(true)
	faceSt     = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#fabd2f")).Bold(true)
	statusSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
	helpSt     = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))

	numberColors = [9]string{"", "#83a598", "#b8bb26", "#fb4934", "#d3869b", "#fe8019", "#8ec07c", "#ebdbb2", "#928374"}
	borderColors = map[State]string{Playing: "#665c54", Won: "#b8bb26", Lost: "#fb4934"}
	faces        = map[State]string{Playing: ":)", Won: "B)", Lost: "X("}
)

type hitTarget int

const (
	hitNone hitTarget = iota
	hitFace
	hitCell
)

// layout places the board centred on screen: a header row with the counters
// and face, the bordered board, then the status and help lines. When the board
// is larger than the terminal only a viewport of it is drawn: cols × rows cells
// starting at x0, y0, which follows the cursor.
type layout struct {
	left, top, cw int
	pad           int // trailing column so two-wide cells sit centred in the border
	boardW        int // drawn width, border included
	cols, rows    int // cells in the viewport
	x0, y0        int // first drawn cell
	totalCols     int
	totalRows     int
	fits          bool // the whole board is on screen
	viable        bool // the terminal can show a usable viewport
}

func (g *Game) layout() layout {
	l := boardLayout(g.board.Width(), g.board.Height(), g.width, g.height)
	l.x0, l.y0 = l.clampOrigin(g.vx, g.vy)
	return l
}

// fullW and fullH are the size the whole board would need on screen.
func (l layout) fullW() int { return l.totalCols*l.cw + l.pad + 2 }
func (l layout) fullH() int { return l.totalRows + chromeLines }

// scrolls reports whether the board is larger than the terminal.
func (l layout) scrolls() bool { return l.viable && !l.fits }

func boardLayout(cols, rows, width, height int) layout {
	l := layout{cw: 3, totalCols: cols, totalRows: rows}
	if cols*3+2 > width {
		l.cw, l.pad = 2, 1
	}
	l.cols = min(cols, max(0, (width-2-l.pad)/l.cw))
	l.rows = min(rows, max(0, height-chromeLines))
	l.viable = l.cols >= min(minViewCols, cols) && l.rows >= min(minViewRows, rows)
	l.fits = l.cols == cols && l.rows == rows
	l.boardW = l.cols*l.cw + l.pad + 2
	l.left = max(0, (width-l.boardW)/2)
	l.top = max(0, (height-(l.rows+chromeLines))/2)
	return l
}

// clampOrigin keeps the viewport's first cell inside the board.
func (l layout) clampOrigin(x0, y0 int) (int, int) {
	return max(0, min(x0, l.totalCols-l.cols)), max(0, min(y0, l.totalRows-l.rows))
}

func (l layout) faceX() int { return l.left + (l.boardW-faceWidth)/2 }

func (l layout) hit(x, y int) (hitTarget, int, int) {
	if !l.viable {
		return hitNone, 0, 0
	}
	if y == l.top && x >= l.faceX() && x < l.faceX()+faceWidth {
		return hitFace, 0, 0
	}
	col, row := x-l.left-1, y-l.top-2
	if col < 0 || row < 0 || col >= l.cols*l.cw || row >= l.rows {
		return hitNone, 0, 0
	}
	return hitCell, l.x0 + col/l.cw, l.y0 + row
}

func (g *Game) View() string {
	if g.width <= 0 || g.height <= 0 {
		return ""
	}
	if g.setup != nil {
		return g.customView()
	}
	l := g.layout()
	if !l.viable {
		return g.tooSmall(l)
	}
	pad := strings.Repeat(" ", l.left)
	lines := make([]string, l.top, g.height)
	lines = append(lines, pad+g.header(l))
	for _, row := range strings.Split(g.grid(l), "\n") {
		lines = append(lines, pad+row)
	}
	lines = append(lines, g.centre(statusSt.Render(truncate(g.status(), g.width))), g.centre(helpSt.Render(truncate(help, g.width))))
	for len(lines) < g.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (g *Game) header(l layout) string {
	left := counterSt.Render(fmt.Sprintf(" %03d ", max(-99, min(999, g.board.MinesLeft()))))
	right := counterSt.Render(fmt.Sprintf(" %03d ", min(999, int(g.Elapsed().Seconds()))))
	face := faceSt.Render(" " + faces[g.board.State()] + " ")
	gapL := l.faceX() - l.left - lipgloss.Width(left)
	gapR := l.boardW - gapL - lipgloss.Width(left) - faceWidth - lipgloss.Width(right)
	return left + strings.Repeat(" ", max(0, gapL)) + face + strings.Repeat(" ", max(0, gapR)) + right
}

// grid draws the viewport only, so a huge board costs no more than a small one.
func (g *Game) grid(l layout) string {
	b := g.board
	rows := make([]string, 0, l.rows)
	var sb strings.Builder
	for y := l.y0; y < l.y0+l.rows; y++ {
		sb.Reset()
		for x := l.x0; x < l.x0+l.cols; x++ {
			glyph, st := g.cell(x, y)
			if x == g.cx && y == g.cy && b.State() == Playing {
				st = cursorSt
			}
			sb.WriteString(st.Render(cellText(glyph, l.cw)))
		}
		rows = append(rows, sb.String()+strings.Repeat(" ", l.pad))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColors[b.State()])).
		Render(strings.Join(rows, "\n"))
}

func cellText(glyph string, cw int) string {
	if cw == 3 {
		return " " + glyph + " "
	}
	return " " + glyph
}

func (g *Game) cell(x, y int) (string, lipgloss.Style) {
	b := g.board
	over := b.State() == Lost
	switch {
	case b.Flagged(x, y) && over && !b.IsMine(x, y):
		return "x", flagSt
	case b.Flagged(x, y):
		return "⚑", flagSt
	case b.IsMine(x, y) && over:
		if ex, ey, _ := b.Exploded(); ex == x && ey == y {
			return "*", explodedSt
		}
		return "*", mineSt
	case !b.Revealed(x, y):
		return "·", hiddenSt
	case b.Adjacent(x, y) == 0:
		return " ", lipgloss.NewStyle()
	}
	n := b.Adjacent(x, y)
	return fmt.Sprint(n), lipgloss.NewStyle().Foreground(lipgloss.Color(numberColors[n])).Bold(true)
}

func (g *Game) status() string {
	secs := int(g.Elapsed().Seconds())
	switch g.board.State() {
	case Won:
		return fmt.Sprintf("Cleared in %ds! n for another", secs)
	case Lost:
		return "Boom. n to try again"
	}
	s := fmt.Sprintf("%s  %d×%d  %d mines", g.level.Name, g.level.W, g.level.H, g.board.Mines())
	// With a scrolling viewport the status line also says which cells are shown.
	if l := g.layout(); l.scrolls() {
		s += fmt.Sprintf("  cols %d-%d/%d  rows %d-%d/%d",
			l.x0+1, l.x0+l.cols, l.totalCols, l.y0+1, l.y0+l.rows, l.totalRows)
	}
	return s
}

func (g *Game) tooSmall(l layout) string {
	need := fmt.Sprintf("%d×%d", l.fullW(), l.fullH())
	msg := []string{
		statusSt.Render(truncate("Terminal too small for "+g.level.Name+" ("+need+")", g.width)),
		helpSt.Render(truncate("1-3 change level  c custom  q quit", g.width)),
	}
	return strings.Join(msg, "\n")
}

func (g *Game) centre(s string) string {
	return strings.Repeat(" ", max(0, (g.width-lipgloss.Width(s))/2)) + s
}

func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:max(0, w)])
}

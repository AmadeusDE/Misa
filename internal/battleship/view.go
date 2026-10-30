package battleship

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	title = "Battleship"
	help  = "arrows move  space place/fire  r rotate  f random fleet  n new  q quit"
	// chromeLines is what surrounds the boards: header, titles, status, help.
	chromeLines = 4
	// gap is the space between the two boards when they sit side by side.
	gap = 2
)

var (
	waterSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#458588"))
	fogSt     = lipgloss.NewStyle().Foreground(lipgloss.Color("#3c3836"))
	shipSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#a89984")).Bold(true)
	missSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598"))
	hitSt     = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Bold(true)
	sunkSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbf1c7")).Background(lipgloss.Color("#cc241d")).Bold(true)
	cursorSt  = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#fabd2f")).Bold(true)
	previewSt = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#b8bb26"))
	headerSt  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2")).Bold(true)
	labelSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	statusSt  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
	helpSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))

	borderColors = map[State]string{
		Playing: "#665c54",
		Won:     "#b8bb26",
		Lost:    "#fb4934",
	}
)

// hitTarget says which board a screen cell belongs to.
type hitTarget int

const (
	hitNone hitTarget = iota
	hitPlayer
	hitEnemy
)

// layout places the two boards: side by side when they fit, otherwise stacked,
// with the header above and the status and help lines below.
type layout struct {
	left, top      int
	cw, pad        int // cell width and the trailing column that centres narrow cells
	boardW, boardH int // one bordered board
	stacked        bool
	totalW, totalH int
	fits           bool
}

func (g *Game) layout() layout {
	var last layout
	for _, c := range []struct {
		cw      int
		stacked bool
	}{{3, false}, {2, false}, {3, true}, {2, true}} {
		l := layout{cw: c.cw, stacked: c.stacked, boardH: Size + 2}
		if l.cw == 2 {
			l.pad = 1
		}
		l.boardW = Size*l.cw + l.pad + 2
		l.totalW, l.totalH = l.dims()
		if l.totalW <= g.width && l.totalH <= g.height {
			l.fits = true
			l.left = max(0, (g.width-l.totalW)/2)
			l.top = max(0, (g.height-l.totalH)/2)
			return l
		}
		last = l
	}
	return last // the narrowest arrangement, for the too-small message
}

func (l layout) dims() (int, int) {
	if l.stacked {
		return l.boardW, 2*l.boardH + chromeLines + 1
	}
	return 2*l.boardW + gap, l.boardH + chromeLines
}

// origin is the screen position of a board's top-left border corner.
func (l layout) origin(enemy bool) (int, int) {
	x, y := l.left, l.top+2 // header, then the title line
	if l.stacked {
		if enemy {
			y += l.boardH + 1
		}
		return x, y
	}
	if enemy {
		x += l.boardW + gap
	}
	return x, y
}

// hit maps a screen cell to a square on one of the boards.
func (l layout) hit(x, y int) (hitTarget, int, int) {
	if !l.fits {
		return hitNone, 0, 0
	}
	for _, b := range []struct {
		target hitTarget
		enemy  bool
	}{{hitPlayer, false}, {hitEnemy, true}} {
		ox, oy := l.origin(b.enemy)
		col, row := x-ox-1, y-oy-1
		if col < 0 || row < 0 || col >= Size*l.cw || row >= Size {
			continue
		}
		return b.target, col / l.cw, row
	}
	return hitNone, 0, 0
}

func (g *Game) View() string {
	if g.width <= 0 || g.height <= 0 {
		return ""
	}
	l := g.layout()
	if !l.fits {
		return g.tooSmall(l)
	}
	pad := strings.Repeat(" ", l.left)
	lines := make([]string, l.top, g.height)
	// The header is centred on the screen: with the boards centred too, that
	// keeps it over them when it is narrower, and uses the whole width when not.
	lines = append(lines, centreIn(headerSt, g.header(), g.width))
	if l.stacked {
		lines = append(lines, pad+centreIn(labelSt, "Your fleet", l.boardW))
		lines = append(lines, framed(g.board(l, false), pad)...)
		lines = append(lines, pad+centreIn(labelSt, "Your shots", l.boardW))
		lines = append(lines, framed(g.board(l, true), pad)...)
	} else {
		titles := centreIn(labelSt, "Your fleet", l.boardW) +
			strings.Repeat(" ", gap) + centreIn(labelSt, "Your shots", l.boardW)
		lines = append(lines, pad+titles)
		left, right := framed(g.board(l, false), ""), framed(g.board(l, true), "")
		for i := range left {
			lines = append(lines, pad+left[i]+strings.Repeat(" ", gap)+right[i])
		}
	}
	lines = append(lines, g.align(l, statusSt, g.status()), g.align(l, helpSt, help))
	for len(lines) < g.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// framed splits a rendered board into lines, each with the left margin.
func framed(board, pad string) []string {
	lines := strings.Split(board, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return lines
}

// board draws one bordered grid: the player's own fleet, or the enemy's with
// unfired squares fogged over.
func (g *Game) board(l layout, enemy bool) string {
	rows := make([]string, Size)
	var sb strings.Builder
	for y := range Size {
		sb.Reset()
		for x := range Size {
			sb.WriteString(g.cell(l, enemy, x, y))
		}
		rows[y] = sb.String() + strings.Repeat(" ", l.pad)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColors[g.state])).
		Render(strings.Join(rows, "\n"))
}

func (g *Game) cell(l layout, enemy bool, x, y int) string {
	glyph, st := g.glyph(enemy, x, y)
	switch {
	case !enemy && g.preview(x, y):
		st = previewSt
	case g.cursorOn(enemy, x, y):
		st = cursorSt
	}
	if l.cw == 3 {
		return st.Render(" " + glyph + " ")
	}
	return st.Render(" " + glyph)
}

// glyph is what a square holds: water or fog, a ship, a miss or a hit, with a
// sunk ship's squares filled in.
func (g *Game) glyph(enemy bool, x, y int) (string, lipgloss.Style) {
	b := g.player
	if enemy {
		b = g.enemy
	}
	ship := b.ShipAt(x, y)
	switch {
	case ship != nil && ship.Sunk():
		return "#", sunkSt
	case ship != nil && b.Fired(x, y):
		return "✕", hitSt
	case ship != nil && enemy:
		return "·", fogSt // the enemy's fleet stays hidden
	case ship != nil:
		return "■", shipSt
	case b.Fired(x, y):
		return "o", missSt
	case enemy:
		return "·", fogSt
	}
	return "·", waterSt
}

// cursorOn reports whether the cursor sits on a square of that board.
func (g *Game) cursorOn(enemy bool, x, y int) bool {
	if x != g.cx || y != g.cy {
		return false
	}
	switch g.phase {
	case Placing:
		return !enemy
	case Firing:
		return enemy
	}
	return false
}

// preview reports whether the pending ship would cover a square under the
// cursor, so the player can see where it goes before placing it.
func (g *Game) preview(x, y int) bool {
	pending := g.player.Pending()
	if len(pending) == 0 || !g.player.CanPlace(pending[0].Len, g.cx, g.cy, g.horizontal) {
		return false
	}
	ship := &Ship{Len: pending[0].Len, X: g.cx, Y: g.cy, Horizontal: g.horizontal}
	for _, p := range ship.Cells() {
		if p.X == x && p.Y == y {
			return true
		}
	}
	return false
}

func (g *Game) header() string {
	secs := int(g.Elapsed().Seconds())
	if g.phase == Placing {
		return fmt.Sprintf("%s · laying out your fleet · %d ships to go · %ds",
			title, len(g.player.Pending()), secs)
	}
	return fmt.Sprintf("%s · enemy %d afloat · your fleet %d afloat · %d shots · %ds",
		title, g.enemy.Afloat(), g.player.Afloat(), g.shots, secs)
}

func (g *Game) status() string {
	switch g.state {
	case Won:
		return fmt.Sprintf("Fleet sunk in %ds!  n for another game", int(g.Elapsed().Seconds()))
	case Lost:
		return "Your fleet is at the bottom.  n to try again"
	}
	if g.note != "" {
		return g.note
	}
	if g.phase == Placing {
		if pending := g.player.Pending(); len(pending) > 0 {
			lie := "vertical"
			if g.horizontal {
				lie = "horizontal"
			}
			return fmt.Sprintf("Place your %s (%d, %s)  r rotates", pending[0].Name, pending[0].Len, lie)
		}
	}
	return "Fire at the enemy fleet"
}

func (g *Game) tooSmall(l layout) string {
	msg := fmt.Sprintf("Terminal too small (needs %d×%d)", l.boardW, l.totalH)
	if len([]rune(msg)) > g.width {
		msg = fmt.Sprintf("too small: needs %d×%d", l.boardW, l.totalH)
	}
	lines := []string{
		statusSt.Render(truncate(msg, g.width)),
		helpSt.Render(truncate("n new  q quit", g.width)),
	}
	return strings.Join(lines, "\n")
}

// align lines a line up with the boards, moving it left as far as it needs to
// fit, and cuts it at the screen edge.
func (g *Game) align(l layout, st lipgloss.Style, text string) string {
	text = truncate(text, g.width)
	left := min(l.left, max(0, g.width-len([]rune(text))))
	return strings.Repeat(" ", left) + st.Render(text)
}

// centreIn centres plain text in a width and styles it.
func centreIn(st lipgloss.Style, text string, width int) string {
	text = truncate(text, width)
	return strings.Repeat(" ", max(0, (width-len([]rune(text)))/2)) + st.Render(text)
}

func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:max(0, w)])
}

package chess

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	chromeLines = 6 // header, two border rows, file letters, status, help
	panelWidth  = 26
	help        = "space select/move  u undo  c computer  f flip  t letters  v variant  n new  q quit"
)

var (
	lightSq   = lipgloss.Color("#a89984")
	darkSq    = lipgloss.Color("#665c54")
	cursorBg  = lipgloss.Color("#d65d0e")
	selectBg  = lipgloss.Color("#458588")
	lastBg    = lipgloss.Color("#98971a")
	captureBg = lipgloss.Color("#b16286")
	checkBg   = lipgloss.Color("#fb4934")

	pieceFg = map[Color]lipgloss.Style{
		White: lipgloss.NewStyle().Foreground(lipgloss.Color("#fbf1c7")).Bold(true),
		Black: lipgloss.NewStyle().Foreground(lipgloss.Color("#1d2021")).Bold(true),
	}
	dotSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828"))
	labelSt  = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	headerSt = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2")).Bold(true)
	statusSt = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
	helpSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	choiceSt = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#fabd2f")).Bold(true)

	borderPlaying = lipgloss.Color("#665c54")
	borderCheck   = lipgloss.Color("#fb4934")
	borderOver    = lipgloss.Color("#b8bb26")
)

// layout places the board, with rank numbers on its left and file letters
// below, and the move list to its right when there is room.
type layout struct {
	left, top  int
	labelW     int
	cw, pad    int
	boardW     int
	panel      bool
	cols, rows int
	fits       bool
}

func (g *Game) layout() layout {
	v := g.match.Variant()
	l := layout{cw: 3, cols: v.Files, rows: v.Ranks, labelW: len(strconv.Itoa(v.Ranks)) + 1}
	if l.labelW+l.cols*3+2 > g.width {
		l.cw, l.pad = 2, 1
	}
	l.boardW = l.labelW + l.cols*l.cw + l.pad + 2
	width := l.boardW
	if width+2+panelWidth <= g.width {
		l.panel = true
		width += 2 + panelWidth
	}
	height := l.rows + chromeLines
	l.fits = l.boardW <= g.width && height <= g.height
	l.left = max(0, (g.width-width)/2)
	l.top = max(0, (g.height-height)/2)
	return l
}

func (l layout) statusY() int { return l.top + l.rows + 4 }

// square returns the board square under a screen cell.
func (l layout) square(g *Game, x, y int) (int, bool) {
	if !l.fits {
		return 0, false
	}
	col, row := x-l.left-l.labelW-1, y-l.top-2
	if col < 0 || row < 0 || col >= l.cols*l.cw || row >= l.rows {
		return 0, false
	}
	fx, fy := g.boardXY(col/l.cw, row)
	return g.match.Variant().Square(fx, fy), true
}

// boardXY turns a screen column and row of the board into file and rank.
func (g *Game) boardXY(col, row int) (int, int) {
	v := g.match.Variant()
	if g.flipped {
		return v.Files - 1 - col, row
	}
	return col, v.Ranks - 1 - row
}

const promoPrefix = "Promote to: "

// promoChoice returns which promotion choice a click on the status line hit.
func (l layout) promoChoice(g *Game, x, y int) int {
	if !l.fits || y != l.statusY() {
		return -1
	}
	at := l.left + len(promoPrefix)
	for i, mv := range g.promo {
		w := lipgloss.Width(g.choiceLabel(mv))
		if x >= at && x < at+w {
			return i
		}
		at += w + 1
	}
	return -1
}

func (g *Game) choiceLabel(mv Move) string {
	t := g.match.Variant().Pieces[mv.Promote-1]
	return " " + g.glyph(t, g.match.Turn()) + " " + t.Name + " "
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
	board := strings.Split(g.board(l), "\n")
	panel := g.panel(len(board))
	lines := make([]string, l.top, g.height)
	lines = append(lines, g.place(l, headerSt, g.header()))
	for i, row := range board {
		if l.panel {
			row += "  " + panel[i]
		}
		lines = append(lines, pad+row)
	}
	status := pad + g.promoLine()
	if g.promo == nil {
		status = g.place(l, statusSt, g.status())
	}
	lines = append(lines, status, g.place(l, helpSt, help))
	for len(lines) < g.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (g *Game) header() string {
	m := g.match
	st, winner := m.Status()
	switch st {
	case Playing:
		turn := m.Turn().String() + " to move"
		if m.Position().InCheck() {
			turn += " · check"
		}
		if g.thinking {
			turn += " · thinking…"
		}
		return m.Variant().Name + " · " + turn
	case Checkmate:
		return m.Variant().Name + " · checkmate, " + winner.String() + " wins"
	}
	return m.Variant().Name + " · " + st.String()
}

// board renders the bordered squares with rank numbers to the left and file
// letters underneath.
func (g *Game) board(l layout) string {
	v := g.match.Variant()
	hints := g.hints()
	checked := g.checkedRoyals()
	last := g.lastMove()
	rows := make([]string, l.rows)
	var sb strings.Builder
	for row := range l.rows {
		sb.Reset()
		for col := range l.cols {
			x, y := g.boardXY(col, row)
			sq := v.Square(x, y)
			sb.WriteString(g.square(sq, x, y, l.cw, hints, checked[sq], last))
		}
		rows[row] = sb.String() + strings.Repeat(" ", l.pad)
	}
	border := borderPlaying
	switch {
	case g.match.Over():
		border = borderOver
	case g.match.Position().InCheck():
		border = borderCheck
	}
	grid := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Render(strings.Join(rows, "\n"))
	lines := strings.Split(grid, "\n")
	for i := range lines {
		label := strings.Repeat(" ", l.labelW)
		if i > 0 && i <= l.rows {
			_, y := g.boardXY(0, i-1)
			label = labelSt.Render(fmt.Sprintf("%*d ", l.labelW-1, y+1))
		}
		lines[i] = label + lines[i]
	}
	var files strings.Builder
	files.WriteString(strings.Repeat(" ", l.labelW+1))
	for col := range l.cols {
		x, _ := g.boardXY(col, 0)
		cell := string(rune('a' + x))
		if l.cw == 3 {
			cell = " " + cell + " "
		} else {
			cell = " " + cell
		}
		files.WriteString(cell)
	}
	lines = append(lines, labelSt.Render(files.String()))
	return strings.Join(lines, "\n")
}

type hint uint8

const (
	hintNone hint = iota
	hintMove
	hintCapture
)

// hints marks where the selected piece may go.
func (g *Game) hints() map[int]hint {
	h := map[int]hint{}
	if g.sel < 0 {
		return h
	}
	for _, mv := range g.movesBetween(g.sel, -1) {
		if mv.Take >= 0 {
			h[mv.To] = hintCapture
		} else {
			h[mv.To] = hintMove
		}
	}
	return h
}

func (g *Game) checkedRoyals() map[int]bool {
	p := g.match.Position()
	out := map[int]bool{}
	for _, sq := range p.Royals(p.Turn) {
		if p.anyAttacked([]int{sq}, p.Turn.Other()) {
			out[sq] = true
		}
	}
	return out
}

func (g *Game) lastMove() map[int]bool {
	played := g.match.Played()
	if len(played) == 0 {
		return nil
	}
	mv := played[len(played)-1]
	return map[int]bool{mv.From: true, mv.To: true}
}

func (g *Game) square(sq, x, y, cw int, hints map[int]hint, checked bool, last map[int]bool) string {
	p := g.match.Position()
	bg := darkSq
	if (x+y)%2 == 1 {
		bg = lightSq
	}
	switch {
	case x == g.cx && y == g.cy && !g.match.Over():
		bg = cursorBg
	case sq == g.sel:
		bg = selectBg
	case checked:
		bg = checkBg
	case hints[sq] == hintCapture:
		bg = captureBg
	case last[sq]:
		bg = lastBg
	}
	text, st := " ", dotSt
	if pc := p.Board[sq]; pc.Type != 0 {
		text, st = g.glyph(p.V.Type(pc), pc.Color), pieceFg[pc.Color]
	} else if hints[sq] == hintMove {
		text = "•"
	}
	if cw == 3 {
		text = " " + text + " "
	} else {
		text = " " + text
	}
	return st.Background(bg).Render(text)
}

func (g *Game) glyph(t *PieceType, c Color) string {
	if t.Glyph != "" && !g.letters {
		return t.Glyph
	}
	if c == Black {
		return strings.ToLower(string(t.Letter))
	}
	return string(t.Letter)
}

// panel lists the moves so far in numbered pairs, newest at the bottom.
func (g *Game) panel(height int) []string {
	notation := g.match.Notation()
	var rows []string
	for i := 0; i < len(notation); i += 2 {
		black := ""
		if i+1 < len(notation) {
			black = notation[i+1]
		}
		rows = append(rows, fmt.Sprintf("%3d. %-10s %-10s", i/2+1, notation[i], black))
	}
	lines := []string{labelSt.Render("Moves")}
	if len(rows) > height-1 {
		rows = rows[len(rows)-(height-1):]
	}
	for _, r := range rows {
		lines = append(lines, statusSt.Render(truncate(r, panelWidth)))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

// place aligns a line with the board, moving it left as far as it needs to
// fit, and cuts it at the screen edge.
func (g *Game) place(l layout, st lipgloss.Style, text string) string {
	text = truncate(text, g.width)
	left := min(l.left, max(0, g.width-len([]rune(text))))
	return strings.Repeat(" ", left) + st.Render(text)
}

func (g *Game) promoLine() string {
	var b strings.Builder
	b.WriteString(statusSt.Render(promoPrefix))
	for i, mv := range g.promo {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(choiceSt.Render(g.choiceLabel(mv)))
	}
	return b.String()
}

func (g *Game) status() string {
	text := g.note
	switch {
	case text != "":
	case g.thinking:
		text = "The computer is thinking…"
	case g.sel >= 0 && len(g.movesBetween(g.sel, g.sel)) > 0:
		text = "Choose its own square again to pass"
	default:
		if n := g.match.Notation(); len(n) > 0 {
			text = "Last move: " + n[len(n)-1]
		}
		if g.match.Over() {
			text += "  ·  n for a new game, u to take back"
		}
	}
	return text
}

func (g *Game) tooSmall(l layout) string {
	need := fmt.Sprintf("%d×%d", l.boardW, l.rows+chromeLines)
	msg := []string{
		statusSt.Render(truncate("Terminal too small for "+g.match.Variant().Name+" ("+need+")", g.width)),
		helpSt.Render(truncate("v change variant  q quit", g.width)),
	}
	return strings.Join(msg, "\n")
}

func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:max(0, w)])
}

package chess

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var teamPieceStyles = map[FourSide]lipgloss.Style{
	FourRed:    lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Bold(true),
	FourBlue:   lipgloss.NewStyle().Foreground(lipgloss.Color("#83a5ff")).Bold(true),
	FourYellow: lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f")).Bold(true),
	FourGreen:  lipgloss.NewStyle().Foreground(lipgloss.Color("#b8e36b")).Bold(true),
}

func fourSideName(s FourSide) string {
	switch s {
	case FourRed:
		return "Red"
	case FourBlue:
		return "Blue"
	case FourYellow:
		return "Yellow"
	case FourGreen:
		return "Green"
	}
	return "?"
}

type fourThoughtMsg struct {
	gen   int
	move  FourMove
	found bool
}

func (g *Game) fourThink() tea.Cmd {
	if g.thinking || g.quit || g.four.Finished || g.four.Turn == FourRed {
		return nil
	}
	g.thinking = true
	g.gen++
	gen, match := g.gen, g.four.Clone()
	return func() tea.Msg { mv, ok := match.ComputerMove(); return fourThoughtMsg{gen, mv, ok} }
}

func (g *Game) fourThought(msg fourThoughtMsg) {
	if msg.gen != g.gen || g.four == nil {
		return
	}
	g.thinking = false
	if msg.found && g.four.Apply(msg.move) == nil {
		g.sel = -1
	}
}

func (g *Game) fourKey(k string) {
	g.note = ""
	switch k {
	case "q", "ctrl+c":
		g.quit = true
	case "esc":
		if g.sel >= 0 {
			g.sel = -1
		} else {
			g.quit = true
		}
	case "up", "k":
		g.fourCursor(0, -1)
	case "down", "j":
		g.fourCursor(0, 1)
	case "left", "h":
		g.fourCursor(-1, 0)
	case "right", "l":
		g.fourCursor(1, 0)
	case "space", "enter":
		g.fourActivate(FourSquare{Row: g.cy, Col: g.cx})
	case "u", "backspace":
		g.cancel()
		g.sel = -1
		if !g.four.Undo() {
			g.note = "Nothing to take back"
			return
		}
		for g.four.Turn != FourRed {
			if !g.four.Undo() {
				break
			}
		}
	case "n":
		g.newGame(g.variant)
	case "v", "V":
		g.cycleVariant(map[string]int{"v": 1, "V": -1}[k])
	case "t":
		g.letters = !g.letters
	case "f":
		g.flipped = !g.flipped
	case "c", "d":
		g.note = "Teams: Red is human; Blue, Yellow and Green are computers"
	}
}

func (g *Game) fourCursor(dx, dy int) {
	if g.flipped {
		dx, dy = -dx, -dy
	}
	x, y := g.cx, g.cy
	for {
		x, y = x+dx, y+dy
		if x < 0 || x >= 14 || y < 0 || y >= 14 {
			return
		}
		if g.four.Board[y][x].Type != FourHole {
			g.cx, g.cy = x, y
			return
		}
	}
}

func (g *Game) fourActivate(s FourSquare) {
	if g.four.Finished || g.four.Turn != FourRed || g.four.Board[s.Row][s.Col].Type == FourHole {
		return
	}
	from := FourSquare{Row: g.sel / 14, Col: g.sel % 14}
	for _, mv := range g.four.LegalMoves() {
		if g.sel >= 0 && mv.From == from && mv.To == s {
			_ = g.four.Apply(mv)
			g.sel = -1
			return
		}
	}
	old := g.sel
	g.sel = -1
	if old != s.Row*14+s.Col {
		for _, mv := range g.four.LegalMoves() {
			if mv.From == s {
				g.sel = s.Row*14 + s.Col
				break
			}
		}
	}
}

func (g *Game) fourLayout() layout {
	l := layout{cols: 14, rows: 14, labelW: 3, cw: 3}
	if l.labelW+14*3+2 > g.width {
		l.cw = 2
	}
	l.boardW = l.labelW + 14*l.cw + 2
	l.fits = l.boardW <= g.width && 14+chromeLines <= g.height
	l.left = max(0, (g.width-l.boardW)/2)
	l.top = max(0, (g.height-14-chromeLines)/2)
	return l
}
func (g *Game) fourXY(col, row int) (int, int) {
	if g.flipped {
		return 13 - col, 13 - row
	}
	return col, row
}
func (g *Game) fourClick(m tea.Mouse) {
	l := g.fourLayout()
	if !l.fits {
		return
	}
	col, row := m.X-l.left-l.labelW-1, m.Y-l.top-2
	if col < 0 || col >= 14*l.cw || row < 0 || row >= 14 {
		return
	}
	x, y := g.fourXY(col/l.cw, row)
	if g.four.Board[y][x].Type == FourHole {
		return
	}
	if m.Button == tea.MouseRight {
		g.sel = -1
	} else if m.Button == tea.MouseLeft {
		g.cx, g.cy = x, y
		g.fourActivate(FourSquare{Row: y, Col: x})
	}
}

func (g *Game) fourView() string {
	if g.width <= 0 || g.height <= 0 {
		return ""
	}
	l := g.fourLayout()
	if !l.fits {
		return g.tooSmall(l)
	}
	header := g.variant.Name + " · " + fourSideName(g.four.Turn) + " to move"
	if g.four.Finished {
		switch g.four.Winner {
		case FourRedYellow:
			header = g.variant.Name + " · Red + Yellow win"
		case FourBlueGreen:
			header = g.variant.Name + " · Blue + Green win"
		default:
			header = g.variant.Name + " · draw"
		}
	} else {
		if g.four.InCheck(g.four.Turn) {
			header += " · check"
		}
		if g.thinking {
			header += " · thinking…"
		}
	}
	hints := map[FourSquare]bool{}
	if g.sel >= 0 {
		from := FourSquare{Row: g.sel / 14, Col: g.sel % 14}
		for _, mv := range g.four.LegalMoves() {
			if mv.From == from {
				hints[mv.To] = true
			}
		}
	}
	rows := make([]string, 14)
	for row := range 14 {
		var b strings.Builder
		for col := range 14 {
			x, y := g.fourXY(col, row)
			pc := g.four.Board[y][x]
			if pc.Type == FourHole {
				b.WriteString(strings.Repeat(" ", l.cw))
				continue
			}
			bg := darkSq
			if (x+y)%2 == 1 {
				bg = lightSq
			}
			switch {
			case x == g.cx && y == g.cy && !g.four.Finished:
				bg = cursorBg
			case g.sel == y*14+x:
				bg = selectBg
			case hints[FourSquare{Row: y, Col: x}] && pc.Type != FourEmpty:
				bg = captureBg
			}
			text, st := " ", dotSt
			if pc.Type != FourEmpty {
				text = fourGlyph(pc.Type, g.letters)
				st = teamPieceStyles[pc.Side]
			} else if hints[FourSquare{Row: y, Col: x}] {
				text = "•"
			}
			if l.cw == 3 {
				text = " " + text + " "
			} else {
				text = " " + text
			}
			b.WriteString(st.Background(bg).Render(text))
		}
		rows[row] = b.String()
	}
	grid := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderPlaying).Render(strings.Join(rows, "\n"))
	board := strings.Split(grid, "\n")
	for i := range board {
		label := strings.Repeat(" ", l.labelW)
		if i > 0 && i <= 14 {
			_, y := g.fourXY(0, i-1)
			label = labelSt.Render(fmt.Sprintf("%2d ", 14-y))
		}
		board[i] = label + board[i]
	}
	var files strings.Builder
	files.WriteString(strings.Repeat(" ", l.labelW+1))
	for col := range 14 {
		x, _ := g.fourXY(col, 0)
		if l.cw == 3 {
			files.WriteString(" " + fileName(x) + " ")
		} else {
			files.WriteString(" " + fileName(x))
		}
	}
	board = append(board, labelSt.Render(files.String()))
	lines := make([]string, l.top)
	lines = append(lines, g.place(l, headerSt, header))
	pad := strings.Repeat(" ", l.left)
	for _, row := range board {
		lines = append(lines, pad+row)
	}
	status := g.note
	if status == "" {
		status = "Red + Yellow vs Blue + Green · you play Red"
	}
	lines = append(lines, g.place(l, statusSt, status), g.place(l, helpSt, "space select/move  u undo  f flip  t letters  v variant  n new  q quit"))
	for len(lines) < g.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func fourGlyph(t FourPieceType, letters bool) string {
	switch t {
	case FourPawn:
		if letters {
			return "P"
		}
		return "♙"
	case FourKnight:
		if letters {
			return "N"
		}
		return "♘"
	case FourBishop:
		if letters {
			return "B"
		}
		return "♗"
	case FourRook:
		if letters {
			return "R"
		}
		return "♖"
	case FourQueen:
		if letters {
			return "Q"
		}
		return "♕"
	case FourKing:
		if letters {
			return "K"
		}
		return "♔"
	}
	return " "
}

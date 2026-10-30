package chess

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Each board has its pockets on separate lines. The layouts are side by side
// when possible and stacked otherwise; the same geometry drives hit testing.
type bugGeometry struct {
	a, b          layout
	side, fits    bool
	width, height int
}

func (g *Game) bugLayout() bugGeometry {
	v := g.match.Variant()
	cw := 3
	// Compact cells make a stacked view usable in a narrow terminal.
	if g.width < 2*(v.Files*3+4)+3 {
		cw = 2
	}
	l := layout{cw: cw, cols: v.Files, rows: v.Ranks, labelW: len(fmt.Sprint(v.Ranks)) + 1}
	l.boardW = l.labelW + l.cols*cw + 2
	width := l.boardW
	for _, m := range []*Match{g.bug.Board(0), g.bug.Board(1)} {
		copy := *g
		copy.match = m
		width = max(width, lipgloss.Width(copy.pocketText(White)), lipgloss.Width(copy.pocketText(Black)))
	}
	// Global header, then board title and black pocket, board, white pocket.
	boardHeight := l.rows + 6
	geo := bugGeometry{width: width, height: boardHeight}
	geo.side = 2*width+3 <= g.width
	neededW, neededH := width, 1+2*boardHeight+2
	if geo.side {
		neededW, neededH = 2*width+3, 1+boardHeight+2
	}
	geo.fits = neededW <= g.width && neededH <= g.height
	left := max(0, (g.width-neededW)/2)
	top := max(0, (g.height-neededH)/2)
	l.left, l.top, l.fits = left, top+2, geo.fits
	geo.a = l
	if geo.side {
		l.left = left + width + 3
	} else {
		l.top += boardHeight
	}
	geo.b = l
	return geo
}

func (g *Game) bugView() string {
	geo := g.bugLayout()
	if !geo.fits {
		return statusSt.Render(truncate("Terminal too small for both bughouse boards", g.width)) + "\n" + helpSt.Render(truncate("v change variant  q quit", g.width))
	}
	a, b := *g, *g
	a.flipped = false
	b.match = g.bug.Board(1)
	b.flipped = false
	b.cx, b.cy, b.sel, b.drop = -1, -1, -1, 0
	render := func(game *Game, l layout, title string) []string {
		rows := []string{headerSt.Render(title), game.pocketRender(Black)}
		rows = append(rows, strings.Split(game.board(l), "\n")...)
		return append(rows, game.pocketRender(White))
	}
	ar := render(&a, geo.a, "Board A · you White")
	br := render(&b, geo.b, "Board B · partner Black")
	lines := make([]string, geo.a.top-2)
	st, board, winner := g.bug.Status()
	header := fmt.Sprintf("Bughouse · %s · board %s %s to move", g.level.String(), []string{"A", "B"}[g.bugPly%2], g.bug.Board(g.bugPly%2).Turn())
	if g.thinking {
		header += " · thinking…"
	}
	if st != Playing {
		header = fmt.Sprintf("Bughouse · board %s · %s", []string{"A", "B"}[board], st)
		if st == Checkmate {
			header += fmt.Sprintf(", %s wins (team %d)", winner, g.bug.WinnerTeam())
		}
	}
	lines = append(lines, g.place(geo.a, headerSt, header))
	if geo.side {
		for i := range ar {
			lines = append(lines, strings.Repeat(" ", geo.a.left)+ar[i]+strings.Repeat(" ", max(0, geo.width-lipgloss.Width(ar[i])))+"   "+br[i])
		}
	} else {
		for _, row := range ar {
			lines = append(lines, strings.Repeat(" ", geo.a.left)+row)
		}
		for _, row := range br {
			lines = append(lines, strings.Repeat(" ", geo.b.left)+row)
		}
	}
	status := g.place(geo.a, statusSt, g.status())
	if g.promo != nil {
		status = strings.Repeat(" ", geo.a.left) + ansi.Truncate(g.promoLine(), g.width-geo.a.left, "")
	}
	lines = append(lines, status, helpSt.Render(truncate("A–Z pocket  arrows move  enter select  u undo  d level  t letters  v variant  n new  q quit", g.width)))
	for len(lines) < g.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (g *Game) bugPocketChoice(l layout, x, y int) uint8 {
	if !l.fits || y != l.top+l.rows+4 {
		return 0
	}
	p := g.match.Position()
	at := l.left + len(White.String()) + 1
	for i, pt := range p.V.Pieces {
		typ := i + 1
		if typ >= len(p.Hands[White]) || p.Hands[White][typ] <= 0 {
			continue
		}
		w := lipgloss.Width(g.pocketLabel(pt, White, p.Hands[White][typ]))
		if x >= at && x < at+w {
			return uint8(typ)
		}
		at += w + 1
	}
	return 0
}

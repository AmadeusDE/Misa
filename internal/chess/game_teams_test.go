package chess

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestTeamsUI(t *testing.T) {
	g := newGame(t, "fourplayer", 80, 24)
	if g.four == nil || g.four.Turn != FourRed {
		t.Fatal("not a red Teams game")
	}
	view := ansi.Strip(g.View())
	if !strings.Contains(view, "Red") || !strings.Contains(view, "Yellow") {
		t.Fatal(view)
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > 80 {
			t.Fatal("line too wide")
		}
	}
	l := g.fourLayout()
	// A click on a corner hole cannot move the cursor or select.
	g.fourClick(tea.Mouse{X: l.left + l.labelW + 1, Y: l.top + 2, Button: tea.MouseLeft})
	if g.sel != -1 || g.cx != 6 || g.cy != 12 {
		t.Fatal("corner click selected a hole")
	}
	moves := g.four.LegalMoves()
	if len(moves) == 0 {
		t.Fatal("no opening moves")
	}
	mv := moves[0]
	g.fourActivate(mv.From)
	g.fourActivate(mv.To)
	if g.four.Turn != FourBlue {
		t.Fatal("red move was not applied")
	}
	cmd := g.think()
	if cmd == nil {
		t.Fatal("no computer command")
	}
	g.Update(cmd())
	if g.four.Turn != FourYellow {
		t.Fatal("blue did not reply")
	}
	g.fourKey("u")
	if g.four.Turn != FourRed {
		t.Fatal("undo did not return to red")
	}
}

func TestTeamsAbandonedSearch(t *testing.T) {
	g := newGame(t, "fourplayer", 80, 24)
	mv := g.four.LegalMoves()[0]
	g.fourActivate(mv.From)
	g.fourActivate(mv.To)
	cmd := g.think()
	g.fourKey("n")
	g.Update(cmd())
	if g.four.Turn != FourRed {
		t.Fatal("obsolete computer move was applied")
	}
}

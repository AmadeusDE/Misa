package chess

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func dropGame(t *testing.T) (*Game, uint8) {
	t.Helper()
	g := newGame(t, "crazyhouse", 100, 24)
	v := g.match.Variant()
	var knight uint8
	for i, pt := range v.Pieces {
		if pt.Letter == 'N' {
			knight = uint8(i + 1)
		}
	}
	if knight == 0 {
		t.Fatal("no knight")
	}
	p := NewPosition(v)
	p.Hands[White] = make([]int, len(v.Pieces)+1)
	p.Hands[Black] = make([]int, len(v.Pieces)+1)
	p.Hands[White][knight] = 2
	p.Hands[Black][knight] = 1
	g.Load(p)
	return g, knight
}

func TestDropPocketClickAndView(t *testing.T) {
	g, knight := dropGame(t)
	out := ansi.Strip(g.View())
	if !strings.Contains(out, "White:") || !strings.Contains(out, "Black:") || !strings.Contains(out, "×2") {
		t.Fatalf("pockets missing: %s", out)
	}
	l := g.layout()
	found := false
	for y := range g.height {
		for x := range g.width {
			if l.pocketChoice(g, x, y) == knight {
				g.click(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft})
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found || g.drop != knight {
		t.Fatal("pocket selection failed")
	}
	e4, _ := g.match.Variant().ParseSquare("e4")
	if g.hints()[e4] != hintMove {
		t.Fatal("missing legal drop hint")
	}
	clickSquare(t, g, "e2", tea.MouseLeft)
	if g.drop != knight || len(g.match.Played()) != 0 {
		t.Fatal("illegal drop changed selection or position")
	}
	clickSquare(t, g, "e4", tea.MouseLeft)
	if g.drop != 0 || g.match.Position().Board[e4].Type != knight || g.match.Position().Hands[White][knight] != 1 {
		t.Fatal("drop failed")
	}
	keys(g, "u")
	if g.match.Position().Hands[White][knight] != 2 || g.drop != 0 {
		t.Fatal("undo did not restore pocket")
	}
}

func TestDropKeyboardAndReset(t *testing.T) {
	g, knight := dropGame(t)
	// Modified text must not masquerade as an unmodified uppercase key.
	g.Update(tea.KeyPressMsg{Code: 'n', Text: "N", Mod: tea.ModCtrl})
	if g.drop != 0 {
		t.Fatal("ctrl-modified text selected pocket")
	}
	// Text is what Key.String uses for printable keys, even with Shift set.
	g.Update(tea.KeyPressMsg{Code: 'n', Text: "N", Mod: tea.ModShift})
	if g.drop != knight {
		t.Fatal("shifted uppercase did not select pocket")
	}
	keys(g, "esc")
	if g.drop != 0 || g.quit {
		t.Fatal("escape did not clear selection")
	}
	g.Update(tea.KeyPressMsg{Code: 'n', Text: "N", Mod: tea.ModShift})
	// Cursor starts at d2. Move to d3 and drop there.
	keys(g, "up", "enter")
	d3, _ := g.match.Variant().ParseSquare("d3")
	if g.match.Position().Board[d3].Type != knight || g.drop != 0 {
		t.Fatal("keyboard drop failed")
	}
	keys(g, "u")
	g.Update(tea.KeyPressMsg{Code: 'n', Text: "N", Mod: tea.ModShift})
	keys(g, "n")
	if g.drop != 0 || len(g.match.Played()) != 0 {
		t.Fatal("new game did not reset selection")
	}
}

func TestDropFlippedPocketOwnership(t *testing.T) {
	g, knight := dropGame(t)
	g.flipped = true
	l := g.layout()
	// White is at the top when flipped; Black's lower pocket must not select.
	topY, bottomY := l.top+2, l.top+1+l.rows
	x := l.left + l.boardW + 2 + len("Black:") + 1
	g.click(tea.Mouse{X: x, Y: bottomY, Button: tea.MouseLeft})
	if g.drop != 0 {
		t.Fatal("selected opponent pocket")
	}
	x = l.left + l.boardW + 2 + len("White:") + 1
	g.click(tea.Mouse{X: x, Y: topY, Button: tea.MouseLeft})
	if g.drop != knight {
		t.Fatal("flipped own pocket not selectable")
	}
}

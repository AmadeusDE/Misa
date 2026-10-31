package chess

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDuckUIPlacementAndCancel(t *testing.T) {
	g := newGame(t, "duck", 80, 24)
	clickSquare(t, g, "e2", tea.MouseLeft)
	clickSquare(t, g, "e4", tea.MouseLeft)
	if len(g.duck) == 0 || len(g.match.Played()) != 0 {
		t.Fatal("piece move must await duck placement")
	}
	e2, _ := g.match.Variant().ParseSquare("e2")
	if g.hints()[e2] != hintMove {
		t.Fatal("vacated square should be a duck destination")
	}
	clickSquare(t, g, "e4", tea.MouseLeft)
	if len(g.duck) == 0 {
		t.Fatal("occupied destination accepted")
	}
	g.key("esc")
	if g.duck != nil || len(g.match.Played()) != 0 {
		t.Fatal("escape must cancel the whole turn")
	}
	clickSquare(t, g, "e2", tea.MouseLeft)
	clickSquare(t, g, "e4", tea.MouseLeft)
	clickSquare(t, g, "e2", tea.MouseLeft)
	if len(g.match.Played()) != 1 || g.match.Position().Blocker != e2 {
		t.Fatal("duck move not committed")
	}
	if !strings.Contains(g.View(), "D") {
		t.Fatal("duck glyph missing")
	}
	g.undo()
	if g.match.Position().Blocker != -1 {
		t.Fatal("undo did not restore duck")
	}
}

func TestDuckUIKeyboardAndPromotion(t *testing.T) {
	g := newGame(t, "duck", 80, 24)
	keys(g, "l", "space", "k", "k", "enter")
	if len(g.duck) == 0 {
		t.Fatal("keyboard piece move did not prompt for duck")
	}
	keys(g, "j", "j", "enter")
	if len(g.match.Played()) != 1 {
		t.Fatal("keyboard duck placement failed")
	}
	v := g.match.Variant()
	p, err := ParseFEN(v, "k7/4P3/8/8/8/8/8/K7 w -")
	if err != nil {
		t.Fatal(err)
	}
	g.Load(p)
	clickSquare(t, g, "e7", tea.MouseLeft)
	clickSquare(t, g, "e8", tea.MouseLeft)
	if len(g.promo) != 4 {
		t.Fatalf("promotion choices = %d", len(g.promo))
	}
	g.key("q")
	if len(g.duck) == 0 || g.quit {
		t.Fatal("promotion must proceed to duck placement")
	}
	clickSquare(t, g, "e7", tea.MouseLeft)
	if len(g.match.Played()) != 1 {
		t.Fatal("promoted turn not committed")
	}
}

package chess

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func fogGame(t *testing.T) *Game {
	t.Helper()
	g := newGame(t, "chess", 100, 30)
	v := *g.match.Variant()
	v.Fog = true
	g.newGame(&v)
	g.letters = true
	return g
}

func fogCell(g *Game, name string) string {
	v := g.match.Variant()
	sq, _ := v.ParseSquare(name)
	x, y := v.XY(sq)
	return ansi.Strip(g.square(sq, x, y, 3, g.hints(), false, g.lastMove(), g.match.Position().Visible(g.perspective())))
}

func TestFogPerspectiveAndFlipping(t *testing.T) {
	g := fogGame(t)
	g.Load(fen(t, g.match.Variant(), "7k/8/8/8/8/8/8/K7 w -"))
	if strings.Contains(fogCell(g, "h8"), "k") || !strings.Contains(fogCell(g, "a1"), "K") {
		t.Fatal("white perspective leaks hidden king or hides own king")
	}
	g.flipped = true
	if strings.Contains(fogCell(g, "h8"), "k") {
		t.Fatal("flip changed fog perspective")
	}
	g.SetComputer(true, false)
	if strings.Contains(fogCell(g, "a1"), "K") || !strings.Contains(fogCell(g, "h8"), "k") {
		t.Fatal("human Black perspective incorrect while White to move")
	}
	g.SetComputer(false, false)
	if g.perspective() != White {
		t.Fatal("hotseat must use side to move")
	}
	g.match.Position().Turn = Black
	if g.perspective() != Black {
		t.Fatal("hotseat perspective did not follow turn")
	}
}

func TestFogHidesHistoryAndComputerAnalysis(t *testing.T) {
	g := fogGame(t)
	g.SetComputer(false, true)
	mv := findMove(t, g.match.Position(), "e2", "e4")
	g.play(mv)
	if g.layout().panel || len(g.lastMove()) != 0 || strings.Contains(g.status(), "e2") {
		t.Fatal("fog exposed move history")
	}
	reply := findMove(t, g.match.Position(), "e7", "e5")
	g.thought(thoughtMsg{gen: g.gen, result: Result{Move: reply, Found: true, Depth: 5, Score: 1234}})
	out := ansi.Strip(g.View())
	for _, secret := range []string{"e7", "e5", "depth 5", "+12.34", "Moves", "Last move"} {
		if strings.Contains(out, secret) {
			t.Errorf("fog leaked %q", secret)
		}
	}
	if !strings.Contains(out, "Computer moved") {
		t.Error("missing safe computer notice")
	}
}

func TestFogShowsVisibleEnemyButNotDistantEnemy(t *testing.T) {
	g := fogGame(t)
	g.Load(fen(t, g.match.Variant(), "7k/8/8/8/8/1p6/8/R6K w -"))
	// A rook sees b1, not b3; the enemy king is also out of sight.
	if strings.Contains(fogCell(g, "b3"), "p") || strings.Contains(fogCell(g, "h8"), "k") {
		t.Fatal("distant enemy visible")
	}
	g.Load(fen(t, g.match.Variant(), "7k/8/8/8/8/p7/8/R6K w -"))
	if !strings.Contains(fogCell(g, "a3"), "p") {
		t.Fatal("enemy on rook ray hidden")
	}
}

func TestFogCaptureDoesNotExposeNotation(t *testing.T) {
	g := newGame(t, "fog", 100, 30)
	g.Load(fen(t, g.match.Variant(), "7k/8/8/8/8/p7/8/R6K w -"))
	g.play(findMove(t, g.match.Position(), "a1", "a3"))
	if len(g.match.Notation()) != 1 {
		t.Fatal("capture was not recorded")
	}
	if strings.Contains(ansi.Strip(g.View()), "Ra1") || strings.Contains(g.status(), "a3") || g.layout().panel {
		t.Fatal("capture notation exposed in fog")
	}
}

func TestFogKingCaptureHeader(t *testing.T) {
	g := newGame(t, "fog", 100, 30)
	g.Load(fen(t, g.match.Variant(), "4k3/8/8/8/8/8/4R3/4K3 w -"))
	g.play(findMove(t, g.match.Position(), "e2", "e8"))
	if out := ansi.Strip(g.header()); !strings.Contains(out, "king captured") {
		t.Errorf("header = %q", out)
	}
}

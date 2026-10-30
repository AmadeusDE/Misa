package chess

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestBugGameScheduleAndUndo(t *testing.T) {
	g := newGame(t, "bughouse", 80, 24)
	for ply := 0; ply < 4; ply++ {
		if g.computerToMove() != (ply != 0) {
			t.Fatalf("ply %d computer=%v", ply, g.computerToMove())
		}
		m := g.bug.Board(ply % 2)
		if m.Turn() != Color(ply/2) {
			t.Fatalf("ply %d wrong turn", ply)
		}
		g.play(m.Legal()[0])
	}
	if g.computerToMove() || len(g.bug.Board(0).Played()) != 2 || len(g.bug.Board(1).Played()) != 2 {
		t.Fatal("schedule did not return to human")
	}
	g.undo()
	if g.bugPly != 0 || len(g.bug.Board(0).Played()) != 0 || len(g.bug.Board(1).Played()) != 0 {
		t.Fatal("undo did not restore both boards")
	}
}

func TestBugViewResponsive(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 40}} {
		g := newGame(t, "bughouse", size[0], size[1])
		out := ansi.Strip(g.View())
		if strings.Contains(out, "too small") || !strings.Contains(out, "Board A") || !strings.Contains(out, "Board B") {
			t.Fatalf("%v: %s", size, out)
		}
		for _, line := range strings.Split(out, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("%v: overwide %q", size, line)
			}
		}
		if len(strings.Split(out, "\n")) > size[1] {
			t.Fatalf("%v: too tall", size)
		}
		l := g.bugLayout().a
		sq, ok := l.square(g, l.left+l.labelW+1, l.top+2)
		if !ok || sq != g.match.Variant().Square(0, 7) {
			t.Fatalf("%v: bad hit test", size)
		}
	}
}

func TestBugPocketClickAndStaleThought(t *testing.T) {
	g := newGame(t, "bughouse", 80, 24)
	p := fen(t, g.match.Variant(), "4k3/8/8/8/8/8/8/4K3[N] w - - 0 1")
	g.Load(p)
	l := g.bugLayout().a
	x := l.left + len("White:") + 1
	if got := g.bugPocketChoice(l, x, l.top+l.rows+4); got != p.V.TypeOf('N') {
		t.Fatalf("pocket hit = %d", got)
	}
	if !g.selectDrop(p.V.TypeOf('N')) {
		t.Fatal("could not select drop")
	}
	g.activate(p.V.Square(2, 2))
	if g.bugPly != 1 || g.match.Position().Hands[White][p.V.TypeOf('N')] != 0 {
		t.Fatal("drop not played")
	}
	cmd := g.think()
	if cmd == nil {
		t.Fatal("computer did not start")
	}
	stale := g.gen
	g.undo()
	g.thought(thoughtMsg{gen: stale, result: Result{Found: true, Move: g.bug.Board(1).Legal()[0]}})
	if g.bugPly != 0 || len(g.bug.Board(1).Played()) != 0 {
		t.Fatal("stale search played after undo")
	}
}

func TestBugComputerModeDoesNotFlipCoordinates(t *testing.T) {
	g := newGame(t, "bughouse", 80, 24)
	g.SetComputer(true, false)
	if g.flipped {
		t.Fatal("computer mode flipped bughouse")
	}
	l := g.bugLayout().a
	sq, ok := l.square(g, l.left+l.labelW+1, l.top+2)
	if !ok || sq != g.match.Variant().Square(0, 7) {
		t.Fatal("board coordinates do not match display")
	}
	g.moveCursor(0, -1)
	if g.cy != 2 {
		t.Fatalf("up moved cursor to rank %d, want 2", g.cy)
	}
}

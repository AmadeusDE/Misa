package mines

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func press(g *Game, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		case "up":
			msg = tea.KeyPressMsg{Code: tea.KeyUp}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		default:
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		g.Update(msg)
	}
}

func click(g *Game, x, y int, button tea.MouseButton) tea.Cmd {
	return g.Update(tea.MouseClickMsg{X: x, Y: y, Button: button})
}

func sized(level Level, w, h int) *Game {
	g := New(level)
	g.Resize(w, h)
	return g
}

func TestCursorMovesAndClamps(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "h", "h", "h", "h", "h", "h", "k", "up", "up", "up", "up", "up")
	if x, y := g.Cursor(); x != 0 || y != 0 {
		t.Fatalf("cursor = %d,%d, want 0,0", x, y)
	}
	press(g, "l", "j")
	if x, y := g.Cursor(); x != 1 || y != 1 {
		t.Fatalf("cursor = %d,%d, want 1,1", x, y)
	}
}

func TestSpaceRevealsUnderCursor(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "space")
	if !g.Board().Revealed(4, 4) {
		t.Fatal("cell under cursor not revealed")
	}
}

func TestFlagKey(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "f")
	if !g.Board().Flagged(4, 4) {
		t.Fatal("flag not set")
	}
}

func TestLevelKeys(t *testing.T) {
	g := sized(Levels[0], 120, 40)
	press(g, "3")
	if g.Board().Width() != 30 || g.Board().Mines() != 99 {
		t.Fatalf("board %dx%d/%d, want expert", g.Board().Width(), g.Board().Height(), g.Board().Mines())
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "esc"} {
		g := sized(Levels[0], 80, 24)
		press(g, k)
		if !g.IsQuit() {
			t.Fatalf("%s did not quit", k)
		}
	}
}

func TestMouseHitsCell(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	l := g.layout()
	x, y := l.left+1+2*l.cw+1, l.top+2+3
	click(g, x, y, tea.MouseLeft)
	if !g.Board().Revealed(2, 3) {
		t.Fatalf("click at %d,%d did not reveal 2,3", x, y)
	}
	if cx, cy := g.Cursor(); cx != 2 || cy != 3 {
		t.Fatalf("cursor = %d,%d, want 2,3", cx, cy)
	}
}

func TestRightClickFlags(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	l := g.layout()
	click(g, l.left+1, l.top+2, tea.MouseRight)
	if !g.Board().Flagged(0, 0) {
		t.Fatal("right click did not flag")
	}
}

func TestClickOnBorderIgnored(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	l := g.layout()
	click(g, l.left, l.top+2, tea.MouseLeft)
	if g.Board().Started() {
		t.Fatal("click on border started the game")
	}
}

func TestFaceRestarts(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "space")
	old := g.Board()
	click(g, g.layout().faceX(), g.layout().top, tea.MouseLeft)
	if g.Board() == old || g.Board().Started() {
		t.Fatal("face click did not start a new game")
	}
}

func TestHeaderPutsFaceWhereHitTestLooks(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	l := g.layout()
	line := strings.Split(g.View(), "\n")[l.top]
	plain := []rune(ansi.Strip(line))
	face := string(plain[l.faceX() : l.faceX()+faceWidth])
	if face != " :) " {
		t.Fatalf("face at hit-test position = %q", face)
	}
}

func TestBoardRowsAlignWithHitTest(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "f")
	l := g.layout()
	line := []rune(ansi.Strip(strings.Split(g.View(), "\n")[l.top+2+4]))
	_, x, _ := l.hit(l.left+1+4*l.cw+1, l.top+2+4)
	if x != 4 || string(line[l.left+1+4*l.cw+1]) != "⚑" {
		t.Fatalf("flag not drawn at cell 4: %q", string(line))
	}
}

func TestClockRunsFromFirstReveal(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	now := time.Unix(1000, 0)
	g.now = func() time.Time { return now }
	if cmd := g.Update(tea.KeyPressMsg{Code: 'f', Text: "f"}); cmd != nil {
		t.Fatal("flag before first reveal started the clock")
	}
	if cmd := g.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}); cmd == nil && g.Board().State() == Playing {
		t.Fatal("first reveal did not start ticking")
	}
	now = now.Add(5 * time.Second)
	if g.Board().State() == Playing && g.Elapsed() != 5*time.Second {
		t.Fatalf("elapsed = %v, want 5s", g.Elapsed())
	}
}

func TestStaleTickIgnored(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "space")
	gen := g.gen
	press(g, "n")
	if cmd := g.Update(tickMsg{gen}); cmd != nil {
		t.Fatal("tick from the previous game kept ticking")
	}
}

func TestNarrowTerminalUsesTwoColumnCells(t *testing.T) {
	g := sized(Levels[2], 80, 24)
	if l := g.layout(); l.cw != 2 || !l.fits {
		t.Fatalf("expert on 80x24: cw=%d fits=%v", l.cw, l.fits)
	}
}

func TestTooSmall(t *testing.T) {
	g := sized(Levels[2], 40, 10)
	if !strings.Contains(g.View(), "too small") {
		t.Fatal("no too-small message")
	}
	if target, _, _ := g.layout().hit(5, 5); target != hitNone {
		t.Fatal("hit test active while board is hidden")
	}
}

func TestViewFillsHeight(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	lines := strings.Split(g.View(), "\n")
	if len(lines) != 24 {
		t.Fatalf("view has %d lines, want 24", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 80 {
			t.Fatalf("line %d is %d wide", i, w)
		}
	}
}

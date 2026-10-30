package mines

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestCustomCancelPreservesAndPauses(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	now := time.Unix(1000, 0)
	g.now = func() time.Time { return now }
	// Start a clock without relying on a random first move remaining in play.
	g.started = now
	g.board.placed = true
	g.board.ToggleFlag(0, 0)
	board, cx, cy := g.board, g.cx, g.cy
	now = now.Add(5 * time.Second)
	oldGen := g.gen
	press(g, "c", "l", "j", "l", "q", "f", "n", "space")
	click(g, g.layout().faceX(), g.layout().top, tea.MouseLeft)
	now = now.Add(10 * time.Second)
	if g.Elapsed() != 5*time.Second || g.board != board || !board.Flagged(0, 0) || g.IsQuit() || g.cx != cx || g.cy != cy {
		t.Fatal("panel changed game or failed to pause")
	}
	if g.Update(tickMsg{oldGen}) != nil {
		t.Fatal("old tick active")
	}
	cmd := g.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil || g.setup != nil {
		t.Fatal("cancel did not resume ticking")
	}
	now = now.Add(2 * time.Second)
	if g.Elapsed() != 7*time.Second {
		t.Fatalf("elapsed %v", g.Elapsed())
	}
	if g.Update(tickMsg{oldGen}) != nil {
		t.Fatal("stale tick resumed")
	}
}

func TestCustomNewGameAndClamping(t *testing.T) {
	g := sized(Level{"test", 2, 2, 3}, 80, 24)
	old := g.board
	press(g, "c", "h", "j", "h", "j", "l")
	if g.setup.level.W != 2 || g.setup.level.H != 2 || g.setup.level.Mines != 3 {
		t.Fatalf("limits: %+v", g.setup.level)
	}
	press(g, "k", "l", "k", "l", "j", "j", "l")
	if g.setup.level != (Level{"Custom", 3, 3, 4}) {
		t.Fatalf("setup: %+v", g.setup.level)
	}
	g.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if g.setup != nil || g.board == old || g.level != (Level{"Custom", 3, 3, 4}) || g.board.Started() {
		t.Fatal("custom game not started")
	}
	press(g, "c", "c")
	if g.setup != nil {
		t.Fatal("c did not cancel")
	}
}

func TestCustomViewFitAndBounds(t *testing.T) {
	g := sized(Levels[2], 80, 24)
	press(g, "c")
	view := ansi.Strip(g.View())
	if !strings.Contains(view, "Custom game") || !strings.Contains(view, "Board fits") || strings.Contains(view, ":)") {
		t.Fatalf("panel: %s", view)
	}
	for _, line := range strings.Split(g.View(), "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatal("line too wide")
		}
	}
	if len(strings.Split(view, "\n")) != 24 {
		t.Fatal("wrong height")
	}
	g.Resize(60, 20)
	if !strings.Contains(ansi.Strip(g.View()), "won't fit") || g.layout().fits {
		t.Fatal("fit warning disagrees with board layout")
	}
	g.Resize(5, 3)
	lines := strings.Split(g.View(), "\n")
	if len(lines) != 3 {
		t.Fatal("small panel too tall")
	}
	for _, line := range lines {
		if lipgloss.Width(line) > 5 {
			t.Fatal("small panel too wide")
		}
	}
}

func TestCustomShrinkingClampsMines(t *testing.T) {
	g := sized(Level{"Dense", 3, 3, 8}, 80, 24)
	press(g, "c", "h")
	if g.setup.level.Mines != 5 {
		t.Fatalf("mines after shrinking = %d", g.setup.level.Mines)
	}
	press(g, "j", "h")
	if g.setup.level.Mines != 3 {
		t.Fatalf("mines after shrinking height = %d", g.setup.level.Mines)
	}
}

func TestCustomCancelFinishedClock(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	now := time.Unix(1000, 0)
	g.now = func() time.Time { return now }
	g.started = now
	g.ended = now.Add(4 * time.Second)
	now = now.Add(10 * time.Second)
	press(g, "c")
	now = now.Add(time.Minute)
	if g.Elapsed() != 4*time.Second {
		t.Fatal("finished clock changed")
	}
	if cmd := g.Update(tea.KeyPressMsg{Code: 'c', Text: "c"}); cmd != nil {
		t.Fatal("finished game resumed ticking")
	}
	if g.Elapsed() != 4*time.Second {
		t.Fatal("finished clock changed after cancel")
	}
}

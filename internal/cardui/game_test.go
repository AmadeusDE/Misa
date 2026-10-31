package cardui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/AmadeusDE/Misa/internal/cards"
)

type fixture struct {
	n     int
	works bool
}

func (f *fixture) Piles() []Pile {
	var p []Pile
	for i := 0; i < 16; i++ {
		p = append(p, VisiblePile("Tableau", []cards.Card{{Rank: 1}, {Rank: 2}}, 0))
	}
	return p
}
func (f *fixture) Status() string             { return strings.Repeat("status ", 30) }
func (f *fixture) Act(from, row, to int) bool { f.n++; return true }
func (f *fixture) Undo() bool                 { return false }
func (f *fixture) Redo() bool                 { return false }
func (f *fixture) Auto() bool                 { return false }
func (f *fixture) Won() bool                  { return false }
func (f *fixture) Hint() string               { return "hint" }
func (f *fixture) Help() string               { return "help" }
func (f *fixture) Work() func() any {
	if !f.works {
		return nil
	}
	return func() any { return 1 }
}
func (f *fixture) Accept(v any) { f.n++ }
func makeFixture() *Game {
	return New("Test", "test", Options{Seed: 42, Theme: "classic"}, func(uint64) Table { return &fixture{} })
}
func TestLayoutSizes(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {32, 18}, {24, 14}, {18, 12}} {
		g := makeFixture()
		g.Resize(size[0], size[1])
		for focus := 0; focus < 16; focus++ {
			g.focus = focus
			view, targets := g.draw()
			if len(strings.Split(view, "\n")) > size[1] {
				t.Fatal("height", size)
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > size[0] {
					t.Fatalf("width %v: %q", size, line)
				}
			}
			for _, target := range targets {
				if target.x < 0 || target.x+target.w > size[0] || target.y >= size[1] {
					t.Fatal("target bounds", target)
				}
			}
		}
	}
}
func TestSmallPaneAndHover(t *testing.T) {
	g := makeFixture()
	before := g.View()
	g.Update(tea.MouseMotionMsg{})
	if g.View() != before {
		t.Fatal("hover changed view")
	}
	g.Resize(18, 12)
	g.key("enter")
	g.key("a")
	if g.table.(*fixture).n != 0 {
		t.Fatal("invisible action")
	}
	g.key("q")
	if !strings.Contains(g.View(), "y/n") {
		t.Fatal("invisible confirmation")
	}
}
func TestStaleResultAndIsolation(t *testing.T) {
	g := makeFixture()
	other := makeFixture()
	old := g.gen
	g.reset(7)
	g.Update(result{g, old, 1})
	g.Update(result{other, g.gen, 1})
	if g.table.(*fixture).n != 0 {
		t.Fatal("stale/foreign result")
	}
	g.Close()
	g.Update(result{g, g.gen, 1})
	if g.table.(*fixture).n != 0 {
		t.Fatal("closed result")
	}
}
func TestConfirmation(t *testing.T) {
	g := makeFixture()
	g.key("q")
	if g.IsQuit() {
		t.Fatal("no confirmation")
	}
	g.key("n")
	if g.IsQuit() {
		t.Fatal("cancel failed")
	}
	g.key("q")
	g.key("y")
	if !g.IsQuit() {
		t.Fatal("quit")
	}
}

func TestPendingWorkCancelledOnRestart(t *testing.T) {
	g := makeFixture()
	g.table.(*fixture).works = true
	cmd := g.schedule()
	if cmd == nil {
		t.Fatal("not scheduled")
	}
	g.reset(7)
	if msg := cmd(); msg != nil {
		t.Fatal("cancelled command returned result", msg)
	}
}

func TestHelpScrollClampsAndMouseTargets(t *testing.T) {
	g := makeFixture()
	g.Resize(24, 14)
	g.key("?")
	for i := 0; i < 100; i++ {
		g.key("down")
	}
	old := g.helpOffset
	g.key("up")
	if old > 0 && g.helpOffset != old-1 {
		t.Fatal("help scroll")
	}
	view, targets := g.draw()
	if len(strings.Split(view, "\n")) > 14 || len(targets) != 3 {
		t.Fatal("help layout")
	}
	g.key("esc")
	g.key("q")
	_, targets = g.draw()
	yes := targets[0]
	g.Update(tea.MouseClickMsg{X: yes.x, Y: yes.y, Button: tea.MouseLeft})
	if !g.IsQuit() {
		t.Fatal("mouse confirmation")
	}
}

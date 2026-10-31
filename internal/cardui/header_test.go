package cardui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/AmadeusDE/Misa/internal/cards"
	"github.com/charmbracelet/x/ansi"
)

func TestHeaderOverflowIsExplicit(t *testing.T) {
	g := makeFixture()
	g.Resize(24, 14)
	view, targets := g.draw()
	if !strings.Contains(ansi.Strip(view), "? more status in Help") {
		t.Fatal("status was silently clipped", view)
	}
	assertHeaderBounds(t, g, view, targets)
}

func TestNarrowHeaderPreservesFullReplayID(t *testing.T) {
	for _, tc := range []struct {
		game string
		rule int
	}{{"freecell", 0}, {"hearts", int(^uint(0)>>1) - 26}} {
		g := New("Test", tc.game, Options{Seed: ^uint64(0), Rule: tc.rule}, func(uint64) Table { return &fixture{} })
		g.Resize(24, 14)
		view, targets := g.draw()
		var joined string
		for _, line := range strings.Split(ansi.Strip(view), "\n") {
			joined += strings.TrimSpace(line)
		}
		if !strings.Contains(joined, cards.DealID(tc.game, ^uint64(0), tc.rule)) {
			t.Fatal("table lost part of replay ID", view)
		}
		assertHeaderBounds(t, g, view, targets)
		g.key("?")
		view, targets = g.draw()
		assertHeaderBounds(t, g, view, targets)
	}
}

func assertHeaderBounds(t *testing.T, g *Game, view string, targets []target) {
	t.Helper()
	if len(strings.Split(view, "\n")) > g.height {
		t.Fatal("header overflowed height", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > g.width {
			t.Fatal("header overflowed width", line)
		}
	}
	boardTargets := 0
	for _, hit := range targets {
		if hit.x < 0 || hit.x+hit.w > g.width || hit.y >= g.height {
			t.Fatal("target overflow", hit)
		}
		if hit.key == "" {
			boardTargets++
		}
	}
	if !g.help && boardTargets == 0 {
		t.Fatal("header displaced every board target")
	}
}

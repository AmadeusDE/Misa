package hearts

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/AmadeusDE/Misa/internal/cardui"
	"github.com/charmbracelet/x/ansi"
)

func TestStatusPrioritizesPhaseWithLongTarget(t *testing.T) {
	tab := &table{state: NewState(42, int(^uint(0)>>1)-26), selected: map[int]bool{}}
	g := cardui.New("Hearts", "hearts", cardui.Options{Seed: ^uint64(0), Rule: tab.state.Target}, func(uint64) cardui.Table { return tab })
	g.Resize(24, 14)
	view := ansi.Strip(g.View())
	if !strings.HasPrefix(tab.Status(), "Pass 3 left (0 selected)") || !strings.Contains(view, "Pass 3 left (0 selected)") {
		t.Fatal("passing instructions not visible", view)
	}
	if !strings.Contains(view, "? more status in Help") {
		t.Fatal("missing overflow indicator", view)
	}
	if len(strings.Split(view, "\n")) > 14 {
		t.Fatal("height", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 24 {
			t.Fatal("width", line)
		}
	}
}

func TestRoundAndMatchKeepFinalTrickWinner(t *testing.T) {
	for _, over := range []bool{false, true} {
		tab := &table{state: State{RoundOver: true, Over: over, LastWinner: 2, LastRound: [4]int{0, 1, 25, 0}, Scores: [4]int{10, 15, 100, 10}, Target: 100}}
		status := tab.Status()
		if !strings.Contains(status, "Last trick winner: 2") || !strings.Contains(status, "Round points [0 1 25 0]") {
			t.Fatal("lost final trick/round result", status)
		}
		phase := "Round over: select Next"
		if over {
			phase = "Winning seats: [0 3]"
		}
		if !strings.HasPrefix(status, phase) {
			t.Fatal("result not prioritized", status)
		}
		g := cardui.New("Hearts", "hearts", cardui.Options{Seed: 42, Rule: 100}, func(uint64) cardui.Table { return tab })
		g.Resize(24, 14)
		view := ansi.Strip(g.View())
		if !strings.Contains(view, phase) || !strings.Contains(view, "Last trick winner: 2") {
			t.Fatal("result missing from narrow table", view)
		}
	}
}

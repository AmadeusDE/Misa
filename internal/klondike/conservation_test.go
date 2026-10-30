package klondike

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"reflect"
	"testing"
)

func TestFuzzMoveEncodingCoversAllPilePairs(t *testing.T) {
	for from := 1; from <= 12; from++ {
		for to := 2; to <= 12; to++ {
			for row := 0; row < 52; row++ {
				draw, gotFrom, gotRow, gotTo := decodeFuzzAction([]byte{1, byte(from + 1), byte(row + 1), byte(to)})
				if draw || gotFrom != from || gotRow != row || gotTo != to {
					t.Fatalf("unreachable source %d row %d destination %d", from, row, to)
				}
			}
		}
	}
}

func FuzzActions(f *testing.F) {
	f.Add(uint64(42), []byte{0, 0, 0, 0, 1, 2, 1, 9, 1, 10, 1, 2, 1, 2, 24, 10})
	f.Fuzz(func(t *testing.T, seed uint64, actions []byte) {
		tab := &table{state: NewState(seed, 1+2*int(seed%2))}
		actions = actions[:min(len(actions), 2000)]
		for i := 0; i+3 < len(actions); i += 4 {
			draw, from, row, to := decodeFuzzAction(actions[i : i+4])
			before := tab.state.Clone()
			ok := false
			if draw {
				ok = tab.Act(-1, 0, 0)
			} else {
				ok = tab.Act(from, row, to)
			}
			after := tab.state.Clone()
			if !ok && !reflect.DeepEqual(before, after) {
				t.Fatal("rejected mutation")
			}
			if ok {
				if !tab.Undo() || !reflect.DeepEqual(tab.state, before) || !tab.Redo() || !reflect.DeepEqual(tab.state, after) {
					t.Fatal("history")
				}
			}
			seen := map[int]bool{}
			check := func(p []cards.Card) {
				for _, c := range p {
					if seen[c.ID] {
						t.Fatal("duplicate")
					}
					seen[c.ID] = true
				}
			}
			check(tab.state.Stock)
			check(tab.state.Waste)
			for j, p := range tab.state.Columns {
				check(p)
				if h := tab.state.Hidden[j]; h < 0 || h > max(0, len(p)-1) {
					t.Fatal("hidden boundary")
				}
			}
			for _, p := range tab.state.Foundations {
				check(p)
			}
			if len(seen) != 52 {
				t.Fatal("conservation")
			}
		}
	})
}

func decodeFuzzAction(b []byte) (draw bool, from, row, to int) {
	return b[0]%4 == 0, int(b[1])%15 - 1, int(b[2])%54 - 1, int(b[3]) % 14
}

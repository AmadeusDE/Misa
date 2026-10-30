package klondike

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"reflect"
	"testing"
)

func FuzzActions(f *testing.F) {
	f.Add(uint64(42), []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	f.Fuzz(func(t *testing.T, seed uint64, actions []byte) {
		tab := &table{state: NewState(seed, 1+2*int(seed%2))}
		for i, b := range actions[:min(len(actions), 500)] {
			before := tab.state.Clone()
			ok := false
			if b%4 == 0 {
				ok = tab.Act(-1, 0, 0)
			} else {
				ok = tab.Act(int(b)%14-1, i%20, int(b/14)%14)
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

package spider

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"reflect"
	"testing"
)

func FuzzActions(f *testing.F) {
	f.Add(uint64(42), []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	f.Fuzz(func(t *testing.T, seed uint64, actions []byte) {
		tab := &table{state: NewState(seed, []int{1, 2, 4}[seed%3])}
		for i, b := range actions[:min(len(actions), 500)] {
			before := tab.state.Clone()
			ok := false
			if b%4 == 0 {
				ok = tab.Act(-1, 0, 10)
			} else {
				ok = tab.Act(int(b)%12-1, i%20, int(b/12)%12)
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
			check(tab.state.Removed)
			for j, p := range tab.state.Columns {
				check(p)
				if h := tab.state.Hidden[j]; h < 0 || h > max(0, len(p)-1) {
					t.Fatal("hidden boundary")
				}
			}
			if len(seen) != 104 {
				t.Fatal("conservation")
			}
		}
	})
}

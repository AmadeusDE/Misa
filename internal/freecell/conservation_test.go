package freecell

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"reflect"
	"testing"
)

func FuzzActions(f *testing.F) {
	f.Add(uint64(42), []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	f.Fuzz(func(t *testing.T, seed uint64, actions []byte) {
		tab := &table{state: NewState(seed)}
		for i, b := range actions[:min(len(actions), 500)] {
			before := tab.state.Clone()
			ok := tab.Act(int(b)%17-1, i%14, int(b/17)%17)
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
			for _, p := range tab.state.Columns {
				check(p)
			}
			for j, p := range tab.state.Cells {
				check(p)
				if len(p) > 1 {
					t.Fatal("cell overflow")
				}
				check(tab.state.Foundations[j])
			}
			if len(seen) != 52 {
				t.Fatal("conservation")
			}
		}
	})
}

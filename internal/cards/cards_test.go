package cards

import (
	"reflect"
	"testing"
)

func TestDeckAndShuffle(t *testing.T) {
	for _, suits := range []int{1, 2, 4} {
		d := SpiderDeck(suits)
		seen := map[int]bool{}
		counts := map[[2]int]int{}
		for _, c := range d {
			if seen[c.ID] {
				t.Fatal("duplicate physical ID")
			}
			seen[c.ID] = true
			counts[[2]int{c.Suit, c.Rank}]++
		}
		if len(d) != 104 {
			t.Fatal(len(d))
		}
		if len(counts) != suits*13 {
			t.Fatal("suit/rank coverage", suits)
		}
		for _, n := range counts {
			if n != 8/suits {
				t.Fatal("copy count", suits, n)
			}
		}
	}
	a, b := Deck(), Deck()
	Shuffle(a, 42)
	Shuffle(b, 42)
	if !reflect.DeepEqual(a, b) || reflect.DeepEqual(a, Deck()) {
		t.Fatal("shuffle replay")
	}
}

func TestHistory(t *testing.T) {
	h := History[int]{}
	h.Push(1)
	h.Push(2)
	if v, ok := h.Undo(3); !ok || v != 2 {
		t.Fatal(v, ok)
	}
	if v, ok := h.Redo(2); !ok || v != 3 {
		t.Fatal(v, ok)
	}
	h.Undo(3)
	h.Push(4)
	if _, ok := h.Redo(4); ok {
		t.Fatal("redo survived branch")
	}
}

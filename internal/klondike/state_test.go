package klondike

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"reflect"
	"testing"
)

func TestDealAndStock(t *testing.T) {
	s := NewState(42, 3)
	for i, p := range s.Columns {
		if len(p) != i+1 || s.Hidden[i] != i {
			t.Fatal("deal")
		}
	}
	original := append([]cards.Card(nil), s.Stock...)
	for len(s.Stock) > 0 {
		if !s.DrawStock() {
			t.Fatal("draw")
		}
	}
	if len(s.Waste) != 24 || !s.DrawStock() || !reflect.DeepEqual(s.Stock, original) {
		t.Fatal("recycle order")
	}
}

func TestIllegalAtomicAndFoundation(t *testing.T) {
	s := NewState(1, 1)
	before := s.Clone()
	if s.Move(2, 0, 3) || !reflect.DeepEqual(s, before) {
		t.Fatal("illegal mutation")
	}
	s = State{Draw: 1, Waste: []cards.Card{{Rank: 1, Suit: 0}}}
	if !s.Move(1, 0, 9) || s.Score != 10 {
		t.Fatal("foundation")
	}
	s.Columns[0] = []cards.Card{{Rank: 2, Suit: 1}}
	if !s.Move(9, 0, 2) || s.Score != 0 {
		t.Fatal("foundation return")
	}
}

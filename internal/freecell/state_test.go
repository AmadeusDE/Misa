package freecell

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"testing"
)

func TestDeal(t *testing.T) {
	s := NewState(42)
	for i, p := range s.Columns {
		n := 6
		if i < 4 {
			n = 7
		}
		if len(p) != n {
			t.Fatal("deal")
		}
	}
}

func TestSequence(t *testing.T) {
	s := State{}
	s.Columns[0] = []cards.Card{{Rank: 7, Suit: 0}, {Rank: 6, Suit: 1}, {Rank: 5, Suit: 0}}
	s.Columns[1] = []cards.Card{{Rank: 8, Suit: 1}}
	if !s.Move(0, 0, 1) || len(s.Columns[1]) != 4 {
		t.Fatal("decomposition")
	}
	for i := range s.Cells {
		s.Cells[i] = []cards.Card{{Rank: 13}}
	}
	for i := 2; i < 8; i++ {
		s.Columns[i] = []cards.Card{{Rank: 13}}
	}
	if s.Move(1, 1, 0) {
		t.Fatal("capacity accepted")
	}
}

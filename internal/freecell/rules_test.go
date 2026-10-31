package freecell

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"reflect"
	"testing"
)

func TestCapacityAndAtomicity(t *testing.T) {
	for cells := 0; cells <= 4; cells++ {
		for empty := 0; empty <= 3; empty++ {
			for n := 1; n <= 12; n++ {
				s := State{}
				for i := cells; i < 4; i++ {
					s.Cells[i] = []cards.Card{{Rank: 13}}
				}
				for i := 2 + empty; i < 8; i++ {
					s.Columns[i] = []cards.Card{{Rank: 13}}
				}
				for r := n; r >= 1; r-- {
					s.Columns[0] = append(s.Columns[0], cards.Card{Rank: r, Suit: r % 2})
				}
				s.Columns[1] = []cards.Card{{Rank: n + 1, Suit: (n + 1) % 2}}
				before := s.Clone()
				got := s.Move(0, 0, 1)
				want := n <= (cells+1)<<empty
				if got != want {
					t.Fatalf("cells=%d empty=%d n=%d got %v", cells, empty, n, got)
				}
				if !got && !reflect.DeepEqual(s, before) {
					t.Fatal("mutation")
				}
			}
		}
	}
}
func TestEmptyDestinationExcluded(t *testing.T) {
	s := State{}
	for i := range s.Cells {
		s.Cells[i] = []cards.Card{{Rank: 13}}
	}
	for i := 2; i < 8; i++ {
		s.Columns[i] = []cards.Card{{Rank: 13}}
	}
	s.Columns[0] = []cards.Card{{Rank: 2, Suit: 0}, {Rank: 1, Suit: 1}}
	if s.Move(0, 0, 1) {
		t.Fatal("counted empty destination")
	}
}
func TestOneWayAndHistory(t *testing.T) {
	tab := &table{}
	tab.state.Cells[0] = []cards.Card{{Rank: 1, Suit: 0}}
	before := tab.state.Clone()
	if !tab.Act(8, 0, 12) || tab.Act(12, 0, 0) {
		t.Fatal("foundation rules")
	}
	after := tab.state.Clone()
	if !tab.Undo() || !reflect.DeepEqual(tab.state, before) || !tab.Redo() || !reflect.DeepEqual(tab.state, after) {
		t.Fatal("history")
	}
}

func TestWinningFixture(t *testing.T) {
	tab := &table{}
	for suit := 0; suit < 4; suit++ {
		for r := 1; r < 13; r++ {
			tab.state.Foundations[suit] = append(tab.state.Foundations[suit], cards.Card{ID: suit*13 + r - 1, Rank: r, Suit: suit})
		}
		tab.state.Cells[suit] = []cards.Card{{ID: suit*13 + 12, Rank: 13, Suit: suit}}
	}
	for i := 0; i < 4; i++ {
		if !tab.Auto() {
			t.Fatal("safe king")
		}
	}
	if !tab.Won() {
		t.Fatal("win")
	}
	if !tab.Undo() || tab.Won() {
		t.Fatal("undo win")
	}
}

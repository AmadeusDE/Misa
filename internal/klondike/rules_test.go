package klondike

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"reflect"
	"testing"
)

func TestMoveBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name        string
		run         []cards.Card
		dest        []cards.Card
		hidden, row int
		want        bool
	}{
		{"alternating", []cards.Card{{Rank: 7, Suit: 0}, {Rank: 6, Suit: 1}}, []cards.Card{{Rank: 8, Suit: 1}}, 0, 0, true},
		{"same color", []cards.Card{{Rank: 7, Suit: 0}}, []cards.Card{{Rank: 8, Suit: 2}}, 0, 0, false},
		{"bad run", []cards.Card{{Rank: 7, Suit: 0}, {Rank: 5, Suit: 1}}, []cards.Card{{Rank: 8, Suit: 1}}, 0, 0, false},
		{"empty king", []cards.Card{{Rank: 13, Suit: 0}, {Rank: 12, Suit: 1}}, nil, 0, 0, true},
		{"empty nonking", []cards.Card{{Rank: 12, Suit: 0}}, nil, 0, 0, false},
		{"hidden", []cards.Card{{Rank: 7, Suit: 0}}, []cards.Card{{Rank: 8, Suit: 1}}, 1, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := State{Draw: 1}
			s.Columns[0] = cards.Copy(tc.run)
			s.Columns[1] = cards.Copy(tc.dest)
			s.Hidden[0] = tc.hidden
			before := s.Clone()
			got := s.Move(2, tc.row, 3)
			if got != tc.want {
				t.Fatal(got)
			}
			if !got && !reflect.DeepEqual(s, before) {
				t.Fatal("rejection mutated")
			}
		})
	}
}

func TestFlipUndoRedoAndWin(t *testing.T) {
	tab := &table{state: State{Draw: 1}}
	tab.state.Columns[0] = []cards.Card{{ID: 0, Rank: 4, Suit: 0}, {ID: 1, Rank: 1, Suit: 0}}
	tab.state.Hidden[0] = 1
	before := tab.state.Clone()
	if !tab.Act(2, 1, 9) || tab.state.Score != 15 || tab.state.Hidden[0] != 0 {
		t.Fatal(tab.state)
	}
	after := tab.state.Clone()
	if !tab.Undo() || !reflect.DeepEqual(tab.state, before) || !tab.Redo() || !reflect.DeepEqual(tab.state, after) {
		t.Fatal("history")
	}
	s := State{Draw: 1}
	for suit := 0; suit < 4; suit++ {
		for r := 1; r <= 12; r++ {
			s.Foundations[suit] = append(s.Foundations[suit], cards.Card{Rank: r, Suit: suit})
		}
		s.Columns[suit] = []cards.Card{{Rank: 13, Suit: suit}}
	}
	tab = &table{state: s}
	if !tab.Auto() || !tab.Won() || !tab.Undo() || tab.Won() {
		t.Fatal("verified finish")
	}
}

func TestHiddenHintInvariant(t *testing.T) {
	a := &table{state: NewState(42, 1)}
	b := &table{state: a.state.Clone()}
	for i := range b.state.Columns {
		for j := 0; j < b.state.Hidden[i]; j++ {
			b.state.Columns[i][j] = cards.Card{Rank: 13, Suit: 3, ID: 999}
		}
	}
	for i := range b.state.Stock {
		b.state.Stock[i] = cards.Card{Rank: 1, Suit: 0, ID: 999}
	}
	if !reflect.DeepEqual(a.Piles(), b.Piles()) || a.Hint() != b.Hint() {
		t.Fatal("hidden information leak")
	}
}

func TestDrawOrderingAndShortDraw(t *testing.T) {
	s := State{Draw: 3, Stock: []cards.Card{{ID: 0}, {ID: 1}, {ID: 2}, {ID: 3}}}
	s.DrawStock()
	if cards.Top(s.Waste).ID != 1 || len(s.Stock) != 1 {
		t.Fatal(s)
	}
	s.DrawStock()
	if cards.Top(s.Waste).ID != 0 {
		t.Fatal(s)
	}
	s.DrawStock()
	s.DrawStock()
	if cards.Top(s.Waste).ID != 1 {
		t.Fatal("recycle")
	}
}

func TestOnlyTopWasteAndSuitFoundation(t *testing.T) {
	s := State{Draw: 1, Waste: []cards.Card{{Rank: 1, Suit: 0}, {Rank: 1, Suit: 1}}}
	before := s.Clone()
	if s.Move(1, 0, 9) || s.Move(1, 1, 9) || !reflect.DeepEqual(before, s) {
		t.Fatal("waste/suit legality")
	}
	if !s.Move(1, 1, 10) || !s.Move(1, 0, 9) || s.Score != 20 {
		t.Fatal("waste top")
	}
	s.Waste = []cards.Card{{Rank: 3, Suit: 0}}
	before = s.Clone()
	if s.Move(1, 0, 9) || !reflect.DeepEqual(s, before) {
		t.Fatal("foundation gap")
	}
}

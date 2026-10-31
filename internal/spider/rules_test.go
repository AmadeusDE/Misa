package spider

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"reflect"
	"testing"
)

func TestStockHistoryAndEmptyRejection(t *testing.T) {
	tab := &table{state: NewState(42, 4)}
	before := tab.state.Clone()
	if !tab.Act(-1, 0, 10) {
		t.Fatal("deal")
	}
	after := tab.state.Clone()
	if !tab.Undo() || !reflect.DeepEqual(tab.state, before) || !tab.Redo() || !reflect.DeepEqual(tab.state, after) {
		t.Fatal("history")
	}
	tab.state.Columns[5] = nil
	before = tab.state.Clone()
	if tab.Act(-1, 0, 10) || !reflect.DeepEqual(tab.state, before) {
		t.Fatal("empty stock")
	}
}
func TestMultiCompletionAndFlip(t *testing.T) {
	tab := &table{state: State{Score: 500}}
	for r := 13; r >= 1; r-- {
		tab.state.Columns[0] = append(tab.state.Columns[0], cards.Card{Rank: r, Suit: 0})
	}
	tab.state.Hidden[0] = 13
	for r := 13; r >= 2; r-- {
		tab.state.Columns[0] = append(tab.state.Columns[0], cards.Card{Rank: r, Suit: 0})
	}
	// A completion reveals only one card, not a hidden whole run. The second run is exposed.
	tab.state.Hidden[0] = 0
	tab.state.Columns[1] = []cards.Card{{Rank: 1, Suit: 0}}
	before := tab.state.Clone()
	if !tab.Act(1, 0, 0) || tab.state.Completed != 2 || tab.state.Score != 699 {
		t.Fatal(tab.state)
	}
	after := tab.state.Clone()
	if !tab.Undo() || !reflect.DeepEqual(tab.state, before) || !tab.Redo() || !reflect.DeepEqual(tab.state, after) {
		t.Fatal("multi undo")
	}
}
func TestMixedSuitRunRejected(t *testing.T) {
	s := State{}
	s.Columns[0] = []cards.Card{{Rank: 3, Suit: 0}, {Rank: 2, Suit: 1}}
	s.Columns[1] = []cards.Card{{Rank: 4, Suit: 2}}
	before := s.Clone()
	if s.Move(0, 0, 1) || !reflect.DeepEqual(s, before) {
		t.Fatal("mixed run")
	}
	if !s.Move(0, 1, 2) {
		t.Fatal("single into empty")
	}
}
func TestHiddenHintInvariant(t *testing.T) {
	a := &table{state: NewState(9, 4)}
	b := &table{state: a.state.Clone()}
	for i := range b.state.Columns {
		for j := 0; j < b.state.Hidden[i]; j++ {
			b.state.Columns[i][j] = cards.Card{ID: 999, Rank: 1, Suit: 1}
		}
	}
	for i := range b.state.Stock {
		b.state.Stock[i] = cards.Card{ID: 999}
	}
	if !reflect.DeepEqual(a.Piles(), b.Piles()) || a.Hint() != b.Hint() {
		t.Fatal("hidden leak")
	}
}

func TestWinAllSuitModes(t *testing.T) {
	for _, suits := range []int{1, 2, 4} {
		s := State{Score: 0, Completed: 7}
		d := cards.SpiderDeck(suits)
		s.Removed = cards.Copy(d[13:])
		for rank := 13; rank >= 2; rank-- {
			s.Columns[0] = append(s.Columns[0], d[rank-1])
		}
		s.Columns[1] = []cards.Card{d[0]}
		if !s.Move(1, 0, 0) || !s.Won() || s.Score != 99 {
			t.Fatal(suits, s)
		}
	}
	s := NewState(1, 1)
	s.Score = 0
	if !s.DealRow() || s.Score != -1 {
		t.Fatal("zero clamp")
	}
}

func TestStockRowMultipleCompletionsAndExposure(t *testing.T) {
	d := cards.SpiderDeck(1)
	tab := &table{state: State{Score: 500}}
	remaining := cards.Copy(d[91:])
	tab.state.Columns[0] = cards.Copy(remaining[:7])
	tab.state.Hidden[0] = 7
	for copy := 0; copy < 7; copy++ {
		for rank := 13; rank >= 2; rank-- {
			tab.state.Columns[copy] = append(tab.state.Columns[copy], d[copy*13+rank-1])
		}
	}
	for i := 7; i < 10; i++ {
		tab.state.Columns[i] = []cards.Card{remaining[i]}
	}
	tab.state.Stock = cards.Copy(remaining[10:])
	for copy := 0; copy < 7; copy++ {
		tab.state.Stock = append(tab.state.Stock, d[copy*13])
	}
	before := tab.state.Clone()
	if !tab.Act(-1, 0, 10) || tab.state.Completed != 7 || tab.state.Score != 1199 || tab.state.Hidden[0] != 6 || len(tab.state.Columns[0]) != 7 {
		t.Fatal(tab.state)
	}
	after := tab.state.Clone()
	if !tab.Undo() || !reflect.DeepEqual(tab.state, before) || !tab.Redo() || !reflect.DeepEqual(tab.state, after) {
		t.Fatal("stock multi-completion history")
	}
}

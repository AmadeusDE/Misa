package spider

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"testing"
)

func TestDeal(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		s := NewState(42, n)
		if len(s.Stock) != 50 || s.Score != 500 {
			t.Fatal("stock")
		}
		for i, p := range s.Columns {
			want := 5
			if i < 4 {
				want = 6
			}
			if len(p) != want || s.Hidden[i] != want-1 {
				t.Fatal("layout")
			}
		}
		if !s.DealRow() || s.Score != 499 {
			t.Fatal("row")
		}
	}
}

func TestCompletion(t *testing.T) {
	s := State{Score: 500}
	for r := 13; r >= 2; r-- {
		s.Columns[0] = append(s.Columns[0], cards.Card{Rank: r})
	}
	s.Columns[1] = []cards.Card{{Rank: 1}}
	if !s.Move(1, 0, 0) || s.Completed != 1 || s.Score != 599 || len(s.Columns[0]) != 0 {
		t.Fatal(s)
	}
}

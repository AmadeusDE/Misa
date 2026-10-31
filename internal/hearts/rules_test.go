package hearts

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"reflect"
	"testing"
)

func TestTrickLegality(t *testing.T) {
	for _, tc := range []struct {
		name          string
		hand          []cards.Card
		trick         []Play
		first, broken bool
		want          []int
	}{
		{"2C required", []cards.Card{{Rank: 1, Suit: 0}, {Rank: 2, Suit: 0}}, nil, true, false, []int{1}},
		{"follow suit", []cards.Card{{Rank: 1, Suit: 1}, {Rank: 12, Suit: 2}}, []Play{{1, cards.Card{Rank: 3, Suit: 1}}}, false, true, []int{0}},
		{"first discard", []cards.Card{{Rank: 3, Suit: 1}, {Rank: 12, Suit: 2}, {Rank: 4, Suit: 3}}, []Play{{1, cards.Card{Rank: 2, Suit: 0}}}, true, false, []int{0}},
		{"only penalties", []cards.Card{{Rank: 12, Suit: 2}, {Rank: 4, Suit: 3}}, []Play{{1, cards.Card{Rank: 2, Suit: 0}}}, true, false, []int{0, 1}},
		{"hearts lead blocked", []cards.Card{{Rank: 5, Suit: 3}, {Rank: 6, Suit: 0}}, nil, false, false, []int{1}},
		{"only hearts lead", []cards.Card{{Rank: 5, Suit: 3}}, nil, false, false, []int{0}},
		{"broken lead", []cards.Card{{Rank: 5, Suit: 3}, {Rank: 6, Suit: 0}}, nil, false, true, []int{0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := legal(tc.hand, tc.trick, tc.first, tc.broken); !reflect.DeepEqual(got, tc.want) {
				t.Fatal(got, tc.want)
			}
		})
	}
}
func TestPassingAtomicAndDirections(t *testing.T) {
	for round := 0; round < 3; round++ {
		s := NewState(42, 100)
		s.Round = round
		s.deal()
		before := s
		var choices [4][]int
		for p := range choices {
			choices[p] = []int{0, 1, 2}
		}
		choices[3] = []int{0, 0, 1}
		if s.Pass(choices) || !reflect.DeepEqual(s, before) {
			t.Fatal("invalid pass mutated")
		}
		choices[3] = []int{0, 1, 2}
		if !s.Pass(choices) {
			t.Fatal("pass")
		}
		for p := 0; p < 4; p++ {
			to := (p + []int{1, 3, 2}[round]) % 4
			if !reflect.DeepEqual(s.Received[to], before.Hands[p][:3]) {
				t.Fatal("direction")
			}
		}
		seen := map[int]bool{}
		for _, h := range s.Hands {
			if len(h) != 13 {
				t.Fatal("count")
			}
			for _, c := range h {
				if seen[c.ID] {
					t.Fatal("duplicate")
				}
				seen[c.ID] = true
			}
		}
	}
	s := NewState(0, 100)
	s.Round = 3
	s.deal()
	if s.Passing {
		t.Fatal("hold")
	}
}
func TestQueenDoesNotBreakAndAceWins(t *testing.T) {
	s := State{Turn: 0, Tricks: 1, Target: 100}
	s.Hands[0] = []cards.Card{{Rank: 1, Suit: 0}}
	s.Hands[1] = []cards.Card{{Rank: 12, Suit: 2}}
	s.Hands[2] = []cards.Card{{Rank: 13, Suit: 0}}
	s.Hands[3] = []cards.Card{{Rank: 2, Suit: 0}}
	for p := 0; p < 4; p++ {
		if !s.Play(p, 0) {
			t.Fatal(p)
		}
	}
	if s.Broken || s.Points[0] != 13 || s.Turn != 0 || !s.Void[1][0] {
		t.Fatal(s)
	}
}
func TestMoonAndJointWinner(t *testing.T) {
	s := State{Turn: 0, Tricks: 12, Target: 26, Points: [4]int{25, 0, 0, 0}}
	for p := 0; p < 4; p++ {
		s.Hands[p] = []cards.Card{{Rank: 13 - p, Suit: 0}}
	}
	s.Hands[3][0] = cards.Card{Rank: 2, Suit: 3}
	for p := 0; p < 4; p++ {
		if !s.Play(p, 0) {
			t.Fatal(p)
		}
	}
	if s.LastRound != [4]int{0, 26, 26, 26} || !s.Over || !reflect.DeepEqual(s.Winners(), []int{0}) {
		t.Fatal(s)
	}
	s.Scores = [4]int{20, 20, 100, 30}
	if !reflect.DeepEqual(s.Winners(), []int{0, 1}) {
		t.Fatal("joint winners")
	}
}
func TestObservationDoesNotExposeHiddenCards(t *testing.T) {
	a := NewState(42, 100)
	b := NewState(99, 100)
	b.Hands[0] = cards.Copy(a.Hands[0])
	b.Turn = a.Turn
	oa, ob := a.Observe(0), b.Observe(0)
	if !reflect.DeepEqual(oa, ob) || !reflect.DeepEqual(ChoosePass(oa), ChoosePass(ob)) || ChoosePlay(oa) != ChoosePlay(ob) {
		t.Fatal("hidden information affected decision")
	}
	oa.Hand[0].Rank = 99
	if a.Hands[0][0].Rank == 99 {
		t.Fatal("aliased hand")
	}
}
func TestCompleteHandTotalsAndConservation(t *testing.T) {
	s := NewState(11, 100)
	var pass [4][]int
	for p := range pass {
		pass[p] = ChoosePass(s.Observe(p))
	}
	s.Pass(pass)
	for !s.RoundOver {
		p := s.Turn
		if !s.Play(p, ChoosePlay(s.Observe(p))) {
			t.Fatal("play")
		}
		count := len(s.Public)
		seen := map[int]bool{}
		for _, v := range s.Public {
			seen[v.Card.ID] = true
		}
		for _, h := range s.Hands {
			count += len(h)
			for _, c := range h {
				if seen[c.ID] {
					t.Fatal("duplicate")
				}
				seen[c.ID] = true
			}
		}
		if count != 52 || len(seen) != 52 {
			t.Fatal("conservation")
		}
	}
	total := 0
	for _, n := range s.Points {
		total += n
	}
	if total != 26 {
		t.Fatal(total)
	}
}

func TestPublicMoonDefense(t *testing.T) {
	o := Observation{Seat: 0, Points: [4]int{0, 13, 0, 0}, Legal: []int{0, 1}, Trick: []Play{{Seat: 1, Card: cards.Card{Rank: 13, Suit: 0}}, {Seat: 2, Card: cards.Card{Rank: 3, Suit: 3}}}}
	o.Hand = []cards.Card{{Rank: 2, Suit: 0}, {Rank: 1, Suit: 0}}
	if ChoosePlay(o) != 1 {
		t.Fatal("did not take a heart from a public moon threat")
	}
	o.Hand = []cards.Card{{Rank: 12, Suit: 3}, {Rank: 13, Suit: 1}}
	if ChoosePlay(o) != 1 {
		t.Fatal("fed a public moon threat despite a non-penalty option")
	}
}

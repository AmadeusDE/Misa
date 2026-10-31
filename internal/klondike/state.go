// Package klondike implements draw-one/draw-three Klondike without hidden-card hints.
package klondike

import "github.com/AmadeusDE/Misa/internal/cards"

type State struct {
	Columns            [7][]cards.Card
	Hidden             [7]int
	Stock, Waste       []cards.Card
	Foundations        [4][]cards.Card
	Draw, Score, Moves int
}

func NewState(seed uint64, draw int) State {
	s := State{Draw: draw}
	d := cards.Deck()
	cards.Shuffle(d, seed)
	for i := range s.Columns {
		s.Columns[i] = cards.Copy(d[:i+1])
		d = d[i+1:]
		s.Hidden[i] = i
	}
	s.Stock = cards.Copy(d)
	return s
}
func (s State) Clone() State {
	for i := range s.Columns {
		s.Columns[i] = cards.Copy(s.Columns[i])
	}
	for i := range s.Foundations {
		s.Foundations[i] = cards.Copy(s.Foundations[i])
	}
	s.Stock = cards.Copy(s.Stock)
	s.Waste = cards.Copy(s.Waste)
	return s
}
func (s *State) DrawStock() bool {
	if len(s.Stock) == 0 {
		if len(s.Waste) == 0 {
			return false
		}
		for i := len(s.Waste) - 1; i >= 0; i-- {
			s.Stock = append(s.Stock, s.Waste[i])
		}
		s.Waste = nil
	} else {
		for n := 0; n < s.Draw && len(s.Stock) > 0; n++ {
			s.Waste = append(s.Waste, cards.Top(s.Stock))
			s.Stock = s.Stock[:len(s.Stock)-1]
		}
	}
	s.Moves++
	return true
}

// Piles: stock 0, waste 1, tableau 2..8, foundations 9..12.
func (s *State) pile(i int) *[]cards.Card {
	switch {
	case i == 1:
		return &s.Waste
	case i >= 2 && i <= 8:
		return &s.Columns[i-2]
	case i >= 9 && i <= 12:
		return &s.Foundations[i-9]
	}
	return nil
}
func (s *State) Move(from, row, to int) bool {
	a, b := s.pile(from), s.pile(to)
	if a == nil || b == nil || from == to || row < 0 || row >= len(*a) {
		return false
	}
	run := (*a)[row:]
	c := run[0]
	if from >= 2 && from <= 8 {
		if row < s.Hidden[from-2] || !cards.Descending(run, false) {
			return false
		}
	} else if len(run) != 1 {
		return false
	}
	if to >= 9 {
		if len(run) != 1 || c.Suit != to-9 || !cards.Foundation(c, *b) {
			return false
		}
	} else if to >= 2 {
		top := cards.Top(*b)
		if top.Rank == 0 {
			if c.Rank != 13 {
				return false
			}
		} else if top.Rank != c.Rank+1 || top.Red() == c.Red() {
			return false
		}
	} else {
		return false
	}
	*b = append(*b, run...)
	*a = (*a)[:row]
	if from >= 9 {
		s.Score -= 10
	}
	if to >= 9 {
		s.Score += 10
	}
	if from >= 2 && from <= 8 && s.Hidden[from-2] > 0 && len(*a) == s.Hidden[from-2] {
		s.Hidden[from-2]--
		s.Score += 5
	}
	s.Moves++
	return true
}
func (s State) Won() bool {
	n := 0
	for _, p := range s.Foundations {
		n += len(p)
	}
	return n == 52
}

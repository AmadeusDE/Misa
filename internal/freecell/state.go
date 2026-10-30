// Package freecell implements FreeCell with verified single-card sequence decompositions.
package freecell

import "github.com/AmadeusDE/Misa/internal/cards"

type State struct {
	Columns     [8][]cards.Card
	Cells       [4][]cards.Card
	Foundations [4][]cards.Card
	Moves       int
}

func NewState(seed uint64) State {
	s := State{}
	d := cards.Deck()
	cards.Shuffle(d, seed)
	for i, c := range d {
		s.Columns[i%8] = append(s.Columns[i%8], c)
	}
	return s
}
func (s State) Clone() State {
	for i := range s.Columns {
		s.Columns[i] = cards.Copy(s.Columns[i])
	}
	for i := range s.Cells {
		s.Cells[i] = cards.Copy(s.Cells[i])
		s.Foundations[i] = cards.Copy(s.Foundations[i])
	}
	return s
}
func (s *State) pile(i int) *[]cards.Card {
	switch {
	case i >= 0 && i < 8:
		return &s.Columns[i]
	case i >= 8 && i < 12:
		return &s.Cells[i-8]
	case i >= 12 && i < 16:
		return &s.Foundations[i-12]
	}
	return nil
}
func (s *State) single(from, to int) bool {
	a, b := s.pile(from), s.pile(to)
	if a == nil || b == nil || from == to || from >= 12 || len(*a) == 0 {
		return false
	}
	c := cards.Top(*a)
	if to >= 12 {
		if c.Suit != to-12 || !cards.Foundation(c, *b) {
			return false
		}
	} else if to >= 8 {
		if len(*b) != 0 {
			return false
		}
	} else if top := cards.Top(*b); top.Rank != 0 && (top.Rank != c.Rank+1 || top.Red() == c.Red()) {
		return false
	}
	*b = append(*b, c)
	*a = (*a)[:len(*a)-1]
	return true
}
func (s *State) Move(from, row, to int) bool {
	a := s.pile(from)
	if a == nil || from >= 12 || from == to || row < 0 || row >= len(*a) {
		return false
	}
	n := len(*a) - row
	candidate := s.Clone()
	if n == 1 {
		if !candidate.single(from, to) {
			return false
		}
	} else {
		if from >= 8 || to < 0 || to >= 8 || !cards.Descending((*a)[row:], false) {
			return false
		}
		var cells, cols []int
		for i, p := range s.Cells {
			if len(p) == 0 {
				cells = append(cells, i+8)
			}
		}
		for i, p := range s.Columns {
			if i != from && i != to && len(p) == 0 {
				cols = append(cols, i)
			}
		}
		if n > (len(cells)+1)<<len(cols) || !candidate.transfer(from, to, n, cells, cols) {
			return false
		}
	}
	candidate.Moves = s.Moves + 1
	*s = candidate
	return true
}
func (s *State) transfer(from, to, n int, cells, cols []int) bool {
	if n <= len(cells)+1 {
		for i := 0; i < n-1; i++ {
			if !s.single(from, cells[i]) {
				return false
			}
		}
		if !s.single(from, to) {
			return false
		}
		for i := n - 2; i >= 0; i-- {
			if !s.single(cells[i], to) {
				return false
			}
		}
		return true
	}
	if len(cols) == 0 {
		return false
	}
	temp := cols[len(cols)-1]
	rest := cols[:len(cols)-1]
	k := min(n-1, (len(cells)+1)<<len(rest))
	return s.transfer(from, temp, k, cells, rest) && s.transfer(from, to, n-k, cells, rest) && s.transfer(temp, to, k, cells, rest)
}
func (s State) Won() bool {
	n := 0
	for _, p := range s.Foundations {
		n += len(p)
	}
	return n == 52
}

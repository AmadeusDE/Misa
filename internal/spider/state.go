// Package spider implements Spider Solitaire with unique physical cards in all suit modes.
package spider

import "github.com/AmadeusDE/Misa/internal/cards"

type State struct {
	Columns                 [10][]cards.Card
	Hidden                  [10]int
	Stock, Removed          []cards.Card
	Score, Moves, Completed int
}

func NewState(seed uint64, suits int) State {
	s := State{Score: 500}
	d := cards.SpiderDeck(suits)
	cards.Shuffle(d, seed)
	for i := range s.Columns {
		n := 5
		if i < 4 {
			n = 6
		}
		s.Columns[i] = cards.Copy(d[:n])
		s.Hidden[i] = n - 1
		d = d[n:]
	}
	s.Stock = cards.Copy(d)
	return s
}
func (s State) Clone() State {
	for i := range s.Columns {
		s.Columns[i] = cards.Copy(s.Columns[i])
	}
	s.Stock = cards.Copy(s.Stock)
	s.Removed = cards.Copy(s.Removed)
	return s
}
func (s *State) settle() {
	for i := range s.Columns {
		for {
			p := s.Columns[i]
			if len(p) > 0 && len(p) == s.Hidden[i] {
				s.Hidden[i]--
			}
			n := len(p)
			if n < 13 || n-13 < s.Hidden[i] || p[n-13].Rank != 13 || !cards.Descending(p[n-13:], true) {
				break
			}
			s.Removed = append(s.Removed, p[n-13:]...)
			s.Columns[i] = p[:n-13]
			s.Completed++
			s.Score += 100
		}
	}
}
func (s *State) Move(from, row, to int) bool {
	if from < 0 || from >= 10 || to < 0 || to >= 10 || from == to || row < s.Hidden[from] || row >= len(s.Columns[from]) {
		return false
	}
	run := s.Columns[from][row:]
	top := cards.Top(s.Columns[to])
	if !cards.Descending(run, true) || (top.Rank != 0 && top.Rank != run[0].Rank+1) {
		return false
	}
	s.Columns[to] = append(s.Columns[to], run...)
	s.Columns[from] = s.Columns[from][:row]
	s.Score--
	s.Moves++
	s.settle()
	return true
}
func (s *State) DealRow() bool {
	if len(s.Stock) < 10 {
		return false
	}
	for _, p := range s.Columns {
		if len(p) == 0 {
			return false
		}
	}
	for i := range s.Columns {
		s.Columns[i] = append(s.Columns[i], cards.Top(s.Stock))
		s.Stock = s.Stock[:len(s.Stock)-1]
	}
	s.Score--
	s.Moves++
	s.settle()
	return true
}
func (s State) Won() bool { return s.Completed == 8 }

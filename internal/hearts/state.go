// Package hearts implements four-seat Hearts and public-information computer decisions.
package hearts

import (
	"github.com/AmadeusDE/Misa/internal/cards"
	"sort"
)

type Play struct {
	Seat int
	Card cards.Card
}
type State struct {
	Hands                                           [4][]cards.Card
	Sent, Received                                  [4][]cards.Card
	Trick, Public                                   []Play
	Void                                            [4][4]bool
	Scores, Points, LastRound                       [4]int
	Round, Tricks, Turn, Leader, LastWinner, Target int
	Broken, Passing, RoundOver, Over                bool
	seed                                            uint64
}

func NewState(seed uint64, target int) State {
	s := State{seed: seed, Target: target, LastWinner: -1}
	s.deal()
	return s
}
func (s *State) deal() {
	d := cards.Deck()
	cards.Shuffle(d, s.seed+uint64(s.Round)*0x9e3779b97f4a7c15)
	s.Hands = [4][]cards.Card{}
	s.Sent = [4][]cards.Card{}
	s.Received = [4][]cards.Card{}
	s.Trick = nil
	s.Public = nil
	s.Void = [4][4]bool{}
	s.Points = [4]int{}
	s.Tricks = 0
	s.Broken = false
	s.RoundOver = false
	s.Passing = s.Round%4 != 3
	s.LastWinner = -1
	for i, c := range d {
		s.Hands[i%4] = append(s.Hands[i%4], c)
	}
	for i := range s.Hands {
		sort.Slice(s.Hands[i], func(a, b int) bool { x, y := s.Hands[i][a], s.Hands[i][b]; return x.Suit*14+x.Rank < y.Suit*14+y.Rank })
	}
	s.findLeader()
}
func (s *State) findLeader() {
	for p, h := range s.Hands {
		for _, c := range h {
			if c.Suit == 0 && c.Rank == 2 {
				s.Turn = p
				s.Leader = p
				return
			}
		}
	}
}
func (s *State) NextRound() bool {
	if !s.RoundOver || s.Over {
		return false
	}
	s.Round++
	s.deal()
	return true
}
func (s *State) Pass(choices [4][]int) bool {
	if !s.Passing || s.Over {
		return false
	}
	var sent, remaining [4][]cards.Card
	for p, choice := range choices {
		if len(choice) != 3 {
			return false
		}
		selected := map[int]bool{}
		for _, i := range choice {
			if i < 0 || i >= len(s.Hands[p]) || selected[i] {
				return false
			}
			selected[i] = true
			sent[p] = append(sent[p], s.Hands[p][i])
		}
		for i, c := range s.Hands[p] {
			if !selected[i] {
				remaining[p] = append(remaining[p], c)
			}
		}
	}
	offset := []int{1, 3, 2}[s.Round%4]
	for p := range remaining {
		to := (p + offset) % 4
		remaining[to] = append(remaining[to], sent[p]...)
		s.Received[to] = cards.Copy(sent[p])
	}
	s.Hands = remaining
	s.Sent = sent
	s.Passing = false
	s.findLeader()
	return true
}
func penalty(c cards.Card) int {
	if c.Suit == 3 {
		return 1
	}
	if c.Suit == 2 && c.Rank == 12 {
		return 13
	}
	return 0
}
func high(c cards.Card) int {
	if c.Rank == 1 {
		return 14
	}
	return c.Rank
}
func legal(hand []cards.Card, trick []Play, first, broken bool) []int {
	var result []int
	hasSuit, hasNonHeart, hasNonPenalty := false, false, false
	for _, c := range hand {
		if len(trick) > 0 && c.Suit == trick[0].Card.Suit {
			hasSuit = true
		}
		if c.Suit != 3 {
			hasNonHeart = true
		}
		if penalty(c) == 0 {
			hasNonPenalty = true
		}
	}
	for i, c := range hand {
		if first && len(trick) == 0 {
			if c.Suit == 0 && c.Rank == 2 {
				result = append(result, i)
			}
			continue
		}
		if len(trick) > 0 && hasSuit {
			if c.Suit == trick[0].Card.Suit {
				result = append(result, i)
			}
			continue
		}
		if len(trick) == 0 && !broken && hasNonHeart && c.Suit == 3 {
			continue
		}
		if first && hasNonPenalty && penalty(c) > 0 {
			continue
		}
		result = append(result, i)
	}
	return result
}
func (s *State) Legal(p int) []int {
	if p < 0 || p > 3 || s.Turn != p || s.Passing || s.RoundOver || s.Over {
		return nil
	}
	return legal(s.Hands[p], s.Trick, s.Tricks == 0, s.Broken)
}
func (s *State) Play(p, index int) bool {
	found := false
	for _, i := range s.Legal(p) {
		if i == index {
			found = true
		}
	}
	if !found {
		return false
	}
	c := s.Hands[p][index]
	s.Hands[p] = append(s.Hands[p][:index], s.Hands[p][index+1:]...)
	if len(s.Trick) > 0 && c.Suit != s.Trick[0].Card.Suit {
		s.Void[p][s.Trick[0].Card.Suit] = true
	}
	if c.Suit == 3 {
		s.Broken = true
	}
	play := Play{p, c}
	s.Trick = append(s.Trick, play)
	s.Public = append(s.Public, play)
	if len(s.Trick) < 4 {
		s.Turn = (p + 1) % 4
		return true
	}
	winner := s.Trick[0]
	points := 0
	for _, v := range s.Trick {
		points += penalty(v.Card)
		if v.Card.Suit == winner.Card.Suit && high(v.Card) > high(winner.Card) {
			winner = v
		}
	}
	s.Points[winner.Seat] += points
	s.LastWinner = winner.Seat
	s.Turn = winner.Seat
	s.Leader = winner.Seat
	s.Tricks++
	s.Trick = nil
	if s.Tricks == 13 {
		s.LastRound = s.Points
		for p, n := range s.Points {
			if n == 26 {
				for q := range s.LastRound {
					s.LastRound[q] = 26
				}
				s.LastRound[p] = 0
				break
			}
		}
		for p, n := range s.LastRound {
			s.Scores[p] += n
			if s.Scores[p] >= s.Target {
				s.Over = true
			}
		}
		s.RoundOver = true
	}
	return true
}
func (s State) Winners() []int {
	if !s.Over {
		return nil
	}
	low := s.Scores[0]
	for _, n := range s.Scores {
		low = min(low, n)
	}
	var winners []int
	for p, n := range s.Scores {
		if n == low {
			winners = append(winners, p)
		}
	}
	return winners
}

// Observation deliberately contains no seed, other hands, or shared bot memory.
type Observation struct {
	Seat, Direction, Tricks int
	Hand, Sent, Received    []cards.Card
	Trick, Public           []Play
	Void                    [4][4]bool
	Scores, Points          [4]int
	Legal                   []int
}

func (s State) Observe(p int) Observation {
	return Observation{Seat: p, Direction: s.Round % 4, Tricks: s.Tricks, Hand: cards.Copy(s.Hands[p]), Sent: cards.Copy(s.Sent[p]), Received: cards.Copy(s.Received[p]), Trick: append([]Play(nil), s.Trick...), Public: append([]Play(nil), s.Public...), Void: s.Void, Scores: s.Scores, Points: s.Points, Legal: append([]int(nil), s.Legal(p)...)}
}

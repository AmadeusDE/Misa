package klondike

import (
	"fmt"
	"github.com/AmadeusDE/Misa/internal/cards"
	"github.com/AmadeusDE/Misa/internal/cardui"
)

type Game = cardui.Game
type table struct {
	state   State
	history cards.History[State]
}

func New(opts cardui.Options) *Game {
	if opts.Rule == 0 {
		opts.Rule = 1
	}
	return cardui.New("Solitaire", "solitaire", opts, func(seed uint64) cardui.Table { return &table{state: NewState(seed, opts.Rule)} })
}
func (t *table) Piles() []cardui.Pile {
	s := t.state
	p := []cardui.Pile{cardui.VisiblePile("Stock", s.Stock, len(s.Stock)), cardui.VisiblePile("Waste", s.Waste, max(0, len(s.Waste)-1))}
	for i, c := range s.Columns {
		p = append(p, cardui.VisiblePile(fmt.Sprint("T", i+1), c, s.Hidden[i]))
	}
	for i, c := range s.Foundations {
		p = append(p, cardui.VisiblePile([]string{"F-C", "F-D", "F-S", "F-H"}[i], c, 0))
	}
	return p
}
func (t *table) Status() string {
	return fmt.Sprintf("Draw %d · unlimited passes · Misa score %d · moves %d", t.state.Draw, t.state.Score, t.state.Moves)
}
func (t *table) Act(from, row, to int) bool {
	if t.state.Won() {
		return false
	}
	before := t.state.Clone()
	ok := false
	if from < 0 {
		if to == 0 {
			ok = t.state.DrawStock()
		}
	} else {
		ok = t.state.Move(from, row, to)
	}
	if ok {
		t.history.Push(before)
	}
	return ok
}
func (t *table) Undo() bool {
	s, ok := t.history.Undo(t.state.Clone())
	if ok {
		t.state = s
	}
	return ok
}
func (t *table) Redo() bool {
	s, ok := t.history.Redo(t.state.Clone())
	if ok {
		t.state = s
	}
	return ok
}
func (t *table) Won() bool { return t.state.Won() }

// hint sees only a masked pile observation, never the hidden deal.
func hint(p []cardui.Pile) string {
	s := State{Draw: 1, Waste: cards.Copy(p[1].Cards)}
	for i := range s.Columns {
		s.Columns[i] = cards.Copy(p[i+2].Cards)
		s.Hidden[i] = p[i+2].Hidden
	}
	for i := range s.Foundations {
		s.Foundations[i] = cards.Copy(p[i+9].Cards)
	}
	for from := 1; from < len(p); from++ {
		for row := p[from].Hidden; row < len(p[from].Cards); row++ {
			for to := 2; to < len(p); to++ {
				trial := s.Clone()
				if trial.Move(from, row, to) {
					return fmt.Sprintf("%s[%d] → %s", p[from].Name, row+1, p[to].Name)
				}
			}
		}
	}
	if len(p[0].Cards) > 0 || len(p[1].Cards) > 0 {
		return "Try drawing/recycling stock; this is not a loss proof"
	}
	return "No immediate visible move; not a proof of loss"
}
func (t *table) Hint() string { return hint(t.Piles()) }
func (t *table) Auto() bool {
	if len(t.state.Stock) > 0 {
		return false
	}
	for _, h := range t.state.Hidden {
		if h > 0 {
			return false
		}
	}
	candidate := t.state.Clone()
	for !candidate.Won() {
		moved := false
		for from := 1; from <= 8; from++ {
			a := candidate.pile(from)
			if len(*a) == 0 {
				continue
			}
			c := cards.Top(*a)
			if candidate.Move(from, len(*a)-1, 9+c.Suit) {
				moved = true
				break
			}
		}
		if !moved {
			return false
		}
	}
	if t.state.Won() {
		return false
	}
	t.history.Push(t.state.Clone())
	t.state = candidate
	return true
}
func (t *table) Help() string {
	return "Klondike: alternate colors down; empty columns need kings. Foundations build A–K by suit and may return to tableau. Select Stock to draw/recycle. Only top waste is playable. Newly exposed cards flip automatically. Misa score: +10 net foundation, +5 flip. Auto verifies a complete finish only with no stock or hidden tableau. Random deals may be unwinnable; hints are not a solver."
}

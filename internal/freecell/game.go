package freecell

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
	return cardui.New("FreeCell", "freecell", opts, func(seed uint64) cardui.Table { return &table{state: NewState(seed)} })
}
func (t *table) Piles() []cardui.Pile {
	var p []cardui.Pile
	for i, c := range t.state.Columns {
		p = append(p, cardui.VisiblePile(fmt.Sprint("T", i+1), c, 0))
	}
	for i, c := range t.state.Cells {
		p = append(p, cardui.VisiblePile(fmt.Sprint("Cell", i+1), c, 0))
	}
	for i, c := range t.state.Foundations {
		p = append(p, cardui.VisiblePile([]string{"F-C", "F-D", "F-S", "F-H"}[i], c, 0))
	}
	return p
}
func (t *table) Status() string {
	return fmt.Sprintf("Four free cells · auto off · moves %d", t.state.Moves)
}
func (t *table) Act(from, row, to int) bool {
	if from < 0 || t.state.Won() {
		return false
	}
	before := t.state.Clone()
	if !t.state.Move(from, row, to) {
		return false
	}
	t.history.Push(before)
	return true
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
func (t *table) Hint() string {
	p := t.Piles()
	for from := 0; from < 12; from++ {
		for row := range p[from].Cards {
			for to := 15; to >= 0; to-- {
				s := t.state.Clone()
				if s.Move(from, row, to) {
					return fmt.Sprintf("%s[%d] → %s", p[from].Name, row+1, p[to].Name)
				}
			}
		}
	}
	return "No immediate move; not a proof of loss"
}
func (t *table) Auto() bool {
	for from := 0; from < 12; from++ {
		p := t.state.pile(from)
		c := cards.Top(*p)
		if c.Rank > 0 && cardui.SafeFoundation(c, t.state.Foundations) && t.Act(from, len(*p)-1, 12+c.Suit) {
			return true
		}
	}
	return false
}
func (t *table) Help() string {
	return "FreeCell: alternate colors down; any card fills an empty column. Each free cell holds one card. Foundations build A–K by suit, with no returns. A sequence move is accepted only after a legal single-card decomposition using available cells and empty columns. Auto explicitly moves one conservatively safe foundation card. Random deals are not guaranteed solvable. Seeds are Misa deals, not Microsoft deal numbers."
}

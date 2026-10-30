package spider

import (
	"fmt"
	"github.com/AmadeusDE/Misa/internal/cards"
	"github.com/AmadeusDE/Misa/internal/cardui"
)

type Game = cardui.Game
type table struct {
	state   State
	history cards.History[State]
	suits   int
}

func New(opts cardui.Options) *Game {
	if opts.Rule == 0 {
		opts.Rule = 1
	}
	return cardui.New("Spider Solitaire", "spider", opts, func(seed uint64) cardui.Table { return &table{state: NewState(seed, opts.Rule), suits: opts.Rule} })
}
func (t *table) Piles() []cardui.Pile {
	var p []cardui.Pile
	for i, c := range t.state.Columns {
		p = append(p, cardui.VisiblePile(fmt.Sprint("T", i+1), c, t.state.Hidden[i]))
	}
	p = append(p, cardui.VisiblePile("Stock", t.state.Stock, len(t.state.Stock)))
	return p
}
func (t *table) Status() string {
	return fmt.Sprintf("%d suit(s) · score %d · moves %d · complete %d/8 · stock %d rows", t.suits, t.state.Score, t.state.Moves, t.state.Completed, len(t.state.Stock)/10)
}
func (t *table) Act(from, row, to int) bool {
	if t.state.Won() {
		return false
	}
	before := t.state.Clone()
	ok := false
	if from < 0 {
		if to == 10 {
			ok = t.state.DealRow()
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
func (t *table) Hint() string {
	p := t.Piles()
	s := State{}
	for i := range s.Columns {
		s.Columns[i] = cards.Copy(p[i].Cards)
		s.Hidden[i] = p[i].Hidden
	}
	for from := 0; from < 10; from++ {
		for row := p[from].Hidden; row < len(p[from].Cards); row++ {
			for to := 0; to < 10; to++ {
				trial := s.Clone()
				if trial.Move(from, row, to) {
					return fmt.Sprintf("T%d[%d] → T%d", from+1, row+1, to+1)
				}
			}
		}
	}
	if len(p[10].Cards) > 0 {
		for _, c := range s.Columns {
			if len(c) == 0 {
				return "Fill empty columns before dealing stock; not a loss proof"
			}
		}
		return "Try dealing a stock row"
	}
	return "No immediate visible move; not a proof of loss"
}
func (t *table) Auto() bool { return false }
func (t *table) Help() string {
	return "Spider: build down by rank, but only same-suit descending runs move together. Any movable card/run fills an empty column. Select Stock for one card per column; all columns must be occupied. Exposed suited K–A runs are removed automatically, including resulting flips/completions. Score: 500 − moves + 100 per completed run. Undo/redo includes entire stock rows and all completion effects. Hints cannot prove a loss."
}

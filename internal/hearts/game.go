package hearts

import (
	"fmt"
	"github.com/AmadeusDE/Misa/internal/cardui"
)

type Game = cardui.Game
type table struct {
	state     State
	selected  map[int]bool
	committed bool
	lastTrick []Play
}
type decision struct {
	pass    [4][]int
	index   int
	passing bool
}

func New(opts cardui.Options) *Game {
	if opts.Rule == 0 {
		opts.Rule = 100
	}
	return cardui.New("Hearts", "hearts", opts, func(seed uint64) cardui.Table {
		return &table{state: NewState(seed, opts.Rule), selected: map[int]bool{}}
	})
}
func (t *table) Piles() []cardui.Pile {
	p := []cardui.Pile{cardui.VisiblePile("You", t.state.Hands[0], 0)}
	p[0].Marked = t.selected
	name := "Commit"
	if t.state.RoundOver {
		name = "Next"
	}
	p = append(p, cardui.Pile{Name: name})
	for i := 1; i < 4; i++ {
		p = append(p, cardui.VisiblePile(fmt.Sprint("Bot", i), t.state.Hands[i], len(t.state.Hands[i])))
	}
	trick := t.lastTrick
	if len(t.state.Trick) > 0 {
		trick = t.state.Trick
	}
	shown := cardui.Pile{Name: "Trick", Prefix: map[int]string{}}
	for i, v := range trick {
		shown.Cards = append(shown.Cards, v.Card)
		shown.Prefix[i] = fmt.Sprint(v.Seat)
	}
	return append(p, shown)
}
func (t *table) Status() string {
	s := t.state
	phase := fmt.Sprintf("seat %d plays · trick %d/13 · winner %d", s.Turn, s.Tricks+1, s.LastWinner)
	if s.Passing {
		phase = "Pass 3 " + []string{"left", "right", "across", "hold"}[s.Round%4] + fmt.Sprintf(" (%d selected)", len(t.selected))
	}
	if s.RoundOver {
		phase = fmt.Sprintf("Round points %v · select Next", s.LastRound)
	}
	if s.Over {
		phase = fmt.Sprintf("Round points %v · winning seats %v", s.LastRound, s.Winners())
	}
	return fmt.Sprintf("Hand %d · target %d · scores %v\n%s", s.Round+1, s.Target, s.Scores, phase)
}
func (t *table) Act(from, row, to int) bool {
	if from >= 0 || t.committed || t.state.Over {
		return false
	}
	if t.state.RoundOver && to == 1 {
		t.selected = map[int]bool{}
		t.lastTrick = nil
		return t.state.NextRound()
	}
	if t.state.Passing {
		if to == 0 && row >= 0 && row < len(t.state.Hands[0]) {
			if t.selected[row] {
				delete(t.selected, row)
			} else if len(t.selected) < 3 {
				t.selected[row] = true
			}
			return true
		}
		if to == 1 && len(t.selected) == 3 {
			t.committed = true
			return true
		}
		return false
	}
	if to == 0 {
		return t.play(0, row)
	}
	return false
}
func (t *table) play(p, index int) bool {
	trick := append([]Play(nil), t.state.Trick...)
	if !t.state.Play(p, index) {
		return false
	}
	if len(trick) == 3 {
		t.lastTrick = append(trick, t.state.Public[len(t.state.Public)-1])
	}
	return true
}
func (t *table) Undo() bool { return false }
func (t *table) Redo() bool { return false }
func (t *table) Auto() bool { return false }
func (t *table) Won() bool  { return t.state.Over }
func (t *table) Hint() string {
	if t.state.Passing {
		return "Select three cards in You, then select Commit; toggle to change selection"
	}
	i := ChoosePlay(t.state.Observe(0))
	if i >= 0 {
		return fmt.Sprintf("Try your card %d", i+1)
	}
	return "Wait for the bots, or select Next after the round"
}
func (t *table) Help() string {
	return "You are seat 0; bots are seats 1–3 clockwise. Select three cards, then Commit. Passes rotate left/right/across/hold. Select a legal hand card to play it. Follow suit; 2C opens. No hearts lead until broken unless only hearts remain. No first-trick penalty discard when a non-penalty discard exists. QS does not break hearts. Hearts cost 1; QS costs 13. Moon: shooter 0, others 26. Lowest total wins at target; ties share victory. Select Next after each round. No undo of revealed information. Restart explicitly replays the match."
}
func (t *table) Work() func() any {
	if t.state.Over || t.state.RoundOver {
		return nil
	}
	if t.state.Passing {
		if !t.committed {
			return nil
		}
		var obs [4]Observation
		for p := range obs {
			obs[p] = t.state.Observe(p)
		}
		var human []int
		for i := range t.state.Hands[0] {
			if t.selected[i] {
				human = append(human, i)
			}
		}
		return func() any {
			d := decision{passing: true}
			d.pass[0] = human
			for p := 1; p < 4; p++ {
				d.pass[p] = ChoosePass(obs[p])
			}
			return d
		}
	}
	if t.state.Turn == 0 {
		return nil
	}
	o := t.state.Observe(t.state.Turn)
	return func() any { return decision{index: ChoosePlay(o)} }
}
func (t *table) Accept(v any) {
	d, ok := v.(decision)
	if !ok {
		return
	}
	if d.passing {
		if t.state.Pass(d.pass) {
			t.selected = map[int]bool{}
		}
		t.committed = false
	} else {
		t.play(t.state.Turn, d.index)
	}
}

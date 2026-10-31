package chess

import (
	"strings"
)

type Status int

const (
	Playing Status = iota
	Checkmate
	Stalemate
	DrawMoveLimit
	DrawRepetition
	DrawMaterial
	GoalWin
	CheckWin
	DrawGoalReply
	ExtinctionWin
)

func (s Status) String() string {
	return [...]string{"playing", "checkmate", "stalemate", "draw by the move limit", "draw by repetition", "draw: only royal pieces left", "goal reached", "check limit reached", "draw by the goal reply", "extinction"}[s]
}

// Match is a game in progress: the positions so far, so moves can be taken
// back, and the rules that end a game.
type Match struct {
	variant  *Variant
	history  []*Position
	notation []string
	moves    []Move
	legal    []Move
	status   Status
	winner   Color
}

func NewMatch(v *Variant) *Match {
	m := &Match{variant: v}
	m.Load(NewPosition(v))
	return m
}

// Load starts the match over from p.
func (m *Match) Load(p *Position) {
	m.history = []*Position{p}
	m.notation, m.moves = nil, nil
	m.refresh()
}

func (m *Match) Variant() *Variant   { return m.variant }
func (m *Match) Position() *Position { return m.history[len(m.history)-1] }
func (m *Match) Legal() []Move       { return m.legal }
func (m *Match) Notation() []string  { return m.notation }
func (m *Match) Played() []Move      { return m.moves }
func (m *Match) CanUndo() bool       { return len(m.history) > 1 }
func (m *Match) Turn() Color         { return m.Position().Turn }
func (m *Match) Over() bool          { return m.status != Playing }

// Status reports how the game stands; the winner only means something when
// the game is decisive, which a goal arrival, a check limit, an extinction and
// checkmate are, and so is a stalemate under a stalemate rule that names a
// winner. The drawn endings, the goal reply draw among them, name no winner.
func (m *Match) Status() (Status, Color) {
	return m.status, m.winner
}

// Decisive reports whether the game ended with a winner rather than a draw:
// a checkmate, a goal arrival, a check limit, an extinction, or a stalemate
// under whichever rule says one side wins or loses it. The goal reply draw and
// the other draw rules are not decisive.
func (m *Match) Decisive() bool {
	switch m.status {
	case Checkmate, GoalWin, CheckWin, ExtinctionWin:
		return true
	case Stalemate:
		return m.variant.Stalemate != StalemateDraw
	}
	return false
}

// Play makes a move, which must be one of Legal.
func (m *Match) Play(mv Move) {
	before := m.Position()
	m.history = append(m.history, before.Apply(mv))
	m.moves = append(m.moves, mv)
	m.refresh()
	text := describe(before, mv)
	switch {
	case m.status == Checkmate || m.status == ExtinctionWin:
		text += "#"
	case m.Position().InCheck():
		text += "+"
	}
	m.notation = append(m.notation, text)
}

func (m *Match) Undo() bool {
	if !m.CanUndo() {
		return false
	}
	m.history = m.history[:len(m.history)-1]
	m.moves = m.moves[:len(m.moves)-1]
	m.notation = m.notation[:len(m.notation)-1]
	m.refresh()
	return true
}

func (m *Match) refresh() {
	p := m.Position()
	m.legal = p.Moves()
	m.status, m.winner = Playing, p.Turn.Other()
	if o, over := adjudicate(p, len(m.legal) > 0); over {
		m.status, m.winner = o.status, o.winner
		return
	}
	switch {
	case m.repetitions() >= 3:
		m.status = DrawRepetition
	case p.Clock >= 2*m.variant.MoveLimit:
		m.status = DrawMoveLimit
	case len(m.variant.Goal) == 0 && onlyRoyals(p):
		m.status = DrawMaterial
	}
}

func (m *Match) repetitions() int {
	key := m.Position().Hash()
	n := 0
	for _, p := range m.history {
		if p.Hash() == key {
			n++
		}
	}
	return n
}

// Hashes returns the hashes of the positions before the current one, oldest
// first, so a search can see repetitions; the slice is the caller's.
func (m *Match) Hashes() []uint64 {
	out := make([]uint64, len(m.history)-1)
	for i, p := range m.history[:len(m.history)-1] {
		out[i] = p.Hash()
	}
	return out
}

func onlyRoyals(p *Position) bool {
	found := false
	for _, pc := range p.Board {
		if pc.Type != 0 {
			if !p.V.Type(pc).Royal {
				return false
			}
			found = true
		}
	}
	return found
}

func b2i(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// describe writes a move in long algebraic notation: Ng1-f3, e7xd8=Q, O-O.
func describe(p *Position, mv Move) string {
	v := p.V
	if mv.Castle >= 0 {
		c := v.Castling[mv.Castle]
		if c.Rook%v.Files > c.King%v.Files {
			return "O-O"
		}
		return "O-O-O"
	}
	var b strings.Builder
	if t := v.Type(p.Board[mv.From]); !t.Pawn {
		b.WriteByte(t.Letter)
	}
	b.WriteString(v.SquareName(mv.From))
	if mv.From == mv.To {
		b.WriteString(" pass")
		return b.String()
	}
	if mv.Take >= 0 {
		b.WriteByte('x')
	} else {
		b.WriteByte('-')
	}
	b.WriteString(v.SquareName(mv.To))
	if mv.Promote != 0 {
		b.WriteByte('=')
		b.WriteByte(v.Pieces[mv.Promote-1].Letter)
	}
	return b.String()
}

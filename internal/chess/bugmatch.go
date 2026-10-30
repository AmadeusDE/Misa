package chess

import "slices"

// BugMatch joins two independently played boards. Board 0 White and board 1
// Black are partners; the other two players form the opposing team. Captures
// go to the partner's hand. Call Play rather than mutating Board's Match.
// Either board may play when it is that board's turn; scheduling is up to the caller.
type BugMatch struct {
	boards [2]*Match
	undo   [][2]Match
}

func NewBugMatch(v *Variant) *BugMatch {
	if !v.PartnerHands {
		panic("chess: BugMatch requires a partner-hands variant")
	}
	return &BugMatch{boards: [2]*Match{NewMatch(v), NewMatch(v)}}
}

// Load resets both boards and the global undo history. Positions must use the
// match's variant; the caller must not subsequently mutate them.
func (b *BugMatch) Load(a, c *Position) {
	if a.V != b.boards[0].variant || c.V != b.boards[1].variant {
		panic("chess: incompatible bughouse positions")
	}
	b.boards[0].Load(a)
	b.boards[1].Load(c)
	b.undo = nil
}
func (b *BugMatch) Board(i int) *Match { return b.boards[i] }
func (b *BugMatch) Over() bool         { s, _, _ := b.Status(); return s != Playing }

// Status identifies the ended board and its winning color on checkmate. On a
// stalemate there is no winner; while playing the board is -1.
func (b *BugMatch) Status() (Status, int, Color) {
	for i, m := range b.boards {
		if s, winner := m.Status(); s != Playing {
			return s, i, winner
		}
	}
	return Playing, -1, White
}

// WinnerTeam returns 1 for A-White/B-Black, 2 for A-Black/B-White, or 0
// when there is no winner.
func (b *BugMatch) WinnerTeam() int {
	s, board, color := b.Status()
	if s != Checkmate {
		return 0
	}
	if (board == 0 && color == White) || (board == 1 && color == Black) {
		return 1
	}
	return 2
}

// Play validates a move on either board and routes any capture atomically.
func (b *BugMatch) Play(i int, mv Move) bool {
	if i < 0 || i > 1 || b.Over() || !slices.Contains(b.boards[i].Legal(), mv) {
		return false
	}
	var snapshot [2]Match
	for n, m := range b.boards {
		snapshot[n] = *m
		snapshot[n].history = slices.Clone(m.history)
		snapshot[n].notation = slices.Clone(m.notation)
		snapshot[n].moves = slices.Clone(m.moves)
	}
	b.undo = append(b.undo, snapshot)
	m := b.boards[i]
	before := m.Position()
	if mv.Take >= 0 {
		captured := before.Board[mv.Take]
		typ := captured.Type
		if captured.Promoted != 0 {
			typ = captured.Promoted
		}
		if typ != 0 {
			partner := b.boards[1-i]
			p := partner.Position().clone()
			p.Hands[before.Turn.Other()][typ]++
			partner.history[len(partner.history)-1] = p
			partner.refresh()
		}
	}
	m.Play(mv)
	return true
}

// Undo reverses the latest move on either board, including its transfer.
func (b *BugMatch) Undo() bool {
	if len(b.undo) == 0 {
		return false
	}
	snapshot := b.undo[len(b.undo)-1]
	b.undo = b.undo[:len(b.undo)-1]
	for i := range b.boards {
		*b.boards[i] = snapshot[i]
	}
	return true
}

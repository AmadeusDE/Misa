package chess

// Visible returns a board-sized mask of c's occupied squares and legal move
// destinations, plus a variant's duck, which both sides always see. It computes
// visibility regardless of whether fog is enabled. En passant is available only
// to the actual side to move.
func (p *Position) Visible(c Color) []bool {
	visible := make([]bool, len(p.Board))
	for sq, pc := range p.Board {
		if pc.Type != 0 && pc.Color == c {
			visible[sq] = true
		}
	}
	if p.V.Blocker && p.Blocker >= 0 && p.Blocker < len(visible) {
		visible[p.Blocker] = true
	}
	q := *p
	if c != p.Turn {
		q.EP = nil
		q.EPPawn = -1
	}
	q.Turn = c
	for _, m := range q.Moves() {
		visible[m.To] = true
	}
	return visible
}

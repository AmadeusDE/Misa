package chess

import "slices"

// dropMoves generates quiet drops; ordinary legality filtering also checks
// whether a drop resolves check against every royal.
func (p *Position) dropMoves() []Move {
	if !p.V.Drops {
		return nil
	}
	var moves []Move
	for typ := 1; typ <= len(p.V.Pieces) && typ < len(p.Hands[p.Turn]); typ++ {
		if p.Hands[p.Turn][typ] <= 0 {
			continue
		}
		t := p.V.Pieces[typ-1]
		for sq, pc := range p.Board {
			if pc.Type != 0 || (p.V.Blocker && sq == p.Blocker) {
				continue // a duck's square is not empty for a drop
			}
			_, y := p.V.XY(sq)
			rank := y + 1
			if p.Turn == Black {
				rank = p.V.Ranks - y
			}
			if slices.Contains(t.DropForbidRanks, rank) {
				continue
			}
			moves = append(moves, Move{From: -1, To: sq, Take: -1, Castle: -1, Drop: uint8(typ)})
		}
	}
	return moves
}

func (p *Position) applyDrop(m Move) {
	p.Hands[p.Turn][m.Drop]--
	t := p.V.Pieces[m.Drop-1]
	moved := true
	// Only pawns dropped on one of their own initial ranks may use an
	// initial-only component. Other dropped pieces never regain such rights.
	if t.Pawn {
		_, y := p.V.XY(m.To)
		for sq, start := range p.V.start {
			_, sy := p.V.XY(sq)
			if sy == y && start.Type == m.Drop && start.Color == p.Turn {
				moved = false
				break
			}
		}
	}
	p.Board[m.To] = Piece{Type: m.Drop, Color: p.Turn, Moved: moved}
	p.Clock++
	if t.Pawn {
		p.Clock = 0
	}
	p.Turn = p.Turn.Other()
	if t.Royal {
		p.findRoyals()
	}
	// A drop may give check, which a variant that counts checks records just
	// as for an ordinary move.
	p.countCheck()
}

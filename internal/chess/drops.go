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
		// Only an unpromoted piece of this exact type and color occupies a
		// restricted file; a promoted pawn no longer counts as a pawn.
		var occupiedFiles []bool
		if t.DropOnePerFile {
			occupiedFiles = make([]bool, p.V.Files)
			for sq, pc := range p.Board {
				if pc.Type == uint8(typ) && pc.Color == p.Turn && pc.Promoted == 0 {
					x, _ := p.V.XY(sq)
					occupiedFiles[x] = true
				}
			}
		}
		for sq, pc := range p.Board {
			if pc.Type != 0 || (p.V.Blocker && sq == p.Blocker) {
				continue // a duck's square is not empty for a drop
			}
			x, y := p.V.XY(sq)
			if t.DropOnePerFile && occupiedFiles[x] {
				continue
			}
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

// legalDrop excludes immediate checkmate by types with the no-mate rule.
// Evaluating all legal replies (including drops) also handles variants where
// a drop can interpose against a checking rider. Recursion is finite: each
// nested no-mate check applies a drop and consumes a piece from a hand.
func (p *Position) legalDrop(m Move, q *Position) bool {
	if m.Drop == 0 || !p.V.Pieces[m.Drop-1].DropNoMate || !q.InCheck() {
		return true
	}
	return len(q.Moves()) != 0
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

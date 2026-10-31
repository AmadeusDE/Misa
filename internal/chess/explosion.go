package chess

import "slices"

// explode removes the capturer at centre and nonimmune pieces in the blast.
// Apply has already removed the captured piece, including an en passant pawn.
func (p *Position) explode(centre int) {
	x, y := p.V.XY(centre)
	for sq, pc := range p.Board {
		if pc.Type == 0 {
			continue
		}
		sx, sy := p.V.XY(sq)
		if max(abs(sx-x), abs(sy-y)) <= p.V.Explosion.Radius && (sq == centre || !slices.Contains(p.V.Explosion.Immune, pc.Type)) {
			p.Board[sq] = Piece{}
		}
	}
}

func (p *Position) connectedRoyal(sq int, enemy Color) bool {
	x, y := p.V.XY(sq)
	for _, other := range p.royalSq[enemy] {
		ox, oy := p.V.XY(other)
		if max(abs(x-ox), abs(y-oy)) <= p.V.Explosion.Radius {
			return true
		}
	}
	return false
}

// extinct reports whether a configured extinction condition has been met.
func (p *Position) extinct(c Color) bool {
	if len(p.V.Extinction) == 0 {
		return false
	}
	for _, pc := range p.Board {
		if pc.Type != 0 && pc.Color == c && slices.Contains(p.V.Extinction, pc.Type) {
			return false
		}
	}
	return true
}

// Test each transit square with the royal actually there: connections and
// blocked lines can change as the king leaves its starting square.
func (p *Position) castlePathAttacked(king int, path []int) bool {
	if p.V.Explosion == nil {
		// The royal travels the path, so royal-only attacks count too.
		return p.attacked(path, p.Turn.Other(), true)
	}
	for _, sq := range path {
		q := p.clone()
		q.Board[king] = Piece{}
		q.Board[sq] = p.Board[king]
		q.findRoyals()
		if q.royalAttacked(p.Turn) {
			return true
		}
	}
	return false
}

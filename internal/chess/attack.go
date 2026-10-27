package chess

import "slices"

// reachTable marks, for one piece type and side, every offset from the piece
// at which it could ever capture on an empty board. anyAttacked walks a
// piece's moves only when a target lies at such an offset.
type reachTable []bool

func (v *Variant) offsetIndex(dx, dy int) int {
	return (dy+v.Ranks-1)*(2*v.Files-1) + dx + v.Files - 1
}

func (v *Variant) buildReach() {
	for _, t := range v.Pieces {
		for _, c := range []Color{White, Black} {
			table := make(reachTable, (2*v.Files-1)*(2*v.Ranks-1))
			mark := func(dx, dy int) {
				dx, dy = dx*c.forward(), dy*c.forward()
				if abs(dx) < v.Files && abs(dy) < v.Ranks && (dx != 0 || dy != 0) {
					table[v.offsetIndex(dx, dy)] = true
				}
			}
			for _, comp := range t.moves {
				if comp.Capture {
					markComponent(v, comp, mark)
				}
			}
			t.reach[c] = table
		}
	}
}

func markComponent(v *Variant, comp Component, mark func(dx, dy int)) {
	if comp.Circular {
		n := len(comp.Ring)
		for _, first := range comp.Vectors {
			start := slices.Index(comp.Ring, first)
			for _, turn := range []int{1, -1} {
				x, y := 0, 0
				for k := range n - 1 {
					leg := comp.Ring[((start+turn*k)%n+n)%n]
					x, y = x+leg[0], y+leg[1]
					mark(x, y)
				}
			}
		}
		return
	}
	reach := comp.Range
	if reach != 1 {
		reach = max(v.Files, v.Ranks)
		if comp.Range > 0 {
			reach = comp.Range
		}
		if comp.Hop != hopNone {
			reach = max(v.Files, v.Ranks)
		}
	}
	for _, vec := range comp.Vectors {
		for k := 1; k <= reach; k++ {
			mark(vec[0]*k, vec[1]*k)
		}
	}
}

// anyAttacked reports whether a piece of side by could capture on any of sqs.
func (p *Position) anyAttacked(sqs []int, by Color) bool {
	if len(sqs) == 0 {
		return false
	}
	v := p.V
	for from, pc := range p.Board {
		if pc.Type == 0 || pc.Color != by {
			continue
		}
		t := v.Pieces[pc.Type-1]
		if !t.mightReach(v, from, sqs, by) {
			continue
		}
		for i := range t.moves {
			comp := &t.moves[i]
			if !comp.Capture || (comp.Initial && pc.Moved) {
				continue
			}
			hit := false
			p.walk(from, comp, by, func(to int) bool {
				hit = slices.Contains(sqs, to)
				return !hit
			})
			if hit {
				return true
			}
		}
	}
	return false
}

func (t *PieceType) mightReach(v *Variant, from int, sqs []int, c Color) bool {
	fx, fy := v.XY(from)
	for _, sq := range sqs {
		x, y := v.XY(sq)
		if t.reach[c][v.offsetIndex(x-fx, y-fy)] {
			return true
		}
	}
	return false
}

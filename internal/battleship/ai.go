package battleship

import "math/rand/v2"

// AI hunts a fleet: it fires on a checkerboard pattern until something is hit,
// then finishes that ship before hunting again. Every choice is read back from
// the board, so the AI needs no record of its earlier shots.
type AI struct {
	rng *rand.Rand
}

// NewAI makes an AI that draws its shots from rng.
func NewAI(rng *rand.Rand) *AI { return &AI{rng: rng} }

// Fire shoots once at b and returns the square it fired at; ok is false when
// every square has already been shot at.
func (a *AI) Fire(b *Board) (Point, bool) {
	for _, p := range a.targets(b) {
		if b.Fire(p.X, p.Y) != Unfired {
			return p, true
		}
	}
	p, ok := a.hunt(b)
	if !ok {
		return Point{}, false
	}
	b.Fire(p.X, p.Y)
	return p, true
}

// targets lists the squares worth trying next because hits sit next to them,
// the ends of a line of hits first: a wounded ship is finished before the AI
// hunts again, which keeps its shot count low.
func (a *AI) targets(b *Board) []Point {
	var line, around []Point
	for _, cells := range a.wounded(b) {
		if ends, ok := a.lineEnds(cells, b); ok {
			line = append(line, ends...)
			continue
		}
		around = append(around, a.neighbours(cells, b)...)
	}
	return dedupe(append(line, around...), b)
}

// wounded groups the hits that sit on ships still afloat by ship.
func (a *AI) wounded(b *Board) map[*Ship][]Point {
	groups := map[*Ship][]Point{}
	for y := range b.Size() {
		for x := range b.Size() {
			if b.Shot(x, y) != Hit {
				continue
			}
			if s := b.ShipAt(x, y); s != nil && !s.Sunk() {
				groups[s] = append(groups[s], Point{x, y})
			}
		}
	}
	return groups
}

// lineEnds reads two or more hits on one ship as a row or a column and returns
// the squares past either end plus any unhit square between them, which must be
// part of that ship. It reports false while a single hit leaves the direction
// open, or if the hits do not line up (which a straight ship never does).
func (a *AI) lineEnds(cells []Point, b *Board) ([]Point, bool) {
	if len(cells) < 2 {
		return nil, false
	}
	row, col := true, true
	for _, p := range cells[1:] {
		row = row && p.Y == cells[0].Y
		col = col && p.X == cells[0].X
	}
	if !row && !col {
		return nil, false
	}
	lo, hi := cells[0], cells[0]
	for _, p := range cells {
		if row {
			lo.X, hi.X = min(lo.X, p.X), max(hi.X, p.X)
		} else {
			lo.Y, hi.Y = min(lo.Y, p.Y), max(hi.Y, p.Y)
		}
	}
	var out []Point
	for x, y := lo.X-1, lo.Y; row && x <= hi.X+1; x++ {
		if b.in(x, y) && !b.Fired(x, y) {
			out = append(out, Point{x, y})
		}
	}
	for x, y := lo.X, lo.Y-1; col && y <= hi.Y+1; y++ {
		if b.in(x, y) && !b.Fired(x, y) {
			out = append(out, Point{x, y})
		}
	}
	return out, true
}

// neighbours lists the unfired orthogonal squares around a group of hits.
func (a *AI) neighbours(cells []Point, b *Board) []Point {
	var out []Point
	for _, p := range cells {
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := Point{p.X + d[0], p.Y + d[1]}
			if b.in(n.X, n.Y) && !b.Fired(n.X, n.Y) {
				out = append(out, n)
			}
		}
	}
	return out
}

// dedupe drops repeated, off-board and already-fired squares.
func dedupe(points []Point, b *Board) []Point {
	seen := make(map[Point]bool, len(points))
	out := points[:0]
	for _, p := range points {
		if seen[p] || !b.in(p.X, p.Y) || b.Fired(p.X, p.Y) {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// hunt picks a square at random, preferring the checkerboard parity every ship
// of two or more squares must touch, so no ship can slip between shots.
func (a *AI) hunt(b *Board) (Point, bool) {
	var even, all []Point
	for y := range b.Size() {
		for x := range b.Size() {
			if b.Fired(x, y) {
				continue
			}
			p := Point{x, y}
			all = append(all, p)
			if (x+y)%2 == 0 {
				even = append(even, p)
			}
		}
	}
	if len(even) > 0 {
		return even[a.rng.IntN(len(even))], true
	}
	if len(all) > 0 {
		return all[a.rng.IntN(len(all))], true
	}
	return Point{}, false
}

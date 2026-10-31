package chess

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

type Color uint8

const (
	White Color = iota
	Black
)

func (c Color) Other() Color { return c ^ 1 }

func (c Color) String() string {
	if c == White {
		return "White"
	}
	return "Black"
}

// forward is the rank direction the side's pieces advance in.
func (c Color) forward() int {
	if c == White {
		return 1
	}
	return -1
}

type Piece struct {
	Type     uint8 // 0 for an empty square, else Variant.Pieces[Type-1]
	Color    Color
	Moved    bool
	Promoted uint8 // original type before promotion, or 0
}

type Move struct {
	From, To int
	Take     int   // square of the captured piece (differs from To en passant), or -1
	Drop     uint8 // type dropped from hand, or 0
	Promote  uint8 // piece type the pawn becomes, or 0
	Castle   int   // index into Variant.Castling, or -1
}

type Position struct {
	V        *Variant
	Board    []Piece
	Hands    [2][]int // drop-variant pockets, counts indexed by piece type
	Turn     Color
	EP       []int  // squares the last move's pawn crossed
	EPPawn   int    // that pawn, or -1
	Clock    int    // plies since the last capture or pawn move
	Fullmove int    // move number, incremented after Black moves
	Checks   [2]int // checks each side has given, for a variant with a CheckLimit

	castling [2][]resolvedCastle
	royalSq  [2][]int // kept by Apply so check tests need not scan the board
}

func NewPosition(v *Variant) *Position { return NewPositionWithRand(v, nil) }

// NewPositionWithRand constructs a position using rng for randomized setups.
func NewPositionWithRand(v *Variant, rng *rand.Rand) *Position {
	var board []Piece
	if rng == nil {
		board = v.shuffledStart(nil)
	} else {
		board = v.shuffledStart(rng)
	}
	p := &Position{V: v, Board: board, EPPawn: -1, Fullmove: 1}
	for c := range p.Hands {
		p.Hands[c] = slices.Clone(v.startHands[c])
		if len(p.Hands[c]) != len(v.Pieces)+1 {
			p.Hands[c] = make([]int, len(v.Pieces)+1)
		}
	}
	p.findRoyals()
	p.resolveCastles("")
	return p
}

// parseChecks reads the optional checks field: "+n+m" counts the checks White
// and Black have given, "n+m" the checks they have left, as Lichess writes a
// three-check FEN. A variant without a CheckLimit ignores the field.
func (p *Position) parseChecks(fields []string) error {
	if p.V.CheckLimit == 0 {
		return nil
	}
	for _, f := range fields {
		if !strings.Contains(f, "+") {
			continue
		}
		given := strings.HasPrefix(f, "+")
		white, black, ok := strings.Cut(strings.TrimPrefix(f, "+"), "+")
		nw, errW := strconv.Atoi(white)
		nb, errB := strconv.Atoi(black)
		if !ok || errW != nil || errB != nil || nw < 0 || nb < 0 {
			return fmt.Errorf("checks %q is not +white+black", f)
		}
		limit := p.V.CheckLimit
		if given {
			if nw > limit || nb > limit {
				return fmt.Errorf("checks %q: a side cannot give more than %d checks", f, limit)
			}
			p.Checks = [2]int{nw, nb}
			return nil
		}
		if nw > limit || nb > limit {
			return fmt.Errorf("checks %q: a side cannot have more than %d checks left", f, limit)
		}
		p.Checks = [2]int{limit - nw, limit - nb}
		return nil
	}
	return nil
}

func (p *Position) clone() *Position {
	q := *p
	q.Board = slices.Clone(p.Board)
	for c := range q.castling {
		q.castling[c] = slices.Clone(p.castling[c])
	}
	for c := range q.Hands {
		q.Hands[c] = slices.Clone(p.Hands[c])
	}
	q.EP = nil
	q.EPPawn = -1
	return &q
}

// Moves returns the legal moves for the side to move.
func (p *Position) Moves() []Move {
	var legal []Move
	for _, m := range p.moveCandidates() {
		if p.Apply(m).legalAfter(p.Turn, len(p.royalSq[p.Turn])) {
			legal = append(legal, m)
		}
	}
	return legal
}

// legalAfter reports whether a move by mover is legal once made: no royal of
// the mover may be left attacked and, where the variant forbids checking, no
// royal of the other side may be attacked either. previousRoyals is the mover's
// royal count before the move: with explosions or extinction in play, a move
// that costs the mover a royal is illegal, while one that extinguishes the
// other side wins even when the mover is left in check.
func (p *Position) legalAfter(mover Color, previousRoyals int) bool {
	if p.V.Explosion != nil || len(p.V.Extinction) > 0 {
		if len(p.royalSq[mover]) < previousRoyals {
			return false
		}
		if p.extinct(mover.Other()) {
			return true
		}
	}
	if p.royalAttacked(mover) {
		return false
	}
	return !p.V.NoChecks || !p.royalAttacked(mover.Other())
}

// InCheck reports whether any of the side to move's royal pieces is attacked.
func (p *Position) InCheck() bool { return p.royalAttacked(p.Turn) }

func (p *Position) royalAttacked(c Color) bool {
	if p.V.Explosion == nil {
		return p.anyAttacked(p.royalSq[c], c.Other())
	}
	for _, sq := range p.royalSq[c] {
		if !p.connectedRoyal(sq, c.Other()) && p.anyAttacked([]int{sq}, c.Other()) {
			return true
		}
	}
	return false
}

func (p *Position) findRoyals() {
	p.royalSq = [2][]int{}
	for sq, pc := range p.Board {
		if pc.Type != 0 && p.V.Type(pc).Royal {
			p.royalSq[pc.Color] = append(p.royalSq[pc.Color], sq)
		}
	}
}

// Royals returns the squares of a side's royal pieces.
func (p *Position) Royals(c Color) []int { return slices.Clone(p.royalSq[c]) }

func (p *Position) pseudoMoves() []Move { return p.generate(false) }

// moveCandidates applies the compulsory capture rule before the common royal
// safety filter. Only legal captures suppress quiet moves, including promotions.
func (p *Position) moveCandidates() []Move {
	if !p.V.ForcedCapture {
		return p.pseudoMoves()
	}
	tactical := p.captures()
	var captures []Move
	for _, m := range tactical {
		if m.Take >= 0 && p.Apply(m).legalAfter(p.Turn, len(p.royalSq[p.Turn])) {
			captures = append(captures, m)
		}
	}
	if len(captures) != 0 {
		return captures
	}
	return p.pseudoMoves()
}

// captures lists the moves that take something or promote, for the
// quiescence search.
func (p *Position) captures() []Move { return p.generate(true) }

func (p *Position) generate(tactical bool) []Move {
	v := p.V
	var moves []Move
	seen := make([]int, v.Size())
	// Captured-only promotion compares the board with the setup, so count the
	// side's pieces once instead of per promoting pawn.
	capturedOnly := v.PromotionCapturedOnly && len(v.Promotion) > 0
	var own []int32 // only allocated when the rule is on
	if capturedOnly {
		own = make([]int32, len(v.Pieces)+1)
		for _, pc := range p.Board {
			if pc.Type != 0 && pc.Color == p.Turn {
				own[pc.Type]++
			}
		}
	}
	// available reports whether the side to move may still promote to promo:
	// fewer of it stand on the board than the setup holds. Attacks never look
	// at this, so an unavailable promotion leaves a pawn's threats intact.
	available := func(promo uint8) bool {
		return !capturedOnly || int(own[promo]) < v.startCount(p.Turn, promo)
	}
	for from, pc := range p.Board {
		if pc.Type == 0 || pc.Color != p.Turn {
			continue
		}
		t := v.Type(pc)
		clear(seen)
		add := func(to, take int) {
			promotes := t.Pawn && to != from && p.promotes(to) && len(v.Promotion) > 0
			// A piece confined to a region (a general in its palace, an
			// elephant that may not cross the river) cannot enter to at all.
			if !t.allows(v, to, pc.Color) {
				return
			}
			if seen[to] != 0 {
				// A move and an en-passant component can reach the same empty
				// square. The capture takes precedence over the quiet move.
				for i := seen[to] - 1; i < len(moves) && moves[i].From == from && moves[i].To == to; i++ {
					if take >= 0 && moves[i].Take < 0 {
						moves[i].Take = take
					}
				}
				return
			}
			seen[to] = len(moves) + 1
			if promotes {
				for _, promo := range v.Promotion {
					if available(promo) {
						moves = append(moves, Move{From: from, To: to, Take: take, Promote: promo, Castle: -1})
					}
				}
				// Promotion is mandatory on the final rank and wherever the
				// variant does not let a pawn decline it; with nothing left to
				// promote to the pawn simply cannot enter the square.
				if !v.PromotionOptional || p.lastRank(to) {
					return
				}
				// Optional inside the zone: arriving as a pawn is a move too.
			}
			if tactical && take < 0 {
				return
			}
			moves = append(moves, Move{From: from, To: to, Take: take, Castle: -1})
		}
		components := t.movesAt(v, from, pc.Color)
		for i := range components {
			comp := &components[i]
			if comp.Initial && pc.Moved {
				continue
			}
			p.walk(from, comp, pc.Color, func(to int) bool {
				target := p.Board[to]
				if t.Pawn && v.PawnDoubleRanks != nil && comp.Initial && p.forwardDouble(from, to, pc.Color) {
					return true // configured double steps are generated separately
				}
				switch {
				case to == from && comp.Move:
					add(to, -1)
				case target.Type == 0 && comp.Capture && t.Pawn && p.enPassant(to):
					add(to, p.EPPawn)
				case target.Type == 0 && comp.Move:
					add(to, -1)
				case target.Type != 0 && target.Color != pc.Color && comp.Capture:
					add(to, to)
				case target.Type != 0 && target.Color == pc.Color && v.SelfCapture && !v.Type(target).Royal && comp.Capture:
					add(to, to)
				}
				return true
			})
		}
		if t.Pawn && v.PawnDoubleRanks != nil && slices.Contains(v.PawnDoubleRanks, p.relativeRank(from, pc.Color)) {
			x, y := v.XY(from)
			step := pc.Color.forward()
			if v.OnBoard(x, y+2*step) {
				middle, to := v.Square(x, y+step), v.Square(x, y+2*step)
				if p.Board[middle].Type == 0 && p.Board[to].Type == 0 {
					add(to, -1)
				}
			}
		}
	}
	if tactical {
		return moves
	}
	moves = append(moves, p.castles()...)
	return append(moves, p.dropMoves()...)
}

func (p *Position) relativeRank(sq int, c Color) int {
	_, y := p.V.XY(sq)
	if c == Black {
		return p.V.Ranks - y
	}
	return y + 1
}

func (p *Position) forwardDouble(from, to int, c Color) bool {
	fx, fy := p.V.XY(from)
	tx, ty := p.V.XY(to)
	return fx == tx && ty-fy == 2*c.forward()
}

func (p *Position) enPassant(sq int) bool {
	return p.EPPawn >= 0 && slices.Contains(p.EP, sq)
}

// lastRank reports whether sq is on the side to move's final rank, where a
// promoting pawn has no alternative.
func (p *Position) lastRank(sq int) bool {
	_, y := p.V.XY(sq)
	if p.Turn == White {
		return y == p.V.Ranks-1
	}
	return y == 0
}

func (p *Position) promotes(sq int) bool {
	_, y := p.V.XY(sq)
	if p.Turn == White {
		return y >= p.V.Ranks-p.V.PromotionZone
	}
	return y < p.V.PromotionZone
}

// castles returns the castling moves available to the side to move: the
// resolved entries still active, with an empty path between the four squares
// and no attack on any square the king passes over or lands on.
func (p *Position) castles() []Move {
	v := p.V
	var moves []Move
	for i, r := range p.castling[p.Turn] {
		if !r.active {
			continue
		}
		c := r.castle
		k, rook := p.Board[c.King], p.Board[c.Rook]
		if k.Type == 0 || rook.Type == 0 || k.Color != p.Turn || rook.Color != p.Turn || k.Moved || rook.Moved || !v.castlesAs(k, c.King) {
			continue
		}
		if !v.Type(k).allows(v, c.KingTo, p.Turn) || !v.Type(rook).allows(v, c.RookTo, p.Turn) {
			continue
		}
		lo := min(c.King, c.Rook, c.KingTo, c.RookTo)
		hi := max(c.King, c.Rook, c.KingTo, c.RookTo)
		clearPath := true
		for sq := lo; sq <= hi; sq++ {
			if sq != c.King && sq != c.Rook && p.Board[sq].Type != 0 {
				clearPath = false
				break
			}
		}
		if !clearPath {
			continue
		}
		var path []int
		for sq := min(c.King, c.KingTo); sq <= max(c.King, c.KingTo); sq++ {
			path = append(path, sq)
		}
		// A royal king may not castle out of, through or into attack; a fog
		// variant's king is no royal, so it castles regardless of attacks.
		if v.Type(k).Royal && p.castlePathAttacked(c.King, path) {
			continue
		}
		moves = append(moves, Move{From: c.King, To: c.KingTo, Take: -1, Castle: i})
	}
	return moves
}

// Apply returns the position after m; p is unchanged.
func (p *Position) Apply(m Move) *Position {
	v := p.V
	q := p.clone()
	if p.Turn == Black {
		q.Fullmove++
	}
	if m.Drop != 0 {
		q.applyDrop(m)
		return q
	}
	mover := p.Board[m.From]
	q.updateCastleRights(m)
	mover.Moved = true
	if m.Castle >= 0 {
		c := p.Castle(m.Castle, p.Turn)
		rook := p.Board[c.Rook]
		rook.Moved = true
		q.Board[m.From] = Piece{}
		q.Board[c.Rook] = Piece{}
		q.Board[m.To] = mover
		q.Board[c.RookTo] = rook
		q.Clock++
		q.Turn = p.Turn.Other()
		q.findRoyals()
		q.countCheck()
		return q
	}
	if m.Take >= 0 {
		if v.Drops {
			captured := p.Board[m.Take]
			typ := captured.Type
			if captured.Promoted != 0 {
				typ = captured.Promoted
			}
			if typ != 0 && int(typ) < len(q.Hands[p.Turn]) {
				q.Hands[p.Turn][typ]++
			}
		}
		q.Board[m.Take] = Piece{}
	}
	if m.Promote != 0 {
		if v.Drops && mover.Promoted == 0 {
			mover.Promoted = mover.Type
		}
		mover.Type = m.Promote
	}
	q.Board[m.From] = Piece{}
	q.Board[m.To] = mover
	if m.Take >= 0 && v.Explosion != nil {
		q.explode(m.To)
	}
	q.Clock++
	if pawn := v.Type(p.Board[m.From]).Pawn; pawn || m.Take >= 0 {
		q.Clock = 0
		if pawn && m.Take < 0 && !(p.forwardDouble(m.From, m.To, p.Turn) && slices.Contains(v.EPExemptRanks, p.relativeRank(m.From, p.Turn))) {
			q.EP = v.crossed(m.From, m.To)
			if len(q.EP) > 0 {
				q.EPPawn = m.To
			}
		}
	}
	q.Turn = p.Turn.Other()
	if (m.Take >= 0 && v.Explosion != nil) || v.Pieces[mover.Type-1].Royal || v.Type(p.Board[m.From]).Royal || (m.Take >= 0 && v.Type(p.Board[m.Take]).Royal) {
		q.findRoyals()
	}
	q.countCheck()
	return q
}

// countCheck records the check the side that just moved gave, which a variant
// with a CheckLimit wins on.
func (q *Position) countCheck() {
	if q.V.CheckLimit > 0 && q.InCheck() {
		q.Checks[q.Turn.Other()]++
	}
}

// crossed lists the squares strictly between from and to on a straight line.
func (v *Variant) crossed(from, to int) []int {
	fx, fy := v.XY(from)
	tx, ty := v.XY(to)
	dx, dy := tx-fx, ty-fy
	if dx != 0 && dy != 0 && abs(dx) != abs(dy) {
		return nil
	}
	n := max(abs(dx), abs(dy))
	var sqs []int
	for k := 1; k < n; k++ {
		sqs = append(sqs, v.Square(fx+k*sign(dx), fy+k*sign(dy)))
	}
	return sqs
}

// walk calls visit for each square comp can reach from `from`, empty or not;
// it is up to visit what may happen there. Paths stop at the first piece in
// the way, which is still visited. visit returns false to stop the walk.
func (p *Position) walk(from int, comp *Component, c Color, visit func(int) bool) {
	v := p.V
	fx, fy := v.XY(from)
	dir := c.forward()
	at := func(vec [2]int, k int) (int, bool) {
		x, y := fx+vec[0]*k*dir, fy+vec[1]*k*dir
		if !v.OnBoard(x, y) {
			return 0, false
		}
		return v.Square(x, y), true
	}
	occupied := func(sq int) bool { return p.Board[sq].Type != 0 }

	if comp.Circular {
		p.walkCircle(fx, fy, comp, dir, visit)
		return
	}
	for _, vec := range comp.Vectors {
		if comp.Range == 1 {
			to, ok := at(vec, 1)
			if !ok {
				continue
			}
			blockers := 0
			if comp.Lame || comp.Hop != hopNone {
				for _, step := range lamePath(vec) {
					if sq, _ := at(step, 1); occupied(sq) {
						blockers++
					}
				}
			}
			if (comp.Hop == hopNone && blockers > 0) || (comp.Hop != hopNone && blockers != 1) {
				continue
			}
			if !visit(to) {
				return
			}
			continue
		}
		hopped := comp.Hop == hopNone
		for k := 1; comp.Range == 0 || k <= comp.Range; k++ {
			to, ok := at(vec, k)
			if !ok {
				break
			}
			if !hopped {
				if occupied(to) {
					hopped = true
					if comp.Hop == hopGrasshopper {
						if next, ok := at(vec, k+1); ok && !visit(next) {
							return
						}
						break
					}
				}
				continue
			}
			if !visit(to) {
				return
			}
			if occupied(to) {
				break
			}
		}
	}
}

// walkCircle follows a circular rider around its ring both ways from each
// allowed first leg. A whole open circle brings it back to its own square,
// which it visits last: a pass.
func (p *Position) walkCircle(fx, fy int, comp *Component, dir int, visit func(int) bool) {
	v := p.V
	n := len(comp.Ring)
	for _, first := range comp.Vectors {
		start := slices.Index(comp.Ring, first)
		for _, turn := range []int{1, -1} {
			x, y := fx, fy
			for k := range n {
				leg := comp.Ring[((start+turn*k)%n+n)%n]
				x, y = x+leg[0]*dir, y+leg[1]*dir
				if !v.OnBoard(x, y) {
					break
				}
				sq := v.Square(x, y)
				if !visit(sq) {
					return
				}
				if p.Board[sq].Type != 0 {
					break
				}
			}
		}
	}
}

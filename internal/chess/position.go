package chess

import (
	"fmt"
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
	Type  uint8 // 0 for an empty square, else Variant.Pieces[Type-1]
	Color Color
	Moved bool
}

type Move struct {
	From, To int
	Take     int   // square of the captured piece (differs from To en passant), or -1
	Promote  uint8 // piece type the pawn becomes, or 0
	Castle   int   // index into Variant.Castling, or -1
}

type Position struct {
	V      *Variant
	Board  []Piece
	Turn   Color
	EP     []int  // squares the last move's pawn crossed
	EPPawn int    // that pawn, or -1
	Clock  int    // plies since the last capture or pawn move
	Checks [2]int // checks each side has given, for a variant with a CheckLimit

	royalSq [2][]int // kept by Apply so check tests need not scan the board
}

func NewPosition(v *Variant) *Position {
	p := &Position{V: v, Board: slices.Clone(v.start), EPPawn: -1}
	p.findRoyals()
	return p
}

// ParseFEN reads "placement [w|b] [castling] [checks]", where a variant with a
// CheckLimit also takes a checks field anywhere after the side to move: "+2+1"
// counts the checks White and Black have already given, and Lichess's "1+2"
// gives the checks each side has left. Pieces on their setup squares count as
// unmoved. The castling field, when given, takes rights away: "-" all of
// them, otherwise each castle whose letter (K when the rook is on the king's
// right, Q on its left; lower case for Black) is missing.
func ParseFEN(v *Variant, fen string) (*Position, error) {
	fields := strings.Fields(fen)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty position")
	}
	board, err := v.parsePlacement(fields[0])
	if err != nil {
		return nil, err
	}
	p := &Position{V: v, Board: board, EPPawn: -1}
	for sq, pc := range board {
		start := v.start[sq]
		board[sq].Moved = pc.Type != 0 && (start.Type != pc.Type || start.Color != pc.Color)
	}
	if len(fields) > 1 {
		switch fields[1] {
		case "w":
		case "b":
			p.Turn = Black
		default:
			return nil, fmt.Errorf("side to move %q is not w or b", fields[1])
		}
	}
	if len(fields) > 2 {
		for _, color := range []Color{White, Black} {
			for _, c := range v.Castling {
				letter := "Q"
				if c.Rook%v.Files > c.King%v.Files {
					letter = "K"
				}
				if color == Black {
					letter = strings.ToLower(letter)
				}
				if !strings.Contains(fields[2], letter) {
					board[v.orient(c.Rook, color)].Moved = true
				}
			}
		}
	}
	if len(fields) > 3 {
		if err := p.parseChecks(fields[3:]); err != nil {
			return nil, err
		}
	}
	p.findRoyals()
	return p, nil
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
	q.EP = nil
	q.EPPawn = -1
	return &q
}

// Moves returns the legal moves for the side to move.
func (p *Position) Moves() []Move {
	var legal []Move
	for _, m := range p.pseudoMoves() {
		if p.Apply(m).legalAfter(p.Turn) {
			legal = append(legal, m)
		}
	}
	return legal
}

// legalAfter reports whether a move by mover is legal once made: no royal of
// the mover may be left attacked and, where the variant forbids checking, no
// royal of the other side may be attacked either.
func (p *Position) legalAfter(mover Color) bool {
	if p.royalAttacked(mover) {
		return false
	}
	return !p.V.NoChecks || !p.royalAttacked(mover.Other())
}

// InCheck reports whether any of the side to move's royal pieces is attacked.
func (p *Position) InCheck() bool { return p.royalAttacked(p.Turn) }

func (p *Position) royalAttacked(c Color) bool {
	return p.anyAttacked(p.royalSq[c], c.Other())
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

// captures lists the moves that take something or promote, for the
// quiescence search.
func (p *Position) captures() []Move { return p.generate(true) }

func (p *Position) generate(tactical bool) []Move {
	v := p.V
	var moves []Move
	seen := make([]bool, v.Size())
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
			if seen[to] {
				return
			}
			seen[to] = true
			if t.Pawn && to != from && p.promotes(to) && len(v.Promotion) > 0 {
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
		for i := range t.moves {
			comp := &t.moves[i]
			if comp.Initial && pc.Moved {
				continue
			}
			p.walk(from, comp, pc.Color, func(to int) bool {
				target := p.Board[to]
				switch {
				case to == from && comp.Move:
					add(to, -1)
				case target.Type == 0 && comp.Move:
					add(to, -1)
				case target.Type == 0 && comp.Capture && t.Pawn && p.enPassant(to):
					add(to, p.EPPawn)
				case target.Type != 0 && target.Color != pc.Color && comp.Capture:
					add(to, to)
				case target.Type != 0 && target.Color == pc.Color && v.SelfCapture && !v.Type(target).Royal && comp.Capture:
					add(to, to)
				}
				return true
			})
		}
	}
	if tactical {
		return moves
	}
	return append(moves, p.castles()...)
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

func (p *Position) castles() []Move {
	v := p.V
	var moves []Move
	for i, c := range v.Castling {
		king, rook := v.orient(c.King, p.Turn), v.orient(c.Rook, p.Turn)
		kingTo, rookTo := v.orient(c.KingTo, p.Turn), v.orient(c.RookTo, p.Turn)
		k, r := p.Board[king], p.Board[rook]
		if k.Type == 0 || r.Type == 0 || k.Color != p.Turn || r.Color != p.Turn || k.Moved || r.Moved || !v.Type(k).Royal {
			continue
		}
		lo := min(king, rook, kingTo, rookTo)
		hi := max(king, rook, kingTo, rookTo)
		clearPath := true
		for sq := lo; sq <= hi; sq++ {
			if sq != king && sq != rook && p.Board[sq].Type != 0 {
				clearPath = false
				break
			}
		}
		if !clearPath {
			continue
		}
		var path []int
		for sq := min(king, kingTo); sq <= max(king, kingTo); sq++ {
			path = append(path, sq)
		}
		if p.anyAttacked(path, p.Turn.Other()) {
			continue
		}
		moves = append(moves, Move{From: king, To: kingTo, Take: -1, Castle: i})
	}
	return moves
}

// Apply returns the position after m; p is unchanged.
func (p *Position) Apply(m Move) *Position {
	v := p.V
	q := p.clone()
	mover := p.Board[m.From]
	mover.Moved = true
	if m.Castle >= 0 {
		c := v.Castling[m.Castle]
		rook := p.Board[v.orient(c.Rook, p.Turn)]
		rook.Moved = true
		q.Board[m.From] = Piece{}
		q.Board[v.orient(c.Rook, p.Turn)] = Piece{}
		q.Board[m.To] = mover
		q.Board[v.orient(c.RookTo, p.Turn)] = rook
		q.Clock++
		q.Turn = p.Turn.Other()
		q.findRoyals()
		q.countCheck()
		return q
	}
	if m.Take >= 0 {
		q.Board[m.Take] = Piece{}
	}
	if m.Promote != 0 {
		mover.Type = m.Promote
	}
	q.Board[m.From] = Piece{}
	q.Board[m.To] = mover
	q.Clock++
	if pawn := v.Type(p.Board[m.From]).Pawn; pawn || m.Take >= 0 {
		q.Clock = 0
		if pawn && m.Take < 0 {
			q.EP = v.crossed(m.From, m.To)
			if len(q.EP) > 0 {
				q.EPPawn = m.To
			}
		}
	}
	q.Turn = p.Turn.Other()
	if v.Pieces[mover.Type-1].Royal || v.Type(p.Board[m.From]).Royal || (m.Take >= 0 && v.Type(p.Board[m.Take]).Royal) {
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

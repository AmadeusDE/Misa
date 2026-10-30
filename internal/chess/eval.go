package chess

// The evaluation knows nothing about particular pieces: material comes from
// each type's value, and the positional terms only use what every variant
// has (ranks towards promotion, the centre, royal pieces).

const (
	tempo        = 10
	centreLeaper = 24 // most a short-range piece gains from the centre
	centreRider  = 8
	royalShelter = 12 // per own pawn in front of a royal piece, middlegame
	royalHome    = 12 // per rank a royal piece strays from home, middlegame
	royalCentre  = 30 // royal centralisation, endgame
	royalEdge    = 30 // royal piece on an edge file, middlegame: where castling puts it
	pawnRace     = 90 // a pawn one step from promotion, endgame
	pawnRaceMid  = 35 // the same for a centre-file pawn in the middlegame; flank pawns get none
	undeveloped  = 15 // a light piece still on its home rank, middlegame
	lightPiece   = 600
)

// estimateValues prices pieces the variant file leaves without a value from
// their average number of moves on an empty board. The line through a
// knight (≈300) and a queen (≈900) on 8×8 is reused everywhere; moves that
// only move or only capture, or need a hurdle, count half.
func (v *Variant) estimateValues() {
	for _, t := range v.Pieces {
		switch {
		case t.Value != 0:
		case t.Royal:
		case t.Pawn:
			t.Value = 100
		default:
			t.Value = 120 + int(34*v.mobility(t)+0.5)
		}
	}
}

func (v *Variant) mobility(t *PieceType) float64 {
	p := &Position{V: v, Board: make([]Piece, v.Size()), EPPawn: -1}
	weight := make([]float64, v.Size())
	total := 0.0
	for from := range p.Board {
		clear(weight)
		p.Board[from] = Piece{Type: 1, Color: White, Moved: true}
		for _, comp := range t.moves {
			if comp.Initial {
				continue
			}
			w := 1.0
			if !comp.Move || !comp.Capture || comp.Hop != hopNone {
				w = 0.5
			}
			comp.Hop = hopNone
			p.walk(from, &comp, White, func(to int) bool {
				if to != from {
					weight[to] = max(weight[to], w)
				}
				return true
			})
		}
		p.Board[from] = Piece{}
		for _, w := range weight {
			total += w
		}
	}
	return total / float64(v.Size())
}

// evaluator holds the per-variant tables the evaluation reads.
type evaluator struct {
	v          *Variant
	centre     []int // 0 on the edge to 100 in the middle
	centreFile []int // the same across files only
	leaper     []bool
	startPower int // both sides' non-pawn, non-royal material at the start
}

func newEvaluator(v *Variant) *evaluator {
	e := &evaluator{v: v, centre: make([]int, v.Size()), centreFile: make([]int, v.Files), leaper: make([]bool, len(v.Pieces)+1)}
	hx, hy := float64(v.Files-1)/2, float64(v.Ranks-1)/2
	for x := range e.centreFile {
		e.centreFile[x] = 100
		if hx > 0 {
			e.centreFile[x] = int(100 * (1 - absf(float64(x)-hx)/hx))
		}
	}
	for sq := range e.centre {
		x, y := v.XY(sq)
		dx, dy := 0.0, 0.0
		if hx > 0 {
			dx = absf(float64(x)-hx) / hx
		}
		if hy > 0 {
			dy = absf(float64(y)-hy) / hy
		}
		e.centre[sq] = int(100 * (1 - (dx+dy)/2))
	}
	for i, t := range v.Pieces {
		e.leaper[i+1] = true
		for _, c := range t.moves {
			if c.Range != 1 {
				e.leaper[i+1] = false
			}
		}
	}
	e.startPower = max(1, e.power(v.start))
	return e
}

func (e *evaluator) power(board []Piece) int {
	n := 0
	for _, pc := range board {
		if pc.Type != 0 {
			if t := e.v.Pieces[pc.Type-1]; !t.Royal && !t.Pawn {
				n += t.Value
			}
		}
	}
	return n
}

// evaluate scores p in centipawns for the side to move.
func (e *evaluator) evaluate(p *Position) int {
	v := e.v
	// phase runs from 256 with all pieces on the board to 0 with none.
	phase := min(256, 256*e.power(p.Board)/e.startPower)
	score := [2]int{}
	for sq, pc := range p.Board {
		if pc.Type == 0 {
			continue
		}
		t := v.Pieces[pc.Type-1]
		x, y := v.XY(sq)
		rank := y
		if pc.Color == Black {
			rank = v.Ranks - 1 - y
		}
		var s int
		switch {
		case t.Royal:
			mid := -royalHome*rank + royalShelter*e.shelter(p, x, y, pc.Color) + royalEdge*(100-e.centreFile[x])/100
			end := royalCentre * e.centre[sq] / 100
			s = (mid*phase + end*(256-phase)) / 256
		case t.Pawn:
			s = t.Value + e.pawnAdvance(rank, e.centreFile[x], phase)
		default:
			weight := centreRider
			if e.leaper[pc.Type] {
				weight = centreLeaper
			}
			s = t.Value + weight*e.centre[sq]/100
			if rank == 0 && t.Value < lightPiece && !t.tracksMoved {
				s -= undeveloped * phase / 256
			}
		}
		score[pc.Color] += s
	}
	for c, hand := range p.Hands {
		for typ, count := range hand {
			if typ > 0 && count > 0 {
				score[c] += count * pocketValue(v.Pieces[typ-1].Value)
			}
		}
	}
	return score[p.Turn] - score[p.Turn.Other()] + tempo
}

// pocketValue discounts material in hand until it is deployed.
func pocketValue(value int) int { return value * 80 / 100 }

// pawnAdvance grows with the square of the distance covered towards the
// promotion zone. In the middlegame only centre pawns gain by it, so flank
// pawns stay home to shelter the king; as pieces come off every pawn races.
func (e *evaluator) pawnAdvance(rank, centreFile, phase int) int {
	goal := e.v.Ranks - e.v.PromotionZone - 1
	if goal <= 1 {
		return 0
	}
	f := float64(max(0, rank-1)) / float64(goal-1)
	mid := pawnRaceMid * centreFile / 100
	race := (mid*phase + pawnRace*(256-phase)) / 256
	return int(f * f * float64(race))
}

// shelter counts own pawns on the three squares in front of a royal piece.
func (e *evaluator) shelter(p *Position, x, y int, c Color) int {
	v := e.v
	n := 0
	for dx := -1; dx <= 1; dx++ {
		fx, fy := x+dx, y+c.forward()
		if !v.OnBoard(fx, fy) {
			continue
		}
		if pc := p.Board[v.Square(fx, fy)]; pc.Type != 0 && pc.Color == c && v.Pieces[pc.Type-1].Pawn {
			n++
		}
	}
	return n
}

func absf(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

package chess

import (
	"fmt"
	"sort"
)

// FourSide is one player, in turn order. Red and Yellow are partners, as are
// Blue and Green.
type FourSide uint8

const (
	FourRed FourSide = iota
	FourBlue
	FourYellow
	FourGreen
)

type FourTeam uint8

const (
	FourNoTeam FourTeam = iota
	FourRedYellow
	FourBlueGreen
)

func (s FourSide) Team() FourTeam {
	if s == FourRed || s == FourYellow {
		return FourRedYellow
	}
	return FourBlueGreen
}
func (s FourSide) String() string {
	switch s {
	case FourRed:
		return "Red"
	case FourBlue:
		return "Blue"
	case FourYellow:
		return "Yellow"
	case FourGreen:
		return "Green"
	}
	return "Unknown"
}

type FourPieceType uint8

const (
	FourEmpty FourPieceType = iota
	FourHole
	FourPawn
	FourKnight
	FourBishop
	FourRook
	FourQueen
	FourKing
)

type FourPiece struct {
	Type  FourPieceType
	Side  FourSide
	Moved bool
}
type FourSquare struct{ Row, Col int }
type FourMove struct{ From, To FourSquare }

type fourSnapshot struct {
	board    [14][14]FourPiece
	turn     FourSide
	winner   FourTeam
	finished bool
}

// FourMatch is the hard-coded four-player Teams game. Board[Row][Col] has
// Row 0 at the top (Yellow's back rank), Row 13 at Red's back rank, and Col 0
// at Blue's side. The 3x3 corners are FourHole. Promotion is to queen on
// the eighth rank from the player's back rank. En passant is not supported.
// A checkmated player loses for their team; stalemate is a draw (FourNoTeam).
type FourMatch struct {
	Board    [14][14]FourPiece
	Turn     FourSide
	Winner   FourTeam
	Finished bool
	history  []fourSnapshot
}

func NewFourMatch() *FourMatch {
	m := &FourMatch{}
	for r := 0; r < 14; r++ {
		for c := 0; c < 14; c++ {
			if (r < 3 || r >= 11) && (c < 3 || c >= 11) {
				m.Board[r][c].Type = FourHole
			}
		}
	}
	back := [...]FourPieceType{FourRook, FourKnight, FourBishop, FourQueen, FourKing, FourBishop, FourKnight, FourRook}
	for side := FourRed; side <= FourGreen; side++ {
		for i, t := range back {
			r, c := 13, 3+i
			for n := FourRed; n < side; n++ {
				r, c = c, 13-r
			}
			m.Board[r][c] = FourPiece{Type: t, Side: side}
			dr, dc := fourForward(side)
			m.Board[r+dr][c+dc] = FourPiece{Type: FourPawn, Side: side}
		}
	}
	return m
}

// Clone copies the match and its undo history, for example for asynchronous AI.
func (m *FourMatch) Clone() *FourMatch {
	n := *m
	n.history = append([]fourSnapshot(nil), m.history...)
	return &n
}
func fourForward(s FourSide) (int, int) {
	switch s {
	case FourRed:
		return -1, 0
	case FourBlue:
		return 0, 1
	case FourYellow:
		return 1, 0
	default:
		return 0, -1
	}
}
func fourValid(r, c int) bool {
	return r >= 0 && r < 14 && c >= 0 && c < 14 && !((r < 3 || r >= 11) && (c < 3 || c >= 11))
}
func (m *FourMatch) at(r, c int) FourPiece {
	if !fourValid(r, c) {
		return FourPiece{Type: FourHole}
	}
	return m.Board[r][c]
}

var fourStraight = [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
var fourDiag = [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}}
var fourKnight = [][2]int{{-2, -1}, {-2, 1}, {-1, -2}, {-1, 2}, {1, -2}, {1, 2}, {2, -1}, {2, 1}}

// fourAttacks reports geometric attacks, independently of pins. This is
// also used for king movement and castling (pinned pieces still attack).
func (m *FourMatch) fourAttacks(from, to FourSquare) bool {
	p := m.at(from.Row, from.Col)
	dr, dc := to.Row-from.Row, to.Col-from.Col
	if !fourValid(to.Row, to.Col) || (dr == 0 && dc == 0) {
		return false
	}
	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	switch p.Type {
	case FourPawn:
		fr, fc := fourForward(p.Side)
		return (fr != 0 && dr == fr && abs(dc) == 1) || (fc != 0 && dc == fc && abs(dr) == 1)
	case FourKnight:
		return (abs(dr) == 2 && abs(dc) == 1) || (abs(dr) == 1 && abs(dc) == 2)
	case FourKing:
		return abs(dr) <= 1 && abs(dc) <= 1
	case FourBishop, FourRook, FourQueen:
		straight := dr == 0 || dc == 0
		diagonal := abs(dr) == abs(dc)
		if !(p.Type == FourQueen && (straight || diagonal) || p.Type == FourRook && straight || p.Type == FourBishop && diagonal) {
			return false
		}
		sr, sc := 0, 0
		if dr < 0 {
			sr = -1
		} else if dr > 0 {
			sr = 1
		}
		if dc < 0 {
			sc = -1
		} else if dc > 0 {
			sc = 1
		}
		for r, c := from.Row+sr, from.Col+sc; r != to.Row || c != to.Col; r, c = r+sr, c+sc {
			if m.at(r, c).Type != FourEmpty {
				return false
			}
		}
		return true
	}
	return false
}
func (m *FourMatch) attacked(s FourSquare, side FourSide) bool {
	for r := 0; r < 14; r++ {
		for c := 0; c < 14; c++ {
			p := m.Board[r][c]
			if p.Type >= FourPawn && p.Side.Team() != side.Team() && m.fourAttacks(FourSquare{r, c}, s) {
				return true
			}
		}
	}
	return false
}

// InCheck reports whether either opposing player attacks side's king.
func (m *FourMatch) InCheck(side FourSide) bool {
	for r := 0; r < 14; r++ {
		for c := 0; c < 14; c++ {
			p := m.Board[r][c]
			if p.Type == FourKing && p.Side == side {
				return m.attacked(FourSquare{r, c}, side)
			}
		}
	}
	return false
}

func (m *FourMatch) pseudo(side FourSide) []FourMove {
	var moves []FourMove
	for r := 0; r < 14; r++ {
		for c := 0; c < 14; c++ {
			p := m.Board[r][c]
			if p.Type < FourPawn || p.Side != side {
				continue
			}
			from := FourSquare{r, c}
			add := func(rr, cc int) bool {
				q := m.at(rr, cc)
				if q.Type == FourHole {
					return false
				}
				if q.Type == FourEmpty {
					moves = append(moves, FourMove{from, FourSquare{rr, cc}})
					return true
				}
				if q.Side.Team() != side.Team() && q.Type != FourKing {
					moves = append(moves, FourMove{from, FourSquare{rr, cc}})
				}
				return false
			}
			switch p.Type {
			case FourPawn:
				dr, dc := fourForward(side)
				if m.at(r+dr, c+dc).Type == FourEmpty {
					add(r+dr, c+dc)
					if !p.Moved && m.at(r+2*dr, c+2*dc).Type == FourEmpty {
						add(r+2*dr, c+2*dc)
					}
				}
				for _, k := range []int{-1, 1} {
					rr, cc := r+dr+k*dc, c+dc+k*dr
					q := m.at(rr, cc)
					if q.Type >= FourPawn && q.Type != FourKing && q.Side.Team() != side.Team() {
						add(rr, cc)
					}
				}
			case FourKnight:
				for _, d := range fourKnight {
					add(r+d[0], c+d[1])
				}
			case FourKing:
				for dr := -1; dr <= 1; dr++ {
					for dc := -1; dc <= 1; dc++ {
						if dr != 0 || dc != 0 {
							add(r+dr, c+dc)
						}
					}
				}
				if !p.Moved && !m.InCheck(side) {
					fr, fc := fourForward(side)
					for _, sign := range []int{-1, 1} {
						dr, dc := fc*sign, fr*sign
						rr, cc := r+dr, c+dc
						for m.at(rr, cc).Type == FourEmpty {
							rr += dr
							cc += dc
						}
						rook := m.at(rr, cc)
						if rook.Type != FourRook || rook.Side != side || rook.Moved {
							continue
						}
						if absFour(rr-r)+absFour(cc-c) < 3 {
							continue
						}
						mid := FourSquare{r + dr, c + dc}
						dest := FourSquare{r + 2*dr, c + 2*dc}
						if m.at(dest.Row, dest.Col).Type != FourEmpty {
							continue
						}
						tmp := *m
						tmp.Board[r][c] = FourPiece{}
						tmp.Board[mid.Row][mid.Col] = p
						if !tmp.attacked(mid, side) {
							moves = append(moves, FourMove{from, dest})
						}
					}
				}
			case FourBishop, FourRook, FourQueen:
				dirs := fourStraight
				if p.Type == FourBishop {
					dirs = fourDiag
				} else if p.Type == FourQueen {
					dirs = append(append([][2]int{}, fourStraight...), fourDiag...)
				}
				for _, d := range dirs {
					for rr, cc := r+d[0], c+d[1]; add(rr, cc); rr, cc = rr+d[0], cc+d[1] {
					}
				}
			}
		}
	}
	return moves
}
func absFour(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func (m *FourMatch) makeMove(mv FourMove) {
	p := m.Board[mv.From.Row][mv.From.Col]
	m.Board[mv.From.Row][mv.From.Col] = FourPiece{}
	dr, dc := mv.To.Row-mv.From.Row, mv.To.Col-mv.From.Col
	if p.Type == FourKing && (absFour(dr) == 2 || absFour(dc) == 2) {
		sr, sc := dr/2, dc/2
		r, c := mv.To.Row+sr, mv.To.Col+sc
		for m.at(r, c).Type == FourEmpty {
			r += sr
			c += sc
		}
		rook := m.Board[r][c]
		m.Board[r][c] = FourPiece{}
		rook.Moved = true
		m.Board[mv.From.Row+sr][mv.From.Col+sc] = rook
	}
	p.Moved = true
	if p.Type == FourPawn {
		switch p.Side {
		case FourRed:
			if mv.To.Row == 6 {
				p.Type = FourQueen
			}
		case FourBlue:
			if mv.To.Col == 7 {
				p.Type = FourQueen
			}
		case FourYellow:
			if mv.To.Row == 7 {
				p.Type = FourQueen
			}
		case FourGreen:
			if mv.To.Col == 6 {
				p.Type = FourQueen
			}
		}
	}
	m.Board[mv.To.Row][mv.To.Col] = p
}

// LegalMoves returns the current player's legal moves. Partners' pieces cannot
// be captured, and only the moving player's king must be safe.
func (m *FourMatch) LegalMoves() []FourMove {
	if m.Finished {
		return nil
	}
	var legal []FourMove
	for _, mv := range m.pseudo(m.Turn) {
		tmp := *m
		tmp.makeMove(mv)
		if !tmp.InCheck(m.Turn) {
			legal = append(legal, mv)
		}
	}
	return legal
}
func (m *FourMatch) Apply(mv FourMove) error {
	found := false
	for _, x := range m.LegalMoves() {
		if x == mv {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("illegal four-player move: %v", mv)
	}
	m.history = append(m.history, fourSnapshot{m.Board, m.Turn, m.Winner, m.Finished})
	m.makeMove(mv)
	m.Turn = (m.Turn + 1) % 4
	if len(m.LegalMoves()) == 0 {
		m.Finished = true
		if m.InCheck(m.Turn) {
			if m.Turn.Team() == FourRedYellow {
				m.Winner = FourBlueGreen
			} else {
				m.Winner = FourRedYellow
			}
		}
	}
	return nil
}
func (m *FourMatch) Undo() bool {
	if len(m.history) == 0 {
		return false
	}
	s := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	m.Board, m.Turn, m.Winner, m.Finished = s.board, s.turn, s.winner, s.finished
	return true
}

// ComputerMove searches up to four plies, with both partners maximizing the
// same material score. Only completed iterations are used when the node budget
// runs out, so a partially searched root cannot bias the choice.
func (m *FourMatch) ComputerMove() (FourMove, bool) {
	moves := m.LegalMoves()
	if len(moves) == 0 {
		return FourMove{}, false
	}
	fourOrderMoves(m, moves)
	best := moves[0]
	nodes := 0
	for depth := 1; depth <= 4; depth++ {
		iterationBest, bestScore := best, -fourInfinity
		alpha := -fourInfinity
		complete := true
		for _, mv := range moves {
			child := *m
			child.makeMove(mv)
			child.Turn = (child.Turn + 1) % 4
			score, ok := fourSearch(&child, depth-1, alpha, fourInfinity, m.Turn.Team(), &nodes)
			if !ok {
				complete = false
				break
			}
			if score > bestScore {
				iterationBest, bestScore = mv, score
			}
			if score > alpha {
				alpha = score
			}
		}
		if !complete {
			break
		}
		best = iterationBest
		// Search the previous principal move first in the next iteration.
		for i, mv := range moves {
			if mv == best {
				moves[0], moves[i] = moves[i], moves[0]
				break
			}
		}
	}
	return best, true
}

const fourInfinity = 1000000
const fourNodeLimit = 1500

var fourValues = [...]int{0, 0, 100, 300, 300, 500, 900, 0}

func fourOrderMoves(m *FourMatch, moves []FourMove) {
	sort.SliceStable(moves, func(i, j int) bool {
		return fourValues[m.Board[moves[i].To.Row][moves[i].To.Col].Type] >
			fourValues[m.Board[moves[j].To.Row][moves[j].To.Col].Type]
	})
}

func fourSearch(m *FourMatch, depth, alpha, beta int, team FourTeam, nodes *int) (int, bool) {
	if *nodes >= fourNodeLimit {
		return 0, false
	}
	*nodes++
	moves := m.LegalMoves()
	if len(moves) == 0 {
		if !m.InCheck(m.Turn) {
			return 0, true
		}
		if m.Turn.Team() == team {
			return -fourInfinity + 10 - depth, true
		}
		return fourInfinity - 10 + depth, true
	}
	if depth == 0 {
		score := 0
		for _, row := range m.Board {
			for _, p := range row {
				if p.Type < FourPawn {
					continue
				}
				if p.Side.Team() == team {
					score += fourValues[p.Type]
				} else {
					score -= fourValues[p.Type]
				}
			}
		}
		return score, true
	}
	fourOrderMoves(m, moves)
	maximizing := m.Turn.Team() == team
	best := fourInfinity
	if maximizing {
		best = -fourInfinity
	}
	for _, mv := range moves {
		child := *m
		child.makeMove(mv)
		child.Turn = (child.Turn + 1) % 4
		score, ok := fourSearch(&child, depth-1, alpha, beta, team, nodes)
		if !ok {
			return 0, false
		}
		if maximizing {
			if score > best {
				best = score
			}
			if best > alpha {
				alpha = best
			}
		} else {
			if score < best {
				best = score
			}
			if best < beta {
				beta = best
			}
		}
		if alpha >= beta {
			break
		}
	}
	return best, true
}

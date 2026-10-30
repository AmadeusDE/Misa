package chess

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseFEN reads "placement [w|b] [castling]". Pieces on their setup squares
// count as unmoved. The castling field, when given, takes rights away: "-"
// all of them, otherwise each castle whose letter (K when the rook is on the
// king's right, Q on its left; lower case for Black) is missing.
func ParseFEN(v *Variant, fen string) (*Position, error) {
	fields := strings.Fields(fen)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty position")
	}
	placement, hands, err := v.splitFENHands(fields[0])
	if err != nil {
		return nil, err
	}
	board, err := v.parsePlacement(placement)
	if err != nil {
		return nil, err
	}
	p := &Position{V: v, Board: board, Hands: hands, EPPawn: -1, Fullmove: 1}
	for sq, pc := range board {
		start := v.start[sq]
		board[sq].Moved = pc.Type != 0 && (start.Type != pc.Type || start.Color != pc.Color || pc.Promoted != 0)
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
	if len(fields) > 3 && fields[3] != "-" {
		sq, err := v.ParseSquare(fields[3])
		if err != nil {
			return nil, fmt.Errorf("en passant: %v", err)
		}
		x, y := v.XY(sq)
		y -= p.Turn.forward()
		if !v.OnBoard(x, y) {
			return nil, fmt.Errorf("invalid en passant square")
		}
		pawn := v.Square(x, y)
		if board[pawn].Type == 0 || board[pawn].Color != p.Turn.Other() || !v.Type(board[pawn]).Pawn {
			return nil, fmt.Errorf("en passant pawn missing")
		}
		p.EP = []int{sq}
		p.EPPawn = pawn
	}
	if len(fields) > 4 {
		n, err := strconv.Atoi(fields[4])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid halfmove clock")
		}
		p.Clock = n
	}
	if len(fields) > 5 {
		n, err := strconv.Atoi(fields[5])
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid fullmove number")
		}
		p.Fullmove = n
	}
	p.findRoyals()
	return p, nil
}

// splitFENHands separates optional Crazyhouse pockets from placement.
func (v *Variant) splitFENHands(s string) (string, [2][]int, error) {
	var hands [2][]int
	for i := range hands {
		hands[i] = make([]int, len(v.Pieces)+1)
	}
	idx := strings.IndexByte(s, '[')
	if idx < 0 {
		if strings.ContainsAny(s, "]") {
			return "", hands, fmt.Errorf("invalid pocket")
		}
		return s, hands, nil
	}
	if !v.Drops || !strings.HasSuffix(s, "]") {
		return "", hands, fmt.Errorf("invalid pocket")
	}
	for _, c := range []byte(s[idx+1 : len(s)-1]) {
		t := v.TypeOf(c)
		if !isLetter(c) || t == 0 || v.Pieces[t-1].Royal {
			return "", hands, fmt.Errorf("invalid pocket piece %q", c)
		}
		color := White
		if c >= 'a' {
			color = Black
		}
		hands[color][t]++
	}
	return s[:idx], hands, nil
}

// FEN returns a conventional FEN, with pockets and promotion marks for drop variants.
func (p *Position) FEN() string {
	v := p.V
	var b strings.Builder
	for y := v.Ranks - 1; y >= 0; y-- {
		if y != v.Ranks-1 {
			b.WriteByte('/')
		}
		empty := 0
		for x := 0; x < v.Files; x++ {
			pc := p.Board[v.Square(x, y)]
			if pc.Type == 0 {
				empty++
				continue
			}
			if empty > 0 {
				b.WriteString(strconv.Itoa(empty))
				empty = 0
			}
			c := v.Pieces[pc.Type-1].Letter
			if pc.Color == Black {
				c += 'a' - 'A'
			}
			b.WriteByte(c)
			if v.Drops && pc.Promoted != 0 {
				b.WriteByte('~')
			}
		}
		if empty > 0 {
			b.WriteString(strconv.Itoa(empty))
		}
	}
	if v.Drops {
		b.WriteByte('[')
		for _, color := range []Color{White, Black} {
			for i, t := range v.Pieces {
				c := t.Letter
				if color == Black {
					c += 'a' - 'A'
				}
				if i+1 < len(p.Hands[color]) {
					for n := p.Hands[color][i+1]; n > 0; n-- {
						b.WriteByte(c)
					}
				}
			}
		}
		b.WriteByte(']')
	}
	if p.Turn == White {
		b.WriteString(" w ")
	} else {
		b.WriteString(" b ")
	}
	rights := ""
	for _, color := range []Color{White, Black} {
		for _, c := range v.Castling {
			k, r := v.orient(c.King, color), v.orient(c.Rook, color)
			king, rook := p.Board[k], p.Board[r]
			if king.Type == 0 || rook.Type == 0 || king.Color != color || rook.Color != color || king.Moved || rook.Moved || !v.Type(king).Royal || rook.Type != v.start[r].Type {
				continue
			}
			letter := "Q"
			if c.Rook%v.Files > c.King%v.Files {
				letter = "K"
			}
			if color == Black {
				letter = strings.ToLower(letter)
			}
			if !strings.Contains(rights, letter) {
				rights += letter
			}
		}
	}
	if rights == "" {
		rights = "-"
	}
	b.WriteString(rights)
	b.WriteByte(' ')
	// Standard FEN has room for only one of a fairy pawn's crossed squares.
	if p.EPPawn >= 0 && len(p.EP) > 0 {
		b.WriteString(v.SquareName(p.EP[0]))
	} else {
		b.WriteByte('-')
	}
	fullmove := p.Fullmove
	if fullmove < 1 {
		fullmove = 1
	}
	fmt.Fprintf(&b, " %d %d", p.Clock, fullmove)
	return b.String()
}

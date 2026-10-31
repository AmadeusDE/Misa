package chess

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ParseFEN reads "placement [w|b] [castling] [ep] [halfmove] [fullmove]", plus
// the optional fields other variants add: a drop variant's bracketed pockets
// after the placement, and - for a variant with a CheckLimit - a checks field
// anywhere after the side to move, where "+2+1" counts the checks White and
// Black have already given and Lichess's "1+2" gives the checks each side has
// left. Pieces on their setup squares count as unmoved. Explicit castling
// rights select eligible origins: K/Q for the right/left rook, or a
// Shredder-FEN file letter for a dynamic entry (lower case for Black); when the
// field is given it takes rights away: "-" all of them, otherwise each castle
// whose letter is missing.
func ParseFEN(v *Variant, fen string) (*Position, error) {
	fields := strings.Fields(fen)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty position")
	}
	placement, hands, err := v.splitFENHands(fields[0])
	if err != nil {
		return nil, err
	}
	board, blocker, err := v.parsePlacementBlocker(placement)
	if err != nil {
		return nil, err
	}
	p := &Position{V: v, Board: board, Hands: hands, Blocker: blocker, EPPawn: -1, Fullmove: 1}
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
	p.findRoyals()
	rights := ""
	if len(fields) > 2 {
		rights = fields[2]
	}
	p.resolveCastles(rights)
	if len(fields) > 3 && fields[3] != "-" {
		// FEN records one crossed square; infer the pawn from the side to move.
		for sq := range board {
			if v.SquareName(sq) == fields[3] {
				pawn := sq - p.Turn.forward()*v.Files
				if pawn >= 0 && pawn < len(board) && board[pawn].Type != 0 && board[pawn].Color != p.Turn && v.Type(board[pawn]).Pawn {
					p.EP = []int{sq}
					p.EPPawn = pawn
				}
				break
			}
		}
	}
	if len(fields) > 3 {
		if err := p.parseChecks(fields[3:]); err != nil {
			return nil, err
		}
	}
	if len(fields) > 4 {
		if n, err := strconv.Atoi(fields[4]); err == nil && n >= 0 {
			p.Clock = n
		}
	}
	if len(fields) > 5 {
		if n, err := strconv.Atoi(fields[5]); err == nil && n > 0 {
			p.Fullmove = n
		}
	}
	return p, nil
}

// splitFENHands separates optional pockets from the placement field.
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

// FEN serializes the board, side, castling rights, en passant, halfmove clock
// and fullmove number and, for a variant that counts them, the checks each
// side has given. A drop variant writes its pockets after the placement and
// marks promoted pieces with ~.
// FEN cannot encode arbitrary fairy-piece moved flags or multiple en-passant
// squares.
func (p *Position) FEN() string {
	var b strings.Builder
	for y := p.V.Ranks - 1; y >= 0; y-- {
		if y != p.V.Ranks-1 {
			b.WriteByte('/')
		}
		empty := 0
		for x := 0; x < p.V.Files; x++ {
			sq := p.V.Square(x, y)
			pc := p.Board[sq]
			if p.V.Blocker && sq == p.Blocker {
				if empty > 0 {
					fmt.Fprint(&b, empty)
					empty = 0
				}
				b.WriteByte('*')
				continue
			}
			if pc.Type == 0 {
				empty++
				continue
			}
			if empty > 0 {
				fmt.Fprint(&b, empty)
				empty = 0
			}
			ch := rune(p.V.Pieces[pc.Type-1].Letter)
			if pc.Color == Black {
				ch = unicode.ToLower(ch)
			}
			b.WriteRune(ch)
			if p.V.Drops && pc.Promoted != 0 {
				b.WriteByte('~')
			}
		}
		if empty > 0 {
			fmt.Fprint(&b, empty)
		}
	}
	if p.V.Drops {
		b.WriteByte('[')
		for _, color := range []Color{White, Black} {
			for i, t := range p.V.Pieces {
				if i+1 >= len(p.Hands[color]) {
					continue
				}
				ch := rune(t.Letter)
				if color == Black {
					ch = unicode.ToLower(ch)
				}
				for n := p.Hands[color][i+1]; n > 0; n-- {
					b.WriteRune(ch)
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
	n := 0
	for color := White; color <= Black; color++ {
		for _, r := range p.castling[color] {
			if !r.active {
				continue
			}
			c := r.castle
			ch := 'Q'
			if c.Rook%p.V.Files > c.King%p.V.Files {
				ch = 'K'
			}
			if c.RookSide != "" {
				ch = rune('A' + c.Rook%p.V.Files)
			}
			if color == Black {
				ch = unicode.ToLower(ch)
			}
			b.WriteRune(ch)
			n++
		}
	}
	if n == 0 {
		b.WriteByte('-')
	}
	b.WriteByte(' ')
	if len(p.EP) > 0 && p.EPPawn >= 0 {
		b.WriteString(p.V.SquareName(p.EP[0]))
	} else {
		b.WriteByte('-')
	}
	fullmove := p.Fullmove
	if fullmove < 1 {
		fullmove = 1
	}
	fmt.Fprintf(&b, " %d %d", p.Clock, fullmove)
	if p.V.CheckLimit > 0 {
		fmt.Fprintf(&b, " +%d+%d", p.Checks[White], p.Checks[Black])
	}
	return b.String()
}

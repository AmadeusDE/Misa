package chess

import (
	"fmt"
	"strings"
	"unicode"
)

type resolvedCastle struct {
	castle Castle
	active bool
}

// Castle returns the position's immutable, color-oriented castling squares.
func (p *Position) Castle(i int, color Color) Castle { return p.castling[color][i].castle }

func (p *Position) resolveCastles(rights string) {
	v := p.V
	for color := White; color <= Black; color++ {
		p.castling[color] = make([]resolvedCastle, len(v.Castling))
		for i, spec := range v.Castling {
			c := spec
			c.King, c.Rook = v.orient(c.King, color), v.orient(c.Rook, color)
			c.KingTo, c.RookTo = v.orient(c.KingTo, color), v.orient(c.RookTo, color)
			if spec.RookSide != "" {
				c.King, c.Rook = -1, -1
				rank := c.KingTo / v.Files
				for x := 0; x < v.Files; x++ {
					sq := rank*v.Files + x
					pc := p.Board[sq]
					if pc.Type != 0 && pc.Color == color && v.Type(pc).Royal {
						c.King = sq
						break
					}
				}
				if c.King >= 0 {
					// Shredder-FEN file rights specify the exact rook, otherwise take the outermost rook on the requested side.
					for x := 0; x < v.Files; x++ {
						sq := rank*v.Files + x
						pc := p.Board[sq]
						if pc.Type != spec.RookType || pc.Color != color || (spec.RookSide == "left" && sq >= c.King) || (spec.RookSide == "right" && sq <= c.King) {
							continue
						}
						if c.Rook < 0 || (spec.RookSide == "right" && sq > c.Rook) {
							c.Rook = sq
						}
						letter := rune('A' + x)
						if color == Black {
							letter = unicode.ToLower(letter)
						}
						if rights != "" && strings.ContainsRune(rights, letter) && letter != 'K' && letter != 'Q' && letter != 'k' && letter != 'q' {
							c.Rook = sq
							break
						}
					}
				}
			}
			active := c.King >= 0 && c.Rook >= 0
			if active && rights != "" {
				letter := 'Q'
				if c.Rook%v.Files > c.King%v.Files {
					letter = 'K'
				}
				file := rune('A' + c.Rook%v.Files)
				if color == Black {
					letter = unicode.ToLower(letter)
					file = unicode.ToLower(file)
				}
				active = strings.ContainsRune(rights, letter) || strings.ContainsRune(rights, file)
			}
			if active {
				k, r := p.Board[c.King], p.Board[c.Rook]
				active = k.Type != 0 && r.Type != 0 && k.Color == color && r.Color == color && v.Type(k).Royal && (spec.RookSide == "" || r.Type == spec.RookType)
				if active && rights != "" {
					p.Board[c.King].Moved = false
					p.Board[c.Rook].Moved = false
				}
				if rights == "" {
					active = active && !k.Moved && !r.Moved
				}
			}
			p.castling[color][i] = resolvedCastle{c, active}
			if spec.RookSide == "" && !active && c.Rook >= 0 && p.Board[c.Rook].Type != 0 {
				p.Board[c.Rook].Moved = true
			}
		}
	}
}

func (p *Position) updateCastleRights(m Move) {
	for color := White; color <= Black; color++ {
		for i := range p.castling[color] {
			r := &p.castling[color][i]
			if m.From == r.castle.King || m.From == r.castle.Rook || m.Take == r.castle.Rook {
				r.active = false
			}
		}
	}
}

// FEN serializes the board, side, castling rights, en passant, halfmove clock
// and, for a variant that counts them, the checks each side has given.
// FEN cannot encode arbitrary fairy-piece moved flags or multiple en-passant squares.
func (p *Position) FEN() string {
	var b strings.Builder
	for y := p.V.Ranks - 1; y >= 0; y-- {
		if y != p.V.Ranks-1 {
			b.WriteByte('/')
		}
		empty := 0
		for x := 0; x < p.V.Files; x++ {
			pc := p.Board[p.V.Square(x, y)]
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
		}
		if empty > 0 {
			fmt.Fprint(&b, empty)
		}
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
	fmt.Fprintf(&b, " %d 1", p.Clock)
	if p.V.CheckLimit > 0 {
		fmt.Fprintf(&b, " +%d+%d", p.Checks[White], p.Checks[Black])
	}
	return b.String()
}

package chess

import (
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

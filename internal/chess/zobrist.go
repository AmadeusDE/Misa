package chess

import "math/rand/v2"

// zobrist holds the random keys that hash positions: one per square, piece
// type, colour and moved flag, one per en passant square, and one for Black
// to move.
type zobrist struct {
	pieces []uint64
	ep     []uint64
	black  uint64
}

func newZobrist(v *Variant) zobrist {
	rng := rand.New(rand.NewPCG(0x6d697361, uint64(v.Size())))
	z := zobrist{
		pieces: make([]uint64, v.Size()*len(v.Pieces)*4),
		ep:     make([]uint64, v.Size()),
		black:  rng.Uint64(),
	}
	for i := range z.pieces {
		z.pieces[i] = rng.Uint64()
	}
	for i := range z.ep {
		z.ep[i] = rng.Uint64()
	}
	return z
}

// Hash identifies the position for repetition and the search's table: the
// pieces, the side to move, en passant chances and the moved flags that can
// still matter (castling, first-move-only moves).
func (p *Position) Hash() uint64 {
	v := p.V
	z := &v.zobrist
	n := len(v.Pieces)
	var h uint64
	for sq, pc := range p.Board {
		if pc.Type == 0 {
			continue
		}
		moved := pc.Moved && v.Pieces[pc.Type-1].tracksMoved
		h ^= z.pieces[((sq*n+int(pc.Type-1))*2+int(pc.Color))*2+int(b2i(moved))]
	}
	if v.Drops {
		for sq, pc := range p.Board {
			if pc.Type != 0 && pc.Promoted != 0 {
				h ^= hashDropKey(uint64(sq)<<8 | uint64(pc.Promoted) | 1<<63)
			}
		}
		for color, hand := range p.Hands {
			for typ, count := range hand {
				if typ > 0 && count > 0 {
					h ^= hashDropKey(uint64(count) ^ uint64(typ)<<48 ^ uint64(color)<<56)
				}
			}
		}
	}
	if p.Turn == Black {
		h ^= z.black
	}
	if p.EPPawn >= 0 {
		for _, sq := range p.EP {
			h ^= z.ep[sq]
		}
	}
	return h
}

// SplitMix64 mixes arbitrary hand counts without a fixed-size key table.
func hashDropKey(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

package chess

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestDoubleBigBoard(t *testing.T) {
	v := builtin(t, "doublebig")
	if v.Files != 32 || v.Ranks != 16 || len(v.Castling) != 4 {
		t.Fatalf("dimensions/castling: %dx%d, %d castles", v.Files, v.Ranks, len(v.Castling))
	}
	if pawn := v.Pieces[v.TypeOf('P')-1]; pawn.Betza != "fmWfcFifmW6" || !pawn.Pawn {
		t.Fatalf("inherited pawn: %+v", pawn)
	}
	p := NewPosition(v)
	for rank, row := range []string{
		"RNDWBOAQKCJBWDNRRNDWBOAQKCJBWDNR",
		"PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP",
	} {
		for file, letter := range row {
			for _, side := range []Color{White, Black} {
				r := rank
				if side == Black {
					r = 15 - r
				}
				pc := p.Board[r*32+file]
				if pc.Type != v.TypeOf(byte(letter)) || pc.Color != side {
					t.Errorf("%s rank %d file %d: %+v", side, r+1, file, pc)
				}
			}
		}
	}
	for r := 2; r < 14; r++ {
		for f := range 32 {
			if p.Board[r*32+f].Type != 0 {
				t.Errorf("occupied empty rank %d file %d", r+1, f)
			}
		}
	}
	for i, names := range [][4]string{{"i1", "p1", "o1", "n1"}, {"i1", "a1", "c1", "d1"}, {"y1", "af1", "ae1", "ad1"}, {"y1", "q1", "s1", "t1"}} {
		c := v.Castling[i]
		for j, got := range []int{c.King, c.Rook, c.KingTo, c.RookTo} {
			want, err := v.ParseSquare(names[j])
			if err != nil || got != want {
				t.Errorf("castle %d square %d = %d, want %s (%v)", i, j, got, names[j], err)
			}
		}
	}
	moves := p.Moves()
	if len(moves) == 0 {
		t.Fatal("no legal opening moves")
	}
	g := newGame(t, "doublebig", 140, 45)
	view := ansi.Strip(g.View())
	if strings.Contains(view, "Terminal too small") || !strings.Contains(view, "af") {
		t.Fatal("wide board did not render its final file")
	}
	r := NewEngine(v).Think(p, nil, Limits{Time: 100 * time.Millisecond})
	if !r.Found || !slices.Contains(moves, r.Move) {
		t.Errorf("computer returned illegal move: %+v", r)
	}
}

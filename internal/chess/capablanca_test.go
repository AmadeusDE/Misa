package chess

import (
	"slices"
	"testing"
	"time"
)

func TestCapablanca(t *testing.T) {
	v := builtin(t, "capablanca")
	if v.Files != 10 || v.Ranks != 8 || len(v.Castling) != 2 {
		t.Fatalf("dimensions or castles: %+v", v)
	}
	p := NewPosition(v)
	for rank, row := range []string{"RNABQKBCNR", "PPPPPPPPPP"} {
		for f, letter := range row {
			for _, side := range []Color{White, Black} {
				r := rank
				if side == Black {
					r = 7 - r
				}
				pc := p.Board[r*10+f]
				if pc.Type != v.TypeOf(byte(letter)) || pc.Color != side {
					t.Errorf("rank %d file %d: %+v", r+1, f, pc)
				}
			}
		}
	}
	for r := 2; r < 6; r++ {
		for f := range 10 {
			if p.Board[r*10+f].Type != 0 {
				t.Errorf("occupied %d,%d", f, r)
			}
		}
	}
	for letter, betza := range map[byte]string{'A': "BN", 'C': "RN"} {
		if v.Pieces[v.TypeOf(letter)-1].Betza != betza {
			t.Errorf("%c movement", letter)
		}
	}
	moves := p.Moves()
	if len(moves) != 28 {
		t.Errorf("opening moves = %d, want 28", len(moves))
	}
	result := NewEngine(v).Think(p, nil, Limits{Time: 200 * time.Millisecond})
	if !result.Found || !slices.Contains(moves, result.Move) {
		t.Errorf("illegal search result: %+v", result)
	}
}

func TestCapablancaCastling(t *testing.T) {
	v := builtin(t, "capablanca")
	for _, tc := range []struct{ fen, from, to, rookTo, rookFrom string }{
		{"r4k3r/10/10/10/10/10/10/R4K3R w", "f1", "i1", "h1", "j1"},
		{"r4k3r/10/10/10/10/10/10/R4K3R w", "f1", "c1", "d1", "a1"},
		{"r4k3r/10/10/10/10/10/10/R4K3R b", "f8", "i8", "h8", "j8"},
		{"r4k3r/10/10/10/10/10/10/R4K3R b", "f8", "c8", "d8", "a8"},
	} {
		p := fen(t, v, tc.fen)
		m := findMove(t, p, tc.from, tc.to)
		if m.Castle < 0 {
			t.Errorf("%s-%s not castling", tc.from, tc.to)
		}
		after := p.Apply(m)
		for square, want := range map[string]byte{tc.to: 'K', tc.rookTo: 'R', tc.from: 0, tc.rookFrom: 0} {
			sq, _ := v.ParseSquare(square)
			if got := pieceLetter(after, sq); got != want {
				t.Errorf("%s: got %q want %q", square, got, want)
			}
		}
	}
	p := fen(t, v, "5k4/10/10/10/10/10/6r3/R4K3R w")
	if slices.Contains(destinations(p, "f1"), "i1") {
		t.Error("castled through attack on g1")
	}
}

func TestCapablancaPromotionAndFairyMoves(t *testing.T) {
	v := builtin(t, "capablanca")
	p := fen(t, v, "9k/4P5/10/10/10/10/10/K9 w -")
	var got []byte
	for _, m := range p.Moves() {
		if m.Promote != 0 {
			got = append(got, v.Pieces[m.Promote-1].Letter)
		}
	}
	if !slices.Equal(got, []byte{'Q', 'C', 'A', 'R', 'B', 'N'}) {
		t.Errorf("promotion = %q", got)
	}
	for _, tc := range []struct {
		piece   byte
		targets []string
	}{{'A', []string{"f5", "f3", "c5"}}, {'C', []string{"e5", "a4", "f6"}}} {
		p := fen(t, v, "9k/10/10/10/4"+string(tc.piece)+"5/10/10/K9 w -")
		dest := destinations(p, "e4")
		for _, target := range tc.targets {
			if !slices.Contains(dest, target) {
				t.Errorf("%c cannot move to %s: %v", tc.piece, target, dest)
			}
		}
	}
}

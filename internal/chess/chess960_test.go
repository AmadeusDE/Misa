package chess

import (
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

func TestChess960Builtin(t *testing.T) {
	v := builtin(t, "chess960")
	if v.Files != 8 || v.Ranks != 8 || len(v.Pieces) != 6 || len(v.Castling) != 2 {
		t.Fatalf("dimensions/pieces/castling: %dx%d, %d pieces, %d castles", v.Files, v.Ranks, len(v.Pieces), len(v.Castling))
	}
	if v.PromotionZone != 1 || v.MoveLimit != 50 || !slices.Equal(v.Promotion, builtin(t, "chess").Promotion) {
		t.Fatal("lost inherited promotion or move limit")
	}
	for i, side := range []string{"right", "left"} {
		c := v.Castling[i]
		if c.RookSide != side || c.RookType != v.TypeOf('R') {
			t.Errorf("castle %d = %+v", i, c)
		}
	}
	p := NewPosition(v)
	for file := range 8 {
		if p.Board[file+8].Type != v.TypeOf('P') || p.Board[file+48].Type != v.TypeOf('P') {
			t.Fatal("pawns are not on their usual ranks")
		}
	}
	r := NewEngine(v).Think(p, nil, Limits{Time: 100 * time.Millisecond, Depth: 2, Nodes: 4000})
	if !r.Found || !slices.Contains(p.Moves(), r.Move) {
		t.Fatalf("computer returned illegal move: %+v", r)
	}
}

func TestChess960OrthodoxEndings(t *testing.T) {
	v := builtin(t, "chess960")
	for _, tt := range []struct {
		name, fen string
		status    Status
	}{
		{"mate", "7k/6Q1/5K2/8/8/8/8/8 b -", Checkmate},
		{"stalemate", "7k/5Q2/5K2/8/8/8/8/8 b -", Stalemate},
		{"bare kings", "7k/8/8/8/8/8/8/K7 w -", DrawMaterial},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMatch(v)
			m.Load(fen(t, v, tt.fen))
			if got, _ := m.Status(); got != tt.status {
				t.Errorf("status = %v, want %v", got, tt.status)
			}
		})
	}
}

func TestChess960SeededMatch(t *testing.T) {
	v := builtin(t, "chess960")
	makeRNG := func() *rand.Rand { return rand.New(rand.NewPCG(960, 518)) }
	a := NewMatchWithRand(v, makeRNG())
	b := NewMatchWithRand(v, makeRNG())
	if !slices.Equal(a.Position().Board, b.Position().Board) || a.Position().Hash() != b.Position().Hash() {
		t.Fatal("seeded match starts differ")
	}
	rng := makeRNG()
	seen := map[uint64]bool{}
	for range 30 {
		seen[NewMatchWithRand(v, rng).Position().Hash()] = true
	}
	if len(seen) < 2 {
		t.Fatal("new matches did not draw fresh starts")
	}
}

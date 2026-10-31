package chess

import "testing"

func TestDynamicCastlingRuntime(t *testing.T) {
	v := builtin(t, "chess960")
	cases := []struct {
		name, fen, king, rook string
		castle                bool
	}{
		{"left", "4k3/8/8/8/8/8/8/RK6 w Q", "c1", "d1", true},
		{"stationary", "4k3/8/8/8/8/8/8/6KR w K", "g1", "f1", true},
		{"swap", "4k3/8/8/8/8/8/8/5KR1 w K", "g1", "f1", true},
		{"blocked", "4k3/8/8/8/8/8/8/5KNR w K", "g1", "f1", false},
		{"transit attacked", "4kr2/8/8/8/8/8/8/4K2R w K", "g1", "f1", false},
		{"rook unmask", "4k3/8/8/8/8/8/8/rR2K3 w Q", "c1", "d1", false},
		{"black", "5kr1/8/8/8/8/8/8/4K3 b k", "g8", "f8", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := fen(t, v, tt.fen)
			found := false
			for _, m := range p.Moves() {
				if m.Castle < 0 {
					continue
				}
				found = true
				q := p.Apply(m)
				king, _ := v.ParseSquare(tt.king)
				rook, _ := v.ParseSquare(tt.rook)
				if q.Board[king].Type != v.TypeOf('K') || q.Board[rook].Type != v.TypeOf('R') {
					t.Fatalf("bad castled board: %s", q.FEN())
				}
				round := fen(t, v, q.FEN())
				if round.Hash() != q.Hash() {
					t.Errorf("castled hash mismatch %s", q.FEN())
				}
			}
			if found != tt.castle {
				t.Errorf("castle = %v want %v", found, tt.castle)
			}
		})
	}
}

func TestDynamicFENRightsAndHash(t *testing.T) {
	v := builtin(t, "chess960")
	p := fen(t, v, "4k3/8/8/8/8/8/8/RK4RR w G")
	if p.Castle(0, White).Rook != 6 {
		t.Fatalf("wrong rook: %+v", p.Castle(0, White))
	}
	q := fen(t, v, p.FEN())
	if p.Hash() != q.Hash() {
		t.Fatalf("roundtrip hash: %s", p.FEN())
	}
	r := fen(t, v, "4k3/8/8/8/8/8/8/RK4RR w H")
	if p.Hash() == r.Hash() {
		t.Fatal("different origins have same hash")
	}
	none := fen(t, v, "4k3/8/8/8/8/8/8/RK4RR w -")
	if p.Hash() == none.Hash() {
		t.Fatal("lost right did not change hash")
	}
}

func TestDynamicFENAfterKingMove(t *testing.T) {
	v := builtin(t, "chess960")
	p := fen(t, v, "4k3/8/8/8/8/8/8/R3K2R w KQ")
	for _, m := range p.Moves() {
		if m.Castle < 0 && v.SquareName(m.From) == "e1" && v.SquareName(m.To) == "e2" {
			q := p.Apply(m)
			r := fen(t, v, q.FEN())
			if r.Hash() != q.Hash() {
				t.Fatalf("king move hash roundtrip: %s", q.FEN())
			}
			return
		}
	}
	t.Fatal("king move unavailable")
}

func TestFENEnPassantRoundTrip(t *testing.T) {
	v := builtin(t, "chess")
	p := fen(t, v, "4k3/8/8/3pP3/8/8/8/4K3 w - d6 7 1")
	if p.EPPawn < 0 || len(p.EP) != 1 {
		t.Fatal("en passant not parsed")
	}
	q := fen(t, v, p.FEN())
	if p.Hash() != q.Hash() || q.Clock != 7 {
		t.Fatalf("roundtrip: %s", p.FEN())
	}
}

func TestDynamicRightsIrrevocable(t *testing.T) {
	v := builtin(t, "chess960")
	play := func(t *testing.T, p *Position, from, to string) *Position {
		t.Helper()
		for _, m := range p.Moves() {
			if m.Castle < 0 && v.SquareName(m.From) == from && v.SquareName(m.To) == to {
				return p.Apply(m)
			}
		}
		t.Fatalf("missing move %s-%s in %s", from, to, p.FEN())
		return nil
	}
	for _, tt := range []struct {
		name  string
		moves [][2]string
		lost  int
	}{
		{"king returns", [][2]string{{"e1", "e2"}, {"e8", "e7"}, {"e2", "e1"}, {"e7", "e8"}}, -1},
		{"rook returns", [][2]string{{"h1", "h2"}, {"e8", "e7"}, {"h2", "h1"}, {"e7", "e8"}}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := fen(t, v, "4k3/8/8/8/8/8/8/R3K2R w KQ")
			original0, original1 := p.Castle(0, White), p.Castle(1, White)
			for _, move := range tt.moves {
				p = play(t, p, move[0], move[1])
				if p.Castle(0, White) != original0 || p.Castle(1, White) != original1 {
					t.Fatal("origins changed after move")
				}
			}
			for _, m := range p.Moves() {
				if m.Castle >= 0 && (tt.lost < 0 || m.Castle == tt.lost) {
					t.Fatalf("lost right restored: %+v", m)
				}
			}
			if tt.lost == 0 && !p.castling[White][1].active {
				t.Fatal("other rook's right lost")
			}
			q := fen(t, v, p.FEN())
			if q.Hash() != p.Hash() {
				t.Fatalf("rights hash roundtrip: %s", p.FEN())
			}
		})
	}
	t.Run("rook captured", func(t *testing.T) {
		p := fen(t, v, "r5k1/8/8/8/8/8/8/R3K2R b KQ")
		old := p.Castle(1, White)
		p = play(t, p, "a8", "a1")
		if p.castling[White][1].active || !p.castling[White][0].active {
			t.Fatal("wrong rights after rook capture")
		}
		if p.Castle(1, White) != old {
			t.Fatal("origin changed on capture")
		}
	})
}

func TestDynamicAdditionalCastlingPaths(t *testing.T) {
	v := builtin(t, "chess960")
	for _, tt := range []struct {
		name, fen string
		want      bool
	}{
		{"stationary rook", "4k3/8/8/8/8/8/8/4KR2 w K", true},
		{"original check", "4r1k1/8/8/8/8/8/8/4K2R w K", false},
		{"destination attacked", "4k1r1/8/8/8/8/8/8/4K2R w K", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := fen(t, v, tt.fen)
			found := false
			for _, m := range p.Moves() {
				if m.Castle >= 0 {
					found = true
					q := p.Apply(m)
					if q.Board[6].Type != v.TypeOf('K') || q.Board[5].Type != v.TypeOf('R') {
						t.Fatalf("bad castled board: %s", q.FEN())
					}
				}
			}
			if found != tt.want {
				t.Fatalf("castle = %v, want %v", found, tt.want)
			}
		})
	}
}

func TestOpeningFENEPClockAndRights(t *testing.T) {
	v := builtin(t, "chess960")
	p := fen(t, v, "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w KQkq")
	var next *Position
	for _, m := range p.Moves() {
		if v.SquareName(m.From) == "e2" && v.SquareName(m.To) == "e4" {
			next = p.Apply(m)
			break
		}
	}
	if next == nil {
		t.Fatal("e2-e4 missing")
	}
	if next.Clock != 0 || len(next.EP) != 1 || v.SquareName(next.EP[0]) != "e3" {
		t.Fatalf("bad EP or clock: %s", next.FEN())
	}
	for c := White; c <= Black; c++ {
		for i := range v.Castling {
			if !next.castling[c][i].active || next.Castle(i, c) != p.Castle(i, c) {
				t.Fatal("opening pawn move changed castling")
			}
		}
	}
	round := fen(t, v, next.FEN())
	if round.Hash() != next.Hash() || round.Clock != next.Clock || round.EPPawn != next.EPPawn {
		t.Fatalf("opening FEN roundtrip: %s", next.FEN())
	}
}

package chess

import (
	"slices"
	"testing"
)

func TestAntichessForcedCaptures(t *testing.T) {
	v := builtin(t, "antichess")
	for _, tc := range []struct {
		name, position string
		want           []string
	}{
		{"single", "8/8/8/3p4/3R4/8/P7/8 w -", []string{"Rd4xd5"}},
		{"multiple", "8/8/8/3p4/3Rp3/8/P7/8 w -", []string{"Rd4xd5", "Rd4xe4"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fen(t, v, tc.position)
			var got []string
			for _, mv := range p.Moves() {
				got = append(got, describe(p, mv))
			}
			slices.Sort(got)
			slices.Sort(tc.want)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("moves = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAntichessEnPassantIsForced(t *testing.T) {
	v := builtin(t, "antichess")
	p := fen(t, v, "8/8/8/8/3p4/8/4P2P/8 w -")
	p = play(t, p, "e2", "e4")
	moves := p.Moves()
	if len(moves) != 1 || describe(p, moves[0]) != "d4xe3" {
		t.Fatalf("en passant moves = %v, want only d4xe3", moves)
	}
	after := p.Apply(moves[0])
	sq, _ := v.ParseSquare("e4")
	if after.Board[sq].Type != 0 {
		t.Error("en passant did not remove the captured pawn")
	}
}

func TestAntichessKingAndPromotion(t *testing.T) {
	v := builtin(t, "antichess")
	king := v.Pieces[v.TypeOf('K')-1]
	if king.Royal || len(v.Castling) != 0 {
		t.Fatalf("king royal = %v, castling = %v", king.Royal, v.Castling)
	}
	p := fen(t, v, "8/8/8/8/8/8/4r3/4K2R w K -")
	if p.InCheck() {
		t.Error("ordinary king is in check")
	}
	moves := p.Moves()
	if len(moves) != 1 || describe(p, moves[0]) != "Ke1xe2" {
		t.Fatalf("ordinary king capture = %v", moves)
	}
	p = fen(t, v, "8/P7/8/8/8/8/8/8 w -")
	var promotions []byte
	for _, mv := range p.Moves() {
		if mv.Promote != 0 {
			promotions = append(promotions, v.Pieces[mv.Promote-1].Letter)
		}
	}
	slices.Sort(promotions)
	if !slices.Equal(promotions, []byte{'B', 'K', 'N', 'Q', 'R'}) {
		t.Fatalf("promotion choices = %q", promotions)
	}
}

func TestAntichessSideUnableToMoveWins(t *testing.T) {
	v := builtin(t, "antichess")
	for _, position := range []string{
		"8/7p/8/8/8/8/8/8 w -",  // no white pieces
		"P7/7p/8/8/8/8/8/8 w -", // white pawn cannot move
		"8/8/8/8/8/8/P7/8 b -",  // no black pieces
	} {
		p := fen(t, v, position)
		m := NewMatch(v)
		m.Load(p)
		if !m.Over() || !m.Decisive() {
			t.Fatalf("%s: expected decisive ending, got %v", position, m.Over())
		}
		_, winner := m.Status()
		if winner != p.Turn {
			t.Errorf("%s: winner %v, want side to move %v", position, winner, p.Turn)
		}
	}
}

func TestAntichessSearch(t *testing.T) {
	v := builtin(t, "antichess")
	p := fen(t, v, "3r4/8/8/3n4/3Rr3/8/8/8 w -")
	r := think(t, p, 3)
	if !slices.Contains(p.Moves(), r.Move) {
		t.Fatalf("search returned illegal move %+v", r.Move)
	}
	// Rxd5 forces ...Rxd5, shedding White's last piece. Rxe4 does not.
	if got := describe(p, r.Move); got != "Rd4xd5" {
		t.Errorf("search chose %s, want Rd4xd5 to lose the last piece", got)
	}
}

func TestAntichessInverseMaterial(t *testing.T) {
	v := builtin(t, "antichess")
	e := newEvaluator(v)
	pawn := fen(t, v, "7r/8/8/8/8/8/P7/8 w -")
	queen := fen(t, v, "7r/8/8/8/8/8/Q7/8 w -")
	if a, b := e.evaluate(pawn), e.evaluate(queen); a <= b {
		t.Errorf("inverse material: pawn score %d <= queen score %d", a, b)
	}
}

func TestAntichessSetupAndOrdinaryKing(t *testing.T) {
	v := builtin(t, "antichess")
	if v.Files != 8 || v.Ranks != 8 || len(v.Pieces) != 6 || len(v.Castling) != 0 {
		t.Fatalf("unexpected setup: %dx%d, %d pieces, %d castles", v.Files, v.Ranks, len(v.Pieces), len(v.Castling))
	}
	start := NewPosition(v)
	r := think(t, start, 2)
	if !slices.Contains(start.Moves(), r.Move) {
		t.Fatalf("start search returned illegal move %+v", r.Move)
	}
	p := fen(t, v, "8/8/8/8/8/8/r7/4K3 w -")
	found := false
	for _, m := range p.Moves() {
		if describe(p, m) == "Ke1-e2" {
			found = true
		}
	}
	if !found {
		t.Fatal("king cannot move into attack")
	}
	p = fen(t, v, "8/8/8/8/8/8/4k3/4R3 w -")
	if m := p.Moves(); len(m) != 1 || describe(p, m[0]) != "Re1xe2" {
		t.Fatalf("king capture: %v", m)
	}
}

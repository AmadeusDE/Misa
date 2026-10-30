package chess

import (
	"slices"
	"testing"
)

func dropMove(t *testing.T, p *Position, piece byte, square string) Move {
	t.Helper()
	sq, err := p.V.ParseSquare(square)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range p.Moves() {
		if m.Drop == p.V.TypeOf(piece) && m.To == sq {
			return m
		}
	}
	t.Fatalf("missing %c drop on %s", piece, square)
	return Move{}
}

func TestDropCapturesAndClone(t *testing.T) {
	v := builtin(t, "crazyhouse")
	for _, tc := range []struct {
		fen  string
		want byte
	}{
		{"4k3/8/8/3q4/4B3/8/8/4K3[] w - - 0 1", 'Q'},
		{"4k3/8/8/3q~4/4B3/8/8/4K3[] w - - 0 1", 'P'},
		{"4k3/8/8/3pP3/8/8/8/4K3[] w - d6 0 1", 'P'},
	} {
		p := fen(t, v, tc.fen)
		var capture Move
		found := false
		for _, m := range p.Moves() {
			if m.Take >= 0 {
				capture = m
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no capture: %s", tc.fen)
		}
		q := p.Apply(capture)
		if q.Hands[White][v.TypeOf(tc.want)] != 1 {
			t.Errorf("capture hand: %s %v", tc.fen, q.Hands)
		}
		if p.Hands[White][v.TypeOf(tc.want)] != 0 {
			t.Fatal("mutated original hand")
		}
	}
}

func TestDropGenerationAndCheck(t *testing.T) {
	v := builtin(t, "crazyhouse")
	p := fen(t, v, "4k3/8/8/8/8/8/8/4K3[PN] w - - 0 1")
	count := map[uint8]int{}
	for _, m := range p.Moves() {
		if m.Drop != 0 {
			count[m.Drop]++
			if m.From != -1 || m.Take != -1 || m.Castle != -1 {
				t.Fatal(m)
			}
		}
	}
	if count[v.TypeOf('P')] != 48 || count[v.TypeOf('N')] != 62 {
		t.Fatalf("drop counts: %v", count)
	}
	for _, m := range p.captures() {
		if m.Drop != 0 {
			t.Fatal("drop in captures")
		}
	}
	p = fen(t, v, "k3r3/8/8/8/8/8/8/4K3[N] w - - 0 1")
	var squares []string
	for _, m := range p.Moves() {
		if m.Drop != 0 {
			squares = append(squares, v.SquareName(m.To))
		}
	}
	slices.Sort(squares)
	if !slices.Equal(squares, []string{"e2", "e3", "e4", "e5", "e6", "e7"}) {
		t.Fatalf("check blocks: %v", squares)
	}
}

func TestDroppedPawnInitialMoves(t *testing.T) {
	v := builtin(t, "crazyhouse")
	for _, tc := range []struct {
		sq, double string
		want       bool
	}{{"a2", "a4", true}, {"a3", "a5", false}} {
		p := fen(t, v, "4k3/8/8/8/8/8/8/4K3[P] w - - 0 1")
		q := p.Apply(dropMove(t, p, 'P', tc.sq))
		q.Turn = White
		got := slices.Contains(destinations(q, tc.sq), tc.double)
		if got != tc.want {
			t.Errorf("%s double step = %v", tc.sq, got)
		}
	}
}

func TestCrazyhouseInitialPosition(t *testing.T) {
	v := builtin(t, "crazyhouse")
	if v.Files != 8 || v.Ranks != 8 || len(v.Pieces) != 6 || len(v.Castling) != 2 {
		t.Fatalf("unexpected variant: %dx%d, %d types, %d castles", v.Files, v.Ranks, len(v.Pieces), len(v.Castling))
	}
	if n := len(NewPosition(v).Moves()); n != 20 {
		t.Fatalf("opening moves: %d", n)
	}
	p := fen(t, v, "4k3/8/8/8/8/8/8/4K3[PN] w - - 0 1")
	if n := perft(p, 1); n != 115 {
		t.Fatalf("depth 1 nodes: %d, want 115", n)
	}
}

func TestDropMateAndStalemate(t *testing.T) {
	v := builtin(t, "crazyhouse")
	m := NewMatch(v)
	p := fen(t, v, "k7/8/2K5/8/8/8/8/8[Q] w - - 0 1")
	m.Load(p)
	m.Play(dropMove(t, p, 'Q', "b7"))
	if status, winner := m.Status(); status != Checkmate || winner != White {
		t.Fatalf("drop mate: %v winner %v", status, winner)
	}
	for _, tc := range []struct {
		fen  string
		want Status
	}{
		{"k7/8/1QK5/8/8/8/8/8[] b - - 0 1", Stalemate},
		{"k7/8/1QK5/8/8/8/8/8[n] b - - 0 1", Playing},
	} {
		m.Load(fen(t, v, tc.fen))
		if status, _ := m.Status(); status != tc.want {
			t.Errorf("%s: %v, want %v", tc.fen, status, tc.want)
		}
	}
}

func TestPromotionCaptureReturnsPawn(t *testing.T) {
	v := builtin(t, "crazyhouse")
	p := fen(t, v, "1r2k3/P7/8/8/8/8/8/4K3[] w - - 0 1")
	from, _ := v.ParseSquare("a7")
	to, _ := v.ParseSquare("a8")
	var promotion Move
	found := false
	for _, m := range p.Moves() {
		if m.From == from && m.To == to && m.Promote == v.TypeOf('Q') {
			promotion = m
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing queen promotion")
	}
	q := p.Apply(promotion)
	if q.Board[to].Promoted != v.TypeOf('P') {
		t.Fatal("promotion provenance not stored")
	}
	r := q.Apply(findMove(t, q, "b8", "a8"))
	if r.Hands[Black][v.TypeOf('P')] != 1 || r.Hands[Black][v.TypeOf('Q')] != 0 {
		t.Fatalf("capture pocket: %v", r.Hands[Black])
	}
}

func TestDroppedRookCannotCastle(t *testing.T) {
	v := builtin(t, "crazyhouse")
	p := fen(t, v, "4k3/8/8/8/8/8/8/4K3[R] w - - 0 1")
	q := p.Apply(dropMove(t, p, 'R', "h1"))
	q.Turn = White
	for _, m := range q.Moves() {
		if m.Castle >= 0 {
			t.Fatal("castled with dropped rook")
		}
	}
}

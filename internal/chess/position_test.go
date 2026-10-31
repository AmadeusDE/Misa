package chess

import (
	"slices"
	"testing"
)

func perft(p *Position, depth int) int {
	moves := p.Moves()
	if depth == 1 {
		return len(moves)
	}
	n := 0
	for _, m := range moves {
		n += perft(p.Apply(m), depth-1)
	}
	return n
}

func fen(t *testing.T, v *Variant, s string) *Position {
	t.Helper()
	p, err := ParseFEN(v, s)
	if err != nil {
		t.Fatalf("%s: %v", s, err)
	}
	return p
}

// Node counts from https://www.chessprogramming.org/Perft_Results.
func TestPerftOrthodox(t *testing.T) {
	v := builtin(t, "chess")
	tests := []struct {
		name  string
		fen   string
		nodes []int
	}{
		{"start", "", []int{20, 400, 8902, 197281}},
		{"kiwipete", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq", []int{48, 2039, 97862}},
		{"position 3", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w -", []int{14, 191, 2812, 43238}},
		{"position 4", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq", []int{6, 264, 9467}},
		{"position 5", "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ", []int{44, 1486, 62379}},
	}
	for _, tt := range tests {
		p := NewPosition(v)
		if tt.fen != "" {
			p = fen(t, v, tt.fen)
		}
		for i, want := range tt.nodes {
			if testing.Short() && want > 50000 {
				break
			}
			if got := perft(p, i+1); got != want {
				t.Errorf("%s perft(%d) = %d, want %d", tt.name, i+1, got, want)
			}
		}
	}
}

func TestOpeningMoveCounts(t *testing.T) {
	tests := []struct {
		id   string
		want int
	}{
		// 16 pawns × 4 steps, 4 knights × 2.
		{"double", 72},
		// Pawns 16 × 6; knights 4; unicorns 2 (c3, n3); elephants 4 (alfil leaps);
		// superknight 8; archbishop 2; chancellor 2; rose 11.
		{"bigboard", 129},
		// Pawns 10 × 2; rooks 9 each (a step up, then along the first rank);
		// knights 3; bishops 2; queen 3; king 3; marshal 5; cardinal 6.
		{"grand", 65},
	}
	for _, tt := range tests {
		p := NewPosition(builtin(t, tt.id))
		if got := len(p.Moves()); got != tt.want {
			t.Errorf("%s: %d opening moves, want %d", tt.id, got, tt.want)
		}
		black := p.Apply(p.Moves()[0])
		if got := len(black.Moves()); got != tt.want {
			t.Errorf("%s: black has %d replies, want %d", tt.id, got, tt.want)
		}
	}
}

func destinations(p *Position, from string) []string {
	v := p.V
	sq, _ := v.ParseSquare(from)
	var out []string
	for _, m := range p.Moves() {
		if m.From == sq {
			out = append(out, v.SquareName(m.To))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func TestRose(t *testing.T) {
	v := builtin(t, "bigboard")
	p := NewPosition(v)
	want := []string{"a3", "b5", "c4", "d6", "e3", "f5", "g3", "h6", "i4", "j5", "k3"}
	if got := destinations(p, "f1"); !slices.Equal(got, want) {
		t.Errorf("rose from f1: %v, want %v", got, want)
	}
}

func TestRoseStopsAtFirstPieceOnItsCircle(t *testing.T) {
	v := builtin(t, "bigboard")
	// The own pawn on f5 cuts the circles through it; the knight on i4 is taken
	// and ends the circle g3–i4–k3–l1.
	p := fen(t, v, "15k/16/16/16/16/16/16/16/16/16/16/5P10/8n7/16/PPPPPPPPPPPPPPPP/K4O10 w -")
	want := []string{"a3", "c4", "e3", "g3", "i4"}
	if got := destinations(p, "f1"); !slices.Equal(got, want) {
		t.Errorf("rose from f1: %v, want %v", got, want)
	}
}

func TestRosePassesAroundAnOpenCircle(t *testing.T) {
	v := builtin(t, "bigboard")
	p := fen(t, v, "15k/16/16/16/16/16/16/16/7O8/16/16/16/16/16/16/K15 w -")
	pass := findMove(t, p, "h8", "h8")
	after := p.Apply(pass)
	if after.Turn != Black || after.Board[pass.From].Type != v.TypeOf('O') {
		t.Error("the pass did not hand the move over with the rose in place")
	}
	m := NewMatch(v)
	m.Load(p)
	m.Play(pass)
	if got := m.Notation()[0]; got != "Oh8 pass" {
		t.Errorf("notation = %q", got)
	}
}

func TestRoseCannotPassWhenItsCircleIsBlocked(t *testing.T) {
	v := builtin(t, "bigboard")
	// Every circle starts and ends on a knight's move from h8; pawns fill them.
	p := fen(t, v, "15k/16/16/16/16/16/6P1P7/5P3P6/7O8/5P3P6/6P1P7/16/16/16/16/K15 w -")
	if slices.Contains(destinations(p, "h8"), "h8") {
		t.Error("rose passed through blocked circles")
	}
	// The opening position's rose has no room either.
	if slices.Contains(destinations(NewPosition(v), "f1"), "f1") {
		t.Error("rose passed from the back rank")
	}
}

func TestCannotPassOutOfCheck(t *testing.T) {
	v := builtin(t, "bigboard")
	p := fen(t, v, "15k/16/16/16/16/16/16/16/7O8/16/16/16/16/16/16/K14r w -")
	if slices.Contains(destinations(p, "h8"), "h8") {
		t.Error("rose passed while the king is in check")
	}
}

func TestPawnLongFirstStepAndEnPassant(t *testing.T) {
	v := builtin(t, "double")
	p := NewPosition(v)
	if got := destinations(p, "e2"); !slices.Equal(got, []string{"e3", "e4", "e5", "e6"}) {
		t.Fatalf("double chess pawn: %v", got)
	}
	// White pushes e2-e6; a black pawn on d? captures it en passant on any
	// square it crossed. Put black's pawn beside e4 first.
	p = fen(t, v, "4k11/16/16/16/16/16/16/16/3p12/16/4P11/4K11 w -")
	p = play(t, p, "e2", "e6")
	got := destinations(p, "d4")
	if !slices.Contains(got, "e3") {
		t.Errorf("black pawn on d4 cannot take en passant on e3: %v", got)
	}
	ep := findMove(t, p, "d4", "e3")
	after := p.Apply(ep)
	if sq, _ := v.ParseSquare("e6"); after.Board[sq].Type != 0 {
		t.Error("en passant left the white pawn on e6")
	}
	// The chance passes after one move.
	p = play(t, p, "e12", "e11")
	p = play(t, p, "e1", "e2")
	if slices.Contains(destinations(p, "d4"), "e3") {
		t.Error("en passant still possible a move later")
	}
}

func TestBigBoardPawnReachesMiddle(t *testing.T) {
	p := NewPosition(builtin(t, "bigboard"))
	got := destinations(p, "i2")
	if len(got) != 6 || got[len(got)-1] != "i8" {
		t.Errorf("i2 pawn: %v, want i3..i8", got)
	}
}

func TestDoubleChessCastlesWithinEachHalf(t *testing.T) {
	v := builtin(t, "double")
	p := fen(t, v, "r3k2rr3k2r/16/16/16/16/16/16/16/16/16/16/R3K2RR3K2R w")
	for _, king := range []string{"e1", "m1"} {
		got := destinations(p, king)
		for _, want := range map[string][]string{"e1": {"c1", "g1"}, "m1": {"k1", "o1"}}[king] {
			if !slices.Contains(got, want) {
				t.Errorf("king %s cannot castle to %s: %v", king, want, got)
			}
		}
	}
	after := play(t, p, "m1", "o1")
	for sq, letter := range map[string]byte{"o1": 'K', "n1": 'R', "p1": 0, "m1": 0} {
		s, _ := v.ParseSquare(sq)
		if got := pieceLetter(after, s); got != letter {
			t.Errorf("after m1-o1, %s holds %q, want %q", sq, got, letter)
		}
	}
}

func TestBigBoardCastlingDistances(t *testing.T) {
	v := builtin(t, "bigboard")
	p := fen(t, v, "r7k6r/16/16/16/16/16/16/16/16/16/16/16/16/16/16/R7K6R w")
	got := destinations(p, "i1")
	for _, want := range []string{"c1", "o1"} {
		if !slices.Contains(got, want) {
			t.Errorf("king cannot castle to %s: %v", want, got)
		}
	}
	after := play(t, p, "i1", "c1")
	if s, _ := v.ParseSquare("d1"); pieceLetter(after, s) != 'R' {
		t.Error("queenside castling did not put the rook on d1")
	}
}

func TestCannotCastleThroughCheck(t *testing.T) {
	v := builtin(t, "chess")
	p := fen(t, v, "4k3/8/8/8/8/8/5r2/R3K2R w KQ")
	got := destinations(p, "e1")
	if slices.Contains(got, "g1") {
		t.Error("castled kingside past the attacked f1")
	}
	if !slices.Contains(got, "c1") {
		t.Error("queenside castling refused although d1 and c1 are safe")
	}
}

func TestEitherKingCheckmated(t *testing.T) {
	v := builtin(t, "double")
	// Ra3 mates the king on a12 (the b-file rook covers b11 and b12), even
	// though the king on p12 could still move.
	m := NewMatch(v)
	m.Load(fen(t, v, "k14k/16/16/16/16/16/16/16/16/7R8/1R14/4K7K3 w -"))
	m.Play(findMove(t, m.Position(), "h3", "a3"))
	if st, winner := m.Status(); st != Checkmate || winner != White {
		t.Errorf("status = %v, winner %v; want checkmate by White", st, winner)
	}
}

func TestCannotLeaveAKingInCheck(t *testing.T) {
	v := builtin(t, "double")
	// The rook on o1 checks both kings; only taking it answers both.
	p := fen(t, v, "4k10k/16/16/16/16/16/16/16/16/16/16/4K9rK w -")
	if !p.InCheck() {
		t.Fatal("white kings not in check")
	}
	moves := p.Moves()
	if len(moves) != 1 || v.SquareName(moves[0].From) != "p1" || v.SquareName(moves[0].To) != "o1" {
		t.Errorf("moves = %v, want only Kp1xo1", moves)
	}
}

func TestCannonAndGrasshopper(t *testing.T) {
	src := `
id: hoppers
name: Hoppers
files: 5
ranks: 5
pieces:
  - {letter: K, name: King, betza: K, royal: true}
  - {letter: X, name: Cannon, betza: mRcpR}
  - {letter: G, name: Grasshopper, betza: gQ}
  - {letter: P, name: Pawn, betza: fmW, pawn: true}
setup: k4/2p2/2P2/5/2X1K
`
	vs, err := parseVariants(map[string][]byte{"h.yaml": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := Find(vs, "hoppers")
	p := NewPosition(v)
	got := destinations(p, "c1")
	want := []string{"a1", "b1", "c2", "c4", "d1"}
	if !slices.Equal(got, want) {
		t.Errorf("cannon: %v, want %v", got, want)
	}
	p = fen(t, v, "k4/2p2/2P2/5/2G1K w")
	if got := destinations(p, "c1"); !slices.Equal(got, []string{"c4"}) {
		t.Errorf("grasshopper: %v, want [c4]", got)
	}
}

func TestLameHorse(t *testing.T) {
	src := `
id: lame
name: Lame
files: 5
ranks: 5
pieces:
  - {letter: K, name: King, betza: K, royal: true}
  - {letter: H, name: Horse, betza: nN}
  - {letter: P, name: Pawn, betza: fmW, pawn: true}
setup: k4/5/2H2/2P2/K4
`
	vs, err := parseVariants(map[string][]byte{"l.yaml": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := Find(vs, "lame")
	got := destinations(NewPosition(v), "c3")
	for _, blocked := range []string{"b1", "d1"} {
		if slices.Contains(got, blocked) {
			t.Errorf("horse jumped the pawn to %s: %v", blocked, got)
		}
	}
	if len(got) != 6 {
		t.Errorf("horse: %v, want 6 moves", got)
	}
}

func TestPromotionChoices(t *testing.T) {
	v := builtin(t, "bigboard")
	p := fen(t, v, "16/7P8/16/16/16/16/16/16/16/16/16/16/16/16/16/K14k w -")
	n := 0
	for _, m := range p.Moves() {
		if v.SquareName(m.From) == "h15" {
			n++
		}
	}
	if n != len(v.Promotion) {
		t.Errorf("%d promotion moves, want %d", n, len(v.Promotion))
	}
}

func play(t *testing.T, p *Position, from, to string) *Position {
	t.Helper()
	return p.Apply(findMove(t, p, from, to))
}

func findMove(t *testing.T, p *Position, from, to string) Move {
	t.Helper()
	f, _ := p.V.ParseSquare(from)
	s, _ := p.V.ParseSquare(to)
	for _, m := range p.Moves() {
		if m.From == f && m.To == s {
			return m
		}
	}
	t.Fatalf("no move %s-%s", from, to)
	return Move{}
}

func pieceLetter(p *Position, sq int) byte {
	pc := p.Board[sq]
	if pc.Type == 0 {
		return 0
	}
	return p.V.Type(pc).Letter
}

package chess

import (
	"slices"
	"testing"
)

func duckSquare(t *testing.T, v *Variant, name string) int {
	t.Helper()
	sq, err := v.ParseSquare(name)
	if err != nil {
		t.Fatal(err)
	}
	return sq
}

func duckMove(t *testing.T, p *Position, from, to, blocker string) Move {
	t.Helper()
	f, d, b := duckSquare(t, p.V, from), duckSquare(t, p.V, to), duckSquare(t, p.V, blocker)
	for _, m := range p.Moves() {
		if m.From == f && m.To == d && m.Blocker == b+1 {
			return m
		}
	}
	t.Fatalf("no compound move %s-%s with duck on %s", from, to, blocker)
	return Move{}
}

func TestDuckBuiltinAndOpeningTurns(t *testing.T) {
	v := builtin(t, "duck")
	base := builtin(t, "chess")
	if v.Files != 8 || v.Ranks != 8 || v.Setup != base.Setup || !v.Blocker || v.Pieces[v.TypeOf('K')-1].Royal || !slices.Equal(v.Extinction, []uint8{v.TypeOf('K')}) {
		t.Fatal("duck variant did not inherit chess with a non-royal extinction king and blocker")
	}
	p := NewPosition(v)
	if p.Blocker != -1 {
		t.Fatalf("initial duck square %d, want off-board", p.Blocker)
	}
	moves := p.Moves()
	if len(moves) != 20*32 {
		t.Fatalf("opening compound turns = %d, want 640", len(moves))
	}
	for _, m := range moves {
		if m.Blocker == 0 {
			t.Fatal("opening move without mandatory duck placement")
		}
		q := p.Apply(m)
		if q.Blocker != m.Blocker-1 || q.Board[q.Blocker].Type != 0 || q.Turn != Black {
			t.Fatalf("bad resulting placement for %+v", m)
		}
	}
}

func TestDuckRelocationAndUndo(t *testing.T) {
	v := builtin(t, "duck")
	p := NewPosition(v)
	m := NewMatch(v)
	m.Play(duckMove(t, p, "e2", "e4", "e2"))
	q := m.Position()
	old := duckSquare(t, v, "e2")
	for _, mv := range q.Moves() {
		if mv.Blocker == 0 || mv.Blocker-1 == old {
			t.Fatalf("duck failed to relocate: %+v", mv)
		}
		r := q.Apply(mv)
		if r.Board[r.Blocker].Type != 0 {
			t.Fatalf("duck placed on a piece: %+v", mv)
		}
	}
	m.Play(duckMove(t, q, "e7", "e5", "e7"))
	if !m.Undo() || m.Position().Hash() != q.Hash() || m.Position().Blocker != old {
		t.Fatal("undo did not restore duck position")
	}
	if !m.Undo() || m.Position().Hash() != p.Hash() || m.Position().Blocker != -1 {
		t.Fatal("undo did not restore initial position")
	}
}

func TestDuckBlocksPieces(t *testing.T) {
	v := builtin(t, "duck")
	for _, tc := range []struct {
		name, position, from string
		forbidden, allowed   []string
	}{
		{"rook", "7k/8/8/8/3*4/8/8/K2R4 w -", "d1", []string{"d4", "d5", "d8"}, []string{"d2", "d3"}},
		{"bishop", "7k/8/8/8/3*4/8/1B6/K7 w -", "b2", []string{"d4", "e5", "h8"}, []string{"c3"}},
		{"knight landing", "7k/8/8/8/2*5/8/1N6/K7 w -", "b2", []string{"c4"}, []string{"a4", "d3"}},
		{"knight jumps", "7k/8/8/8/8/1*6/1N6/K7 w -", "b2", nil, []string{"a4", "c4", "d3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fen(t, v, tc.position)
			dests := destinations(p, tc.from)
			for _, s := range tc.forbidden {
				if slices.Contains(dests, s) {
					t.Errorf("duck did not block %s", s)
				}
			}
			for _, s := range tc.allowed {
				if !slices.Contains(dests, s) {
					t.Errorf("missing legal destination %s", s)
				}
			}
		})
	}
}

func TestDuckKingCaptureAndNoCheck(t *testing.T) {
	v := builtin(t, "duck")
	p := fen(t, v, "4k3/8/8/8/8/8/4r3/4K2R w -")
	if p.InCheck() {
		t.Fatal("duck chess has no check")
	}
	// The king may move into the rook's attack, and ignore an existing attack.
	duckMove(t, p, "e1", "d1", "e1")
	duckMove(t, p, "h1", "h2", "h1")
	p = fen(t, v, "4k3/4R3/8/8/8/8/8/K7 w -")
	m := NewMatch(v)
	m.Load(p)
	mv := duckMove(t, p, "e7", "e8", "e7")
	m.Play(mv)
	if st, winner := m.Status(); st != ExtinctionWin || winner != White || !m.Over() {
		t.Fatalf("king capture: status %v winner %v", st, winner)
	}
	if m.Position().Blocker != duckSquare(t, v, "e7") {
		t.Fatal("winning move did not place duck")
	}
}

func TestDuckCastling(t *testing.T) {
	v := builtin(t, "duck")
	for _, tc := range []struct {
		name, position string
		want           bool
	}{
		{"attacked transit", "k4r2/8/8/8/8/8/8/4K2R w K", true},
		{"duck on transit", "k7/8/8/8/8/8/8/4K*1R w K", false},
		{"duck on destination", "k7/8/8/8/8/8/8/4K1*R w K", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fen(t, v, tc.position)
			found := false
			for _, mv := range p.Moves() {
				if mv.Castle >= 0 {
					found = true
					break
				}
			}
			if found != tc.want {
				t.Fatalf("castle available = %v, want %v", found, tc.want)
			}
		})
	}
}

func TestDuckFENAndHash(t *testing.T) {
	v := builtin(t, "duck")
	p := fen(t, v, "7k/8/8/8/3*4/8/8/K7 b -")
	if p.Blocker != duckSquare(t, v, "d4") {
		t.Fatal("FEN did not locate duck")
	}
	q := fen(t, v, "7k/8/8/8/3*4/8/8/K7 b -")
	other := fen(t, v, "7k/8/8/8/4*3/8/8/K7 b -")
	if p.Hash() != q.Hash() || p.Hash() == other.Hash() || !slices.Equal(p.Board, q.Board) {
		t.Fatal("FEN-equivalent positions or blocker hashes disagree")
	}
	for _, bad := range []string{"7k/8/8/8/3**3/8/8/K7 w -", "7k/8/8/8/3*4/8/8/K7 w -"} {
		variant := v
		if bad == "7k/8/8/8/3*4/8/8/K7 w -" {
			variant = builtin(t, "chess")
		}
		if _, err := ParseFEN(variant, bad); err == nil {
			t.Errorf("accepted invalid FEN %q for %s", bad, variant.ID)
		}
	}
}

func TestDuckSearchReturnsCompoundLegalMove(t *testing.T) {
	v := builtin(t, "duck")
	p := NewPosition(v)
	r := NewEngine(v).Think(p, nil, Limits{Nodes: 2000, Depth: 2})
	if !r.Found || r.Move.Blocker == 0 || !slices.Contains(p.Moves(), r.Move) {
		t.Fatalf("search returned illegal compound move: %+v", r)
	}
}

func TestDuckDoesNotChangeOrthodoxOrAtomic(t *testing.T) {
	for _, id := range []string{"chess", "atomic"} {
		v := builtin(t, id)
		p := NewPosition(v)
		if v.Blocker || len(p.Moves()) != 20 {
			t.Fatalf("%s opening altered", id)
		}
		for _, mv := range p.Moves() {
			if mv.Blocker != 0 {
				t.Fatalf("%s has a duck move", id)
			}
		}
	}
}

func TestDuckStalematedSideWins(t *testing.T) {
	v := builtin(t, "duck")
	p := fen(t, v, "kp*5/pp6/pp6/pp6/pp6/pp6/pp6/pp5K b -")
	m := NewMatch(v)
	m.Load(p)
	if len(m.Legal()) != 0 {
		t.Fatal("expected no legal turns")
	}
	// stalemate: loss leaves the stalemated side the winner.
	if st, winner := m.Status(); st != Stalemate || winner != Black || !m.Over() {
		t.Fatalf("stalemate: status %v winner %v", st, winner)
	}
}

// The duck is board-agnostic: it works on a 10-wide board, alongside pockets,
// and stays visible in a fog variant that inherits the blocker.
func TestDuckOnWideBoardsDropsAndFog(t *testing.T) {
	vs, err := parseVariants(map[string][]byte{
		"duckgrand.yaml": []byte(`id: duckgrand
extends: duck
files: 10
ranks: 10
setup: rnbqkbnn1r/pppppppppp/10/10/10/10/10/10/PPPPPPPPPP/RNBQKBN2R
`),
		"duckdrops.yaml": []byte(`id: duckdrops
extends: duck
drops: true
`),
		"duckfog.yaml": []byte(`id: duckfog
extends: fog
blocker: true
`),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tf := range []struct{ id, fen string }{
		{"duckgrand", "rnbqkbnn1r/pppppppppp/4*5/10/10/10/10/10/PPPPPPPPPP/RNBQKBN2R w -"},
		{"duckdrops", "7k/8/8/8/4*3/8/8/K7 w -"},
		{"duckfog", "7k/8/8/8/4*3/8/8/K7 w -"},
	} {
		v, err := Find(vs, tf.id)
		if err != nil {
			t.Fatal(err)
		}
		if !v.Blocker {
			t.Fatalf("%s: blocker not inherited", tf.id)
		}
		p, err := ParseFEN(v, tf.fen)
		if err != nil {
			t.Fatalf("%s: %v", tf.id, err)
		}
		if p.Blocker < 0 {
			t.Fatalf("%s: duck not parsed", tf.id)
		}
		moves := p.Moves()
		if len(moves) == 0 || moves[0].Blocker == 0 {
			t.Fatalf("%s: compound moves missing", tf.id)
		}
		sampled := p.searchMoves(false, Move{})
		for _, m := range moves {
			// Every piece move survives sampling, with some duck destination.
			if !slices.ContainsFunc(sampled, func(s Move) bool {
				return s.From == m.From && s.To == m.To && s.Take == m.Take && s.Promote == m.Promote
			}) {
				t.Fatalf("%s: sampled search misses %+v", tf.id, m)
			}
		}
		q, err := ParseFEN(v, p.FEN())
		if err != nil || q.Blocker != p.Blocker || q.Hash() != p.Hash() {
			t.Fatalf("%s: FEN round trip %q: %v", tf.id, p.FEN(), err)
		}
		if !p.Visible(White)[p.Blocker] {
			t.Fatalf("%s: duck hidden from fog", tf.id)
		}
	}
}

// A duck is not an empty square: nothing may be dropped on it, and a pawn may
// not step over or onto it.
func TestDuckBlocksDropsAndDoubleSteps(t *testing.T) {
	vs, err := parseVariants(map[string][]byte{"duckdrops.yaml": []byte(`id: duckdrops
extends: duck
drops: true
`)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "duckdrops")
	if err != nil {
		t.Fatal(err)
	}
	p := fen(t, v, "7k/8/8/8/4*3/8/8/K7[N] w -")
	drop := false
	for _, m := range p.Moves() {
		if m.Drop == 0 {
			continue
		}
		drop = true
		if m.To == p.Blocker {
			t.Fatal("dropped a pocket piece on the duck")
		}
		if m.Blocker == 0 {
			t.Fatal("drop without a duck relocation")
		}
	}
	if !drop {
		t.Fatal("no drops generated")
	}
	from, to := duckSquare(t, v, "e2"), duckSquare(t, v, "e4")
	for _, tc := range []struct {
		duck string
		want bool
	}{
		{"e3", false}, // the duck blocks the passed square
		{"e4", false}, // and the landing square
		{"d2", true},
	} {
		pos := fen(t, v, "7k/8/8/8/8/8/4P3/K7 w -")
		pos.Blocker = duckSquare(t, v, tc.duck)
		found := false
		for _, m := range pos.Moves() {
			if m.From == from && m.To == to {
				found = true
			}
		}
		if found != tc.want {
			t.Fatalf("pawn e2-e4 with the duck on %s = %v, want %v", tc.duck, found, tc.want)
		}
	}
}

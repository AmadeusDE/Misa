package chess

import "testing"

func TestFogVariantInheritance(t *testing.T) {
	v := builtin(t, "fog")
	if !v.Fog || v.Pieces[v.TypeOf('K')-1].Royal || len(v.Extinction) != 1 {
		t.Fatal("fog must have visibility and non-royal extinction king")
	}
	vs, err := parseVariants(map[string][]byte{
		"inherit.yaml": []byte("id: inherit\nextends: fog\n"),
		"clear.yaml":   []byte("id: clear\nextends: fog\nfog: false\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		id   string
		want bool
	}{{"inherit", true}, {"clear", false}} {
		v, err := Find(vs, tt.id)
		if err != nil {
			t.Fatal(err)
		}
		if v.Fog != tt.want {
			t.Errorf("%s fog = %v", tt.id, v.Fog)
		}
	}
}

func assertVisible(t *testing.T, p *Position, c Color, names ...string) {
	t.Helper()
	want := make([]bool, len(p.Board))
	for _, name := range names {
		want[mustFogSquare(t, p.V, name)] = true
	}
	got := p.Visible(c)
	for sq := range want {
		if got[sq] != want[sq] {
			t.Errorf("%s visibility = %v, want %v", formatSquare(p.V, sq), got[sq], want[sq])
		}
	}
}

func TestFogVisibility(t *testing.T) {
	v := builtin(t, "fog")
	p := fen(t, v, "4k3/8/8/8/3p4/8/3P4/R3K3 w -")
	assertVisible(t, p, White, "a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "b1", "c1", "d1", "e1", "f1", "d2", "e2", "f2", "d3")
	assertVisible(t, p, Black, "e8", "d8", "f8", "d7", "e7", "f7", "d4", "d3")
}

func TestVisibilityRespectsRoyalPins(t *testing.T) {
	v := builtin(t, "chess")
	p := fen(t, v, "k3r3/8/8/8/8/8/4R3/4K3 w -")
	assertVisible(t, p, White, "e1", "e2", "d1", "f1", "d2", "f2", "e3", "e4", "e5", "e6", "e7", "e8")
}

func TestVisibilityEnPassantOnlyForSideToMove(t *testing.T) {
	v := builtin(t, "fog")
	p := fen(t, v, "4k3/3p4/8/4P3/8/8/8/4K3 b -")
	from, to := mustFogSquare(t, v, "d7"), mustFogSquare(t, v, "d5")
	played := false
	for _, m := range p.Moves() {
		if m.From == from && m.To == to {
			p = p.Apply(m)
			played = true
			break
		}
	}
	if !played {
		t.Fatal("black double pawn push unavailable")
	}
	d6 := mustFogSquare(t, v, "d6")
	if !p.Visible(White)[d6] {
		t.Fatal("en passant target should be visible")
	}
	p.Turn = Black
	if p.Visible(White)[d6] {
		t.Fatal("expired en passant target visible to other side")
	}
}

func TestFogCaptureKingAndCastleThroughAttack(t *testing.T) {
	v := builtin(t, "fog")
	m := NewMatch(v)
	m.Load(fen(t, v, "4k3/8/8/8/8/8/4R3/4K3 w -"))
	playAll(t, m, "e2", "e8")
	if st, winner := m.Status(); st != ExtinctionWin || winner != White {
		t.Fatalf("status = %v winner %v", st, winner)
	}
	p := fen(t, v, "k4r2/8/8/8/8/8/8/4K2R w K")
	found := false
	for _, mv := range p.Moves() {
		if mv.Castle >= 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("non-royal king cannot castle through attack")
	}
}

func TestFogSearchCapturesKing(t *testing.T) {
	v := builtin(t, "fog")
	p := fen(t, v, "4k3/8/8/8/8/8/4R3/4K3 w -")
	r := think(t, p, 2)
	if !IsMateScore(r.Score) || p.Apply(r.Move).Board[mustFogSquare(t, v, "e8")].Color != White {
		t.Fatalf("search failed to capture king: %+v", r)
	}
}

func mustFogSquare(t *testing.T, v *Variant, name string) int {
	t.Helper()
	sq, err := v.ParseSquare(name)
	if err != nil {
		t.Fatal(err)
	}
	return sq
}

func TestFogStartAndSearch(t *testing.T) {
	v := builtin(t, "fog")
	if v.Files != 8 || v.Ranks != 8 || len(v.Pieces) != 6 || len(v.Castling) != 2 {
		t.Fatalf("unexpected fog setup: %dx%d, %d pieces, %d castles", v.Files, v.Ranks, len(v.Pieces), len(v.Castling))
	}
	p := NewPosition(v)
	r := NewEngine(v).Think(p, nil, Limits{Depth: 2, Nodes: 500})
	if !r.Found {
		t.Fatal("no search move")
	}
	for _, m := range p.Moves() {
		if m == r.Move {
			return
		}
	}
	t.Fatalf("search returned illegal move: %+v", r.Move)
}

// TestVisibilityEveryBundledVariant runs the mask on every bundled board: the
// mask is board sized, a side always sees its own pieces, and a kingless side
// (horde's Black) still gets a mask. This covers the 10-wide and palace-zone
// boards and the boards whose drops open extra squares.
func TestVisibilityEveryBundledVariant(t *testing.T) {
	vs, errs := Builtins()
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, v := range vs {
		p := NewPosition(v)
		for _, c := range []Color{White, Black} {
			vis := p.Visible(c)
			if len(vis) != v.Size() {
				t.Fatalf("%s: mask has %d squares, board has %d", v.ID, len(vis), v.Size())
			}
			for sq, pc := range p.Board {
				if pc.Type != 0 && pc.Color == c && !vis[sq] {
					t.Fatalf("%s: %s does not see its own piece on %s", v.ID, c, formatSquare(v, sq))
				}
			}
		}
	}
	// Horde's White has no royal at all; the mask must still work for it.
	horde := NewPosition(builtin(t, "horde"))
	if len(horde.Royals(White)) != 0 {
		t.Fatal("horde's White should have no royal")
	}
	if !horde.Visible(White)[mustFogSquare(t, horde.V, "e4")] {
		t.Fatal("kingless horde side has no visible squares")
	}
}

// TestVisibilityIncludesDropSquares checks that a pocket square a side may drop
// onto is visible to that side, so a fog drop variant can show its drops.
func TestVisibilityIncludesDropSquares(t *testing.T) {
	v := builtin(t, "crazyhouse")
	p := fen(t, v, "4k3/8/8/8/8/8/8/4K3 w -")
	p.Hands[White][v.TypeOf('Q')] = 1
	vis := p.Visible(White)
	drops := 0
	for _, m := range p.Moves() {
		if m.Drop == 0 {
			continue
		}
		drops++
		if !vis[m.To] {
			t.Fatalf("drop on %s is invisible", formatSquare(v, m.To))
		}
	}
	if drops == 0 {
		t.Fatal("no drop moves to check")
	}
}

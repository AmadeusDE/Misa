package chess

import "testing"

func restrictedDropVariant(t *testing.T) *Variant {
	t.Helper()
	src := `id: restricted
extends: crazyhouse
pieces:
  - {letter: P, name: Pawn, betza: fmWfcFifmW2, pawn: true, drop_one_per_file: true}
  - {letter: Q, name: Queen, betza: Q, demote: P, drop_no_mate: true}
  - {letter: R, name: Rook, betza: R, demote: P, drop_no_mate: true}
`
	vs, err := parseVariants(map[string][]byte{"restricted.yaml": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "restricted")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func hasDrop(p *Position, letter byte, square string) bool {
	sq, _ := p.V.ParseSquare(square)
	for _, m := range p.Moves() {
		if m.Drop == p.V.TypeOf(letter) && m.To == sq {
			return true
		}
	}
	return false
}

func TestDropOnePerFile(t *testing.T) {
	v := restrictedDropVariant(t)
	if !v.Pieces[v.TypeOf('P')-1].DropOnePerFile || !v.Pieces[v.TypeOf('Q')-1].DropNoMate {
		t.Fatal("drop rule YAML not loaded")
	}
	for _, tc := range []struct {
		name, fen, square string
		want              bool
	}{
		{"own pawn", "4k3/8/8/8/2P5/8/8/4K3[P] w - - 0 1", "c3", false},
		{"other file", "4k3/8/8/8/2P5/8/8/4K3[P] w - - 0 1", "d3", true},
		{"opponent pawn", "4k3/8/8/8/2p5/8/8/4K3[P] w - - 0 1", "c3", true},
		{"promoted pawn", "4k3/8/8/8/2Q~5/8/8/4K3[P] w - - 0 1", "c3", true},
		{"black own pawn", "4k3/8/8/2p5/8/8/8/4K3[p] b - - 0 1", "c6", false},
		{"black opponent pawn", "4k3/8/8/2P5/8/8/8/4K3[p] b - - 0 1", "c6", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fen(t, v, tc.fen)
			if got := hasDrop(p, 'P', tc.square); got != tc.want {
				t.Errorf("drop at %s = %v, want %v", tc.square, got, tc.want)
			}
		})
	}
}

func TestDropNoMate(t *testing.T) {
	v := restrictedDropVariant(t)
	for _, tc := range []struct {
		name, fen, square string
		want              bool
	}{
		{"mate forbidden", "k7/8/2K5/8/8/8/8/8[Q] w - - 0 1", "b7", false},
		{"check with escape", "k7/8/8/8/8/8/8/7K[Q] w - - 0 1", "b7", true},
		{"stalemate allowed", "k7/8/2K5/8/8/8/8/8[Q] w - - 0 1", "b6", true},
		{"black mate forbidden", "8/8/8/8/8/2k5/8/K7[q] b - - 0 1", "b2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fen(t, v, tc.fen)
			if got := hasDrop(p, 'Q', tc.square); got != tc.want {
				t.Errorf("drop at %s = %v, want %v", tc.square, got, tc.want)
			}
		})
	}
	// A pocket drop can interpose against a sliding check and avert mate.
	for _, tc := range []struct {
		pocket string
		want   bool
	}{{"R", false}, {"Rn", true}} {
		p := fen(t, v, "k7/2K5/8/8/8/8/8/8["+tc.pocket+"] w - - 0 1")
		if got := hasDrop(p, 'R', "a4"); got != tc.want {
			t.Errorf("rook drop with pockets %s = %v, want %v", tc.pocket, got, tc.want)
		}
	}
	// The restriction belongs to the dropped piece, not to the variant.
	plain := builtin(t, "crazyhouse")
	if !hasDrop(fen(t, plain, "k7/8/2K5/8/8/8/8/8[Q] w - - 0 1"), 'Q', "b7") {
		t.Error("unrestricted queen drop mate was rejected")
	}
}

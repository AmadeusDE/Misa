package chess

import (
	"slices"
	"strings"
	"testing"
)

func builtin(t *testing.T, id string) *Variant {
	t.Helper()
	vs, errs := Builtins()
	if len(errs) > 0 {
		t.Fatalf("builtin variants: %v", errs)
	}
	v, err := Find(vs, id)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestBuiltinVariants(t *testing.T) {
	tests := []struct {
		id           string
		files, ranks int
		pieces       int
		castles      int
	}{
		{"chess", 8, 8, 6, 2},
		{"double", 16, 12, 6, 4},
		{"bigboard", 16, 16, 12, 2},
		{"doublebig", 32, 16, 12, 4},
		{"amazon", 8, 8, 7, 2},
		{"nocastling", 8, 8, 6, 0},
		{"capablanca", 10, 8, 8, 2},
		{"berolina", 8, 8, 6, 2},
		{"stalematewins", 8, 8, 6, 2},
		{"selfcapture", 8, 8, 6, 2},
		{"kingofthehill", 8, 8, 6, 2},
		{"grand", 10, 10, 8, 0},
		{"threecheck", 8, 8, 6, 2},
	}
	for _, tt := range tests {
		v := builtin(t, tt.id)
		if v.Files != tt.files || v.Ranks != tt.ranks || len(v.Pieces) != tt.pieces || len(v.Castling) != tt.castles {
			t.Errorf("%s: %d×%d, %d pieces, %d castles", tt.id, v.Files, v.Ranks, len(v.Pieces), len(v.Castling))
		}
	}
}

func TestExtendsOverridesPieces(t *testing.T) {
	v := builtin(t, "double")
	pawn := v.Pieces[v.TypeOf('P')-1]
	if pawn.Betza != "fmWfcFifmW4" || !pawn.Pawn {
		t.Errorf("double pawn = %+v", pawn)
	}
	if v.TypeOf('Q') == 0 || v.Pieces[v.TypeOf('K')-1].Glyph != "♚" {
		t.Error("double chess lost the inherited pieces")
	}
}

func TestNoCastlingMoveRulesAndSearch(t *testing.T) {
	v := builtin(t, "nocastling")
	p := fen(t, v, "r3k2r/8/8/8/8/8/8/R3K2R w")
	if got := destinations(p, "e1"); !slices.Equal(got, []string{"d1", "d2", "e2", "f1", "f2"}) {
		t.Fatalf("king moves = %v", got)
	}
	for _, m := range p.Moves() {
		if m.From == mustSquare(t, v, "e1") && (m.To == mustSquare(t, v, "g1") || m.To == mustSquare(t, v, "c1")) {
			t.Fatal("castling move generated")
		}
	}
	result := NewEngine(v).Think(p, nil, Limits{Depth: 1})
	if !result.Found || !slices.Contains(p.Moves(), result.Move) {
		t.Fatalf("search move %v is not legal", result.Move)
	}
}

func mustSquare(t *testing.T, v *Variant, name string) int {
	t.Helper()
	sq, err := v.ParseSquare(name)
	if err != nil {
		t.Fatalf("invalid square %s", name)
	}
	return sq
}

func TestNoCastlingVariant(t *testing.T) {
	v := builtin(t, "nocastling")
	if v.Files != 8 || v.Ranks != 8 || len(v.Pieces) != 6 || len(v.Castling) != 0 {
		t.Fatalf("unexpected variant: %+v", v)
	}
}

func TestFind(t *testing.T) {
	vs, _ := Builtins()
	for name, want := range map[string]string{"chess": "chess", "Double Chess": "double", "big": "bigboard", "DOUBLE": "double"} {
		v, err := Find(vs, name)
		if err != nil || v.ID != want {
			t.Errorf("Find(%q) = %v, %v; want %s", name, v, err, want)
		}
	}
	if _, err := Find(vs, "nope"); err == nil {
		t.Error("Find(nope) found something")
	}
}

func TestParseVariantErrors(t *testing.T) {
	base := `
id: tiny
name: Tiny
files: 3
ranks: 3
pieces:
  - {letter: K, name: King, betza: K, royal: true}
  - {letter: R, name: Rook, betza: R}
setup: k2/3/K1R
`
	if _, err := parseVariants(map[string][]byte{"tiny.yaml": []byte(base)}); err != nil {
		t.Fatalf("valid variant: %v", err)
	}
	tests := map[string]string{
		"setup width":    strings.Replace(base, "K1R", "K2R", 1),
		"setup ranks":    strings.Replace(base, "k2/3/K1R", "k2/K1R", 1),
		"unknown piece":  strings.Replace(base, "K1R", "K1X", 1),
		"bad betza":      strings.Replace(base, "betza: R}", "betza: Y}", 1),
		"duplicate":      strings.Replace(base, "letter: R", "letter: K", 1),
		"too wide":       strings.Replace(base, "files: 3", "files: 703", 1),
		"missing parent": base + "extends: nothing\n",
		"wide glyph":     strings.Replace(base, "betza: R}", "betza: R, glyph: 漢}", 1),
		"castle square":  base + "castling: [{king: a1, rook: z1, king_to: b1, rook_to: c1}]\n",
		"promotion":      base + "promotion: [X]\n",
		"goal square":    base + "goal: [d4]\n",
		"goal duplicate": base + "goal: [a1, a1]\n",
	}
	for name, src := range tests {
		if _, err := parseVariants(map[string][]byte{"tiny.yaml": []byte(src)}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestExplicitEmptyCastlingDropsInherited(t *testing.T) {
	src := `
id: nocastle
name: No castling
extends: chess
castling: []
`
	vs, err := parseVariants(map[string][]byte{"a.yaml": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := Find(vs, "nocastle")
	if len(v.Castling) != 0 {
		t.Errorf("castling = %v, want none", v.Castling)
	}
}

func TestExternalOverridesBuiltin(t *testing.T) {
	src := `
id: chess
name: Chess but bigger
extends: chess
files: 10
setup: rnbqkbnr2/pppppppp2/10/10/10/10/PPPPPPPP2/RNBQKBNR2
`
	vs, err := parseVariants(map[string][]byte{"chess.yaml": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := Find(vs, "chess")
	if v.Files != 10 || v.Name != "Chess but bigger" {
		t.Errorf("got %s %d files", v.Name, v.Files)
	}
}

func TestWideVariantIntegration(t *testing.T) {
	const src = `
id: wide-test
name: Wide test
files: 30
ranks: 3
pieces:
  - {letter: K, name: King, betza: K, royal: true}
  - {letter: R, name: Rook, betza: R, value: 500}
setup: k29/30/K25R3
`
	vs, err := parseVariants(map[string][]byte{"wide-test.yaml": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "wide-test")
	if err != nil {
		t.Fatal(err)
	}
	p := NewPosition(v)
	// The rook on file 27 can move across the old 26-file boundary.
	var move Move
	found := false
	for _, m := range p.Moves() {
		if m.From == 26 && m.To == 27 {
			move, found = m, true
			break
		}
	}
	if !found {
		t.Fatal("missing legal rook move from file 27 to file 28")
	}
	before := p.Hash()
	if before != NewPosition(v).Hash() {
		t.Fatal("identical positions have different hashes")
	}
	next := p.Apply(move)
	if next.Hash() == before {
		t.Fatal("hash did not change after move")
	}
	if next.Hash() != p.Apply(move).Hash() {
		t.Fatal("same move produced different hashes")
	}
}

func TestVariantDimensionBounds(t *testing.T) {
	// An invalid setup lets us test the dimension gate without building the
	// potentially enormous reach tables for a maximum-sized board.
	for _, tt := range []struct {
		files, ranks int
		valid        bool
	}{
		{1, 1, true}, {26, 1, true}, {27, 1, true}, {702, 1, true},
		{1, 99, true}, {0, 1, false}, {703, 1, false},
		{1, 0, false}, {1, 100, false},
	} {
		d := variantDef{Files: tt.files, Ranks: tt.ranks, Pieces: []pieceDef{{Letter: "K", Betza: "K"}}}
		_, err := build(d)
		if err == nil {
			t.Fatalf("%dx%d: expected setup error", tt.files, tt.ranks)
		}
		if got := !strings.Contains(err.Error(), "board must be"); got != tt.valid {
			t.Errorf("%dx%d: %v; dimension validity want %v", tt.files, tt.ranks, err, tt.valid)
		}
	}
}

func TestAmazonMovesAsQueenAndKnight(t *testing.T) {
	v := builtin(t, "amazon")
	p := fen(t, v, "4k3/8/8/8/8/8/8/3AK3 w")
	got := destinations(p, "d1")
	for _, want := range []string{"b2", "c3", "e3", "f2", "d8", "a1", "a4", "h5"} {
		if !slices.Contains(got, want) {
			t.Fatalf("amazon on d1 cannot reach %s: %v", want, got)
		}
	}
	start := NewPosition(v)
	result := NewEngine(v).Think(start, nil, Limits{Depth: 1})
	if !result.Found || !slices.Contains(start.Moves(), result.Move) {
		t.Fatalf("search move %v is not legal", result.Move)
	}
}
func TestGoalSquares(t *testing.T) {
	v := builtin(t, "kingofthehill")
	want := []int{}
	for _, name := range []string{"d4", "d5", "e4", "e5"} {
		sq, err := v.ParseSquare(name)
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, sq)
	}
	if !slices.Equal(v.Goal, want) {
		t.Errorf("%s goal = %v, want %v", v.ID, v.Goal, want)
	}
	if goal := builtin(t, "chess").Goal; len(goal) != 0 {
		t.Errorf("chess goal = %v, want none", goal)
	}
}

func TestGoalInheritanceAndClearing(t *testing.T) {
	src := map[string][]byte{
		"inherits.yaml":  []byte("id: inherits\nextends: kingofthehill\n"),
		"overrides.yaml": []byte("id: overrides\nextends: kingofthehill\ngoal: [a1]\n"),
		"cleared.yaml":   []byte("id: cleared\nextends: kingofthehill\ngoal: []\n"),
	}
	vs, err := parseVariants(src)
	if err != nil {
		t.Fatal(err)
	}
	check := func(id string, n int) *Variant {
		t.Helper()
		v, err := Find(vs, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(v.Goal) != n {
			t.Errorf("%s goal = %v, want %d squares", id, v.Goal, n)
		}
		return v
	}
	check("inherits", 4)
	over := check("overrides", 1)
	if a1, _ := over.ParseSquare("a1"); len(over.Goal) == 1 && over.Goal[0] != a1 {
		t.Errorf("overrides goal = %v, want a1", over.Goal)
	}
	check("cleared", 0)
}

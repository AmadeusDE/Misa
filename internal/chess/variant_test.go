package chess

import (
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
		"too wide":       strings.Replace(base, "files: 3", "files: 27", 1),
		"missing parent": base + "extends: nothing\n",
		"wide glyph":     strings.Replace(base, "betza: R}", "betza: R, glyph: 漢}", 1),
		"castle square":  base + "castling: [{king: a1, rook: z1, king_to: b1, rook_to: c1}]\n",
		"promotion":      base + "promotion: [X]\n",
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

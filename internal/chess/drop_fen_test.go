package chess

import (
	"strings"
	"testing"
)

func TestDropSchemaAndFEN(t *testing.T) {
	vs, errs := Builtins()
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	v, err := Find(vs, "crazyhouse")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Drops || len(v.Pieces[v.TypeOf('P')-1].DropForbidRanks) != 2 {
		t.Fatal("bad drop schema")
	}
	fen := "4k3/8/8/3Q~4/8/8/8/4K3[PPn] b - - 12 37"
	p, err := ParseFEN(v, fen)
	if err != nil {
		t.Fatal(err)
	}
	if p.Board[v.Square(3, 4)].Promoted != v.TypeOf('P') || p.Hands[White][v.TypeOf('P')] != 2 || p.Hands[Black][v.TypeOf('N')] != 1 {
		t.Fatal("lost promoted or pocket")
	}
	q, err := ParseFEN(v, p.FEN())
	if err != nil {
		t.Fatal(err)
	}
	if q.Hash() != p.Hash() || q.Clock != 12 || q.Fullmove != 37 {
		t.Fatalf("roundtrip %s", p.FEN())
	}
	plain, err := ParseFEN(v, strings.Replace(fen, "Q~", "Q", 1))
	if err != nil {
		t.Fatal(err)
	}
	if plain.Hash() == p.Hash() {
		t.Fatal("promotion not hashed")
	}
	plain, err = ParseFEN(v, strings.Replace(fen, "PPn", "Pn", 1))
	if err != nil {
		t.Fatal(err)
	}
	if plain.Hash() == p.Hash() {
		t.Fatal("hand not hashed")
	}
	for _, bad := range []string{"4k3/8/8/8/8/8/8/4K3[K]", "4k3/8/8/8/8/8/8/4K3[P~]", "4k3/8/8/8/8/8/8/4K~3[]"} {
		if _, err := ParseFEN(v, bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestDropSchemaValidation(t *testing.T) {
	for _, spec := range []string{"drop_forbid_ranks: [0]", "drop_forbid_ranks: [9]", "demote: Z"} {
		_, err := parseVariants(map[string][]byte{"bad.yaml": []byte("id: bad\nextends: chess\npieces:\n  - {letter: P, betza: W, " + spec + "}\n")})
		if err == nil {
			t.Errorf("accepted %s", spec)
		}
	}
	vs, err := parseVariants(map[string][]byte{"child.yaml": []byte("id: child\nextends: crazyhouse\ndrops: false\n")})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := Find(vs, "child")
	if v.Drops {
		t.Fatal("false not inherited")
	}
}

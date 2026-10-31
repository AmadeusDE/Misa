package chess

import "testing"

// A configured castle historically permits any royal on the king square,
// even if it is not the type occupying that square in the setup.
func TestCastleOtherRoyalOnKingSquare(t *testing.T) {
	vs, err := parseVariants(map[string][]byte{"royal-queen.yaml": []byte(`id: royal-queen
extends: chess
pieces:
  - {letter: Q, name: Queen, betza: Q, royal: true}
`)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "royal-queen")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseFEN(v, "k7/8/8/8/8/8/8/4Q2R w K")
	if err != nil {
		t.Fatal(err)
	}
	// FEN infers a piece not on its setup square as moved.
	p.Board[v.orient(v.Castling[0].King, White)].Moved = false
	for _, mv := range p.Moves() {
		if mv.Castle == 0 {
			return
		}
	}
	t.Fatal("castle with a different royal on the king square was rejected")
}

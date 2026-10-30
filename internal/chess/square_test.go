package chess

import (
	"strings"
	"testing"
)

func TestWideSquareNames(t *testing.T) {
	v := &Variant{Files: 702, Ranks: 99}
	for _, tt := range []struct {
		x, y int
		name string
	}{
		{0, 0, "a1"}, {25, 0, "z1"}, {26, 0, "aa1"},
		{51, 98, "az99"}, {52, 9, "ba10"}, {701, 98, "zz99"},
	} {
		sq := v.Square(tt.x, tt.y)
		if got := v.SquareName(sq); got != tt.name {
			t.Errorf("SquareName(%d) = %q, want %q", sq, got, tt.name)
		}
		if got, err := v.ParseSquare(tt.name); err != nil || got != sq {
			t.Errorf("ParseSquare(%q) = %d, %v; want %d", tt.name, got, err, sq)
		}
	}
	for x := 0; x < v.Files; x++ {
		sq := v.Square(x, 42)
		name := v.SquareName(sq)
		if got, err := v.ParseSquare(name); err != nil || got != sq {
			t.Errorf("round trip %d via %q = %d, %v", sq, name, got, err)
		}
	}
	for _, name := range []string{"", "a", "1", "A1", "aaa1", "zz100", "a0", "aa1x", strings.Repeat("z", 1000) + "1", "a999999999999999999999999999999"} {
		if _, err := v.ParseSquare(name); err == nil {
			t.Errorf("ParseSquare(%q) succeeded", name)
		}
	}
}

func TestWideMoveNotationAndCastlingSquares(t *testing.T) {
	v := &Variant{Files: 702, Ranks: 2, Pieces: []*PieceType{{Letter: 'R'}}}
	c, err := v.parseCastle(castleDef{King: "z1", Rook: "ba1", KingTo: "aa1", RookTo: "az1"})
	if err != nil {
		t.Fatal(err)
	}
	if c.King != 25 || c.Rook != 52 || c.KingTo != 26 || c.RookTo != 51 {
		t.Fatalf("wide castling squares = %+v", c)
	}
	p := &Position{V: v, Board: make([]Piece, v.Size())}
	p.Board[25] = Piece{Type: 1}
	for _, tt := range []struct {
		to   int
		want string
	}{
		{26, "Rz1-aa1"}, {51, "Rz1-az1"}, {52, "Rz1-ba1"}, {701, "Rz1-zz1"},
	} {
		if got := describe(p, Move{From: 25, To: tt.to, Take: -1, Castle: -1}); got != tt.want {
			t.Errorf("describe = %q, want %q", got, tt.want)
		}
	}
}

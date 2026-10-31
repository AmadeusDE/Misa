package chess

import (
	"slices"
	"testing"
)

func TestBlockerFENRoundTrip(t *testing.T) {
	v := builtin(t, "duck")
	for _, text := range []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq",
		"rnbqkbnr/pppppppp/8/8/8/4P3/PPPP*PPP/RNBQKBNR b KQkq",
		"7k/8/8/8/3*4/8/8/K7 b -",
	} {
		p, err := ParseFEN(v, text)
		if err != nil {
			t.Fatal(err)
		}
		q, err := ParseFEN(v, p.FEN())
		if err != nil {
			t.Fatal(err)
		}
		if p.Blocker != q.Blocker || p.Turn != q.Turn || !slices.Equal(p.Board, q.Board) || p.Hash() != q.Hash() {
			t.Fatalf("FEN round trip: %q -> %q", text, p.FEN())
		}
	}
}

package chess

import (
	"slices"
	"testing"
)

func TestBlockerSearchSamplesEachPieceMove(t *testing.T) {
	v := builtin(t, "duck")
	p := NewPosition(v)
	full, sampled := p.Moves(), p.searchMoves(false, Move{})
	if len(sampled) != 20*8 || len(full) != 20*32 {
		t.Fatalf("sampled %d, full %d", len(sampled), len(full))
	}
	for _, m := range sampled {
		if !slices.Contains(full, m) {
			t.Fatalf("sampled illegal move %+v", m)
		}
	}
	for _, m := range full {
		if !slices.ContainsFunc(sampled, func(s Move) bool {
			return s.From == m.From && s.To == m.To && s.Promote == m.Promote && s.Castle == m.Castle && s.Take == m.Take
		}) {
			t.Fatalf("missing piece move %+v", m)
		}
	}
	// A transposition-table move outside the sample is retained.
	for _, m := range full {
		if !slices.Contains(sampled, m) {
			if !slices.Contains(p.searchMoves(false, m), m) {
				t.Fatal("preferred move omitted")
			}
			return
		}
	}
	t.Fatal("test did not find an unsampled move")
}

package chess

import "testing"

// The root uses Moves, but interior nodes use pseudoMoves and must also
// enforce the no-drop-mate restriction. Q@b7 would otherwise score as mate.
func TestSearchRejectsForbiddenDropMate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		variant  *Variant
		wantMate bool
	}{
		{"restricted", restrictedDropVariant(t), false},
		{"unrestricted", builtin(t, "crazyhouse"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fen(t, tc.variant, "k7/8/2K5/8/8/8/8/8[Q] w - - 0 1")
			e := NewEngine(tc.variant)
			e.begin(nil, Limits{})
			got := e.negamax(p, 1, 0, -infinity, infinity, false)
			if IsMateScore(got) != tc.wantMate {
				t.Errorf("score = %d, want mate = %v", got, tc.wantMate)
			}
		})
	}
}

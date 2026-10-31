package chess

import (
	"strings"
	"testing"
)

func TestConfiguredStalemateOutcomes(t *testing.T) {
	for _, tc := range []struct {
		rule   StalemateRule
		winner Color
		score  int
	}{
		{StalemateDraw, White, 0},
		{StalemateWin, White, -mateScore + 2},
		{StalemateLoss, Black, mateScore - 2},
	} {
		t.Run(string(tc.rule), func(t *testing.T) {
			v := builtin(t, "chess")
			v.Stalemate = tc.rule
			p := fen(t, v, "k7/8/1QK5/8/8/8/8/8 b -")
			m := NewMatch(v)
			m.Load(p)
			if status, winner := m.Status(); status != Stalemate || winner != tc.winner {
				t.Fatalf("status = %v, winner = %v", status, winner)
			}
			e := NewEngine(v)
			if got := e.negamax(p, 1, 2, -infinity, infinity, false); got != tc.score {
				t.Errorf("negamax = %d, want %d", got, tc.score)
			}
			// Drawn-stalemate variants keep the cheap quiescence search, which
			// does not test for stalemate at its leaves.
			if got := e.quiesce(p, 2, -infinity, -infinity+1); tc.rule != StalemateDraw && got != tc.score {
				t.Errorf("quiesce = %d, want %d", got, tc.score)
			}
			// Checkmate always takes precedence over the stalemate rule.
			m.Load(fen(t, v, "k7/1Q6/2K5/8/8/8/8/8 b -"))
			if status, winner := m.Status(); status != Checkmate || winner != White {
				t.Errorf("mate status = %v, winner = %v", status, winner)
			}
		})
	}
}

func TestConfiguredRepetitionMatch(t *testing.T) {
	v := builtin(t, "chess")
	v.Repetition = 4
	m := NewMatch(v)
	for i := 1; i <= 3; i++ {
		playAll(t, m, "g1", "f3", "g8", "f6", "f3", "g1", "f6", "g8")
		status, _ := m.Status()
		if i < 3 && status != Playing || i == 3 && status != DrawRepetition {
			t.Fatalf("occurrences %d: status %v", i+1, status)
		}
	}
	if !m.Undo() {
		t.Fatal("undo failed")
	}
	if status, _ := m.Status(); status != Playing {
		t.Fatalf("after undo: %v", status)
	}
}

func TestConfiguredStalemateHeader(t *testing.T) {
	v := builtin(t, "chess")
	v.Stalemate = StalemateLoss
	m := NewMatch(v)
	m.Load(fen(t, v, "k7/8/1QK5/8/8/8/8/8 b -"))
	g := &Game{match: m}
	if got := g.header(); !strings.Contains(got, "stalemate, Black wins") {
		t.Errorf("header = %q", got)
	}
}

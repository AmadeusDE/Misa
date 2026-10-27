package chess

import (
	"slices"
	"testing"
)

func playAll(t *testing.T, m *Match, moves ...string) {
	t.Helper()
	for i := 0; i < len(moves); i += 2 {
		m.Play(findMove(t, m.Position(), moves[i], moves[i+1]))
	}
}

func TestNotation(t *testing.T) {
	m := NewMatch(builtin(t, "chess"))
	playAll(t, m, "e2", "e4", "f7", "f6", "d2", "d4", "g7", "g5", "d1", "h5")
	want := []string{"e2-e4", "f7-f6", "d2-d4", "g7-g5", "Qd1-h5#"}
	if got := m.Notation(); !slices.Equal(got, want) {
		t.Errorf("notation = %v, want %v", got, want)
	}
	if st, winner := m.Status(); st != Checkmate || winner != White {
		t.Errorf("status = %v %v", st, winner)
	}
}

func TestCastlingNotation(t *testing.T) {
	v := builtin(t, "chess")
	m := NewMatch(v)
	m.Load(fen(t, v, "r3k3/8/8/8/8/8/8/4K2R w Kq"))
	playAll(t, m, "e1", "g1", "e8", "c8")
	if got := m.Notation(); !slices.Equal(got, []string{"O-O", "O-O-O"}) {
		t.Errorf("notation = %v", got)
	}
}

func TestUndo(t *testing.T) {
	m := NewMatch(builtin(t, "chess"))
	start := m.Position().Hash()
	playAll(t, m, "e2", "e4")
	if !m.Undo() || m.Position().Hash() != start || len(m.Notation()) != 0 || len(m.Legal()) != 20 {
		t.Error("undo did not restore the start")
	}
	if m.Undo() {
		t.Error("undid past the start")
	}
}

func TestThreefoldRepetition(t *testing.T) {
	m := NewMatch(builtin(t, "chess"))
	for range 2 {
		playAll(t, m, "g1", "f3", "g8", "f6", "f3", "g1", "f6", "g8")
	}
	if st, _ := m.Status(); st != DrawRepetition {
		t.Errorf("status = %v, want repetition", st)
	}
}

func TestMoveLimit(t *testing.T) {
	v := builtin(t, "chess")
	m := NewMatch(v)
	p := fen(t, v, "k7/8/8/8/8/8/8/R6K w -")
	p.Clock = 2*v.MoveLimit - 1
	m.Load(p)
	playAll(t, m, "a1", "b1")
	if st, _ := m.Status(); st != DrawMoveLimit {
		t.Errorf("status = %v, want move limit", st)
	}
}

func TestBareKingsDraw(t *testing.T) {
	v := builtin(t, "chess")
	m := NewMatch(v)
	m.Load(fen(t, v, "k7/1R6/8/8/8/8/8/7K b -"))
	playAll(t, m, "a8", "b7")
	if st, _ := m.Status(); st != DrawMaterial {
		t.Errorf("status = %v, want draw by material", st)
	}
}

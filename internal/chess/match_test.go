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

func TestStalemateRule(t *testing.T) {
	chessVariant := builtin(t, "chess")
	if chessVariant.Stalemate != StalemateDraw {
		t.Fatalf("orthodox stalemate rule = %q", chessVariant.Stalemate)
	}
	v := builtin(t, "stalematewins")
	m := NewMatch(v)
	p := fen(t, v, "7k/5Q2/6K1/8/8/8/8/8 b -")
	m.Load(p)
	if st, winner := m.Status(); st != Stalemate || winner != White || !m.Over() {
		t.Fatalf("status = %v, winner %v, over %v", st, winner, m.Over())
	}
	orthodox := NewMatch(chessVariant)
	orthodox.Load(fen(t, chessVariant, "7k/5Q2/6K1/8/8/8/8/8 b -"))
	if st, _ := orthodox.Status(); st != Stalemate || orthodox.Decisive() {
		t.Fatalf("orthodox stalemate = %v, decisive %v", st, orthodox.Decisive())
	}
	if !m.Decisive() {
		t.Fatal("stalemate win is not decisive")
	}
	loss := *v
	loss.Stalemate = StalemateLoss
	lm := NewMatch(&loss)
	lm.Load(fen(t, &loss, "7k/5Q2/6K1/8/8/8/8/8 b -"))
	if _, winner := lm.Status(); winner != Black || !lm.Decisive() {
		t.Fatalf("stalemate loss winner = %v", winner)
	}
}

func TestGoalWin(t *testing.T) {
	v := builtin(t, "kingofthehill")
	m := NewMatch(v)
	m.Load(fen(t, v, "7k/8/8/8/8/2K5/8/8 w -"))
	playAll(t, m, "c3", "d4")
	if st, winner := m.Status(); st != GoalWin || winner != White || !m.Over() {
		t.Fatalf("status = %v, winner %v, over %v", st, winner, m.Over())
	}
}

func TestGoalTakesPrecedence(t *testing.T) {
	v := builtin(t, "kingofthehill")
	for _, tt := range []struct {
		name    string
		fen     string
		noMoves bool
		inCheck bool
	}{
		{"move-limit draw", "r6k/8/8/8/8/2K5/8/R7 w -", false, false},
		{"stalemate", "7k/5Q2/8/8/8/2K5/8/8 w -", true, false},
		{"checkmate", "8/8/5B2/8/8/R1K4k/8/6Q1 w -", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMatch(v)
			p := fen(t, v, tt.fen)
			if tt.name == "move-limit draw" {
				p.Clock = 2*v.MoveLimit - 1
			}
			m.Load(p)
			playAll(t, m, "c3", "d4")
			after := m.Position()
			hasMoves := len(after.Moves()) != 0
			if hasMoves == tt.noMoves || after.InCheck() != tt.inCheck {
				t.Fatalf("after arrival: hasMoves %v, inCheck %v", hasMoves, after.InCheck())
			}
			if tt.name == "move-limit draw" && after.Clock < 2*v.MoveLimit {
				t.Fatal("arrival did not reach the move limit")
			}
			if st, winner := m.Status(); st != GoalWin || winner != White || !m.Decisive() {
				t.Fatalf("status = %v, winner %v, decisive %v; want goal win before %s", st, winner, m.Decisive(), tt.name)
			}
			if o, over := adjudicate(after, hasMoves); !over || o.status != GoalWin || o.drawn || o.winner != White {
				t.Fatalf("adjudicate = %+v, over %v; want goal win before %s", o, over, tt.name)
			}
		})
	}
}

func TestAdjudicateStalemateRules(t *testing.T) {
	win := builtin(t, "stalematewins")
	loss := *win
	loss.Stalemate = StalemateLoss
	for _, tt := range []struct {
		name   string
		v      *Variant
		winner Color
		drawn  bool
		score  int
	}{
		{"win", win, White, false, -mateScore + 4},
		{"loss", &loss, Black, false, mateScore - 4},
		{"draw", builtin(t, "chess"), White, true, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := fen(t, tt.v, "7k/5Q2/6K1/8/8/8/8/8 b -")
			if len(p.Moves()) != 0 || p.InCheck() {
				t.Fatal("position is not a stalemate")
			}
			o, over := adjudicate(p, false)
			if !over || o.status != Stalemate || o.drawn != tt.drawn || (!tt.drawn && o.winner != tt.winner) {
				t.Fatalf("adjudicate = %+v, over %v", o, over)
			}
			if score := o.score(p.Turn, 4); score != tt.score {
				t.Fatalf("score = %d, want %d", score, tt.score)
			}
		})
	}
}

func TestGoalNeedsRoyalArrival(t *testing.T) {
	v := builtin(t, "kingofthehill")
	m := NewMatch(v)
	m.Load(fen(t, v, "7k/8/8/8/8/8/8/3R3K w -"))
	playAll(t, m, "d1", "d4")
	if st, _ := m.Status(); st != Playing || m.Over() {
		t.Fatalf("rook arrival gave status %v", st)
	}
}

func TestGoalOnlyForSideThatJustMoved(t *testing.T) {
	v := builtin(t, "kingofthehill")
	m := NewMatch(v)
	m.Load(fen(t, v, "7k/8/8/8/3K4/8/8/8 b -"))
	if st, winner := m.Status(); st != GoalWin || winner != White {
		t.Fatalf("status = %v, winner %v", st, winner)
	}
}

func TestGoalSquaresAreAbsolute(t *testing.T) {
	src := []byte("id: goala1\nname: Goal a1\nextends: chess\ngoal: [a1]\n")
	vs, err := parseVariants(map[string][]byte{"goala1.yaml": src})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "goala1")
	if err != nil {
		t.Fatal(err)
	}
	m := NewMatch(v)
	m.Load(fen(t, v, "1k6/8/8/8/8/8/8/1K5R b -"))
	playAll(t, m, "b8", "a8")
	if st, _ := m.Status(); st != Playing {
		t.Fatalf("Black's mirrored a8 gave status %v", st)
	}
	playAll(t, m, "b1", "a1")
	if st, winner := m.Status(); st != GoalWin || winner != White {
		t.Fatalf("White's a1 gave status %v, winner %v", st, winner)
	}
}

func TestOrthodoxVariantsIgnoreGoalSquares(t *testing.T) {
	v := builtin(t, "chess")
	m := NewMatch(v)
	m.Load(fen(t, v, "7k/8/8/8/8/2K5/8/7R w -"))
	playAll(t, m, "c3", "d4")
	if st, _ := m.Status(); st != Playing || m.Over() {
		t.Fatalf("chess king in the centre gave status %v", st)
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

func TestGoalSquareInCheckIsIllegal(t *testing.T) {
	v := builtin(t, "kingofthehill")
	m := NewMatch(v)
	m.Load(fen(t, v, "3r3k/8/8/8/8/4K3/8/8 w -"))
	from, _ := v.ParseSquare("e3")
	to, _ := v.ParseSquare("d4")
	for _, mv := range m.Legal() {
		if mv.From == from && mv.To == to {
			t.Fatal("king stepped into check on a goal square")
		}
	}
	if st, _ := m.Status(); st != Playing {
		t.Fatalf("status = %v, want the game still playing", st)
	}
	playAll(t, m, "e3", "e4")
	if st, winner := m.Status(); st != GoalWin || winner != White {
		t.Fatalf("status = %v, winner %v", st, winner)
	}
}

package chess

import (
	"slices"
	"strings"
	"testing"
)

func TestAtomicExtinctionPrecedesOtherOutcomes(t *testing.T) {
	v := builtin(t, "atomic")
	p := fen(t, v, "8/8/8/8/3K4/8/8/8 b -")
	sq, err := v.ParseSquare("d4")
	if err != nil {
		t.Fatal(err)
	}
	v.Goal = []int{sq}
	p.Clock = 2 * v.MoveLimit
	m := NewMatch(v)
	m.Load(p)
	if st, winner := m.Status(); st != ExtinctionWin || winner != White {
		t.Fatalf("status %v winner %v", st, winner)
	}
	if o, over := adjudicate(p, false); !over || o.status != ExtinctionWin {
		t.Fatalf("adjudication %+v, %v", o, over)
	}
	e := NewEngine(v)
	e.begin(nil, Limits{})
	if score := e.negamax(p, 1, 3, -infinity, infinity, true); score != -mateScore+3 {
		t.Errorf("negamax %d", score)
	}
	if score := e.quiesce(p, 5, -infinity, infinity); score != -mateScore+5 {
		t.Errorf("quiesce %d", score)
	}
	if r := e.Think(p, nil, Limits{Depth: 1}); r.Found {
		t.Errorf("terminal search: %+v", r)
	}
	g := &Game{match: m}
	if !strings.Contains(g.header(), "extinction, White wins") {
		t.Errorf("header %q", g.header())
	}
}

func TestAtomicSearchExplosionWinWhileChecked(t *testing.T) {
	v := builtin(t, "atomic")
	p := fen(t, v, "r3k3/3p4/8/8/8/8/8/K2R4 w -")
	if !p.InCheck() {
		t.Fatal("expected check")
	}
	r := think(t, p, 2)
	if !IsMateScore(r.Score) || r.Score <= 0 || MateIn(r.Score) != 1 {
		t.Fatalf("result %+v", r)
	}
	child := p.Apply(r.Move)
	if !child.extinct(Black) {
		t.Fatalf("move %s did not win", moveName(p, r.Move))
	}
	if child.Hash() == p.Hash() {
		t.Fatal("explosion did not change hash")
	}
	e := NewEngine(v)
	e.begin([]uint64{p.Hash()}, Limits{})
	if e.repeated(child.Hash(), child.Clock) {
		t.Fatal("explosion position repeats parent")
	}
	m := NewMatch(v)
	m.Load(p)
	m.Play(r.Move)
	if st, winner := m.Status(); st != ExtinctionWin || winner != White {
		t.Fatalf("status %v winner %v", st, winner)
	}
	if !m.Undo() || m.Position().Hash() != p.Hash() {
		t.Fatal("undo did not restore hash")
	}
}

func TestAtomicSmallBudgetReturnsLegalMove(t *testing.T) {
	v := builtin(t, "atomic")
	p := NewPosition(v)
	r := NewEngine(v).Think(p, nil, Limits{Nodes: 2000, Depth: 2})
	if !r.Found || r.Depth < 1 || r.Nodes == 0 || !slices.Contains(p.Moves(), r.Move) {
		t.Fatalf("result %+v is not legal", r)
	}
}

func TestAtomicQuiesceDoesNotDeltaPruneWinningBlast(t *testing.T) {
	v := builtin(t, "atomic")
	p := fen(t, v, "4k3/3p4/8/8/8/8/8/K2R4 w -")
	e := NewEngine(v)
	e.begin(nil, Limits{})
	score := e.quiesce(p, 0, mateScore/2, infinity)
	if score != mateScore-1 {
		t.Fatalf("quiesce score %d, want winning blast", score)
	}
}

func TestExtinctionWithoutExplosion(t *testing.T) {
	v := builtin(t, "chess")
	v.Extinction = []uint8{v.TypeOf('K')}
	p := fen(t, v, "8/8/8/8/8/8/8/K7 b -")
	if o, over := adjudicate(p, true); !over || o.status != ExtinctionWin || o.winner != White {
		t.Fatalf("outcome %+v, over %v", o, over)
	}
}

func TestAtomicRepetitionAfterExplosion(t *testing.T) {
	v := builtin(t, "atomic")
	m := NewMatch(v)
	m.Load(fen(t, v, "1n5k/8/8/2ppr3/2PRB3/8/8/KN6 w -"))
	before := m.Position().Hash()
	playAll(t, m, "d4", "d5")
	exploded := m.Position().Hash()
	if exploded == before || m.Position().Clock != 0 {
		t.Fatal("capture did not reset position/hash clock")
	}
	for cycle := 0; cycle < 2; cycle++ {
		playAll(t, m, "b8", "a6", "b1", "c3", "a6", "b8", "c3", "b1")
		if m.Position().Hash() != exploded {
			t.Fatalf("cycle %d did not restore hash", cycle)
		}
		if cycle == 0 && m.Over() {
			t.Fatalf("premature outcome %v", m.status)
		}
	}
	if st, _ := m.Status(); st != DrawRepetition {
		t.Fatalf("status %v, want repetition", st)
	}
	e := NewEngine(v)
	e.begin(m.Hashes(), Limits{})
	if !e.repeated(exploded, m.Position().Clock) {
		t.Fatal("search missed repetition")
	}
}

func TestAtomicVictimValueBlast(t *testing.T) {
	v := builtin(t, "atomic")
	p := fen(t, v, "7k/8/8/2ppr3/2PRB3/8/8/K7 w -")
	e := NewEngine(v)
	mv := findMove(t, p, "d4", "d5")
	value := func(letter byte) int { return v.Pieces[v.TypeOf(letter)-1].Value }
	// Captured pawn and adjacent enemy rook, less capturer and friendly bishop;
	// the adjacent pawns are immune.
	want := value('P') + value('R') - value('R') - value('B')
	if got := e.victimValue(p, mv); got != want {
		t.Fatalf("blast value %d, want %d", got, want)
	}
	p = fen(t, v, "4k3/3p4/8/8/8/8/8/K2R4 w -")
	mv = findMove(t, p, "d1", "d7")
	if got := e.victimValue(p, mv); got < mateScore {
		t.Fatalf("royal blast value %d", got)
	}
}

func TestAtomicMateAndStalemate(t *testing.T) {
	v := builtin(t, "atomic")
	for _, tc := range []struct {
		fen  string
		want Status
	}{
		{"7k/6Q1/8/8/8/8/8/K7 b -", Checkmate},
		{"7k/8/6Q1/8/8/8/8/K7 b -", Stalemate},
	} {
		m := NewMatch(v)
		m.Load(fen(t, v, tc.fen))
		if st, _ := m.Status(); st != tc.want {
			t.Errorf("%s: got %v, want %v", tc.fen, st, tc.want)
		}
	}
}

func TestExtinctionOfMoverOnExplosion(t *testing.T) {
	v := builtin(t, "atomic")
	v.Extinction = []uint8{v.TypeOf('Q')}
	p := fen(t, v, "7k/7q/8/3p4/3Q4/8/8/K7 w -")
	m := NewMatch(v)
	m.Load(p)
	playAll(t, m, "d4", "d5")
	if st, winner := m.Status(); st != ExtinctionWin || winner != Black {
		t.Fatalf("status %v winner %v", st, winner)
	}
	if got := (&Game{match: m}).header(); !strings.Contains(got, "extinction, Black wins") {
		t.Fatalf("header %q", got)
	}
	e := NewEngine(v)
	e.begin(nil, Limits{})
	if got := e.negamax(m.Position(), 1, 3, -infinity, infinity, true); got != mateScore-3 {
		t.Fatalf("score %d", got)
	}
}

func TestExtinctionBothSidesMoverWins(t *testing.T) {
	v := builtin(t, "atomic")
	v.Extinction = []uint8{v.TypeOf('Q')}
	m := NewMatch(v)
	m.Load(fen(t, v, "7k/8/8/3q4/3Q4/8/8/K7 w -"))
	playAll(t, m, "d4", "d5")
	if st, winner := m.Status(); st != ExtinctionWin || winner != White {
		t.Fatalf("status %v winner %v", st, winner)
	}
}

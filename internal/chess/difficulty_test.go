package chess

import (
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestDifficultyProfiles(t *testing.T) {
	for i, name := range []string{"beginner", "easy", "normal", "hard", "expert"} {
		level, err := ParseLevel(name)
		if err != nil || level.String() != name {
			t.Fatalf("%s: %v, %v", name, level, err)
		}
		lim := level.Limits(0)
		if lim.Time != []time.Duration{time.Second, time.Second, DefaultThinkTime, 6 * time.Second, 12 * time.Second}[i] {
			t.Errorf("%s: %+v", name, lim)
		}
		override := level.Limits(time.Minute)
		if override.Time != time.Minute || override.Depth != lim.Depth || override.Nodes != lim.Nodes {
			t.Errorf("override changed non-time limits: %+v -> %+v", lim, override)
		}
		if i < 2 && (lim.Depth != i+1 || lim.Nodes <= 0) {
			t.Errorf("unbounded weak profile: %+v", lim)
		}
	}
	if (Level(0)).String() != "normal" {
		t.Fatal("zero level must preserve default")
	}
	for _, name := range []string{"", "NORMAL", "nope"} {
		if _, err := ParseLevel(name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}

func TestNodeBudgetAndStoppedFallback(t *testing.T) {
	for _, id := range []string{"chess", "double", "bigboard"} {
		p := NewPosition(builtin(t, id))
		for _, nodes := range []int{1, 10, 100, 1024} {
			r := NewEngine(p.V).Think(p, nil, Limits{Nodes: nodes})
			if r.Nodes > nodes || !r.Found || !slices.Contains(p.Moves(), r.Move) {
				t.Fatalf("%s budget %d: %+v", id, nodes, r)
			}
			if nodes == 1 && (r.Depth != 0 || r.Score != 0) {
				t.Fatalf("unfinished analysis reported: %+v", r)
			}
		}
		var stop atomic.Bool
		stop.Store(true)
		r := NewEngine(p.V).Think(p, nil, Limits{Stop: &stop})
		if !r.Found || r.Nodes != 0 || r.Depth != 0 {
			t.Fatalf("pre-cancelled search: %+v", r)
		}
	}
}

func TestDifficultySeedAndTableIsolation(t *testing.T) {
	p := NewPosition(builtin(t, "chess"))
	e := NewEngine(p.V)
	e.Think(p, nil, Limits{Depth: 4})
	for _, level := range []Level{Beginner, Easy} {
		for seed := uint64(0); seed < 10; seed++ {
			lim := Limits{Nodes: 20000}
			a := e.ThinkLevel(p, nil, level, lim, seed)
			b := NewEngine(p.V).ThinkLevel(p, nil, level, lim, seed)
			if a != b || !slices.Contains(p.Moves(), a.Move) || a.Depth > level.Limits(0).Depth {
				t.Fatalf("%s seed %d: %+v != %+v", level, seed, a, b)
			}
		}
	}
}

func TestCandidateSelectionSafety(t *testing.T) {
	best := candidate{Move: Move{From: 1}, Score: 100}
	weak := candidate{Move: Move{From: 2}, Score: 50}
	policy := selection{chance: 100, loss: 200}
	for seed := uint64(0); seed < 20; seed++ {
		if got := chooseCandidate([]candidate{best, weak}, policy, seed); got != weak {
			t.Fatalf("did not choose scored alternative: %+v", got)
		}
		mate := candidate{Move: Move{From: 3}, Score: mateScore - 1}
		if got := chooseCandidate([]candidate{weak, mate}, policy, seed); got != mate {
			t.Fatal("rejected immediate mate")
		}
		mated := candidate{Move: Move{From: 4}, Score: -mateScore + 2}
		if got := chooseCandidate([]candidate{best, mated}, policy, seed); got != best {
			t.Fatal("chose known forced loss")
		}
	}
}

func TestDifficultySpecialPositions(t *testing.T) {
	v := builtin(t, "chess")
	for _, text := range []string{
		"R5k1/5ppp/8/8/8/8/8/6K1 b -",    // mate
		"7k/5K2/6Q1/8/8/8/8/8 b -",       // stalemate
		"7k/8/5KQ1/8/8/8/8/8 b -",        // only Kh7
		"4k3/8/8/8/8/8/4r3/4K3 w -",      // check
		"k7/4P3/8/8/8/8/8/K7 w -",        // promotion
		"r3k2r/8/8/8/8/8/8/R3K2R w KQkq", // castling
	} {
		p := fen(t, v, text)
		for _, level := range []Level{Beginner, Easy, Normal, Hard, Expert} {
			r := NewEngine(v).ThinkLevel(p, nil, level, Limits{Depth: 2, Nodes: 3000}, 42)
			if r.Found != (len(p.Moves()) > 0) || (r.Found && !slices.Contains(p.Moves(), r.Move)) {
				t.Fatalf("%s %s: %+v", level, text, r)
			}
		}
	}
	m := NewMatch(v)
	playAll(t, m, "e2", "e4", "a7", "a6", "e4", "e5", "d7", "d5")
	for _, level := range []Level{Beginner, Easy} {
		r := NewEngine(v).ThinkLevel(m.Position(), m.Hashes(), level, Limits{Nodes: 10000}, 2)
		if !slices.Contains(m.Legal(), r.Move) {
			t.Fatal("illegal en passant search result")
		}
	}
}

func TestIncompleteIterationKeepsCompletedMoveAndScore(t *testing.T) {
	p := NewPosition(builtin(t, "chess"))
	completed := NewEngine(p.V).Think(p, nil, Limits{Depth: 1})
	got := NewEngine(p.V).Think(p, nil, Limits{Depth: 4, Nodes: completed.Nodes + 1})
	if got.Depth != completed.Depth || got.Score != completed.Score || got.Move != completed.Move {
		t.Fatalf("unfinished iteration replaced completed analysis: %+v, want %+v", got, completed)
	}
}

func TestQuiescenceRespectsNodeBudgetAndCancellation(t *testing.T) {
	p := fen(t, builtin(t, "chess"), "4k3/8/2p5/3q4/4Q3/8/8/4K3 w -")
	e := NewEngine(p.V)
	e.begin(nil, Limits{Nodes: 1})
	e.quiesce(p, 0, -infinity, infinity)
	if !e.aborted || e.nodes != 1 {
		t.Fatalf("quiescence ignored budget: nodes %d aborted %v", e.nodes, e.aborted)
	}
	var stop atomic.Bool
	stop.Store(true)
	e.begin(nil, Limits{Stop: &stop})
	e.quiesce(p, 0, -infinity, infinity)
	if !e.aborted || e.nodes != 0 {
		t.Fatal("quiescence ignored cancellation")
	}
}

func TestAllLevelsVariantSelfPlay(t *testing.T) {
	vs, err := parseVariants(map[string][]byte{"fairy.yaml": []byte(`
id: fairy
name: Fairy
files: 5
ranks: 5
pieces:
  - {letter: K, name: King, betza: K, royal: true}
  - {letter: X, name: Cannon, betza: mRcpR, value: 400}
  - {letter: N, name: Nightrider, betza: NN, value: 400}
setup: knx2/5/5/5/2XNK
`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"double", "bigboard", "grand"} {
		vs = append(vs, builtin(t, id))
	}
	for _, v := range vs {
		for _, level := range []Level{Beginner, Easy, Normal, Hard, Expert} {
			m := NewMatch(v)
			e := NewEngine(v)
			for ply := 0; ply < 6 && !m.Over(); ply++ {
				r := e.ThinkLevel(m.Position(), m.Hashes(), level, Limits{Nodes: 500}, uint64(ply))
				if !r.Found || !slices.Contains(m.Legal(), r.Move) || r.Nodes > 500 {
					t.Fatalf("%s %s: %+v", v.ID, level, r)
				}
				m.Play(r.Move)
			}
		}
	}
}

func TestDifficultyRepetitionHistory(t *testing.T) {
	p := fen(t, builtin(t, "chess"), "4k3/8/8/8/8/8/8/1N2K1N1 w -")
	past := []uint64{}
	for _, mv := range p.Moves() {
		past = append(past, p.Apply(mv).Hash())
	}
	// Every child occurred before the root; the search must see draws, not
	// standalone static scores. Give those child positions a reversible clock.
	p.Clock = len(past) + 1
	for _, level := range []Level{Beginner, Easy, Normal, Hard, Expert} {
		r := NewEngine(p.V).ThinkLevel(p, past, level, Limits{Depth: 2, Nodes: 20000}, 1)
		depth := 2
		if level == Beginner {
			depth = 1
		}
		if r.Score != 0 || r.Depth != depth {
			t.Fatalf("%s ignored history: %+v", level, r)
		}
	}
}

func TestSelectedMoveCarriesItsOwnScore(t *testing.T) {
	p := NewPosition(builtin(t, "chess"))
	e := NewEngine(p.V)
	e.begin(nil, Limits{})
	candidates := e.scoreRoot(p, p.Moves(), 1)
	best := chooseCandidate(candidates, selection{}, 0)
	weaker := false
	for seed := uint64(0); seed < 30; seed++ {
		r := NewEngine(p.V).ThinkLevel(p, nil, Beginner, Limits{}, seed)
		for _, c := range candidates {
			if c.Move == r.Move {
				if c.Score != r.Score || r.Depth != 1 {
					t.Fatalf("selected score mismatch: %+v, %+v", r, c)
				}
				weaker = weaker || r.Score < best.Score
			}
		}
	}
	if !weaker {
		t.Fatal("no seeded deliberate mistake observed")
	}
}

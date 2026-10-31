package chess

import (
	"slices"
	"testing"
)

// Grand Chess and King of the Hill meet in the shared ending code: King of the
// Hill ends a game by a goal arrival, Grand by the ordinary mate, stalemate and
// move-limit rules, and Grand's promotion flags change the move list without
// touching those endings. These checks keep the two honest together.

const grandStalemateFEN = "9k/7Q2/8K1/10/10/10/10/10/10/10 b - - 0 1"

// grandCentreFEN puts a white king on c3 with a king move to d4, the square
// King of the Hill wins on, and enough material that the game is not a draw by
// bare royals.
const grandCentreFEN = "9k/10/10/10/10/10/10/2K7/10/R8r w - - 0 1"

func TestGrandHasNoGoalAndObeysItsStalemateRule(t *testing.T) {
	v := builtin(t, "grand")
	if len(v.Goal) != 0 {
		t.Fatalf("grand has %d goal squares", len(v.Goal))
	}
	if v.Stalemate != StalemateDraw {
		t.Fatalf("grand stalemate rule = %q, want the default draw", v.Stalemate)
	}
	// A royal on d4 is an ordinary king move here: no goal arrival.
	m := NewMatch(v)
	m.Load(fen(t, v, grandCentreFEN))
	playAll(t, m, "c3", "d4")
	if st, _ := m.Status(); st != Playing || m.Over() {
		t.Fatalf("c3-d4 gave status %v in grand, want the game to continue", st)
	}
	// The stalemate rule is YAML too: a class of variant that wins by delivering
	// stalemate inherits Grand's promotion rules and still ends in a stalemate.
	wins := rulesVariant(t, "grandwins", "id: grandwins\nname: Grand wins\n"+
		"extends: grand\nstalemate: win\n")
	if !wins.PromotionOptional || !wins.PromotionCapturedOnly || wins.PromotionZone != 3 {
		t.Errorf("extends: grand lost the promotion flags: optional %v, captured-only %v, zone %d",
			wins.PromotionOptional, wins.PromotionCapturedOnly, wins.PromotionZone)
	}
	drawn := NewMatch(v)
	drawn.Load(fen(t, v, grandStalemateFEN))
	if st, _ := drawn.Status(); st != Stalemate || drawn.Decisive() {
		t.Fatalf("grand stalemate = %v, decisive %v", st, drawn.Decisive())
	}
	wm := NewMatch(wins)
	wm.Load(fen(t, wins, grandStalemateFEN))
	if st, winner := wm.Status(); st != Stalemate || winner != White || !wm.Decisive() {
		t.Fatalf("stalemate win = %v, winner %v, decisive %v", st, winner, wm.Decisive())
	}
	// The search scores that stalemate as a mate for the delivering side.
	p := fen(t, wins, grandStalemateFEN)
	if o, over := adjudicate(p, len(p.Moves()) > 0); !over || o.score(p.Turn, 3) != -mateScore+3 {
		t.Fatalf("adjudicate = %+v, over %v", o, over)
	}
}

// TestGrandGoalFromYAML checks that the goal key merges with Grand's promotion
// flags: an inheriting variant gets both and wins on the arrival.
func TestGrandGoalFromYAML(t *testing.T) {
	v := rulesVariant(t, "grandgoal", "id: grandgoal\nname: Grand with a goal\n"+
		"extends: grand\ngoal: [d4]\n")
	if !slices.Equal(v.Goal, []int{rulesSquare(t, v, "d4")}) {
		t.Fatalf("goal = %v", v.Goal)
	}
	if !v.PromotionOptional || !v.PromotionCapturedOnly || v.PromotionZone != 3 {
		t.Errorf("goal variant lost the promotion flags: optional %v, captured-only %v, zone %d",
			v.PromotionOptional, v.PromotionCapturedOnly, v.PromotionZone)
	}
	m := NewMatch(v)
	m.Load(fen(t, v, grandCentreFEN))
	playAll(t, m, "c3", "d4")
	if st, winner := m.Status(); st != GoalWin || winner != White || !m.Decisive() {
		t.Fatalf("goal arrival = %v, winner %v, decisive %v", st, winner, m.Decisive())
	}
}

// grandPromoFEN puts a white pawn on e8, inside Grand's three-rank promotion
// zone, with a black knight on d9 to take: every promotion is available (White
// has lost everything but the pawn) and the pawn may also decline, so the
// generator lists a promoted move and a plain move for the same squares.
const grandPromoFEN = "9k/3n6/4P5/10/10/10/10/10/10/K9 w - - 0 1"

func TestGrandPromotionPairsInSearch(t *testing.T) {
	v := builtin(t, "grand")
	p := fen(t, v, grandPromoFEN)
	from, to := rulesSquare(t, v, "e8"), rulesSquare(t, v, "d9")
	var plain, promoted []Move
	for _, mv := range p.Moves() {
		if mv.From == from && mv.To == to {
			if mv.Promote == 0 {
				plain = append(plain, mv)
			} else {
				promoted = append(promoted, mv)
			}
		}
	}
	if len(plain) != 1 || len(promoted) != len(v.Promotion) {
		t.Fatalf("e8xd9: %d plain, %d promoted moves", len(plain), len(promoted))
	}
	// Quiescence sees the campaign as captures, and the declining capture still
	// counts as one, so both halves of the pair survive the tactical filter.
	tactical := p.captures()
	if !slices.Contains(tactical, plain[0]) {
		t.Error("the declining capture is missing from the quiescence moves")
	}
	for _, mv := range promoted {
		if !slices.Contains(tactical, mv) {
			t.Errorf("promotion %s missing from the quiescence moves", v.SquareName(mv.To))
		}
	}
	// No duplicates: the plain and the promoted move are two distinct moves.
	seen := map[Move]bool{}
	for _, mv := range p.Moves() {
		if seen[mv] {
			t.Fatalf("duplicate move %+v", mv)
		}
		seen[mv] = true
	}
	// The search plays the capture and promotes while doing it; the plain twin
	// must not shadow it in the table or the move ordering.
	r := think(t, p, 3)
	if got := moveName(p, r.Move); got != "e8xd9=Q" {
		t.Errorf("engine played %s, want e8xd9=Q", got)
	}
	if !slices.Contains(p.Moves(), r.Move) {
		t.Fatalf("%+v is not a legal move", r.Move)
	}
	// And the match accepts either half of the pair: the plain move leaves the
	// pawn in place and keeps the game going.
	m := NewMatch(v)
	m.Load(p)
	m.Play(plain[0])
	if pc := m.Position().Board[rulesSquare(t, v, "d9")]; pc.Type != v.TypeOf('P') {
		t.Errorf("declining capture did not keep a pawn: %+v", pc)
	}
}

// TestGrandSelfPlayAndSearchStart keeps the broad engine checks covering Grand
// too, now that grand is a built-in with its own move list.
func TestGrandSearchStartsAndSelfPlays(t *testing.T) {
	v := builtin(t, "grand")
	p := NewPosition(v)
	for _, level := range []Level{Beginner, Easy, Normal} {
		m := NewMatch(v)
		for ply := 0; ply < 4 && !m.Over(); ply++ {
			r := NewEngine(v).ThinkLevel(m.Position(), m.Hashes(), level, Limits{Nodes: 400}, uint64(ply))
			if !r.Found || !slices.Contains(m.Legal(), r.Move) {
				t.Fatalf("%s: %+v is not a legal move", level, r)
			}
			m.Play(r.Move)
		}
	}
	if r := think(t, p, 2); !r.Found || !slices.Contains(p.Moves(), r.Move) {
		t.Fatalf("grand start: %+v is not a legal move", r)
	}
}

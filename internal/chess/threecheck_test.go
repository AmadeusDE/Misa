package chess

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// threeRoom is a quiet three-check position: White's rook can check the black
// king, Black's rook can check the white king, and nobody is near the limit.
const threeRoom = "4k3/1r6/8/8/8/8/R7/7K b - +0+0"

// TestThreeCheckVariantLoads checks the variant file itself: it is orthodox
// chess plus a limit of three checks.
func TestThreeCheckVariantLoads(t *testing.T) {
	vs, errs := Builtins()
	if len(errs) > 0 {
		t.Fatalf("builtin variants: %v", errs)
	}
	v, err := Find(vs, "threecheck")
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != "threecheck" || v.Name != "Three-Check" {
		t.Errorf("variant = %s (%q)", v.ID, v.Name)
	}
	if v.Files != 8 || v.Ranks != 8 || len(v.Pieces) != 6 || len(v.Castling) != 2 {
		t.Errorf("%d×%d, %d pieces, %d castles", v.Files, v.Ranks, len(v.Pieces), len(v.Castling))
	}
	if v.CheckLimit != 3 {
		t.Errorf("check limit = %d, want 3", v.CheckLimit)
	}
	if len(v.Goal) != 0 {
		t.Errorf("goal = %v, want none", v.Goal)
	}

	// It extends chess: same setup, same piece letters, same castling.
	chess := builtin(t, "chess")
	if v.Setup != chess.Setup {
		t.Errorf("setup = %q, want chess's %q", v.Setup, chess.Setup)
	}
	if !slices.Equal(v.start, chess.start) {
		t.Error("start position differs from chess")
	}
	if !slices.Equal(v.Castling, chess.Castling) {
		t.Errorf("castling = %v, want %v", v.Castling, chess.Castling)
	}
	var letters, chessLetters []byte
	for _, p := range v.Pieces {
		letters = append(letters, p.Letter)
	}
	for _, p := range chess.Pieces {
		chessLetters = append(chessLetters, p.Letter)
	}
	if !slices.Equal(letters, chessLetters) {
		t.Errorf("pieces = %q, want %q", letters, chessLetters)
	}
	if p := NewPosition(v); !slices.Equal(p.Board, NewPosition(chess).Board) {
		t.Error("start board differs from chess")
	}
}

// TestThreeCheckCounters covers who gets credited with a check, and that the
// counters survive Undo and a FEN round trip.
func TestThreeCheckCounters(t *testing.T) {
	v := builtin(t, "threecheck")
	p := fen(t, v, threeRoom)

	// Only the mover is credited: Black's Rb1 checks White's king.
	m := NewMatch(v)
	m.Load(p)
	m.Play(findMove(t, p, "b7", "b1"))
	if got := m.Position().Checks; got != [2]int{0, 1} {
		t.Fatalf("checks after Rb1+ = %v, want {0,1}", got)
	}
	// A quiet move gives nothing to either side.
	m.Play(findMove(t, m.Position(), "h1", "g2"))
	if got := m.Position().Checks; got != [2]int{0, 1} {
		t.Fatalf("checks after a quiet move = %v, want {0,1}", got)
	}
	// Taking the move back takes the check back with it.
	if !m.Undo() || !m.Undo() {
		t.Fatal("cannot undo both moves")
	}
	if got := m.Position().Checks; got != [2]int{0, 0} {
		t.Fatalf("checks after undo = %v, want {0,0}", got)
	}

	// A FEN field counts what each side has already given …
	if got := fen(t, v, "4k3/8/8/8/8/8/4Q3/4K3 w - - +2+1").Checks; got != [2]int{2, 1} {
		t.Errorf("+2+1 loaded as %v, want {2,1}", got)
	}
	// … and, written the Lichess way, what each side has left.
	if got := fen(t, v, "4k3/8/8/8/8/8/4Q3/4K3 w - - 1+2").Checks; got != [2]int{2, 1} {
		t.Errorf("1+2 loaded as %v, want {2,1}", got)
	}
	// A round trip through the counters keeps them.
	again := fen(t, v, "4k3/8/8/8/8/8/4Q3/4K3 w - - +2+1")
	trip := fmt.Sprintf("4k3/8/8/8/8/8/4Q3/4K3 w - - +%d+%d", again.Checks[White], again.Checks[Black])
	if got := fen(t, v, trip).Checks; got != again.Checks {
		t.Errorf("%s loaded as %v, want %v", trip, got, again.Checks)
	}
}

// TestThreeCheckNoLimitNeverCounts checks that the rule is off without a
// limit: orthodox chess, and a variant that extends chess without the field,
// neither count checks nor read them from a FEN.
func TestThreeCheckNoLimitNeverCounts(t *testing.T) {
	src := []byte("id: plaincheck\nname: Plain Check\nextends: chess\n")
	vs, err := parseVariants(map[string][]byte{"plaincheck.yaml": src})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Find(vs, "plaincheck")
	if err != nil {
		t.Fatal(err)
	}
	if plain.CheckLimit != 0 {
		t.Fatalf("plaincheck check limit = %d, want 0", plain.CheckLimit)
	}
	for _, v := range []*Variant{builtin(t, "chess"), plain} {
		p := fen(t, v, "4k3/8/8/8/8/8/8/K6R w -")
		// The checks field is ignored without a limit.
		if got := fen(t, v, "4k3/8/8/8/8/8/8/K6R w - - +2+0").Checks; got != [2]int{0, 0} {
			t.Errorf("%s: checks field loaded as %v, want {0,0}", v.ID, got)
		}
		after := p.Apply(findMove(t, p, "h1", "h8"))
		if !after.InCheck() {
			t.Fatalf("%s: Rh8 did not give check", v.ID)
		}
		if got := after.Checks; got != [2]int{0, 0} {
			t.Errorf("%s: checks after a check = %v, want {0,0}", v.ID, got)
		}
	}
}

// TestThreeCheckTerminal covers the outcomes: the third check wins, a
// stalemate below the limit still draws, and one check is not enough.
func TestThreeCheckTerminal(t *testing.T) {
	v := builtin(t, "threecheck")

	// White has given two checks; the third wins at once.
	m := NewMatch(v)
	m.Load(fen(t, v, "4k3/8/8/8/8/8/8/K6R w - +2+0"))
	m.Play(findMove(t, m.Position(), "h1", "h8"))
	if st, winner := m.Status(); st != CheckWin || winner != White || !m.Over() {
		t.Errorf("third check: status %v, winner %v, over %v", st, winner, m.Over())
	}
	if got := m.Position().Checks; got != [2]int{3, 0} {
		t.Errorf("checks = %v, want {3,0}", got)
	}
	if st, _ := m.Status(); st.String() != "check limit reached" {
		t.Errorf("Status.String() = %q", st)
	}

	// The checkmating move that is also the third check. adjudicate reports
	// the check limit before checkmate, so this is a CheckWin, not a
	// Checkmate; the winner is White either way.
	m = NewMatch(v)
	m.Load(fen(t, v, "7k/8/5K2/8/8/8/6Q1/8 w - +2+0"))
	m.Play(findMove(t, m.Position(), "g2", "g7"))
	after := m.Position()
	if !after.InCheck() || len(after.Moves()) != 0 {
		t.Fatalf("Qg7 is not mate: check %v, %d moves", after.InCheck(), len(after.Moves()))
	}
	if st, winner := m.Status(); st != CheckWin || winner != White {
		t.Errorf("checkmating third check: status %v, winner %v", st, winner)
	}

	// Stalemate still draws, with both sides below the limit.
	m = NewMatch(v)
	m.Load(fen(t, v, "7k/5Q2/6K1/8/8/8/8/8 b - +0+0"))
	if st, _ := m.Status(); st != Stalemate || !m.Over() {
		t.Errorf("stalemate: status %v, over %v", st, m.Over())
	}

	// One check is not a win for the side that gave it.
	m = NewMatch(v)
	m.Load(fen(t, v, threeRoom))
	m.Play(findMove(t, m.Position(), "b7", "b1"))
	if st, _ := m.Status(); st != Playing || m.Over() {
		t.Errorf("one check: status %v, over %v", st, m.Over())
	}
	if got := m.Position().Checks; got != [2]int{0, 1} {
		t.Errorf("checks after one check = %v, want {0,1}", got)
	}
}

// TestThreeCheckHash checks that the counters are part of the hash exactly
// when the variant counts them.
func TestThreeCheckHash(t *testing.T) {
	v := builtin(t, "threecheck")
	two := fen(t, v, "4k3/8/8/8/8/8/4Q3/4K3 w - - +2+0")
	twoAgain := fen(t, v, "4k3/8/8/8/8/8/4Q3/4K3 w - - +2+0")
	one := fen(t, v, "4k3/8/8/8/8/8/4Q3/4K3 w - - +1+0")
	if two.Hash() != twoAgain.Hash() {
		t.Error("equal three-check positions hash differently")
	}
	if two.Hash() == one.Hash() {
		t.Error("positions with different check counts hash equally")
	}
	// Black's counter is hashed too.
	if one.Hash() == fen(t, v, "4k3/8/8/8/8/8/4Q3/4K3 w - - +1+1").Hash() {
		t.Error("Black's check count is not hashed")
	}

	// Orthodox chess is untouched by the feature.
	chess := builtin(t, "chess")
	a := fen(t, chess, "4k3/8/8/8/8/8/4Q3/4K3 w -")
	b := fen(t, chess, "4k3/8/8/8/8/8/4Q3/4K3 w -")
	if a.Hash() != b.Hash() {
		t.Error("equal chess positions hash differently")
	}
	if a.Hash() == fen(t, chess, "4k3/8/8/8/8/8/4Q3/3K4 w -").Hash() {
		t.Error("moved chess positions hash equally")
	}
	if a.Checks != [2]int{0, 0} {
		t.Errorf("chess checks = %v, want {0,0}", a.Checks)
	}
}

// TestThreeCheckSearch checks how the search sees the rule: a win in one when
// the third check is available, a lost position once it has landed, and a
// legal move from the start.
func TestThreeCheckSearch(t *testing.T) {
	v := builtin(t, "threecheck")
	p := fen(t, v, "4k3/8/8/8/8/8/8/K6R w - +2+0")
	r := think(t, p, 3)
	if got := moveName(p, r.Move); got != "Rh1-h8" {
		t.Errorf("played %s, want Rh1-h8", got)
	}
	if !IsMateScore(r.Score) || MateIn(r.Score) != 1 {
		t.Errorf("score %d is not a win in one", r.Score)
	}
	after := p.Apply(r.Move)
	if o, over := adjudicate(after, true); !over || o.status != CheckWin || o.winner != White {
		t.Errorf("after the third check: %+v, over %v", o, over)
	}

	// With the limit reached, the side to move is lost, and nothing is
	// searched.
	e := NewEngine(v)
	e.begin(nil, Limits{Depth: 1})
	if score := e.negamax(after, 1, 0, -infinity, infinity, true); score != -mateScore {
		t.Errorf("negamax = %d, want %d", score, -mateScore)
	}
	if score := e.quiesce(after, 0, -infinity, infinity); score != -mateScore {
		t.Errorf("quiesce = %d, want %d", score, -mateScore)
	}
	if r := NewEngine(v).Think(after, nil, Limits{Depth: 2}); r.Found {
		t.Errorf("searched on after the third check: %+v", r)
	}

	// The computer plays something legal from the start.
	start := NewPosition(v)
	if start.Checks != [2]int{0, 0} {
		t.Fatalf("start checks = %v, want {0,0}", start.Checks)
	}
	got := NewEngine(v).Think(start, nil, Limits{Depth: 2, Time: 200 * time.Millisecond})
	if !got.Found || !slices.Contains(start.Moves(), got.Move) {
		t.Errorf("%+v is not a legal move", got)
	}
}

// TestThreeCheckHeader covers the game screen's status line.
func TestThreeCheckHeader(t *testing.T) {
	g := newGame(t, "threecheck", 100, 30)
	if h := g.header(); !strings.Contains(h, "checks left: White 3, Black 3") {
		t.Errorf("header = %q", h)
	}
	v := g.match.Variant()
	g.Load(fen(t, v, "4k3/8/8/8/8/8/8/K6R w - +2+0"))
	if h := g.header(); !strings.Contains(h, "checks left: White 1, Black 3") {
		t.Errorf("header after +2+0 = %q", h)
	}
	g.match.Play(findMove(t, g.match.Position(), "h1", "h8"))
	if h := g.header(); !strings.Contains(h, "check limit reached, White wins") {
		t.Errorf("header after the third check = %q", h)
	}
}

// TestThreeCheckLeavesChessAlone checks that orthodox chess keeps no limit and
// never reaches a check win.
func TestThreeCheckLeavesChessAlone(t *testing.T) {
	v := builtin(t, "chess")
	if v.CheckLimit != 0 {
		t.Fatalf("chess check limit = %d, want 0", v.CheckLimit)
	}
	m := NewMatch(v)
	m.Load(fen(t, v, "4k3/8/8/8/8/8/8/K6R w -"))
	for range 4 {
		if m.Over() {
			break
		}
		if st, _ := m.Status(); st == CheckWin {
			t.Fatalf("chess reached a check win: %v", st)
		}
		if len(m.Legal()) == 0 {
			break
		}
		m.Play(m.Legal()[0])
	}
	if st, _ := m.Status(); st == CheckWin {
		t.Errorf("chess status = %v", st)
	}
	if got := m.Position().Checks; got != [2]int{0, 0} {
		t.Errorf("chess checks = %v, want {0,0}", got)
	}
}

// TestThreeCheckLimitInheritance covers the general check_limit field: a child
// inherits it, may override it, and a nonsense value is rejected.
func TestThreeCheckLimitInheritance(t *testing.T) {
	vs, err := parseVariants(map[string][]byte{
		"inherits.yaml":  []byte("id: inherits\nextends: threecheck\n"),
		"overrides.yaml": []byte("id: overrides\nextends: threecheck\ncheck_limit: 5\n"),
		"negative.yaml":  []byte("id: negative\nextends: threecheck\ncheck_limit: -1\n"),
	})
	if err == nil || !strings.Contains(err.Error(), "check limit must not be negative") {
		t.Errorf("negative check_limit: %v", err)
	}
	for _, tt := range []struct {
		id    string
		limit int
	}{{"inherits", 3}, {"overrides", 5}} {
		v, err := Find(vs, tt.id)
		if err != nil {
			t.Fatal(err)
		}
		if v.CheckLimit != tt.limit {
			t.Errorf("%s: check limit = %d, want %d", tt.id, v.CheckLimit, tt.limit)
		}
	}
}

// TestThreeCheckAdjudication covers the shared ending code: a reached limit is
// a decisive CheckWin worth a mate, it is seen without generating moves, and it
// outranks the variant's stalemate rule exactly as a goal arrival does, while a
// stalemate below the limit still follows that rule.
func TestThreeCheckAdjudication(t *testing.T) {
	v := builtin(t, "threecheck")
	p := fen(t, v, "4k3/8/8/8/8/8/8/K6R w - +2+0")
	after := p.Apply(findMove(t, p, "h1", "h8"))
	o, over := adjudicate(after, false)
	if !over || o.status != CheckWin || o.drawn || o.winner != White {
		t.Fatalf("adjudicate = %+v, over %v", o, over)
	}
	if got := o.score(White, 5); got != mateScore-5 {
		t.Errorf("score for the winner = %d, want %d", got, mateScore-5)
	}
	if got := o.score(Black, 5); got != -mateScore+5 {
		t.Errorf("score for the loser = %d, want %d", got, -mateScore+5)
	}
	// A caller that has not generated the moves still adjudicates the win.
	if o, over := adjudicate(after, true); !over || o.status != CheckWin || o.winner != White {
		t.Errorf("adjudicate with hasMoves = %+v, over %v", o, over)
	}

	// The match calls it over and decisive, and names the winner.
	m := NewMatch(v)
	m.Load(after)
	if st, winner := m.Status(); st != CheckWin || winner != White || !m.Decisive() || !m.Over() {
		t.Errorf("status %v, winner %v, decisive %v, over %v", st, winner, m.Decisive(), m.Over())
	}

	// A stalemate rule and a check limit together: below the limit the rule
	// decides, at the limit the check win does.
	loss := rulesVariant(t, "threecheckloss", "id: threecheckloss\nname: Three-Check, stalemate loses\n"+
		"extends: threecheck\nstalemate: loss\n")
	if loss.CheckLimit != 3 || loss.Stalemate != StalemateLoss {
		t.Fatalf("limit %d, stalemate rule %q", loss.CheckLimit, loss.Stalemate)
	}
	const stalemateFEN = "7k/5Q2/6K1/8/8/8/8/8 b - %s"
	below := NewMatch(loss)
	below.Load(fen(t, loss, fmt.Sprintf(stalemateFEN, "+0+0")))
	if st, winner := below.Status(); st != Stalemate || winner != Black || !below.Decisive() {
		t.Errorf("stalemate below the limit: status %v, winner %v, decisive %v", st, winner, below.Decisive())
	}
	atLimit := NewMatch(loss)
	atLimit.Load(fen(t, loss, fmt.Sprintf(stalemateFEN, "+3+0")))
	if st, winner := atLimit.Status(); st != CheckWin || winner != White || !atLimit.Decisive() {
		t.Errorf("limit at a stalemate: status %v, winner %v, decisive %v", st, winner, atLimit.Decisive())
	}
	// The same position in plain three-check is an ordinary drawn stalemate.
	drawn := NewMatch(v)
	drawn.Load(fen(t, v, fmt.Sprintf(stalemateFEN, "+0+0")))
	if st, _ := drawn.Status(); st != Stalemate || drawn.Decisive() {
		t.Errorf("three-check stalemate: status %v, decisive %v", st, drawn.Decisive())
	}
}

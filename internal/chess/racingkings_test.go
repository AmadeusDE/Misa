package chess

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestRacingKingsLoads checks the bundled Racing Kings definition.
func TestRacingKingsLoads(t *testing.T) {
	v := builtin(t, "racingkings")
	if v.Name != "Racing Kings" {
		t.Errorf("name = %q", v.Name)
	}
	if v.Files != 8 || v.Ranks != 8 {
		t.Errorf("board = %d×%d, want 8×8", v.Files, v.Ranks)
	}
	if len(v.Castling) != 0 {
		t.Errorf("%d castling entries, want none", len(v.Castling))
	}
	if len(v.Promotion) != 0 {
		t.Errorf("%d promotion types, want none", len(v.Promotion))
	}
	var letters []byte
	for _, pc := range v.Pieces {
		if pc.Pawn {
			t.Errorf("piece %c is a pawn: Racing Kings has no pawns", pc.Letter)
		}
		letters = append(letters, pc.Letter)
	}
	slices.Sort(letters)
	if want := []byte{'B', 'K', 'N', 'Q', 'R'}; len(v.Pieces) != len(want) || !slices.Equal(letters, want) {
		t.Errorf("%d pieces %q, want %q", len(v.Pieces), letters, want)
	}
	if want := []int{56, 57, 58, 59, 60, 61, 62, 63}; !slices.Equal(v.Goal, want) {
		t.Errorf("goal squares = %v, want the eighth rank %v", v.Goal, want)
	}
	if !v.NoChecks || !v.GoalReplyDraw {
		t.Errorf("no_checks %v, goal_reply_draw %v; want both", v.NoChecks, v.GoalReplyDraw)
	}
}

// TestRacingKingsStartPosition checks the setup: both armies on ranks 1 and 2,
// Black on the a-d files and White on the e-h files, eight pieces a side.
func TestRacingKingsStartPosition(t *testing.T) {
	v := builtin(t, "racingkings")
	p := NewPosition(v)
	var white, black int
	for _, pc := range p.Board {
		switch pc.Color {
		case White:
			if pc.Type != 0 {
				white++
			}
		case Black:
			if pc.Type != 0 {
				black++
			}
		}
	}
	if white != 8 || black != 8 {
		t.Errorf("%d white, %d black pieces; want 8 each (16 total)", white, black)
	}
	// Ranks 1 and 2: Black's queen and king on a1 and a2, White's on h1 and
	// h2, with Black on the a-d files and White on the e-h files.
	for y, row := range []string{"QRBNNBRQ", "KRBNNBRK"} {
		for f := range len(row) {
			sq := v.Square(f, y)
			pc := p.Board[sq]
			want := White
			if f < 4 {
				want = Black
			}
			if pc.Type == 0 || v.Type(pc).Letter != row[f] || pc.Color != want {
				t.Errorf("%s holds %+v, want a %v %c", v.SquareName(sq), pc, want, row[f])
			}
		}
	}
	for c, name := range map[Color]string{White: "h2", Black: "a2"} {
		sq, err := v.ParseSquare(name)
		if err != nil {
			t.Fatal(err)
		}
		if royals := p.Royals(c); len(royals) != 1 || royals[0] != sq {
			t.Errorf("%v royals = %v, want only %s", c, royals, name)
		}
	}
	if p.Turn != White {
		t.Error("Black starts")
	}
}

// TestRacingKingsOpeningMoveCount pins the legal-move count from the setup and
// the shape of the game: no pawns, so nothing to push, and with no checking
// allowed a whole class of piece moves disappears.
func TestRacingKingsOpeningMoveCount(t *testing.T) {
	v := builtin(t, "racingkings")
	m := NewMatch(v)
	if st, _ := m.Status(); st != Playing {
		t.Fatalf("new match status = %v", st)
	}
	if m.Turn() != White {
		t.Fatal("Black starts")
	}
	// 21 legal moves, counted by hand from the setup:
	//   Ne2: g3, d4, f4 (c3 and c1 both attack the black king on a2) = 3
	//   Bf2: e3, d4, c5, b6, a7, g3, h4 = 7
	//   Rg2: g3-g8 up the file (f2 and h2 hold own pieces) = 6
	//   Kh2: g3, h3 = 2
	//   Ne1: c2, d3, f3 (g2 holds own rook) = 3
	//   Bf1 (Ne2 and Rg2 in the way), Rg1 (Bf1, Qh1) and Qh1 (Kh2, Rg1): 0.
	if got := len(m.Legal()); got != 21 {
		t.Errorf("%d opening moves, want 21", got)
	}
}

// TestRacingKingsNoCastlingOrPromotion checks that the variant offers neither,
// so no generated move can be a castle or a promotion.
func TestRacingKingsNoCastlingOrPromotion(t *testing.T) {
	v := builtin(t, "racingkings")
	if len(v.Castling) != 0 || len(v.Promotion) != 0 {
		t.Fatalf("%d castles, %d promotions", len(v.Castling), len(v.Promotion))
	}
	for _, p := range []*Position{NewPosition(v), fen(t, v, "8/8/8/8/8/8/krbnNBRK/qrbnNBRQ w -")} {
		for _, m := range p.Moves() {
			if m.Castle >= 0 {
				t.Errorf("castle move %s-%s", v.SquareName(m.From), v.SquareName(m.To))
			}
			if m.Promote != 0 {
				t.Errorf("promotion move %s-%s", v.SquareName(m.From), v.SquareName(m.To))
			}
		}
	}
}

// TestRacingKingsForbidsChecking checks that a move leaving the enemy king
// attacked is never generated, and that other variants keep generating it.
func TestRacingKingsForbidsChecking(t *testing.T) {
	v := builtin(t, "racingkings")
	p := fen(t, v, "8/8/8/8/8/7k/8/K6R w -")
	h1, _ := v.ParseSquare("h1")
	h2, _ := v.ParseSquare("h2")
	b1, _ := v.ParseSquare("b1")
	for _, m := range p.Moves() {
		if m.From == h1 && m.To == h2 {
			t.Error("Rh1-h2 gives check and must not be generated")
		}
		if after := p.Apply(m); after.royalAttacked(Black) {
			t.Errorf("%s-%s leaves the black king attacked", v.SquareName(m.From), v.SquareName(m.To))
		}
	}
	if !slices.ContainsFunc(p.Moves(), func(m Move) bool { return m.From == h1 && m.To == b1 }) {
		t.Error("Rh1-b1 is no check and must stay legal")
	}
	for _, id := range []string{"chess", "kingofthehill"} {
		o := builtin(t, id)
		if o.NoChecks || o.GoalReplyDraw {
			t.Errorf("%s: no_checks %v, goal_reply_draw %v; want both off", id, o.NoChecks, o.GoalReplyDraw)
		}
		// The same board in orthodox chess still offers the checking rook move.
		// Black already stands in the rook's line, so only move generation is
		// being compared here.
		op := fen(t, o, "8/8/8/8/8/7k/8/K6R w -")
		if !slices.ContainsFunc(op.Moves(), func(m Move) bool { return m.From == h1 && m.To == h2 }) {
			t.Errorf("%s: Rh1-h2 missing, checks must stay legal there", id)
		}
	}
}

// TestRacingKingsBlackGoalWins checks a Black king reaching the eighth rank
// wins at once: the reply rule only shields the side that moves first.
func TestRacingKingsBlackGoalWins(t *testing.T) {
	v := builtin(t, "racingkings")
	m := NewMatch(v)
	m.Load(fen(t, v, "8/6k1/8/8/8/8/8/K7 b -"))
	if st, _ := m.Status(); st != Playing {
		t.Fatalf("status = %v, want playing", st)
	}
	// White is seven king moves from the nearest goal square, so it cannot
	// answer at all, let alone in reply.
	a1, _ := v.ParseSquare("a1")
	if d := kingDistance(v, a1, v.Goal); d != 7 {
		t.Errorf("White king %d moves from the eighth rank, want 7", d)
	}
	playAll(t, m, "g7", "g8")
	if st, winner := m.Status(); st != GoalWin || winner != Black || !m.Over() {
		t.Fatalf("status = %v, winner %v, over %v; want a Black goal win", st, winner, m.Over())
	}
}

// kingDistance is how many king moves separate from from the nearest of
// squares.
func kingDistance(v *Variant, from int, squares []int) int {
	fx, fy := v.XY(from)
	best := 1 << 20
	for _, sq := range squares {
		x, y := v.XY(sq)
		best = min(best, max(abs(x-fx), abs(y-fy)))
	}
	return best
}

// TestRacingKingsWhiteGoalWins checks White's king run onto the eighth rank
// wins when Black has no arrival to answer with, both over two moves and at
// once.
func TestRacingKingsWhiteGoalWins(t *testing.T) {
	v := builtin(t, "racingkings")
	m := NewMatch(v)
	m.Load(fen(t, v, "8/8/6K1/8/8/8/8/1k6 w -"))
	playAll(t, m, "g6", "g7")
	if st, _ := m.Status(); st != Playing || m.Over() {
		t.Fatalf("the seventh rank gave %v", st)
	}
	playAll(t, m, "b1", "c1", "g7", "g8")
	if st, winner := m.Status(); st != GoalWin || winner != White || !m.Over() {
		t.Fatalf("status = %v, winner %v, over %v; want a White goal win", st, winner, m.Over())
	}

	m.Load(fen(t, v, "8/6K1/8/8/8/8/8/1k6 w -"))
	playAll(t, m, "g7", "g8")
	if st, winner := m.Status(); st != GoalWin || winner != White || !m.Over() {
		t.Fatalf("status = %v, winner %v, over %v; want an immediate White goal win", st, winner, m.Over())
	}
}

// TestRacingKingsGoalReplyDraw checks that White's arrival is only pending
// while Black can arrive in reply: answering on the eighth rank draws, and any
// other Black move loses to the standing king.
func TestRacingKingsGoalReplyDraw(t *testing.T) {
	v := builtin(t, "racingkings")
	start := "8/k6K/8/8/8/8/8/8 w -"

	m := NewMatch(v)
	m.Load(fen(t, v, start))
	playAll(t, m, "h7", "h8")
	if st, _ := m.Status(); st != Playing || m.Over() {
		t.Fatalf("White's arrival gave %v, want pending while Black can answer", st)
	}
	playAll(t, m, "a7", "a8")
	if st, _ := m.Status(); st != DrawGoalReply || !m.Over() {
		t.Fatalf("status = %v, over %v; want the goal reply draw", st, m.Over())
	}
	if got := DrawGoalReply.String(); got != "draw by the goal reply" {
		t.Errorf("status name = %q", got)
	}

	m.Load(fen(t, v, start))
	playAll(t, m, "h7", "h8", "a7", "b7")
	if st, winner := m.Status(); st != GoalWin || winner != White || !m.Over() {
		t.Fatalf("status = %v, winner %v, over %v; declining must lose", st, winner, m.Over())
	}
	// The winner is the side whose king stands on the eighth rank, not the
	// side that just moved, so the header must name White.
	g := newGame(t, "racingkings", 100, 30)
	g.match.Load(fen(t, v, start))
	g.match.Play(findMove(t, g.match.Position(), "h7", "h8"))
	g.match.Play(findMove(t, g.match.Position(), "a7", "b7"))
	if h := ansi.Strip(g.header()); !strings.Contains(h, "White wins") {
		t.Errorf("header = %q, want White to win", h)
	}
}

// TestRacingKingsSearchFindsKingRun checks the search races the king to the
// eighth rank.
func TestRacingKingsSearchFindsKingRun(t *testing.T) {
	v := builtin(t, "racingkings")
	p := fen(t, v, "8/8/6K1/8/8/8/8/1k6 w -")
	r := NewEngine(v).Think(p, nil, Limits{Depth: 4})
	if !r.Found {
		t.Fatal("no move found")
	}
	if got := describe(p, r.Move); got != "Kg6-g7" {
		t.Errorf("played %s, want Kg6-g7", got)
	}
	if !IsMateScore(r.Score) {
		t.Errorf("score %d is not a win", r.Score)
	}
}

// TestRacingKingsSearchNoFalseGoalWin checks the search does not count White's
// arrival as a win while Black can answer on the eighth rank: the arrival is
// pending and the answer draws.
func TestRacingKingsSearchNoFalseGoalWin(t *testing.T) {
	v := builtin(t, "racingkings")
	p := fen(t, v, "8/1k4K1/8/8/8/8/8/8 w -")
	r := NewEngine(v).Think(p, nil, Limits{Depth: 4})
	if !r.Found {
		t.Fatal("no move found")
	}
	if !slices.Contains(p.Moves(), r.Move) {
		t.Fatalf("illegal move %s-%s", v.SquareName(r.Move.From), v.SquareName(r.Move.To))
	}
	if IsMateScore(r.Score) {
		t.Fatalf("%s scored %d: Black answers on the eighth rank", describe(p, r.Move), r.Score)
	}
	after := p.Apply(findMove(t, p, "g7", "g8"))
	if o, over := adjudicate(after, true); over {
		t.Fatalf("the arrival settled the game: %+v", o)
	}
	reply := NewEngine(v).Think(after, nil, Limits{Depth: 4})
	if !reply.Found {
		t.Fatal("no reply found")
	}
	if !slices.Contains(v.Goal, reply.Move.To) {
		t.Errorf("reply %s does not answer on the eighth rank", describe(after, reply.Move))
	}
	if IsMateScore(reply.Score) || reply.Score < 0 {
		t.Errorf("reply score %d: Black is not lost, it draws by arriving", reply.Score)
	}
}

// TestRacingKingsComputerMove checks the engine returns a legal move from the
// setup.
func TestRacingKingsComputerMove(t *testing.T) {
	v := builtin(t, "racingkings")
	p := NewPosition(v)
	r := NewEngine(v).Think(p, nil, Limits{Depth: 2, Nodes: 20000})
	if !r.Found {
		t.Fatal("no move found")
	}
	if !slices.Contains(p.Moves(), r.Move) {
		t.Errorf("%s-%s is not legal", v.SquareName(r.Move.From), v.SquareName(r.Move.To))
	}
}

// TestRacingKingsBareKingsPlayable checks the material draw is skipped where a
// goal square can still win, and kept in orthodox chess.
func TestRacingKingsBareKingsPlayable(t *testing.T) {
	v := builtin(t, "racingkings")
	m := NewMatch(v)
	m.Load(fen(t, v, "8/8/8/8/8/8/8/K6k w -"))
	if st, _ := m.Status(); st != Playing {
		t.Fatalf("K vs K status = %v, want the race still on", st)
	}

	c := builtin(t, "chess")
	cm := NewMatch(c)
	cm.Load(fen(t, c, "7k/8/8/8/8/8/8/K7 w -"))
	if st, _ := cm.Status(); st != DrawMaterial {
		t.Errorf("chess K vs K status = %v, want the material draw", st)
	}
}

// TestRacingKingsDrawRules checks repetition and the move limit still end the
// race.
func TestRacingKingsDrawRules(t *testing.T) {
	v := builtin(t, "racingkings")
	m := NewMatch(v)
	m.Load(fen(t, v, "8/8/8/8/8/8/8/K6k w -"))
	for range 2 {
		playAll(t, m, "a1", "a2", "h1", "h2", "a2", "a1", "h2", "h1")
	}
	if st, _ := m.Status(); st != DrawRepetition {
		t.Errorf("status = %v, want repetition", st)
	}

	m = NewMatch(v)
	p := fen(t, v, "8/8/8/8/8/8/8/K6k w -")
	p.Clock = 2*v.MoveLimit - 1
	m.Load(p)
	playAll(t, m, "a1", "b1")
	if st, _ := m.Status(); st != DrawMoveLimit {
		t.Errorf("status = %v, want the move limit", st)
	}
}

// TestRacingKingsGoalReplyFlagNeedsGoal checks the load error that keeps the
// reply rule from silently doing nothing.
func TestRacingKingsGoalReplyFlagNeedsGoal(t *testing.T) {
	src := []byte("id: replydraw\nname: Reply Draw\nfiles: 3\nranks: 3\n" +
		"pieces:\n  - {letter: K, name: King, betza: K, royal: true}\nsetup: kkk/3/KKK\ngoal_reply_draw: true\n")
	if _, err := parseVariants(map[string][]byte{"replydraw.yaml": src}); err == nil {
		t.Error("goal_reply_draw without goal squares loaded")
	}
}

// TestRacingKingsFlagsInheritAndClear checks no_checks and goal_reply_draw
// follow extends like every other field, and that a child file may turn them
// off again.
func TestRacingKingsFlagsInheritAndClear(t *testing.T) {
	vs, err := parseVariants(map[string][]byte{
		"keeps.yaml":   []byte("id: keeps\nname: Keeps\nextends: racingkings\n"),
		"cleared.yaml": []byte("id: cleared\nname: Cleared\nextends: racingkings\nno_checks: false\ngoal_reply_draw: false\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	keeps, err := Find(vs, "keeps")
	if err != nil {
		t.Fatal(err)
	}
	if !keeps.NoChecks || !keeps.GoalReplyDraw || !slices.Equal(keeps.Goal, builtin(t, "racingkings").Goal) {
		t.Errorf("inherited no_checks %v, goal_reply_draw %v, goal %v", keeps.NoChecks, keeps.GoalReplyDraw, keeps.Goal)
	}
	cleared, err := Find(vs, "cleared")
	if err != nil {
		t.Fatal(err)
	}
	if cleared.NoChecks || cleared.GoalReplyDraw {
		t.Errorf("cleared no_checks %v, goal_reply_draw %v", cleared.NoChecks, cleared.GoalReplyDraw)
	}
}

package chess

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// rulesVariant loads one extra variant file and returns the variant with id.
func rulesVariant(t *testing.T, id, src string) *Variant {
	t.Helper()
	return rulesChain(t, id, map[string]string{"rules-" + id + ".yaml": src})
}

// rulesChain loads extra variant files and returns the variant with id. A
// variant can only extend one loaded before it, so the file names decide the
// order.
func rulesChain(t *testing.T, id string, files map[string]string) *Variant {
	t.Helper()
	srcs := make(map[string][]byte, len(files))
	for name, src := range files {
		srcs[name] = []byte(src)
	}
	vs, err := parseVariants(srcs)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	v, err := Find(vs, id)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	return v
}

// rulesSplit separates the moves from one square into the ones that stay a
// pawn ("b7") and the ones that promote ("b8=Q"), both sorted.
func rulesSplit(t *testing.T, v *Variant, moves []Move, from string) (plain, promos []string) {
	t.Helper()
	sq, err := v.ParseSquare(from)
	if err != nil {
		t.Fatalf("%s: %v", from, err)
	}
	for _, m := range moves {
		if m.From != sq {
			continue
		}
		name := v.SquareName(m.To)
		if m.Promote == 0 {
			plain = append(plain, name)
			continue
		}
		promos = append(promos, name+"="+string(v.Pieces[m.Promote-1].Letter))
	}
	slices.Sort(plain)
	slices.Sort(promos)
	return plain, promos
}

func rulesMoves(t *testing.T, p *Position, from string) (plain, promos []string) {
	t.Helper()
	return rulesSplit(t, p.V, p.Moves(), from)
}

func rulesCaptures(t *testing.T, p *Position, from string) (plain, promos []string) {
	t.Helper()
	return rulesSplit(t, p.V, p.captures(), from)
}

// rulesDecline finds the move from-to that stays a pawn.
func rulesDecline(t *testing.T, p *Position, from, to string) Move {
	t.Helper()
	f, _ := p.V.ParseSquare(from)
	sq, _ := p.V.ParseSquare(to)
	for _, m := range p.Moves() {
		if m.From == f && m.To == sq && m.Promote == 0 {
			return m
		}
	}
	t.Fatalf("no declining move %s-%s", from, to)
	return Move{}
}

func rulesSquare(t *testing.T, v *Variant, name string) int {
	t.Helper()
	sq, err := v.ParseSquare(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return sq
}

const rulesGrandish = `
id: rules-captured
name: Rules captured-only
extends: chess
promotion: [Q, R, B, N]
promotion_captured_only: true
`

const rulesOptional = `
id: rules-optional
name: Rules optional
extends: chess
promotion_zone: 3
promotion_optional: true
`

func TestPromotionFlagsFromYAML(t *testing.T) {
	v := rulesVariant(t, "rules-captured", rulesGrandish)
	if !v.PromotionCapturedOnly || v.PromotionOptional {
		t.Errorf("captured_only %v, optional %v; want true, false", v.PromotionCapturedOnly, v.PromotionOptional)
	}
	inherit := rulesChain(t, "rules-inherit", map[string]string{
		"a.yaml": rulesGrandish,
		"b.yaml": `
id: rules-inherit
name: Rules inherited
extends: rules-captured
move_limit: 20
`,
	})
	if !inherit.PromotionCapturedOnly || inherit.PromotionOptional || inherit.MoveLimit != 20 {
		t.Errorf("inherited captured_only %v, optional %v, move limit %d",
			inherit.PromotionCapturedOnly, inherit.PromotionOptional, inherit.MoveLimit)
	}
	both := rulesChain(t, "rules-both", map[string]string{
		"a.yaml": rulesGrandish,
		"b.yaml": `
id: rules-both
name: Rules both
extends: rules-captured
promotion_optional: true
`,
	})
	if !both.PromotionCapturedOnly || !both.PromotionOptional {
		t.Errorf("captured_only %v, optional %v; want true, true", both.PromotionCapturedOnly, both.PromotionOptional)
	}
	// An explicit false overrides what was inherited.
	cleared := rulesChain(t, "rules-off", map[string]string{
		"a.yaml": rulesGrandish,
		"b.yaml": `
id: rules-both
name: Rules both
extends: rules-captured
promotion_optional: true
`,
		"c.yaml": `
id: rules-off
name: Rules both off
extends: rules-both
promotion_captured_only: false
promotion_optional: false
`,
	})
	if cleared.PromotionOptional || cleared.PromotionCapturedOnly {
		t.Errorf("captured_only %v, optional %v; want false, false", cleared.PromotionCapturedOnly, cleared.PromotionOptional)
	}
	// The orthodox variant keeps the plain rules.
	chess := builtin(t, "chess")
	if chess.PromotionOptional || chess.PromotionCapturedOnly {
		t.Error("chess gained a promotion restriction")
	}
}

func TestPromotionCapturedOnlyAvailability(t *testing.T) {
	v := rulesVariant(t, "rules-captured", rulesGrandish)

	// Nothing of White's is left, so every type is available.
	p := fen(t, v, "7k/1P6/8/8/8/8/8/4K3 w -")
	plain, promos := rulesMoves(t, p, "b7")
	if len(plain) != 0 {
		t.Errorf("plain moves to the last rank: %v", plain)
	}
	if want := []string{"b8=B", "b8=N", "b8=Q", "b8=R"}; !slices.Equal(promos, want) {
		t.Errorf("promotions %v, want %v", promos, want)
	}

	// White's queen is still on the board, so it cannot promote to one; the
	// setup holds one, so the count has to drop first.
	p = fen(t, v, "7k/1P6/8/8/8/8/8/3QK3 w -")
	if _, promos := rulesMoves(t, p, "b7"); !slices.Equal(promos, []string{"b8=B", "b8=N", "b8=R"}) {
		t.Errorf("promotions %v with the queen alive, want B, N, R", promos)
	}

	// Every type sits at its setup count: the pawn cannot enter the last rank
	// at all, by push or by capture.
	p = fen(t, v, "2r1k3/1P6/8/8/8/8/8/RNBQKBNR w -")
	plain, promos = rulesMoves(t, p, "b7")
	if len(plain)+len(promos) != 0 {
		t.Errorf("pawn entered the last rank: %v %v", plain, promos)
	}
	if cplain, cpromos := rulesCaptures(t, p, "b7"); len(cplain)+len(cpromos) != 0 {
		t.Errorf("quiescence generated %v %v", cplain, cpromos)
	}

	// Both rooks still stand, so the pawn cannot become a rook, while the
	// queen, bishops and knights it never had are all free.
	p = fen(t, v, "7k/1P6/8/8/8/8/8/R3K2R w -")
	if _, promos := rulesMoves(t, p, "b7"); !slices.Equal(promos, []string{"b8=B", "b8=N", "b8=Q"}) {
		t.Errorf("promotions %v with both rooks alive, want B, N, Q", promos)
	}
	// With the queen gone the same capture promotes, so a pawn that cannot
	// promote loses the capture as well as the push.
	p = fen(t, v, "2r1k3/1P6/8/8/8/8/8/4K3 w -")
	if _, promos := rulesMoves(t, p, "b7"); !slices.Contains(promos, "c8=Q") || !slices.Contains(promos, "c8=R") {
		t.Errorf("promotions %v, want the capture on c8 to promote", promos)
	}
	// Losing one rook frees the rook promotion as well.
	p = fen(t, v, "7k/1P6/8/8/8/8/8/R3K3 w -")
	if _, promos := rulesMoves(t, p, "b7"); !slices.Contains(promos, "b8=R") {
		t.Errorf("promotions %v with one rook, want R among them", promos)
	}
}

func TestPromotionOptionalInsideZone(t *testing.T) {
	v := rulesVariant(t, "rules-optional", rulesOptional)
	// A pawn on the zone's first rank may decline; on the last rank the
	// promotion is forced.
	p := fen(t, v, "k7/8/1P6/8/8/8/8/4K3 w -")
	plain, promos := rulesMoves(t, p, "b6")
	if want := []string{"b7"}; !slices.Equal(plain, want) {
		t.Errorf("plain moves %v, want %v", plain, want)
	}
	if want := []string{"b7=B", "b7=N", "b7=Q", "b7=R"}; !slices.Equal(promos, want) {
		t.Errorf("promotions %v, want %v", promos, want)
	}
	// Quiescence keeps the promotions and drops the quiet arrival.
	cplain, cpromos := rulesCaptures(t, p, "b6")
	if len(cplain) != 0 || !slices.Equal(cpromos, promos) {
		t.Errorf("quiescence plain %v promotions %v", cplain, cpromos)
	}
	// Declining leaves a pawn that resets the clock.
	declined := p.Apply(rulesDecline(t, p, "b6", "b7"))
	if pc := declined.Board[rulesSquare(t, v, "b7")]; pc.Type != v.TypeOf('P') || declined.Clock != 0 {
		t.Errorf("declined promotion left %+v, clock %d", pc, declined.Clock)
	}
	// A capture inside the zone offers the same choice.
	p = fen(t, v, "7k/q7/1P6/8/8/8/8/4K3 w -")
	cplain, cpromos = rulesCaptures(t, p, "b6")
	if !slices.Contains(cplain, "a7") {
		t.Errorf("declining capture missing: %v", cplain)
	}
	// Four ways to take the queen on a7 and four to push to b7.
	if len(cpromos) != 8 || !slices.Contains(cpromos, "a7=Q") || !slices.Contains(cpromos, "b7=Q") {
		t.Errorf("promotions in the zone %v", cpromos)
	}
	// No decline on the last rank.
	p = fen(t, v, "7k/1P6/8/8/8/8/8/4K3 w -")
	plain, promos = rulesMoves(t, p, "b7")
	if len(plain) != 0 {
		t.Errorf("plain moves on the last rank: %v", plain)
	}
	if want := []string{"b8=B", "b8=N", "b8=Q", "b8=R"}; !slices.Equal(promos, want) {
		t.Errorf("promotions %v, want %v", promos, want)
	}
	// Black mirrors it: the third rank is still optional, the first is not.
	p = fen(t, v, "3k4/8/8/8/8/1p6/8/4K3 b -")
	plain, promos = rulesMoves(t, p, "b3")
	if want := []string{"b2"}; !slices.Equal(plain, want) {
		t.Errorf("black plain moves %v, want %v", plain, want)
	}
	if want := []string{"b2=B", "b2=N", "b2=Q", "b2=R"}; !slices.Equal(promos, want) {
		t.Errorf("black promotions on the third rank %v, want %v", promos, want)
	}
	p = fen(t, v, "3k4/8/8/8/8/8/1p6/4K3 b -")
	plain, promos = rulesMoves(t, p, "b2")
	if len(plain) != 0 {
		t.Errorf("black pawn declined on its last rank: %v", plain)
	}
	if want := []string{"b1=B", "b1=N", "b1=Q", "b1=R"}; !slices.Equal(promos, want) {
		t.Errorf("black promotions %v, want %v", promos, want)
	}
}

func TestPromotionUnavailableBlocksEntry(t *testing.T) {
	// Optional promotion inside the zone with the only type already on the
	// board: the pawn may still step up, but only as a pawn.
	v := rulesVariant(t, "rules-captured-optional", `
id: rules-captured-optional
name: Rules captured-only optional
extends: chess
promotion: [Q]
promotion_zone: 3
promotion_optional: true
promotion_captured_only: true
`)
	p := fen(t, v, "k7/8/1P6/8/8/8/8/3QK3 w -")
	plain, promos := rulesMoves(t, p, "b6")
	if want := []string{"b7"}; !slices.Equal(plain, want) || len(promos) != 0 {
		t.Errorf("plain %v promotions %v, want %v and none", plain, promos, want)
	}
	// The last rank stays closed while the queen lives.
	p = fen(t, v, "7k/1P6/8/8/8/8/8/3QK3 w -")
	if plain, promos := rulesMoves(t, p, "b7"); len(plain)+len(promos) != 0 {
		t.Errorf("pawn entered the last rank: %v %v", plain, promos)
	}
	// Without the optional rule the zone entry is closed too.
	strict := rulesVariant(t, "rules-captured-strict", `
id: rules-captured-strict
name: Rules captured-only strict
extends: chess
promotion: [Q]
promotion_zone: 3
promotion_captured_only: true
`)
	p = fen(t, strict, "k7/8/1P6/8/8/8/8/3QK3 w -")
	if plain, promos := rulesMoves(t, p, "b6"); len(plain)+len(promos) != 0 {
		t.Errorf("pawn entered the zone without promoting: %v %v", plain, promos)
	}
	// Once the queen is gone both rules give the promotion back.
	p = fen(t, strict, "k7/8/1P6/8/8/8/8/4K3 w -")
	plain, promos = rulesMoves(t, p, "b6")
	if len(plain) != 0 || !slices.Equal(promos, []string{"b7=Q"}) {
		t.Errorf("plain %v promotions %v, want only b7=Q", plain, promos)
	}
}

func TestPromotionUnavailableKeepsAttack(t *testing.T) {
	v := rulesVariant(t, "rules-captured", rulesGrandish)
	// White's queen is alive, so the pawn on c7 can never promote to one; its
	// attack on b8 and d8 counts all the same.
	p := fen(t, v, "k7/2P5/8/8/8/8/8/3QK3 b -")
	if p.InCheck() {
		t.Fatal("position is already check")
	}
	king := rulesSquare(t, v, "a8")
	stepped := false
	for _, m := range p.Moves() {
		if m.From != king {
			continue
		}
		if name := v.SquareName(m.To); name == "b8" {
			t.Errorf("king stepped onto a square the pawn attacks")
		}
		stepped = stepped || v.SquareName(m.To) == "a7"
	}
	if !stepped {
		t.Error("black king has no quiet step")
	}
	// A pawn that cannot promote still gives check.
	p = fen(t, v, "3k4/2P5/8/8/8/8/8/3QK3 b -")
	if !p.InCheck() {
		t.Error("pawn with no promotion left does not check")
	}
	if len(p.Moves()) == 0 || !slices.ContainsFunc(p.Moves(), func(m Move) bool { return m.Take == rulesSquare(t, v, "c7") }) {
		t.Error("the king may not take the checking pawn")
	}
	// Castling is checked the same way: a pawn attack keeps the king home.
	castle := rulesVariant(t, "rules-castle", `
id: rules-castle
name: Rules castling through attack
extends: chess
promotion: [Q]
promotion_captured_only: true
`)
	p = fen(t, castle, "3qk3/8/8/8/8/8/5p2/R3K2R w -")
	for _, m := range p.Moves() {
		if m.Castle >= 0 {
			t.Errorf("castled through a pawn attack: %+v", m)
		}
	}
	if len(p.Moves()) == 0 {
		t.Error("white has no moves")
	}
}

func TestPromotionRulesSearch(t *testing.T) {
	// A promotion is the only mate: the pawn must take it, on either rule set.
	for _, id := range []string{"chess", "rules-optional"} {
		v := builtin(t, "chess")
		if id != "chess" {
			v = rulesVariant(t, "rules-optional", rulesOptional)
		}
		p := fen(t, v, "k7/ppP5/8/8/8/8/8/4K3 w -")
		r := NewEngine(v).Think(p, nil, Limits{Depth: 3, Time: 20 * time.Second})
		if !r.Found || r.Move.Promote == 0 {
			t.Fatalf("%s: played %s, want a promotion", id, moveName(p, r.Move))
		}
		child := p.Apply(r.Move)
		if !child.InCheck() || len(child.Moves()) != 0 {
			t.Errorf("%s: %s is not mate", id, moveName(p, r.Move))
		}
		if !IsMateScore(r.Score) {
			t.Errorf("%s: score %d is not a mate score", id, r.Score)
		}
	}

	// The grand rules restrict promotions, so the search has to live with
	// fewer of them and still return a legal move.
	grand := builtin(t, "grand")
	pos := fen(t, grand, "9k/10/2P7/10/10/10/10/10/10/4K5 w -")
	if _, promos := rulesMoves(t, pos, "c8"); len(promos) == 0 {
		t.Fatal("no promotions available in the test position")
	}
	r := NewEngine(grand).Think(pos, nil, Limits{Depth: 3, Time: 5 * time.Second})
	if !r.Found || !slices.Contains(pos.Moves(), r.Move) {
		t.Fatalf("engine returned illegal move %+v", r)
	}
}

func TestCapturedOnlyPromotionsInPlay(t *testing.T) {
	grand := builtin(t, "grand")
	// White's queen and both rooks stand, so its two pawns on the eighth rank
	// may only become a bishop, a knight, a marshal or a cardinal, and never
	// more of those than the setup holds.
	m := NewMatch(grand)
	m.Load(fen(t, grand, "r3k3r1/1nbq1mcbn1/ppPP6/10/10/10/10/10/1Q8/R3K3R1 w -"))
	e := NewEngine(grand)
	limits := Limits{Depth: 2, Time: 5 * time.Second}
	promoted := 0
	for plies := 0; plies < 12 && !m.Over(); plies++ {
		r := e.Think(m.Position(), m.Hashes(), limits)
		if !r.Found || !slices.Contains(m.Legal(), r.Move) {
			t.Fatalf("ply %d: illegal move %+v", plies, r)
		}
		if to := r.Move.To; r.Move.Promote == 0 && m.Position().Board[r.Move.From].Type != 0 {
			if grand.Type(m.Position().Board[r.Move.From]).Pawn && m.Position().lastRank(to) {
				t.Fatalf("ply %d: %s entered the last rank without promoting", plies, moveName(m.Position(), r.Move))
			}
		}
		if r.Move.Promote != 0 {
			promoted++
		}
		m.Play(r.Move)
		for _, side := range []Color{White, Black} {
			for typ := range grand.Pieces {
				limit := grand.startCount(side, uint8(typ+1))
				n := 0
				for _, pc := range m.Position().Board {
					if pc.Type == uint8(typ+1) && pc.Color == side {
						n++
					}
				}
				if n > limit {
					t.Fatalf("after %v: %s has %d %s, more than the setup's %d",
						m.Notation(), side, n, grand.Pieces[typ].Name, limit)
				}
			}
		}
	}
	if len(m.Notation()) == 0 || promoted == 0 {
		t.Fatalf("nothing was promoted: %v", m.Notation())
	}
}

func TestPromotionNotation(t *testing.T) {
	v := rulesVariant(t, "rules-optional", rulesOptional)
	p := fen(t, v, "k7/8/1P6/8/8/8/8/4K3 w -")
	declined := rulesDecline(t, p, "b6", "b7")
	if got := describe(p, declined); got != "b6-b7" {
		t.Errorf("declined promotion written %q", got)
	}
	m := NewMatch(v)
	m.Load(p)
	m.Play(declined)
	if got := m.Notation(); len(got) != 1 || strings.Contains(got[0], "=") {
		t.Errorf("notation %v", got)
	}
	if pc := m.Position().Board[rulesSquare(t, v, "b7")]; pc.Type != v.TypeOf('P') {
		t.Errorf("declined promotion left %+v", pc)
	}
	if got := describe(p, findMove(t, p, "b6", "b7")); !strings.Contains(got, "=") {
		t.Errorf("promotion written %q", got)
	}
}

package chess

import (
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestSelfCaptureVariant(t *testing.T) {
	vs, errs := Builtins()
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	v, err := Find(vs, "selfcapture")
	if err != nil {
		t.Fatal(err)
	}
	if v.Files != 8 || v.Ranks != 8 || !v.SelfCapture || len(v.Pieces) != 6 || len(v.Castling) != 2 {
		t.Fatalf("unexpected variant: %+v", v)
	}
	p := NewPosition(v)
	moves := p.Moves()
	if len(moves) != 39 {
		t.Errorf("start has %d moves, want the 20 orthodox plus 19 self-captures", len(moves))
	}
	want := []string{
		"Ra1xb1", "Ra1xa2", "Nb1xd2", "Bc1xd2", "Bc1xb2", "Qd1xd2", "Qd1xc1", "Qd1xe2",
		"Qd1xc2", "Ke1xf1", "Ke1xe2", "Ke1xd1", "Ke1xf2", "Ke1xd2", "Bf1xg2", "Bf1xe2",
		"Ng1xe2", "Rh1xh2", "Rh1xg1",
	}
	var got []string
	for _, m := range moves {
		if m.Take >= 0 {
			got = append(got, describe(p, m))
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("captures from the start = %v, want %v", got, want)
	}
	for _, m := range moves {
		if m.Take >= 0 && v.Type(p.Board[m.Take]).Royal {
			t.Fatal("generated own-king capture")
		}
	}
	// Pawn diagonals can capture friendly pieces, while forward moves cannot.
	p, err = ParseFEN(v, "7k/8/8/8/8/2P5/1P6/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if !hasLong(p, p.Moves(), "b2xc3") {
		t.Fatal("pawn did not self-capture diagonally")
	}
	if hasLong(p, p.Moves(), "b2b3") {
		t.Fatal("pawn moved into occupied own piece")
	}
	// A king's self-capture may not leave it attacked.
	p, err = ParseFEN(v, "7k/8/8/8/8/8/1r6/KR6 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if hasLong(p, p.Moves(), "Ka1xb1") {
		t.Fatal("king self-captured onto attacked square")
	}
	// Own royal pieces are never capture targets.
	p, err = ParseFEN(v, "7k/8/8/8/8/8/8/KR6 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if hasLong(p, p.Moves(), "Rb1xa1") {
		t.Fatal("rook captured own king")
	}
}

func TestSelfCaptureSearchStart(t *testing.T) {
	vs, errs := Builtins()
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	v, err := Find(vs, "selfcapture")
	if err != nil {
		t.Fatal(err)
	}
	p := NewPosition(v)
	r := NewEngine(v).Think(p, nil, Limits{Time: time.Second, Depth: 1})
	if !r.Found || !containsMove(p.Moves(), r.Move) {
		t.Fatalf("search returned illegal move: %+v", r)
	}
}

func TestExistingVariantsDoNotSelfCapture(t *testing.T) {
	for _, id := range []string{"chess", "double", "bigboard", "doublebig"} {
		vs, errs := Builtins()
		if len(errs) > 0 {
			t.Fatal(errs)
		}
		v, err := Find(vs, id)
		if err != nil {
			t.Fatal(err)
		}
		p := NewPosition(v)
		for _, m := range p.Moves() {
			if m.Take >= 0 && p.Board[m.Take].Color == p.Turn {
				t.Fatalf("%s generated self-capture", id)
			}
		}
	}
}

func TestSelfCaptureClick(t *testing.T) {
	g := newGame(t, "selfcapture", 80, 24)
	clickSquare(t, g, "b1", tea.MouseLeft)
	clickSquare(t, g, "d2", tea.MouseLeft)
	if got := g.match.Notation(); len(got) != 1 || got[0] != "Nb1xd2" {
		t.Fatalf("clicking a friendly target did not self-capture: %v", got)
	}
	clickSquare(t, g, "g1", tea.MouseLeft)
	clickSquare(t, g, "g1", tea.MouseLeft)
	if g.sel != -1 {
		t.Error("clicking the selected piece did not deselect it")
	}
}

func TestFriendlyClickReselectsWithoutSelfCapture(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	clickSquare(t, g, "b1", tea.MouseLeft)
	clickSquare(t, g, "d2", tea.MouseLeft)
	if len(g.match.Notation()) != 0 {
		t.Fatalf("orthodox chess self-captured: %v", g.match.Notation())
	}
	if got, _ := g.match.Variant().ParseSquare("d2"); g.sel != got {
		t.Error("friendly click did not reselect")
	}
}

func TestSelfCaptureInheritance(t *testing.T) {
	src := `
id: noselfcapture
name: No self-capture
extends: selfcapture
self_capture: false
`
	vs, err := parseVariants(map[string][]byte{"a.yaml": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "noselfcapture")
	if err != nil {
		t.Fatal(err)
	}
	if v.SelfCapture {
		t.Error("self_capture: false did not drop the inherited rule")
	}
	parent, err := Find(vs, "selfcapture")
	if err != nil {
		t.Fatal(err)
	}
	if !parent.SelfCapture {
		t.Error("parent lost self-capture")
	}
}

func TestSelfCapturePromotesAndResetsTheClock(t *testing.T) {
	vs, errs := Builtins()
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	v, err := Find(vs, "selfcapture")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseFEN(v, "R6k/1P6/8/8/8/8/8/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	var promos []Move
	for _, m := range p.Moves() {
		if m.From == m.To {
			continue
		}
		if describe(p, m) == "b7xa8=Q" || describe(p, m) == "b7xa8=R" || describe(p, m) == "b7xa8=B" || describe(p, m) == "b7xa8=N" {
			promos = append(promos, m)
		}
	}
	if len(promos) != 4 {
		t.Fatalf("self-capture promotions = %d, want 4", len(promos))
	}
	m := NewMatch(v)
	m.Load(p)
	m.Play(promos[0])
	// A self-capture is a capture for the fifty-move clock even when no pawn moves.
	q, err := ParseFEN(v, "7k/8/8/8/8/8/r7/R5K1 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	m.Load(q)
	for _, mv := range m.Legal() {
		if describe(q, mv) == "Ra1xa2" {
			m.Play(mv)
			if clock := m.Position().Clock; clock != 0 {
				t.Errorf("move clock = %d after a rook self-capture, want 0", clock)
			}
			return
		}
	}
	t.Fatal("rook could not capture its own pawn")
}

func hasLong(p *Position, ms []Move, s string) bool {
	for _, m := range ms {
		if describe(p, m) == s {
			return true
		}
	}
	return false
}
func containsMove(ms []Move, m Move) bool {
	for _, x := range ms {
		if x == m {
			return true
		}
	}
	return false
}

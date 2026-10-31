package chess

import "testing"

func TestHordeMovedPawnOnDoubleRank(t *testing.T) {
	v := builtin(t, "horde")
	p := fen(t, v, "7k/8/8/8/8/8/P7/8 w -")
	sq, _ := v.ParseSquare("a2")
	p.Board[sq].Moved = true
	if !hordeHasMove(t, p, "a2", "a4") {
		t.Fatal("rank-based double step must not depend on moved flag")
	}
}

func TestRoyalLessStalemateQuiescence(t *testing.T) {
	v := builtin(t, "horde")
	p := fen(t, v, "P6k/8/8/8/8/8/8/8 w -")
	if len(p.Moves()) != 0 {
		t.Fatal("expected no moves")
	}
	e := NewEngine(v)
	e.begin(nil, Limits{Depth: 1})
	if score := e.quiesce(p, 0, -infinity, infinity); score != 0 {
		t.Fatalf("stalemate score %d", score)
	}
}

func TestDoubleRankConfigInheritance(t *testing.T) {
	src := map[string][]byte{
		"rankchild.yaml": []byte("id: rankchild\nextends: horde\ndouble_step_ranks: []\nep_exempt_ranks: []\n"),
	}
	vs, err := parseVariants(src)
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "rankchild")
	if err != nil {
		t.Fatal(err)
	}
	if v.PawnDoubleRanks == nil || len(v.PawnDoubleRanks) != 0 || v.EPExemptRanks == nil || len(v.EPExemptRanks) != 0 {
		t.Fatalf("empty rank lists lost: %#v %#v", v.PawnDoubleRanks, v.EPExemptRanks)
	}
	p := fen(t, v, "7k/8/8/8/8/8/P7/8 w -")
	if hordeHasMove(t, p, "a2", "a4") {
		t.Fatal("empty double-step ranks did not suppress legacy double")
	}
}

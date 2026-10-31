package chess

import (
	"slices"
	"testing"
)

func dropSearchVariant(t *testing.T) *Variant {
	t.Helper()
	return builtin(t, "crazyhouse")
}

func dropSearchType(t *testing.T, v *Variant, letter byte) uint8 {
	t.Helper()
	for i, typ := range v.Pieces {
		if typ.Letter == letter {
			return uint8(i + 1)
		}
	}
	t.Fatalf("missing piece %c", letter)
	return 0
}

func TestDropSearchLegal(t *testing.T) {
	v := dropSearchVariant(t)
	start := NewPosition(v)
	p := fen(t, v, "4k3/8/8/8/8/8/8/4K3 w -")
	knight := dropSearchType(t, v, 'N')
	p.Hands[White] = make([]int, len(v.Pieces)+1)
	p.Hands[White][knight] = 1
	for _, pos := range []*Position{start, p} {
		r := NewEngine(v).Think(pos, nil, Limits{Nodes: 500, Depth: 3})
		if !r.Found || !slices.Contains(pos.Moves(), r.Move) {
			t.Errorf("search returned illegal result: %+v", r)
		}
	}
}

func TestDropNullAndRepetition(t *testing.T) {
	v := dropSearchVariant(t)
	p := NewPosition(v)
	n := dropSearchType(t, v, 'N')
	p.Hands[White] = make([]int, len(v.Pieces)+1)
	p.Hands[Black] = make([]int, len(v.Pieces)+1)
	p.Hands[White][n], p.Hands[Black][n] = 1, 2
	q := p.null()
	q.Hands[White][n]++
	q.Hands[Black][n]++
	if p.Hands[White][n] != 1 || p.Hands[Black][n] != 2 || q.Turn != Black {
		t.Fatal("null mutated original or failed to switch turn")
	}
	e := NewEngine(v)
	e.path = []uint64{42, 99}
	if !e.repeated(42, 0) {
		t.Error("drop repetition missed earlier history across clock reset")
	}
	ordinary := NewEngine(builtin(t, "chess"))
	ordinary.path = e.path
	if ordinary.repeated(42, 0) {
		t.Error("ordinary chess repetition ignored clock")
	}
}

func TestDropEvaluationAndVictim(t *testing.T) {
	v := dropSearchVariant(t)
	p := fen(t, v, "4k3/8/8/3q4/8/8/8/3RK3 w -")
	e := NewEngine(v)
	n, pawn, queen := dropSearchType(t, v, 'N'), dropSearchType(t, v, 'P'), dropSearchType(t, v, 'Q')
	base := e.eval.evaluate(p)
	p.Hands[White] = make([]int, len(v.Pieces)+1)
	p.Hands[White][n] = 2
	if got := e.eval.evaluate(p) - base; got != 2*pocketValue(v.Pieces[n-1].Value) {
		t.Errorf("pocket evaluation = %d", got)
	}
	mv := findMove(t, p, "d1", "d5")
	want := v.Pieces[queen-1].Value + pocketValue(v.Pieces[queen-1].Value)
	if got := e.victimValue(p, mv); got != want {
		t.Errorf("victim = %d, want %d", got, want)
	}
	p.Board[mv.Take].Promoted = pawn
	want = v.Pieces[queen-1].Value + pocketValue(v.Pieces[pawn-1].Value)
	if got := e.victimValue(p, mv); got != want {
		t.Errorf("promoted victim = %d, want %d", got, want)
	}
}

func TestDropMatchAndNotation(t *testing.T) {
	v := dropSearchVariant(t)
	p := fen(t, v, "4k3/8/8/8/8/8/8/4K3 w -")
	m := NewMatch(v)
	m.Load(p)
	if s, _ := m.Status(); s != DrawMaterial {
		t.Fatalf("bare kings status %v", s)
	}
	n := dropSearchType(t, v, 'N')
	p.Hands[White] = make([]int, len(v.Pieces)+1)
	p.Hands[White][n] = 1
	m.Load(p)
	if s, _ := m.Status(); s != Playing {
		t.Fatalf("pocket status %v", s)
	}
	var drop Move
	for _, mv := range m.Legal() {
		if mv.Drop == n && mv.To == v.Square(5, 2) {
			drop = mv
			break
		}
	}
	if drop.Drop == 0 {
		t.Fatal("N@f3 unavailable")
	}
	m.Play(drop)
	if got := m.Notation()[0]; got != "N@f3" {
		t.Errorf("notation = %q", got)
	}
}

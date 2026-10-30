package chess

import (
	"reflect"
	"slices"
	"testing"
)

func TestBerolinaInheritance(t *testing.T) {
	v, standard := builtin(t, "berolina"), builtin(t, "chess")
	if v.Files != 8 || v.Ranks != 8 || len(v.Pieces) != 6 || len(v.Castling) != 2 {
		t.Fatalf("unexpected board or pieces: %+v", v)
	}
	if !reflect.DeepEqual(v.Castling, standard.Castling) || !reflect.DeepEqual(v.Promotion, standard.Promotion) {
		t.Fatal("castling or promotion differs from orthodox chess")
	}
	for _, letter := range "KQRBN" {
		if !reflect.DeepEqual(v.Pieces[v.TypeOf(byte(letter))-1], standard.Pieces[standard.TypeOf(byte(letter))-1]) {
			t.Errorf("%c differs from chess", letter)
		}
	}
	pawn := v.Pieces[v.TypeOf('P')-1]
	if pawn.Betza != "mfFcfWifmnA" || !pawn.Pawn {
		t.Fatalf("pawn: %+v", pawn)
	}
	if !reflect.DeepEqual(NewPosition(v).Board, NewPosition(standard).Board) {
		t.Fatal("setup differs from orthodox chess")
	}
}

func TestBerolinaBetza(t *testing.T) {
	c, err := ParseBetza("mfFcfWifmnA")
	if err != nil || len(c) != 3 {
		t.Fatalf("components: %v, %v", c, err)
	}
	if !c[0].Move || c[0].Capture || c[0].Initial || c[0].Lame || !slices.Equal(c[0].Vectors, [][2]int{{1, 1}, {-1, 1}}) {
		t.Errorf("diagonal move: %+v", c[0])
	}
	if c[1].Move || !c[1].Capture || !slices.Equal(c[1].Vectors, [][2]int{{0, 1}}) {
		t.Errorf("straight capture: %+v", c[1])
	}
	if !c[2].Move || c[2].Capture || !c[2].Initial || !c[2].Lame || c[2].Range != 1 || !slices.Equal(c[2].Vectors, [][2]int{{2, 2}, {-2, 2}}) {
		t.Errorf("initial move: %+v", c[2])
	}
}

func TestBerolinaOpeningMoves(t *testing.T) {
	p := NewPosition(builtin(t, "berolina"))
	expected := map[string][]string{
		"a2": {"b3", "c4"}, "b2": {"a3", "c3", "d4"}, "c2": {"a4", "b3", "d3", "e4"},
		"d2": {"b4", "c3", "e3", "f4"}, "e2": {"c4", "d3", "f3", "g4"},
		"f2": {"d4", "e3", "g3", "h4"}, "g2": {"e4", "f3", "h3"}, "h2": {"f4", "g3"},
		"b1": {"a3", "c3"}, "g1": {"f3", "h3"},
	}
	total := 0
	for from, want := range expected {
		got := destinations(p, from)
		if !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", from, got, want)
		}
		total += len(want)
	}
	if got := len(p.Moves()); got != total {
		t.Errorf("opening moves %d, want %d", got, total)
	}
}

func TestBerolinaPawnMovesAndBlocking(t *testing.T) {
	v := builtin(t, "berolina")
	p := fen(t, v, "4k3/8/8/8/8/3P1n2/4P3/4K3 w -")
	if got := destinations(p, "e2"); len(got) != 0 {
		t.Errorf("blocked pawn: %v", got)
	}
	p = fen(t, v, "4k3/8/8/8/4n3/4P3/8/4K3 w -")
	if got := destinations(p, "e3"); !slices.Equal(got, []string{"d4", "e4", "f4"}) {
		t.Errorf("capture moves: %v", got)
	}
	p = play(t, p, "e3", "e4")
	sq, _ := v.ParseSquare("e4")
	if p.Board[sq].Type != v.TypeOf('P') {
		t.Error("capture failed")
	}
	p = fen(t, v, "4k3/8/8/8/8/8/4P3/4K3 w -")
	p = play(t, p, "e2", "d3")
	p = play(t, p, "e8", "d8")
	if got := destinations(p, "d3"); !slices.Equal(got, []string{"c4", "e4"}) {
		t.Errorf("moved pawn: %v", got)
	}
}

func TestBerolinaEnPassant(t *testing.T) {
	v := builtin(t, "berolina")
	// e2-g4 crosses f3, directly in front of the black pawn on f4.
	p := fen(t, v, "4k3/8/8/8/5p2/8/4P3/4K3 w -")
	p = play(t, p, "e2", "g4")
	ep := findMove(t, p, "f4", "f3")
	sq, _ := v.ParseSquare("g4")
	if ep.Take != sq {
		t.Fatalf("en passant take = %v", ep.Take)
	}
	after := p.Apply(ep)
	if after.Board[sq].Type != 0 {
		t.Error("captured pawn remains")
	}
	p = play(t, p, "e8", "d8")
	p = play(t, p, "e1", "d1")
	if slices.Contains(destinations(p, "f4"), "f3") {
		t.Error("en passant did not expire")
	}
	// Black's diagonal double step and White's straight en passant.
	p = fen(t, v, "4k3/4p3/8/5P2/8/8/8/4K3 b -")
	p = play(t, p, "e7", "g5")
	ep = findMove(t, p, "f5", "f6")
	sq, _ = v.ParseSquare("g5")
	if ep.Take != sq || p.Apply(ep).Board[sq].Type != 0 {
		t.Error("white en passant failed")
	}
}

func TestBerolinaCastleAndSearch(t *testing.T) {
	v := builtin(t, "berolina")
	p := fen(t, v, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq")
	for _, to := range []string{"c1", "g1"} {
		if !slices.Contains(destinations(p, "e1"), to) {
			t.Errorf("missing castle %s", to)
		}
	}
	p = NewPosition(v)
	r := think(t, p, 2)
	if !slices.Contains(p.Moves(), r.Move) {
		t.Errorf("illegal search move: %+v", r.Move)
	}
}

func TestBerolinaBlackMovesAndPromotion(t *testing.T) {
	v := builtin(t, "berolina")
	p := fen(t, v, "4k3/4p3/8/8/8/8/8/4K3 b -")
	if got := destinations(p, "e7"); !slices.Equal(got, []string{"c5", "d6", "f6", "g5"}) {
		t.Errorf("black pawn: %v", got)
	}
	p = fen(t, v, "k7/8/8/8/4p3/3NRN2/8/K7 b -")
	if got := destinations(p, "e4"); !slices.Equal(got, []string{"e3"}) {
		t.Errorf("black straight capture and diagonal blockers: %v", got)
	}
	p = fen(t, v, "k7/4P3/8/8/8/8/8/4K3 w -")
	count := 0
	for _, m := range p.Moves() {
		if v.SquareName(m.From) == "e7" && m.Promote != 0 {
			count++
		}
	}
	if count != 8 {
		t.Errorf("promotion moves = %d, want 8", count)
	}
}

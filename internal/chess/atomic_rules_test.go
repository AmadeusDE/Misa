package chess

import (
	"slices"
	"testing"
)

func TestAtomicBlastAndEnPassant(t *testing.T) {
	v := builtin(t, "atomic")
	p := fen(t, v, "7k/8/8/2ppr3/2PRB3/8/8/K7 w -")
	before := slices.Clone(p.Board)
	q := p.Apply(findMove(t, p, "d4", "d5"))
	for _, name := range []string{"d4", "d5", "e5", "e4"} {
		sq, _ := v.ParseSquare(name)
		if q.Board[sq].Type != 0 {
			t.Errorf("%s survived", name)
		}
	}
	for _, name := range []string{"c4", "c5"} {
		sq, _ := v.ParseSquare(name)
		if q.Board[sq].Type != v.TypeOf('P') {
			t.Errorf("immune pawn %s removed", name)
		}
	}
	if !slices.Equal(p.Board, before) {
		t.Fatal("Apply mutated parent")
	}
	p = fen(t, v, "7k/8/8/3pP3/2N5/8/8/K7 w -")
	p.EPPawn, _ = v.ParseSquare("d5")
	ep, _ := v.ParseSquare("d6")
	p.EP = []int{ep}
	q = p.Apply(findMove(t, p, "e5", "d6"))
	for _, name := range []string{"e5", "d5", "d6"} {
		sq, _ := v.ParseSquare(name)
		if q.Board[sq].Type != 0 {
			t.Errorf("EP %s survived", name)
		}
	}
	sq, _ := v.ParseSquare("c4")
	if q.Board[sq].Type == 0 {
		t.Error("EP blast centred on captured pawn")
	}
}

func TestAtomicLegality(t *testing.T) {
	v := builtin(t, "atomic")
	t.Run("king cannot capture", func(t *testing.T) {
		p := fen(t, v, "7k/8/8/8/8/8/1r6/K7 w -")
		if slices.Contains(destinations(p, "a1"), "b2") {
			t.Fatal("king captured")
		}
	})
	t.Run("own royal explodes", func(t *testing.T) {
		p := fen(t, v, "7k/8/8/8/8/1r6/KR6/8 w -")
		if slices.Contains(destinations(p, "b2"), "b3") {
			t.Fatal("suicidal capture legal")
		}
	})
	t.Run("extinction before check", func(t *testing.T) {
		p := fen(t, v, "7k/6p1/8/8/8/8/6Q1/K6r w -")
		if !p.InCheck() {
			t.Fatal("expected check")
		}
		q := p.Apply(findMove(t, p, "g2", "g7"))
		if !q.extinct(Black) {
			t.Fatal("black not extinct")
		}
	})
	t.Run("connected royals", func(t *testing.T) {
		p := fen(t, v, "8/8/8/8/8/8/4k3/4K2r w -")
		if p.InCheck() {
			t.Fatal("connected king in check")
		}
		q := fen(t, v, "8/8/8/8/8/4k3/8/4K2r w -")
		if !q.InCheck() {
			t.Fatal("disconnected king not in check")
		}
	})
}

func TestExplosionMultipleRoyalsAndExtinction(t *testing.T) {
	v := builtin(t, "atomic")
	p := fen(t, v, "7k/8/8/8/8/1r6/KR6/7K w -")
	if slices.Contains(destinations(p, "b2"), "b3") {
		t.Fatal("loss of one of two royals was allowed")
	}
	// Extinction is independent of explosions and takes precedence over check.
	v.Explosion = nil
	p = fen(t, v, "7k/6Q1/8/8/8/8/8/K6r w -")
	m := findMove(t, p, "g7", "h8")
	if !p.Apply(m).extinct(Black) {
		t.Fatal("captured royal should be extinct")
	}
}

func TestAtomicCastleConnectedRoyal(t *testing.T) {
	v := builtin(t, "atomic")
	p := fen(t, v, "8/8/8/8/8/8/5k2/4K2R w K")
	moves := p.Moves()
	found := false
	for _, m := range moves {
		if m.Castle >= 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("connected royals should not forbid castling")
	}
}

func TestExplosionBothRoyalsAndCustomRadius(t *testing.T) {
	v := builtin(t, "atomic")
	t.Run("both royals", func(t *testing.T) {
		p := fen(t, v, "8/8/8/8/8/1rk5/KR6/8 w -")
		if slices.Contains(destinations(p, "b2"), "b3") {
			t.Fatal("capture exploding both royals was allowed")
		}
	})
	t.Run("radius and immunity", func(t *testing.T) {
		custom := *v
		custom.Explosion = &Explosion{Radius: 2, Immune: []uint8{v.TypeOf('N')}}
		p := fen(t, &custom, "7k/8/8/3r4/1N1R1P2/8/8/K7 w -")
		q := p.Apply(findMove(t, p, "d4", "d5"))
		knight, _ := v.ParseSquare("b4")
		pawn, _ := v.ParseSquare("f4")
		if q.Board[knight].Type != v.TypeOf('N') {
			t.Error("custom immune knight destroyed")
		}
		if q.Board[pawn].Type != 0 {
			t.Error("pawn at radius two survived")
		}
	})
}

func TestExtinctionAllTypes(t *testing.T) {
	v := *builtin(t, "atomic")
	v.Extinction = nil
	for i := range v.Pieces {
		v.Extinction = append(v.Extinction, uint8(i+1))
	}
	p := fen(t, &v, "7k/8/8/8/8/8/P7/K7 w -")
	if p.extinct(White) || p.extinct(Black) {
		t.Fatal("side with pieces is extinct")
	}
	king, _ := v.ParseSquare("a1")
	p.Board[king] = Piece{}
	if p.extinct(White) {
		t.Fatal("side with pawn is extinct")
	}
	pawn, _ := v.ParseSquare("a2")
	p.Board[pawn] = Piece{}
	if !p.extinct(White) {
		t.Fatal("empty side is not extinct")
	}
}

func TestEnPassantBlastCentredOnDestination(t *testing.T) {
	v := builtin(t, "atomic")
	p := fen(t, v, "7k/8/2N5/3pP3/2B5/8/8/K7 w -")
	p.EPPawn, _ = v.ParseSquare("d5")
	ep, _ := v.ParseSquare("d6")
	p.EP = []int{ep}
	q := p.Apply(findMove(t, p, "e5", "d6"))
	knight, _ := v.ParseSquare("c6")
	bishop, _ := v.ParseSquare("c4")
	if q.Board[knight].Type != 0 {
		t.Error("nonpawn beside destination survived")
	}
	if q.Board[bishop].Type == 0 {
		t.Error("nonpawn beside captured pawn but outside blast was removed")
	}
}

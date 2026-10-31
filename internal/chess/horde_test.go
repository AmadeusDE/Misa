package chess

import (
	"slices"
	"testing"
)

func hordeHasMove(t *testing.T, p *Position, from, to string) bool {
	t.Helper()
	a, err := p.V.ParseSquare(from)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.V.ParseSquare(to)
	if err != nil {
		t.Fatal(err)
	}
	return slices.ContainsFunc(p.Moves(), func(m Move) bool { return m.From == a && m.To == b })
}

func TestHordeSetup(t *testing.T) {
	v := builtin(t, "horde")
	p := NewPosition(v)
	counts := [2]int{}
	for _, pc := range p.Board {
		if pc.Type != 0 {
			counts[pc.Color]++
		}
	}
	if counts != [2]int{36, 16} {
		t.Fatalf("piece counts = %v", counts)
	}
	if len(p.royalSq[White]) != 0 || len(p.royalSq[Black]) != 1 {
		t.Fatalf("royals = %v", p.royalSq)
	}
	if v.Setup != "rnbqkbnr/pppppppp/8/1PP2PP1/PPPPPPPP/PPPPPPPP/PPPPPPPP/PPPPPPPP" {
		t.Errorf("setup = %s", v.Setup)
	}
	for _, sq := range []struct {
		name  string
		color Color
		piece string
	}{
		{"a4", White, "P"}, {"b4", White, "P"}, {"f4", White, "P"}, {"g4", White, "P"},
		{"a3", White, "P"}, {"b3", White, "P"}, {"f3", White, "P"}, {"g3", White, "P"},
		{"a7", Black, "P"}, {"e8", Black, "K"}, {"a8", Black, "R"},
	} {
		i, err := v.ParseSquare(sq.name)
		if err != nil {
			t.Fatal(err)
		}
		pc := p.Board[i]
		if pc.Type == 0 || pc.Color != sq.color || v.Pieces[pc.Type-1].Letter != sq.piece[0] {
			t.Errorf("%s = %+v, want %s", sq.name, pc, sq.piece)
		}
	}
	if len(v.Castling) == 0 {
		t.Fatal("missing castling definitions")
	}
	if st, _ := NewMatch(v).Status(); st != Playing {
		t.Errorf("start status = %v", st)
	}
}

func TestHordePawnDoubleSteps(t *testing.T) {
	v := builtin(t, "horde")
	for _, tt := range []struct {
		name, fen, from, to string
		want                bool
	}{
		{"white first", "7k/8/8/8/8/8/8/P7 w -", "a1", "a3", true},
		{"white second", "7k/8/8/8/8/8/P7/8 w -", "a2", "a4", true},
		{"white third", "7k/8/8/8/8/P7/8/8 w -", "a3", "a5", false},
		{"blocked", "7k/8/8/8/8/P7/8/P7 w -", "a1", "a3", false},
		{"black second", "7k/p7/8/8/8/8/8/P7 b -", "a7", "a5", true},
		{"black third", "7k/8/p7/8/8/8/8/P7 b -", "a6", "a4", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := fen(t, v, tt.fen)
			if got := hordeHasMove(t, p, tt.from, tt.to); got != tt.want {
				t.Errorf("%s-%s legal = %v", tt.from, tt.to, got)
			}
		})
	}
}

func TestHordeEnPassant(t *testing.T) {
	v := builtin(t, "horde")
	for _, tt := range []struct {
		name, fen, from, to, captureFrom, captureTo string
		want                                        bool
	}{
		{"first rank exempt", "7k/8/8/8/8/1p6/8/P7 w -", "a1", "a3", "b3", "a2", false},
		{"second rank", "7k/8/8/8/1p6/8/P7/8 w -", "a2", "a4", "b4", "a3", true},
		{"black second rank", "7k/p7/8/1P6/8/8/8/8 b -", "a7", "a5", "b5", "a6", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := fen(t, v, tt.fen)
			p = p.Apply(findMove(t, p, tt.from, tt.to))
			if got := hordeHasMove(t, p, tt.captureFrom, tt.captureTo); got != tt.want {
				t.Errorf("en passant legal = %v", got)
			}
		})
	}
}

func TestHordeEndings(t *testing.T) {
	v := builtin(t, "horde")
	t.Run("last horde piece captured", func(t *testing.T) {
		m := NewMatch(v)
		m.Load(fen(t, v, "7k/8/8/8/8/8/r7/P7 b -"))
		m.Play(findMove(t, m.Position(), "a2", "a1"))
		if st, w := m.Status(); st != ExtinctionWin || w != Black {
			t.Errorf("status %v winner %v", st, w)
		}
		if !m.Undo() || m.Over() {
			t.Error("undo did not restore game")
		}
	})
	t.Run("promoted horde piece survives", func(t *testing.T) {
		m := NewMatch(v)
		m.Load(fen(t, v, "7k/8/8/8/8/8/8/Q7 w -"))
		if st, _ := m.Status(); st != Playing {
			t.Errorf("status %v", st)
		}
	})
	t.Run("checkmate", func(t *testing.T) {
		m := NewMatch(v)
		m.Load(fen(t, v, "7k/6Q1/5Q2/8/8/8/8/8 b -"))
		if st, w := m.Status(); st != Checkmate || w != White {
			t.Errorf("status %v winner %v", st, w)
		}
	})
	t.Run("stalemate", func(t *testing.T) {
		m := NewMatch(v)
		m.Load(fen(t, v, "7k/5Q2/6Q1/8/8/8/8/8 b -"))
		if st, _ := m.Status(); st != Stalemate {
			t.Errorf("status %v", st)
		}
	})
}

func TestHordePromotionAndCastling(t *testing.T) {
	v := builtin(t, "horde")
	p := fen(t, v, "7k/P7/8/8/8/8/8/8 w -")
	count := 0
	for _, m := range p.Moves() {
		if v.SquareName(m.From) == "a7" && v.SquareName(m.To) == "a8" && m.Promote != 0 {
			count++
		}
	}
	if count != 4 {
		t.Errorf("promotion choices = %d", count)
	}
	p = fen(t, v, "r3k2r/8/8/8/8/8/8/P7 b KQkq")
	if !hordeHasMove(t, p, "e8", "g8") || !hordeHasMove(t, p, "e8", "c8") {
		t.Error("Black cannot castle")
	}
}

func TestHordeEngineFindsMate(t *testing.T) {
	v := builtin(t, "horde")
	p := fen(t, v, "7k/8/4Q3/8/8/8/8/6R1 w -")
	r := think(t, p, 2)
	if !r.Found || !IsMateScore(r.Score) {
		t.Fatalf("search = %+v, want mate", r)
	}
	m := NewMatch(v)
	m.Load(p)
	m.Play(r.Move)
	if st, w := m.Status(); st != Checkmate || w != White {
		t.Errorf("search played %s: status %v winner %v", moveName(p, r.Move), st, w)
	}
}

func TestHordeRoyalLessNotInCheckUnderAttack(t *testing.T) {
	v := builtin(t, "horde")
	p := fen(t, v, "7k/8/8/8/8/8/r7/P7 w -")
	if p.InCheck() {
		t.Fatal("royal-less White reported in check under attack")
	}
}

func TestHordeStartingPositionSearchBothSides(t *testing.T) {
	v := builtin(t, "horde")
	for _, side := range []Color{White, Black} {
		p := NewPosition(v)
		p.Turn = side
		r := think(t, p, 1)
		if !r.Found {
			t.Errorf("search for %s found no legal move", side)
		}
	}
}

func TestHordeKinglessStalemate(t *testing.T) {
	v := builtin(t, "horde")
	m := NewMatch(v)
	m.Load(fen(t, v, "P6k/8/8/8/8/8/8/8 w -"))
	if st, _ := m.Status(); st != Stalemate {
		t.Errorf("status = %v", st)
	}
}

package chess

import "testing"

func TestFourSetupAndTurns(t *testing.T) {
	m := NewFourMatch()
	for _, s := range []FourSide{FourRed, FourBlue, FourYellow, FourGreen} {
		if m.Turn != s {
			t.Fatalf("turn = %v", m.Turn)
		}
		if m.InCheck(s) {
			t.Fatalf("%v initially checked", s)
		}
		moves := m.LegalMoves()
		if len(moves) != 20 {
			t.Fatalf("%v: got %d initial moves", s, len(moves))
		}
		if err := m.Apply(moves[0]); err != nil {
			t.Fatal(err)
		}
	}
	if m.Turn != FourRed {
		t.Fatal(m.Turn)
	}
	if m.Board[0][0].Type != FourHole {
		t.Fatal("missing hole")
	}
	for i := 0; i < 4; i++ {
		if !m.Undo() {
			t.Fatal("undo")
		}
	}
	if m.Board != NewFourMatch().Board || m.Turn != FourRed {
		t.Fatal("undo did not restore initial state")
	}
}
func fourEmptyMatch() *FourMatch {
	m := NewFourMatch()
	for r := range m.Board {
		for c := range m.Board[r] {
			if m.Board[r][c].Type != FourHole {
				m.Board[r][c] = FourPiece{}
			}
		}
	}
	return m
}
func fourContains(ms []FourMove, a, b FourSquare) bool {
	for _, m := range ms {
		if m == (FourMove{a, b}) {
			return true
		}
	}
	return false
}
func TestFourPawnAndAllies(t *testing.T) {
	for _, tc := range []struct {
		s        FourSide
		from, to FourSquare
	}{{FourRed, FourSquare{7, 6}, FourSquare{6, 6}}, {FourBlue, FourSquare{6, 6}, FourSquare{6, 7}}, {FourYellow, FourSquare{6, 6}, FourSquare{7, 6}}, {FourGreen, FourSquare{6, 7}, FourSquare{6, 6}}} {
		m := fourEmptyMatch()
		m.Turn = tc.s
		m.Board[tc.from.Row][tc.from.Col] = FourPiece{Type: FourPawn, Side: tc.s, Moved: true}
		if err := m.Apply(FourMove{tc.from, tc.to}); err != nil {
			t.Fatal(err)
		}
		if m.Board[tc.to.Row][tc.to.Col].Type != FourQueen {
			t.Fatalf("%v did not promote", tc.s)
		}
	}
	m := fourEmptyMatch()
	m.Board[8][7] = FourPiece{Type: FourRook, Side: FourRed}
	m.Board[8][8] = FourPiece{Type: FourPawn, Side: FourYellow}
	if fourContains(m.LegalMoves(), FourSquare{8, 7}, FourSquare{8, 8}) {
		t.Fatal("captured partner")
	}
}
func TestFourCastleAndCheck(t *testing.T) {
	m := fourEmptyMatch()
	m.Board[13][7] = FourPiece{Type: FourKing, Side: FourRed}
	m.Board[13][10] = FourPiece{Type: FourRook, Side: FourRed}
	m.Board[0][7] = FourPiece{Type: FourKing, Side: FourYellow}
	m.Board[3][0] = FourPiece{Type: FourKing, Side: FourBlue}
	m.Board[3][13] = FourPiece{Type: FourKing, Side: FourGreen}
	mv := FourMove{FourSquare{13, 7}, FourSquare{13, 9}}
	if !fourContains(m.LegalMoves(), mv.From, mv.To) {
		t.Fatal("castle absent")
	}
	clone := m.Clone()
	if err := clone.Apply(mv); err != nil {
		t.Fatal(err)
	}
	if clone.Board[13][8].Type != FourRook || clone.Board[13][9].Type != FourKing {
		t.Fatal("castle placement")
	}
	if m.Board[13][7].Type != FourKing {
		t.Fatal("clone mutated original")
	}
	m.Board[10][8] = FourPiece{Type: FourRook, Side: FourBlue}
	if fourContains(m.LegalMoves(), mv.From, mv.To) {
		t.Fatal("castled through check")
	}
}
func TestFourMateAndStalemate(t *testing.T) {
	// Red moves a rook to mate Blue's king in the top left arm.
	m := fourEmptyMatch()
	m.Board[3][0] = FourPiece{Type: FourKing, Side: FourBlue}
	m.Board[5][0] = FourPiece{Type: FourKing, Side: FourRed}
	m.Board[4][2] = FourPiece{Type: FourRook, Side: FourRed}
	m.Board[10][10] = FourPiece{Type: FourKing, Side: FourYellow}
	m.Board[10][13] = FourPiece{Type: FourKing, Side: FourGreen}
	if err := m.Apply(FourMove{FourSquare{4, 2}, FourSquare{3, 2}}); err != nil {
		t.Fatal(err)
	}
	if !m.Finished || m.Winner != FourRedYellow {
		t.Fatalf("mate: finished=%v winner=%v check=%v moves=%v", m.Finished, m.Winner, m.InCheck(FourBlue), m.LegalMoves())
	}
	if !m.Undo() || m.Finished {
		t.Fatal("undo mate")
	}
}

func TestFourStalemateDraw(t *testing.T) {
	m := fourEmptyMatch()
	m.Board[3][0] = FourPiece{Type: FourKing, Side: FourBlue}
	m.Board[5][0] = FourPiece{Type: FourKing, Side: FourRed}
	m.Board[5][3] = FourPiece{Type: FourBishop, Side: FourRed}
	m.Board[10][10] = FourPiece{Type: FourKing, Side: FourYellow}
	m.Board[10][13] = FourPiece{Type: FourKing, Side: FourGreen}
	if err := m.Apply(FourMove{FourSquare{5, 3}, FourSquare{4, 2}}); err != nil {
		t.Fatal(err)
	}
	if !m.Finished || m.Winner != FourNoTeam || m.InCheck(FourBlue) {
		t.Fatalf("expected stalemate draw: %+v", m)
	}
}

func TestFourComputerAvoidsHangingQueen(t *testing.T) {
	m := fourEmptyMatch()
	m.Board[13][7] = FourPiece{Type: FourKing, Side: FourRed}
	m.Board[3][0] = FourPiece{Type: FourKing, Side: FourBlue}
	m.Board[0][7] = FourPiece{Type: FourKing, Side: FourYellow}
	m.Board[3][13] = FourPiece{Type: FourKing, Side: FourGreen}
	m.Board[8][7] = FourPiece{Type: FourQueen, Side: FourRed}
	m.Board[8][3] = FourPiece{Type: FourRook, Side: FourBlue}
	before := *m
	mv, ok := m.ComputerMove()
	if !ok {
		t.Fatal("no move")
	}
	if m.Board != before.Board || m.Turn != before.Turn || m.Finished != before.Finished || m.Winner != before.Winner {
		t.Fatal("computer search mutated match")
	}
	if err := m.Apply(mv); err != nil {
		t.Fatal(err)
	}
	for _, reply := range m.LegalMoves() {
		if m.Board[reply.To.Row][reply.To.Col].Type == FourQueen && m.Board[reply.To.Row][reply.To.Col].Side == FourRed {
			t.Fatalf("move %v leaves queen capturable by %v", mv, reply)
		}
	}
}

func TestFourComputerTakesFreeCapture(t *testing.T) {
	m := fourEmptyMatch()
	m.Board[13][7] = FourPiece{Type: FourKing, Side: FourRed}
	m.Board[3][0] = FourPiece{Type: FourKing, Side: FourBlue}
	m.Board[0][7] = FourPiece{Type: FourKing, Side: FourYellow}
	m.Board[3][13] = FourPiece{Type: FourKing, Side: FourGreen}
	m.Board[8][7] = FourPiece{Type: FourRook, Side: FourRed}
	m.Board[8][3] = FourPiece{Type: FourQueen, Side: FourBlue}
	mv, ok := m.ComputerMove()
	if !ok || mv != (FourMove{FourSquare{8, 7}, FourSquare{8, 3}}) {
		t.Fatalf("expected free queen capture, got %v, %v", mv, ok)
	}
}

func TestFourComputerInitialPosition(t *testing.T) {
	m := NewFourMatch()
	mv, ok := m.ComputerMove()
	if !ok || !fourContains(m.LegalMoves(), mv.From, mv.To) {
		t.Fatalf("invalid computer move: %v, %v", mv, ok)
	}
}

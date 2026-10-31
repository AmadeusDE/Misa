package chess

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// xqPosition builds a sparse FEN, so the parser also initializes royal squares.
func xqPosition(t *testing.T, v *Variant, turn string, pieces map[string]byte) *Position {
	t.Helper()
	var ranks []string
	for r := 10; r >= 1; r-- {
		var row strings.Builder
		empty := 0
		for _, f := range "abcdefghi" {
			pc := pieces[fmt.Sprintf("%c%d", f, r)]
			if pc == 0 {
				empty++
				continue
			}
			if empty > 0 {
				fmt.Fprint(&row, empty)
				empty = 0
			}
			row.WriteByte(pc)
		}
		if empty > 0 {
			fmt.Fprint(&row, empty)
		}
		ranks = append(ranks, row.String())
	}
	return fen(t, v, strings.Join(ranks, "/")+" "+turn+" -")
}

func TestXiangqiSetup(t *testing.T) {
	v := builtin(t, "xiangqi")
	if v.Files != 9 || v.Ranks != 10 || len(v.Pieces) != 7 || len(v.Castling) != 0 || len(v.Promotion) != 0 || v.Stalemate != StalemateWin {
		t.Fatalf("unexpected variant: %+v", v)
	}
	if v.Setup != "rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR" {
		t.Errorf("setup: %s", v.Setup)
	}
	for _, x := range []struct {
		letter      byte
		betza       string
		value       int
		royal, pawn bool
	}{{'K', "W", 0, true, false}, {'A', "F", 200, false, false}, {'B', "nA", 200, false, false}, {'N', "nN", 400, false, false}, {'R', "R", 900, false, false}, {'C', "mRcpR", 450, false, false}, {'P', "fW", 100, false, true}} {
		id := v.TypeOf(x.letter)
		if id == 0 {
			t.Errorf("missing %c", x.letter)
			continue
		}
		p := v.Pieces[id-1]
		if p.Betza != x.betza || (x.value != 0 && p.Value != x.value) || p.Royal != x.royal || p.Pawn != x.pawn || p.Glyph != "" {
			t.Errorf("%c: %+v", x.letter, p)
		}
	}
	p := NewPosition(v)
	counts := [2]int{}
	for _, pc := range p.Board {
		if pc.Type != 0 {
			counts[pc.Color]++
		}
	}
	if counts != [2]int{16, 16} {
		t.Errorf("piece counts: %v", counts)
	}
}

func TestXiangqiMovement(t *testing.T) {
	v := builtin(t, "xiangqi")
	tests := []struct {
		name, turn, from string
		pieces           map[string]byte
		want             []string
	}{
		{"general white palace", "w", "d2", map[string]byte{"d2": 'K', "f10": 'k'}, []string{"d1", "d3", "e2"}},
		{"general black palace", "b", "f9", map[string]byte{"d1": 'K', "f9": 'k'}, []string{"e9", "f10", "f8"}},
		{"advisor white palace", "w", "d2", map[string]byte{"d1": 'K', "f10": 'k', "d2": 'A'}, []string{"e1", "e3"}},
		{"advisor black palace", "b", "f9", map[string]byte{"d1": 'K', "f10": 'k', "f9": 'a'}, []string{"e10", "e8"}},
		{"elephant river", "w", "c5", map[string]byte{"d1": 'K', "f10": 'k', "c5": 'B'}, []string{"a3", "e3"}},
		{"elephant eye", "w", "c3", map[string]byte{"d1": 'K', "f10": 'k', "c3": 'B', "b4": 'P'}, []string{"a1", "e1", "e5"}},
		{"black elephant river", "b", "c6", map[string]byte{"d1": 'K', "f10": 'k', "c6": 'b'}, []string{"a8", "e8"}},
		{"horse leg", "w", "e5", map[string]byte{"d1": 'K', "f10": 'k', "e5": 'N', "e6": 'P'}, []string{"c4", "c6", "d3", "f3", "g4", "g6"}},
		{"cannon screen", "w", "a3", map[string]byte{"d1": 'K', "f10": 'k', "a3": 'C', "a5": 'P', "a7": 'p', "a9": 'r'}, []string{"a1", "a2", "a4", "a7", "b3", "c3", "d3", "e3", "f3", "g3", "h3", "i3"}},
		{"cannon cannot capture without screen", "w", "a3", map[string]byte{"d1": 'K', "f10": 'k', "a3": 'C', "a5": 'p'}, []string{"a1", "a2", "a4", "b3", "c3", "d3", "e3", "f3", "g3", "h3", "i3"}},
		{"cannon cannot capture past two screens", "w", "a3", map[string]byte{"d1": 'K', "f10": 'k', "a3": 'C', "a4": 'P', "a5": 'P', "a6": 'p'}, []string{"a1", "a2", "b3", "c3", "d3", "e3", "f3", "g3", "h3", "i3"}},
		{"soldier no initial double", "w", "a4", map[string]byte{"d1": 'K', "f10": 'k', "a4": 'P'}, []string{"a5"}},
		{"white soldier before river", "w", "a5", map[string]byte{"d1": 'K', "f10": 'k', "a5": 'P'}, []string{"a6"}},
		{"white soldier after river", "w", "b6", map[string]byte{"d1": 'K', "f10": 'k', "b6": 'P'}, []string{"a6", "b7", "c6"}},
		{"black soldier before river", "b", "a6", map[string]byte{"d1": 'K', "f10": 'k', "a6": 'p'}, []string{"a5"}},
		{"black soldier after river", "b", "b5", map[string]byte{"d1": 'K', "f10": 'k', "b5": 'p'}, []string{"a5", "b4", "c5"}},
		{"white soldier on last rank", "w", "a10", map[string]byte{"d1": 'K', "f10": 'k', "a10": 'P'}, []string{"b10"}},
		{"black soldier on last rank", "b", "a1", map[string]byte{"d1": 'K', "f10": 'k', "a1": 'p'}, []string{"b1"}},
		{"soldier captures forward and sideways", "w", "b6", map[string]byte{"d1": 'K', "f10": 'k', "b6": 'P', "a6": 'p', "c6": 'p', "b7": 'p'}, []string{"a6", "b7", "c6"}},
		{"black elephant eye", "b", "c8", map[string]byte{"d1": 'K', "f10": 'k', "c8": 'b', "b7": 'p'}, []string{"a10", "e10", "e6"}},
		{"black horse leg", "b", "e6", map[string]byte{"d1": 'K', "f10": 'k', "e6": 'n', "e5": 'p'}, []string{"c5", "c7", "d8", "f8", "g5", "g7"}},
		{"soldier last rank", "w", "a9", map[string]byte{"d1": 'K', "f10": 'k', "a9": 'P'}, []string{"a10", "b9"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := xqPosition(t, v, tt.turn, tt.pieces)
			slices.Sort(tt.want)
			if got := destinations(p, tt.from); !slices.Equal(got, tt.want) {
				t.Errorf("moves = %v, want %v", got, tt.want)
			}
			for _, m := range p.Moves() {
				sq, _ := v.ParseSquare(tt.from)
				if m.From == sq && m.Promote != 0 {
					t.Errorf("unexpected promotion: %+v", m)
				}
			}
		})
	}
}

func TestXiangqiFlyingGenerals(t *testing.T) {
	v := builtin(t, "xiangqi")
	open := xqPosition(t, v, "w", map[string]byte{"e1": 'K', "e10": 'k'})
	if !open.InCheck() {
		t.Error("facing generals must check")
	}
	black := xqPosition(t, v, "b", map[string]byte{"e1": 'K', "e10": 'k'})
	if !black.InCheck() {
		t.Error("facing generals must check black")
	}
	blocked := xqPosition(t, v, "w", map[string]byte{"e1": 'K', "e10": 'k', "e5": 'R'})
	if blocked.InCheck() {
		t.Error("blocked generals are not in check")
	}
	moves := destinations(blocked, "e5")
	if slices.Contains(moves, "d5") || !slices.Contains(moves, "e6") {
		t.Errorf("blocking rook moves: %v", moves)
	}
}

func TestXiangqiEndings(t *testing.T) {
	v := builtin(t, "xiangqi")
	for _, tt := range []struct {
		name   string
		pieces map[string]byte
		status Status
	}{
		{"stalemate", map[string]byte{"d1": 'K', "e10": 'k', "d8": 'R', "f8": 'R', "a9": 'R'}, Stalemate},
		{"checkmate", map[string]byte{"d1": 'K', "e10": 'k', "d8": 'R', "f8": 'R', "a9": 'R', "a10": 'R'}, Checkmate},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMatch(v)
			m.Load(xqPosition(t, v, "b", tt.pieces))
			status, winner := m.Status()
			if status != tt.status || !m.Decisive() || winner != White {
				t.Errorf("status=%v decisive=%v winner=%v", status, m.Decisive(), winner)
			}
		})
	}
}

func TestXiangqiSearchStart(t *testing.T) {
	v := builtin(t, "xiangqi")
	for _, turn := range []string{"w", "b"} {
		t.Run(turn, func(t *testing.T) {
			p := fen(t, v, v.Setup+" "+turn+" -")
			r := NewEngine(v).Think(p, nil, Limits{Depth: 2, Nodes: 2000})
			if !r.Found || !slices.Contains(p.Moves(), r.Move) {
				t.Errorf("search returned %+v", r)
			}
		})
	}
}

func TestXiangqiOpeningMoveCount(t *testing.T) {
	v := builtin(t, "xiangqi")
	for _, turn := range []string{"w", "b"} {
		p := fen(t, v, v.Setup+" "+turn+" -")
		if got := len(p.Moves()); got != 44 {
			t.Errorf("%s: %d opening moves, want 44", turn, got)
		}
	}
}

func TestXiangqiSoldierRemainsSoldier(t *testing.T) {
	v := builtin(t, "xiangqi")
	for _, tt := range []struct {
		name, turn, from, to, notation string
		soldier                        byte
	}{
		{"white crosses", "w", "a5", "a6", "a5-a6", 'P'},
		{"black crosses", "b", "a6", "a5", "a6-a5", 'p'},
		{"white last rank", "w", "a9", "a10", "a9-a10", 'P'},
		{"black last rank", "b", "a2", "a1", "a2-a1", 'p'},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := xqPosition(t, v, tt.turn, map[string]byte{"d1": 'K', "f10": 'k', tt.from: tt.soldier})
			p.Clock = 7
			from, _ := v.ParseSquare(tt.from)
			to, _ := v.ParseSquare(tt.to)
			var found []Move
			for _, move := range p.Moves() {
				if move.From == from && move.To == to {
					found = append(found, move)
				}
			}
			if len(found) != 1 || found[0].Promote != 0 {
				t.Fatalf("moves to %s = %+v, want one non-promoting move", tt.to, found)
			}
			m := NewMatch(v)
			m.Load(p)
			m.Play(found[0])
			after := m.Position()
			if after.Board[to].Type != v.TypeOf('P') || after.Board[to].Color != p.Turn {
				t.Errorf("soldier after move = %+v", after.Board[to])
			}
			if len(after.EP) != 0 || after.Clock != 0 {
				t.Errorf("EP = %v, clock = %d", after.EP, after.Clock)
			}
			if len(after.Moves()) == 0 {
				t.Error("next side has no legal moves")
			}
			if got := m.Notation()[0]; got != tt.notation {
				t.Errorf("notation = %q, want %q", got, tt.notation)
			}
		})
	}
}

// This behaviour is pinned on purpose: under AXF/WXF rules the side that gives
// perpetual check (or a perpetual chase) loses, but the engine adjudicates such
// repetition as an ordinary draw by repetition. If adjudication is ever
// implemented, this test must be changed deliberately rather than by accident.
func TestXiangqiPerpetualCheckIsDrawForNow(t *testing.T) {
	v := builtin(t, "xiangqi")
	// White's chariot shuttles between d1 and e1, checking the black general on
	// whichever file it stands on; the general steps between e10 and d10 to
	// escape. Both files are open, no other piece interferes, and the generals
	// never face each other, so the position repeats forever.
	m := NewMatch(v)
	m.Load(xqPosition(t, v, "w", map[string]byte{"d1": 'R', "f1": 'K', "e10": 'k'}))

	// play makes the one legal move between two squares.
	play := func(from, to string) {
		t.Helper()
		f, _ := v.ParseSquare(from)
		d, _ := v.ParseSquare(to)
		var found []Move
		for _, mv := range m.Legal() {
			if mv.From == f && mv.To == d {
				found = append(found, mv)
			}
		}
		if len(found) != 1 {
			t.Fatalf("%s-%s: %d matching legal moves in %v", from, to, len(found), m.Notation())
		}
		m.Play(found[0])
	}

	// step plays a move unless the match is already over; with check true the
	// move must leave the black general in check.
	checks := 0
	step := func(from, to string, check bool) bool {
		if m.Over() {
			return false
		}
		play(from, to)
		if check {
			checks++
			if !m.Position().InCheck() {
				t.Fatalf("%s-%s does not give check", from, to)
			}
		}
		return true
	}

	for cycle := 0; cycle < 5; cycle++ {
		if !step("d1", "e1", true) || !step("e10", "d10", false) ||
			!step("e1", "d1", true) || !step("d10", "e10", false) {
			break
		}
	}

	status, _ := m.Status()
	if status != DrawRepetition {
		t.Fatalf("status = %v, want %v: perpetual check is only a draw for now", status, DrawRepetition)
	}
	if m.Decisive() {
		t.Error("a repetition draw must not be decisive")
	}
	if checks < 4 {
		t.Errorf("played %d checking moves, want at least 4", checks)
	}
}

package chess

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestShogiSetupAndOrientation(t *testing.T) {
	v := builtin(t, "shogi")
	if v.Files != 9 || v.Ranks != 9 || !v.Drops {
		t.Fatal("wrong shogi board or drops")
	}
	p := NewPosition(v)
	count := [2]int{}
	for _, piece := range p.Board {
		if piece.Type != 0 {
			count[piece.Color]++
		}
	}
	if count != [2]int{20, 20} {
		t.Fatalf("armies: %v", count)
	}
	for _, tc := range []struct{ from, to string }{{"a3", "a4"}, {"h2", "g2"}, {"g1", "g2"}} {
		if !slices.Contains(destinations(p, tc.from), tc.to) {
			t.Errorf("missing %s-%s", tc.from, tc.to)
		}
	}
	p.Turn = Black
	for _, tc := range []struct{ from, to string }{{"a7", "a6"}, {"b8", "c8"}, {"g9", "g8"}} {
		if !slices.Contains(destinations(p, tc.from), tc.to) {
			t.Errorf("missing %s-%s", tc.from, tc.to)
		}
	}
}

func TestShogiFENAndSearch(t *testing.T) {
	v := builtin(t, "shogi")
	p := fen(t, v, "4k4/9/9/9/4D~4/9/9/9/4K4[Ps] w - - 0 1")
	q, err := ParseFEN(v, p.FEN())
	if err != nil || q.Hash() != p.Hash() {
		t.Fatalf("FEN roundtrip %s: %v", p.FEN(), err)
	}
	for _, side := range []Color{White, Black} {
		pos := NewPosition(v)
		pos.Turn = side
		result := NewEngine(v).Think(pos, nil, Limits{Nodes: 500, Depth: 2})
		if !result.Found || !slices.Contains(pos.Moves(), result.Move) {
			t.Errorf("%v search: %+v", side, result)
		}
	}
}

func TestShogiHandRestrictions(t *testing.T) {
	v := builtin(t, "shogi")
	p := fen(t, v, "4k4/9/9/9/9/9/P8/9/4K4[PNL] w - - 0 1")
	for _, m := range p.Moves() {
		if m.Drop == 0 {
			continue
		}
		sq := v.SquareName(m.To)
		if m.Drop == v.TypeOf('P') && (sq[0] == 'a' || sq[1] == '9') {
			t.Errorf("illegal pawn drop %s", sq)
		}
		if m.Drop == v.TypeOf('L') && sq[1] == '9' {
			t.Errorf("illegal lance drop %s", sq)
		}
		if m.Drop == v.TypeOf('N') && (sq[1] == '8' || sq[1] == '9') {
			t.Errorf("illegal knight drop %s", sq)
		}
	}
}

func TestShogiPromotionAndCapture(t *testing.T) {
	v := builtin(t, "shogi")
	for _, tc := range []struct {
		fen, from, to string
		promote       byte
		optional      bool
	}{
		{"4k4/9/9/P8/9/9/9/9/4K4[] w - - 0 1", "a6", "a7", 'T', true},
		{"4k4/P8/9/9/9/9/9/9/4K4[] w - - 0 1", "a8", "a9", 'T', false},
		{"4k4/9/9/9/9/9/9/p8/4K4[] b - - 0 1", "a2", "a1", 'T', false},
		{"4k4/9/9/9/9/9/9/N8/4K4[] w - - 0 1", "a2", "b4", 0, false},
	} {
		p := fen(t, v, tc.fen)
		from, _ := v.ParseSquare(tc.from)
		to, _ := v.ParseSquare(tc.to)
		plain, promoted := false, false
		for _, m := range p.Moves() {
			if m.From == from && m.To == to {
				if m.Promote == 0 {
					plain = true
				}
				if tc.promote != 0 && m.Promote == v.TypeOf(tc.promote) {
					promoted = true
				}
			}
		}
		if tc.promote != 0 && (!promoted || plain != tc.optional) {
			t.Errorf("%s-%s: plain %v promoted %v", tc.from, tc.to, plain, promoted)
		}
		if tc.promote == 0 && !plain {
			t.Errorf("missing %s-%s", tc.from, tc.to)
		}
	}
	p := fen(t, v, "4k4/9/9/3d~5/4B4/9/9/9/4K4[] w - - 0 1")
	q := p.Apply(findMove(t, p, "e5", "d6"))
	if q.Hands[White][v.TypeOf('R')] != 1 {
		t.Fatalf("captured dragon did not demote: %v", q.Hands[White])
	}
}

func TestShogiInteraction(t *testing.T) {
	g := newGame(t, "shogi", 100, 30)
	clickSquare(t, g, "a3", tea.MouseLeft)
	clickSquare(t, g, "a4", tea.MouseLeft)
	if len(g.match.Notation()) != 1 {
		t.Fatal("pawn move not played")
	}
	keys(g, "u")
	if len(g.match.Notation()) != 0 {
		t.Fatal("undo did not restore the position")
	}
}

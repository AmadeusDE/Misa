package chess

import (
	"slices"
	"testing"
)

func TestBughouseTransferAndUndo(t *testing.T) {
	v := builtin(t, "bughouse")
	for _, tc := range []struct {
		fen  string
		want byte
	}{
		{"4k3/8/8/3q4/4B3/8/8/4K3[] w - - 0 1", 'Q'},
		{"4k3/8/8/3q~4/4B3/8/8/4K3[] w - - 0 1", 'P'},
		{"4k3/8/8/3pP3/8/8/8/4K3[] w - d6 0 1", 'P'},
	} {
		b := NewBugMatch(v)
		a := fen(t, v, tc.fen)
		other := NewPosition(v)
		b.Load(a, other)
		var capture Move
		found := false
		for _, m := range a.Moves() {
			if m.Take >= 0 {
				capture = m
				found = true
				break
			}
		}
		if !found || !b.Play(0, capture) {
			t.Fatal("missing capture", tc.fen)
		}
		if b.Board(0).Position().Hands[White][v.TypeOf(tc.want)] != 0 || b.Board(1).Position().Hands[Black][v.TypeOf(tc.want)] != 1 {
			t.Fatal("wrong transfer", tc.fen)
		}
		if other.Hands[Black][v.TypeOf(tc.want)] != 0 {
			t.Fatal("mutated original")
		}
		if !b.Undo() || b.Board(0).Position() != a || b.Board(1).Position() != other || b.Undo() {
			t.Fatal("undo did not restore boards")
		}
	}
}

func TestBughouseMateAndSearch(t *testing.T) {
	v := builtin(t, "bughouse")
	b := NewBugMatch(v)
	a := fen(t, v, "7k/5Q2/6K1/8/8/8/8/8[] w - - 0 1")
	b.Load(a, NewPosition(v))
	result := NewEngine(v).Think(a, nil, Limits{Depth: 2})
	if !result.Found || !slices.Contains(b.Board(0).Legal(), result.Move) {
		t.Fatal("search returned illegal move", result)
	}
	var mate Move
	found := false
	for _, mv := range b.Board(0).Legal() {
		p := a.Apply(mv)
		if p.InCheck() && len(p.Moves()) == 0 {
			mate = mv
			found = true
			break
		}
	}
	if !found || !b.Play(0, mate) {
		t.Fatal("no mate")
	}
	status, board, winner := b.Status()
	if status != Checkmate || board != 0 || winner != White || b.WinnerTeam() != 1 || !b.Over() {
		t.Fatal(status, board, winner, b.WinnerTeam())
	}
	if b.Play(1, b.Board(1).Legal()[0]) {
		t.Fatal("played after mate")
	}
	if !b.Undo() || b.Over() {
		t.Fatal("undo mate")
	}
}

func TestBughouseYAMLInheritance(t *testing.T) {
	vs, err := parseVariants(map[string][]byte{"child.yaml": []byte("id: child\nextends: bughouse\n"), "off.yaml": []byte("id: off\nextends: bughouse\nboards: 1\npartner_hands: false\ndrops: false\n")})
	if err != nil {
		t.Fatal(err)
	}
	child, _ := Find(vs, "child")
	off, _ := Find(vs, "off")
	if !child.PartnerHands || !child.Drops || child.Boards != 2 || off.PartnerHands || off.Drops || off.Boards != 1 {
		t.Fatal("inheritance")
	}
	_, err = parseVariants(map[string][]byte{"bad.yaml": []byte("id: bad\nextends: chess\npartner_hands: true\n")})
	if err == nil {
		t.Fatal("partner hands requires two boards and drops")
	}
	for _, settings := range []string{"boards: 2\n", "boards: 3\n", "boards: 0\n", "boards: 2\npartner_hands: true\n"} {
		_, err = parseVariants(map[string][]byte{"bad.yaml": []byte("id: bad\nextends: chess\n" + settings)})
		if err == nil {
			t.Fatalf("accepted invalid settings %q", settings)
		}
	}
}

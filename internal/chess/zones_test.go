package chess

import (
	"fmt"
	"strings"
	"testing"
)

func zoneVariant(t *testing.T, pieces string) *Variant {
	t.Helper()
	data := fmt.Sprintf("id: zonetest\nfiles: 5\nranks: 5\nsetup: 2k2/5/5/5/2K2\npromotion: []\npieces:\n%s\n", pieces)
	vs, err := parseVariants(map[string][]byte{"zones.yaml": []byte(data)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "zonetest")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestZoneMechanisms(t *testing.T) {
	v := zoneVariant(t, "  - letter: K\n    betza: W\n    royal: true\n    zone: [c1, c2]\n    royal_attack: vR\n  - letter: P\n    betza: fW\n    pawn: true\n    move_zones:\n      - zone: [b3]\n        betza: fWsW\n")
	p, err := ParseFEN(v, "2k2/5/1P3/5/2K2 w")
	if err != nil {
		t.Fatal(err)
	}
	if !p.InCheck() {
		t.Fatal("flying royal attack missed")
	}
	a3, _ := v.ParseSquare("a3")
	if !p.anyAttacked([]int{a3}, White) {
		t.Fatal("alternative reach not detected")
	}
	// A nonroyal blocker stops the flying attack and cannot itself be attacked by it.
	p, _ = ParseFEN(v, "2k2/5/2P2/5/2K2 w")
	if p.InCheck() {
		t.Fatal("blocked flying attack")
	}
	c3, _ := v.ParseSquare("c3")
	if p.anyAttacked([]int{c3}, Black) {
		t.Fatal("royal attack hit nonroyal")
	}
	// Normal attacks obey destination confinement even when the target is occupied.
	p, _ = ParseFEN(v, "2k2/5/5/3p1/2K2 w")
	d2, _ := v.ParseSquare("d2")
	if p.anyAttacked([]int{d2}, White) {
		t.Fatal("attack escaped destination zone")
	}
}

func TestZoneValidation(t *testing.T) {
	for _, bad := range []string{"zone: []", "zone: [z9]", "zone: [a1, a1]", "move_zones: [{zone: [], betza: W}]", "move_zones: [{zone: [a1], betza: bad}]", "move_zones: [{zone: [a1], betza: W}, {zone: [a1], betza: F}]", "royal_attack: bad", "royal_attack: ''"} {
		data := fmt.Sprintf("id: invalidzone\nfiles: 2\nranks: 2\nsetup: 2/2\npieces:\n  - letter: K\n    betza: W\n    %s\n", bad)
		_, err := parseVariants(map[string][]byte{"invalid.yaml": []byte(data)})
		if err == nil || !strings.Contains(err.Error(), "piece K") {
			t.Errorf("%s: got %v", bad, err)
		}
	}
}

func TestZonesInheritanceAndMirroring(t *testing.T) {
	parent := `id: parentzone
files: 5
ranks: 5
setup: 2k2/5/5/5/2K2
promotion: []
pieces:
  - letter: K
    betza: W
    royal: true
    zone: [c1, c2]
  - letter: P
    betza: fW
    pawn: true
    move_zones:
      - zone: [b3]
        betza: ifWsW
`
	child := "id: childzone\nextends: parentzone\n"
	vs, err := parseVariants(map[string][]byte{"a.yaml": []byte(parent), "b.yaml": []byte(child)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "childzone")
	if err != nil {
		t.Fatal(err)
	}
	typ := v.Pieces[v.TypeOf('P')-1]
	if !typ.tracksMoved {
		t.Fatal("initial alternative not tracked")
	}
	p, err := ParseFEN(v, "2k2/5/1p3/5/2K2 b")
	if err != nil {
		t.Fatal(err)
	}
	a3, _ := v.ParseSquare("a3")
	if !p.anyAttacked([]int{a3}, Black) {
		t.Fatal("black alternative missing")
	}
	c4, _ := v.ParseSquare("c4")
	if !p.anyAttacked([]int{c4}, Black) {
		t.Fatal("black mirrored confinement missing")
	}
	d5, _ := v.ParseSquare("d5")
	if p.anyAttacked([]int{d5}, Black) {
		t.Fatal("black confinement escaped")
	}
}

func TestRoyalAttackCastlingTransit(t *testing.T) {
	data := `id: royalcastle
files: 5
ranks: 5
setup: 2kr1/5/5/5/2KR1
pieces:
  - letter: K
    betza: W
    royal: true
    royal_attack: vR
  - letter: R
    betza: R
castling:
  - {king: c1, rook: d1, king_to: a1, rook_to: c1}
`
	// The opponent's royal attacks b1/b5, an empty transit square,
	// without attacking the king's starting square.
	vs, err := parseVariants(map[string][]byte{"castle.yaml": []byte(data)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "royalcastle")
	if err != nil {
		t.Fatal(err)
	}
	for _, fen := range []string{"1k3/5/5/5/2KR1 w", "2kr1/5/5/5/1K3 b"} {
		p, err := ParseFEN(v, fen)
		if err != nil {
			t.Fatal(err)
		}
		if p.InCheck() {
			t.Fatalf("%s: starting square is in check", fen)
		}
		transit := "b1"
		if p.Turn == Black {
			transit = "b5"
		}
		sq, _ := v.ParseSquare(transit)
		if p.anyAttacked([]int{sq}, p.Turn.Other()) || !p.attacked([]int{sq}, p.Turn.Other(), true) {
			t.Fatalf("%s: hypothetical royal attack mismatch", fen)
		}
		if len(p.castles()) != 0 {
			t.Fatalf("%s: castled through royal attack", fen)
		}
	}
}

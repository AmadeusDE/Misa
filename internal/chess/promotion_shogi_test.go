package chess

import (
	"slices"
	"testing"
)

func TestPiecePromotionZones(t *testing.T) {
	v := builtin(t, "shogi")
	for _, tc := range []struct {
		fen, from, to   string
		promoted, plain bool
	}{
		{"4k4/9/9/4P4/9/9/9/9/4K4 w - - 0 1", "e6", "e7", true, true},
		{"4k4/9/4P4/9/9/9/9/9/4K4 w - - 0 1", "e7", "e8", true, true},
		{"k8/4P4/9/9/9/9/9/9/4K4 w - - 0 1", "e8", "e9", true, false},
		{"4k4/9/9/9/9/9/4p4/9/K8 b - - 0 1", "e3", "e2", true, true},
		{"4k4/9/9/9/9/9/9/4p4/K8 b - - 0 1", "e2", "e1", true, false},
		{"4k4/9/9/4S4/9/9/9/9/4K4 w - - 0 1", "e6", "d7", true, true},
		{"4k4/9/4S4/9/9/9/9/9/4K4 w - - 0 1", "e7", "d6", true, true},
		{"4k4/9/9/4S4/9/9/9/9/4K4 w - - 0 1", "e6", "d5", false, true},
	} {
		p := fen(t, v, tc.fen)
		from, _ := v.ParseSquare(tc.from)
		to, _ := v.ParseSquare(tc.to)
		var promoted, plain bool
		for _, m := range p.Moves() {
			if m.From == from && m.To == to {
				if m.Promote != 0 {
					promoted = true
				} else {
					plain = true
				}
			}
		}
		if promoted != tc.promoted || plain != tc.plain {
			t.Errorf("%s-%s: promoted=%v plain=%v", tc.from, tc.to, promoted, plain)
		}
	}
}

func TestOptionalPromotionUI(t *testing.T) {
	v := builtin(t, "shogi")
	p := fen(t, v, "4k4/9/9/4P4/9/9/9/9/4K4 w - - 0 1")
	// The prompt offers the promoted piece and the pawn itself: T promotes,
	// and the pawn's own letter declines.
	for _, tc := range []struct {
		key      string
		promoted bool
	}{{"T", true}, {"p", false}} {
		g := New([]*Variant{v}, v)
		g.SetComputer(false, false)
		g.Load(p)
		from, _ := v.ParseSquare("e6")
		to, _ := v.ParseSquare("e7")
		g.activate(from)
		g.activate(to)
		if len(g.promo) != 2 {
			t.Fatal("missing optional promotion prompt")
		}
		g.promotionKey(tc.key)
		pc := g.match.Position().Board[to]
		if (pc.Type == v.TypeOf('T')) != tc.promoted {
			t.Fatalf("%s: got %+v", tc.key, pc)
		}
	}
	// Quiet optional nonpromotion must not appear in quiescence.
	from, _ := v.ParseSquare("e6")
	to, _ := v.ParseSquare("e7")
	if slices.ContainsFunc(p.captures(), func(m Move) bool { return m.From == from && m.To == to && m.Promote == 0 }) {
		t.Fatal("quiet nonpromotion in captures")
	}
}

func TestPromotionYAMLValidation(t *testing.T) {
	for _, field := range []string{"promotes_to: Z", "promotion: [Z]", "promotes_to: Q, promotion: [Q]", "promotes_to: Q, promotion_zone: 10", "promotes_to: Q, promotion_mandatory: -1"} {
		data := []byte("id: invalid\nextends: chess\npieces:\n  - {letter: P, betza: fW, " + field + "}\n")
		if _, err := parseVariants(map[string][]byte{"invalid.yaml": data}); err == nil {
			t.Errorf("accepted %s", field)
		}
	}
}

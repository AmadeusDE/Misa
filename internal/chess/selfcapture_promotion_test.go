package chess

import (
	"slices"
	"strings"
	"testing"
)

// The move generator applies the promotion rules inside the one add() that
// every move goes through, so a pawn that self-captures gets the same
// treatment as one that captures an enemy: optional promotion offers the plain
// move as well, and captured-only promotion drops the types the side still
// owns. These positions pin that interaction.

// selfCapturePromo builds a chess variant with self-capture on and the two
// Grand Chess promotion rules spelled out in YAML.
func selfCapturePromo(t *testing.T, capturedOnly bool) *Variant {
	t.Helper()
	src := "id: scpromo\nname: Self-capture promotion\nextends: chess\n" +
		"self_capture: true\npromotion_zone: 3\npromotion_optional: true\n"
	if capturedOnly {
		src += "promotion_captured_only: true\n"
	}
	return rulesVariant(t, "scpromo", src)
}

// selfCapturePawnFEN puts a white pawn on b6 with a white rook on a7 to take:
// b6xa7 promotes inside a three-rank zone and is not the final rank, and a
// second white rook on h1 makes the rook a type White still owns.
func selfCapturePawnFEN(t *testing.T, v *Variant) *Position {
	t.Helper()
	p, err := ParseFEN(v, "k7/R7/1P6/8/8/8/8/4K2R w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSelfCapturePaysThePromotionRules(t *testing.T) {
	for _, tc := range []struct {
		name         string
		capturedOnly bool
		wantPromos   []string
	}{
		{
			name:       "optional offers the plain move too",
			wantPromos: []string{"a7=B", "a7=N", "a7=Q", "a7=R", "b7=B", "b7=N", "b7=Q", "b7=R"},
		},
		{
			name:         "captured-only drops the types White still owns",
			capturedOnly: true,
			wantPromos:   []string{"a7=B", "a7=N", "a7=Q", "b7=B", "b7=N", "b7=Q"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := selfCapturePromo(t, tc.capturedOnly)
			p := selfCapturePawnFEN(t, v)
			plain, promos := rulesMoves(t, p, "b6")
			// The self-capture b6xa7 and the forward b7 both promote inside the
			// zone, and both keep the plain pawn move beside them.
			if want := []string{"a7", "b7"}; !slices.Equal(plain, want) {
				t.Errorf("plain = %v, want %v", plain, want)
			}
			if !slices.Equal(promos, tc.wantPromos) {
				t.Errorf("promotions = %v, want %v", promos, tc.wantPromos)
			}
			// A self-capture is a capture, so an empty square never offers it.
			for _, m := range p.Moves() {
				if m.From == rulesSquare(t, v, "b6") && m.To == rulesSquare(t, v, "a7") && m.Take != m.To {
					t.Errorf("b6xa7 does not record the self-capture: %+v", m)
				}
			}
		})
	}
}

// TestSelfCaptureForcedOnLastRank checks the other half of the rule: a
// self-capture onto the final rank promotes, with no plain move beside it, and
// under captured-only it is not offered at all when nothing is available.
func TestSelfCaptureForcedOnLastRank(t *testing.T) {
	v := selfCapturePromo(t, false)
	p, err := ParseFEN(v, "R6k/1P6/8/8/8/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	// b7xa8 takes the own rook on the last rank: the promoted moves only.
	plain, promos := rulesSplit(t, v, p.Moves(), "b7")
	if len(plain) != 0 {
		t.Errorf("plain = %v, want none on the last rank", plain)
	}
	if want := []string{"a8=B", "a8=N", "a8=Q", "a8=R", "b8=B", "b8=N", "b8=Q", "b8=R"}; !slices.Equal(promos, want) {
		t.Errorf("promotions = %v, want %v", promos, want)
	}

	// With captured-only and both rooks still on the board, a rook is not on
	// offer, and the queen, bishop and knight still are.
	only := selfCapturePromo(t, true)
	q, err := ParseFEN(only, "R6k/1P6/8/8/8/8/8/4K2R w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	plain, promos = rulesSplit(t, only, q.Moves(), "b7")
	if len(plain) != 0 || !slices.Equal(promos, []string{"a8=B", "a8=N", "a8=Q", "b8=B", "b8=N", "b8=Q"}) {
		t.Errorf("captured-only last rank: plain = %v, promotions = %v", plain, promos)
	}
	// Quiescence sees both, since promotions are tactical there too.
	if _, promos = rulesCaptures(t, q, "b7"); !slices.Equal(promos, []string{"a8=B", "a8=N", "a8=Q", "b8=B", "b8=N", "b8=Q"}) {
		t.Errorf("quiescence promotions = %v", promos)
	}

	// Self-capture never shadows an ordinary capture: an enemy king on the
	// self-capture target is still attacked, whatever promotion offers.
	e, err := ParseFEN(only, "k7/1P6/8/8/8/8/8/4K2R w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if !e.anyAttacked([]int{rulesSquare(t, only, "a8")}, White) {
		t.Error("b7 no longer attacks a8")
	}
}

// TestSelfCapturePromotionNotation keeps the played move's name honest: the
// self-capture takes a piece, so the notation marks it.
func TestSelfCapturePromotionNotation(t *testing.T) {
	v := selfCapturePromo(t, false)
	m := NewMatch(v)
	p := selfCapturePawnFEN(t, v)
	m.Load(p)
	var chosen Move
	found := false
	for _, mv := range m.Legal() {
		if mv.From == rulesSquare(t, v, "b6") && mv.To == rulesSquare(t, v, "a7") &&
			mv.Promote != 0 && v.Pieces[mv.Promote-1].Letter == 'Q' {
			chosen, found = mv, true
			break
		}
	}
	if !found {
		t.Fatal("no b6xa7=Q")
	}
	m.Play(chosen)
	got := m.Notation()
	if len(got) != 1 || !strings.Contains(got[0], "xa7=Q") {
		t.Fatalf("notation = %v, want a self-capture promotion to a queen", got)
	}
}

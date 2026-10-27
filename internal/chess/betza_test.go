package chess

import (
	"slices"
	"testing"
)

func vectorsOf(t *testing.T, betza string) [][2]int {
	t.Helper()
	comps, err := ParseBetza(betza)
	if err != nil {
		t.Fatalf("%s: %v", betza, err)
	}
	var all [][2]int
	for _, c := range comps {
		all = append(all, c.Vectors...)
	}
	return all
}

func TestBetzaAtomVectorCounts(t *testing.T) {
	tests := map[string]int{"W": 4, "F": 4, "K": 8, "N": 8, "D": 4, "A": 4, "C": 8, "Z": 8, "Q": 8, "NLJ": 24}
	for betza, want := range tests {
		if got := len(vectorsOf(t, betza)); got != want {
			t.Errorf("%s: %d vectors, want %d", betza, got, want)
		}
	}
}

func TestBetzaRanges(t *testing.T) {
	tests := []struct {
		betza string
		want  []int
	}{
		{"W", []int{1}},
		{"R", []int{0}},
		{"NN", []int{0}},
		{"W4", []int{4}},
		{"B3", []int{3}},
		{"Q", []int{0, 0}},
	}
	for _, tt := range tests {
		comps, err := ParseBetza(tt.betza)
		if err != nil {
			t.Fatalf("%s: %v", tt.betza, err)
		}
		var got []int
		for _, c := range comps {
			got = append(got, c.Range)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: ranges %v, want %v", tt.betza, got, tt.want)
		}
	}
}

func TestBetzaPawn(t *testing.T) {
	comps, err := ParseBetza("fmWfcFifmW2")
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 3 {
		t.Fatalf("got %d components, want 3", len(comps))
	}
	push, take, double := comps[0], comps[1], comps[2]
	if !push.Move || push.Capture || !slices.Equal(push.Vectors, [][2]int{{0, 1}}) {
		t.Errorf("push = %+v", push)
	}
	if take.Move || !take.Capture || len(take.Vectors) != 2 {
		t.Errorf("capture = %+v", take)
	}
	for _, v := range take.Vectors {
		if v[1] != 1 {
			t.Errorf("capture vector %v is not forward", v)
		}
	}
	if !double.Initial || double.Range != 2 || double.Capture {
		t.Errorf("double step = %+v", double)
	}
}

func TestBetzaDirections(t *testing.T) {
	tests := []struct {
		betza string
		want  [][2]int
	}{
		{"frF", [][2]int{{1, 1}}},
		{"blF", [][2]int{{-1, -1}}},
		{"fsW", [][2]int{{1, 0}, {0, 1}, {-1, 0}}},
		{"vW", [][2]int{{0, 1}, {0, -1}}},
		{"sW", [][2]int{{1, 0}, {-1, 0}}},
		{"flN", [][2]int{{-1, 2}, {-2, 1}}},
	}
	for _, tt := range tests {
		got := vectorsOf(t, tt.betza)
		if !sameVectors(got, tt.want) {
			t.Errorf("%s: %v, want %v", tt.betza, got, tt.want)
		}
	}
}

func TestBetzaModifiers(t *testing.T) {
	comps, err := ParseBetza("mRcpR")
	if err != nil {
		t.Fatal(err)
	}
	if comps[0].Capture || comps[0].Hop != hopNone || !comps[1].Capture || comps[1].Move || comps[1].Hop != hopCannon {
		t.Errorf("cannon = %+v", comps)
	}
	rose, err := ParseBetza("qN")
	if err != nil {
		t.Fatal(err)
	}
	if !rose[0].Circular || len(rose[0].Ring) != 8 {
		t.Errorf("rose = %+v", rose[0])
	}
	horse, _ := ParseBetza("nN")
	if !horse[0].Lame {
		t.Errorf("nN is not lame")
	}
}

func TestBetzaRejectsBadInput(t *testing.T) {
	for _, s := range []string{"", "x", "fm", "Y", "W0", "mmW"} {
		if _, err := ParseBetza(s); err == nil {
			t.Errorf("%q: no error", s)
		}
	}
}

func TestRingIsAngularOrder(t *testing.T) {
	comps, _ := ParseBetza("qN")
	ring := comps[0].Ring
	for i := range ring {
		a, b := ring[i], ring[(i+1)%len(ring)]
		if cross := a[0]*b[1] - a[1]*b[0]; cross <= 0 {
			t.Fatalf("ring step %v → %v does not turn counterclockwise", a, b)
		}
	}
}

func TestLamePath(t *testing.T) {
	tests := []struct {
		v    [2]int
		want [][2]int
	}{
		{[2]int{2, 1}, [][2]int{{1, 0}}},
		{[2]int{-1, 2}, [][2]int{{0, 1}}},
		{[2]int{2, 0}, [][2]int{{1, 0}}},
		{[2]int{-2, -2}, [][2]int{{-1, -1}}},
		{[2]int{1, 0}, nil},
	}
	for _, tt := range tests {
		if got := lamePath(tt.v); !slices.Equal(got, tt.want) {
			t.Errorf("lamePath(%v) = %v, want %v", tt.v, got, tt.want)
		}
	}
}

func sameVectors(a, b [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range b {
		if !slices.Contains(a, v) {
			return false
		}
	}
	return true
}

package cards

import (
	"reflect"
	"testing"
)

func TestFrozenShuffle(t *testing.T) {
	d := Deck()
	Shuffle(d, 42)
	got := []int{}
	for _, c := range d[:8] {
		got = append(got, c.ID)
	}
	if !reflect.DeepEqual(got, []int{6, 28, 12, 39, 40, 0, 26, 25}) {
		t.Fatal("v1 shuffle changed", got)
	}
}

func TestUnbiasedBoundRejectsLowTail(t *testing.T) {
	values := []uint64{0, 5, 26}
	calls := 0
	got := bounded(func() uint64 { v := values[calls]; calls++; return v }, 10)
	if got != 6 || calls != 3 {
		t.Fatal(got, calls)
	}
}

func FuzzShuffleConservation(f *testing.F) {
	f.Add(uint64(42))
	f.Add(uint64(0))
	f.Fuzz(func(t *testing.T, seed uint64) {
		d := Deck()
		Shuffle(d, seed)
		seen := map[int]bool{}
		for _, c := range d {
			if c.ID < 0 || c.ID >= 52 || seen[c.ID] {
				t.Fatal(c)
			}
			seen[c.ID] = true
		}
	})
}

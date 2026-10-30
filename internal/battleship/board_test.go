package battleship

import (
	"math/rand/v2"
	"testing"
)

func seeded(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed+1)) }

// withShips is a standard board holding exactly these ships, each given as
// {length, x, y, horizontal}.
func withShips(t *testing.T, ships ...[4]int) *Board {
	t.Helper()
	fleet := make([]int, len(ships))
	for i, s := range ships {
		fleet[i] = s[0]
	}
	b := NewBoard(Size, fleet, seeded(1))
	for _, s := range ships {
		if !b.Place(s[0], s[1], s[2], s[3] != 0) {
			t.Fatalf("could not place a %d ship at %d,%d", s[0], s[1], s[2])
		}
	}
	return b
}

func occupied(b *Board) int {
	n := 0
	for y := range b.Size() {
		for x := range b.Size() {
			if b.ShipAt(x, y) != nil {
				n++
			}
		}
	}
	return n
}

func TestPlaceConsumesPending(t *testing.T) {
	b := NewBoard(Size, Fleet, seeded(1))
	if got := len(b.Pending()); got != len(Fleet) {
		t.Fatalf("pending = %d, want %d", got, len(Fleet))
	}
	if !b.Place(5, 0, 0, true) {
		t.Fatal("a 5 ship does not fit at 0,0")
	}
	if len(b.Pending()) != len(Fleet)-1 || len(b.Ships()) != 1 {
		t.Fatalf("pending = %d, ships = %d", len(b.Pending()), len(b.Ships()))
	}
	if got := b.Ships()[0].Name; got != "Carrier" {
		t.Errorf("first ship is %q, want Carrier", got)
	}
	if got := occupied(b); got != 5 {
		t.Errorf("occupied = %d squares, want 5", got)
	}
}

func TestFleetSpecNames(t *testing.T) {
	want := []string{"Carrier", "Battleship", "Cruiser", "Submarine", "Destroyer"}
	specs := NewBoard(Size, Fleet, seeded(1)).Pending()
	if len(specs) != len(want) {
		t.Fatalf("got %d ships, want %d", len(specs), len(want))
	}
	for i, spec := range specs {
		if spec.Name != want[i] || spec.Len != Fleet[i] {
			t.Errorf("ship %d = %dx %s, want %dx %s", i, spec.Len, spec.Name, Fleet[i], want[i])
		}
	}
}

func TestPlaceRefusesOverlap(t *testing.T) {
	b := NewBoard(Size, []int{3, 2}, seeded(1))
	if !b.Place(3, 0, 0, true) {
		t.Fatal("a 3 ship does not fit at 0,0")
	}
	if b.Place(2, 2, 0, true) {
		t.Fatal("a second ship overlapped the first")
	}
	if b.Place(2, 1, 0, false) {
		t.Fatal("a vertical ship overlapped the first")
	}
	if !b.Place(2, 2, 1, true) {
		t.Fatal("touching ships should be allowed")
	}
}

func TestPlaceRefusesOffBoard(t *testing.T) {
	b := NewBoard(Size, Fleet, seeded(1))
	for _, tt := range []struct {
		length, x, y int
		horizontal   bool
	}{
		{5, 6, 0, true},
		{5, 0, 6, false},
		{5, -1, 0, true},
		{5, 0, -1, false},
		{6, 5, 0, true},
	} {
		if b.CanPlace(tt.length, tt.x, tt.y, tt.horizontal) {
			t.Errorf("a %d ship fits at %d,%d horizontal=%v", tt.length, tt.x, tt.y, tt.horizontal)
		}
		if b.Place(tt.length, tt.x, tt.y, tt.horizontal) {
			t.Errorf("a %d ship placed at %d,%d horizontal=%v", tt.length, tt.x, tt.y, tt.horizontal)
		}
	}
	if len(b.Ships()) != 0 {
		t.Fatal("a refused placement still placed a ship")
	}
}

func TestPlaceRefusesUnknownLength(t *testing.T) {
	b := NewBoard(Size, Fleet, seeded(1))
	if b.Place(7, 0, 0, true) || b.Place(0, 0, 0, true) {
		t.Fatal("placed a ship that is not in the fleet")
	}
}

func TestCanPlaceDoesNotPlace(t *testing.T) {
	b := NewBoard(Size, Fleet, seeded(1))
	if !b.CanPlace(5, 3, 3, true) {
		t.Fatal("a 5 ship does not fit at 3,3")
	}
	if len(b.Ships()) != 0 || len(b.Pending()) != len(Fleet) {
		t.Fatal("CanPlace changed the board")
	}
}

func TestCellsRunFromTheBow(t *testing.T) {
	b := withShips(t, [4]int{3, 2, 4, 1}, [4]int{2, 6, 7, 0})
	horizontal := b.Ships()[0].Cells()
	want := []Point{{2, 4}, {3, 4}, {4, 4}}
	for i, p := range want {
		if horizontal[i] != p {
			t.Errorf("horizontal cell %d = %v, want %v", i, horizontal[i], p)
		}
	}
	vertical := b.Ships()[1].Cells()
	wantVertical := []Point{{6, 7}, {6, 8}}
	for i, p := range wantVertical {
		if vertical[i] != p {
			t.Errorf("vertical cell %d = %v, want %v", i, vertical[i], p)
		}
	}
}

func TestFireResults(t *testing.T) {
	b := withShips(t, [4]int{2, 3, 3, 1})
	if got := b.Fire(0, 0); got != Miss {
		t.Errorf("water shot = %v, want Miss", got)
	}
	if !b.Fired(0, 0) {
		t.Error("missed square is not marked fired")
	}
	if got := b.Fire(3, 3); got != Hit {
		t.Errorf("ship shot = %v, want Hit", got)
	}
	if got := b.Fire(3, 3); got != Unfired {
		t.Errorf("repeated shot = %v, want Unfired", got)
	}
	if got := b.Fire(0, 0); got != Unfired {
		t.Errorf("repeated miss = %v, want Unfired", got)
	}
	for _, p := range []Point{{-1, 0}, {0, -1}, {Size, 0}, {0, Size}} {
		if got := b.Fire(p.X, p.Y); got != Unfired {
			t.Errorf("shot off the board at %v = %v, want Unfired", p, got)
		}
	}
	if b.Fired(-1, 0) || b.Shot(Size, Size) != Unfired {
		t.Error("off-board squares report shots")
	}
}

func TestShipSinksOnLastHit(t *testing.T) {
	b := withShips(t, [4]int{2, 3, 3, 1})
	ship := b.ShipAt(3, 3)
	if ship == nil || ship.Name != "Destroyer" {
		t.Fatalf("ship at 3,3 = %v", ship)
	}
	b.Fire(3, 3)
	if ship.Sunk() || b.SunkAt(3, 3) {
		t.Fatal("ship sank on one of two hits")
	}
	if ship.Hits() != 1 || b.Afloat() != 1 {
		t.Errorf("hits = %d, afloat = %d, want 1 and 1", ship.Hits(), b.Afloat())
	}
	b.Fire(4, 3)
	if !ship.Sunk() || !b.SunkAt(3, 3) || !b.SunkAt(4, 3) {
		t.Fatal("ship did not sink on its last hit")
	}
	if b.Afloat() != 0 {
		t.Errorf("afloat = %d, want 0", b.Afloat())
	}
	if !b.AllSunk() {
		t.Fatal("AllSunk is false with every ship sunk")
	}
}

func TestAllSunkNeedsEveryShipPlaced(t *testing.T) {
	b := NewBoard(Size, Fleet, seeded(1))
	if b.AllSunk() {
		t.Fatal("an empty board reports a sunk fleet")
	}
	b.Randomize()
	if b.AllSunk() {
		t.Fatal("a fresh fleet reports itself sunk")
	}
}

func TestRandomizeFillsTheFleet(t *testing.T) {
	for seed := range uint64(20) {
		b := NewBoard(Size, Fleet, seeded(seed))
		if !b.Randomize() {
			t.Fatalf("seed %d: Randomize failed", seed)
		}
		if len(b.Pending()) != 0 || len(b.Ships()) != len(Fleet) {
			t.Fatalf("seed %d: pending = %d, ships = %d", seed, len(b.Pending()), len(b.Ships()))
		}
		if got := occupied(b); got != 17 {
			t.Fatalf("seed %d: %d squares occupied, want 17", seed, got)
		}
		for _, s := range b.Ships() {
			for _, p := range s.Cells() {
				if b.ShipAt(p.X, p.Y) != s {
					t.Fatalf("seed %d: %s does not own %v", seed, s.Name, p)
				}
			}
		}
	}
}

func TestClearResetsShipsAndShots(t *testing.T) {
	b := NewBoard(Size, Fleet, seeded(1))
	if !b.Place(5, 0, 0, true) {
		t.Fatal("a 5 ship does not fit at 0,0")
	}
	b.Fire(0, 0)
	b.Fire(5, 5)
	b.Clear()
	if len(b.Ships()) != 0 || len(b.Pending()) != len(Fleet) {
		t.Fatalf("ships = %d, pending = %d", len(b.Ships()), len(b.Pending()))
	}
	for y := range b.Size() {
		for x := range b.Size() {
			if b.Fired(x, y) || b.ShipAt(x, y) != nil {
				t.Fatalf("square %d,%d survived Clear", x, y)
			}
		}
	}
}

func TestRandomizeKeepsTheFleetOrder(t *testing.T) {
	b := NewBoard(Size, Fleet, seeded(7))
	b.Randomize()
	names := make([]string, 0, len(b.Ships()))
	for _, s := range b.Ships() {
		names = append(names, s.Name)
	}
	want := []string{"Carrier", "Battleship", "Cruiser", "Submarine", "Destroyer"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("ship %d is %q, want %q", i, names[i], want[i])
		}
	}
}

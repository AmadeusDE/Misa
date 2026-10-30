package battleship

import (
	"testing"
)

// fleetBoard is a 10×10 board with the standard fleet placed at random.
func fleetBoard(seed uint64) *Board {
	b := NewBoard(Size, Fleet, seeded(seed))
	b.Randomize()
	return b
}

func TestAIEverySquareOnce(t *testing.T) {
	b := fleetBoard(3)
	a := NewAI(seeded(4))
	seen := map[Point]bool{}
	for shots := 1; ; shots++ {
		p, ok := a.Fire(b)
		if !ok {
			break
		}
		if shots > Size*Size {
			t.Fatalf("shot %d on a board with %d squares", shots, Size*Size)
		}
		if p.X < 0 || p.Y < 0 || p.X >= b.Size() || p.Y >= b.Size() {
			t.Fatalf("shot %d off the board at %v", shots, p)
		}
		if seen[p] {
			t.Fatalf("shot %d repeated %v", shots, p)
		}
		seen[p] = true
		if !b.Fired(p.X, p.Y) {
			t.Fatalf("shot %d at %v did not mark the board", shots, p)
		}
	}
	if len(seen) != Size*Size {
		t.Fatalf("fired at %d squares, want %d", len(seen), Size*Size)
	}
}

func TestAISinksTheFleet(t *testing.T) {
	for seed := range uint64(20) {
		b := fleetBoard(seed)
		a := NewAI(seeded(seed + 100))
		shots := 0
		for !b.AllSunk() {
			p, ok := a.Fire(b)
			if !ok {
				t.Fatalf("seed %d: ran out of squares after %d shots", seed, shots)
			}
			shots++
			if shots > Size*Size {
				t.Fatalf("seed %d: %d shots without sinking the fleet", seed, shots)
			}
			if p.X < 0 || p.Y < 0 || p.X >= Size || p.Y >= Size {
				t.Fatalf("seed %d: shot off the board at %v", seed, p)
			}
		}
	}
}

func TestAIFinishesAShip(t *testing.T) {
	b := withShips(t, [4]int{3, 4, 6, 1})
	a := NewAI(seeded(9))
	shots, firstHit := 0, -1
	for !b.AllSunk() {
		p, ok := a.Fire(b)
		if !ok {
			t.Fatal("ran out of squares")
		}
		shots++
		if shots > Size*Size {
			t.Fatal("never sank the ship")
		}
		if firstHit < 0 && b.Shot(p.X, p.Y) == Hit {
			firstHit = shots
		}
	}
	if firstHit < 0 {
		t.Fatal("never hit the ship")
	}
	if after := shots - firstHit; after > 6 {
		t.Fatalf("took %d shots after the first hit to sink a 3 ship", after)
	}
}

func TestAIExtendsALine(t *testing.T) {
	b := withShips(t, [4]int{5, 2, 7, 1})
	if b.Fire(4, 7) != Hit || b.Fire(5, 7) != Hit {
		t.Fatal("could not hit the ship twice")
	}
	a := NewAI(seeded(2))
	for shots := 0; !b.AllSunk(); shots++ {
		if shots > 20 {
			t.Fatal("never sank the wounded ship")
		}
		p, ok := a.Fire(b)
		if !ok {
			t.Fatal("ran out of squares")
		}
		if p.Y != 7 || p.X < 1 || p.X > 7 {
			t.Fatalf("shot %v is off the line of hits", p)
		}
	}
}

func TestAIForgetsSunkenShips(t *testing.T) {
	b := withShips(t, [4]int{2, 0, 0, 1}, [4]int{2, 8, 9, 1})
	b.Fire(0, 0)
	b.Fire(1, 0)
	if b.Afloat() != 1 {
		t.Fatalf("afloat = %d, want 1", b.Afloat())
	}
	p, ok := NewAI(seeded(5)).Fire(b)
	if !ok {
		t.Fatal("ran out of squares")
	}
	if (p.X+p.Y)%2 != 0 {
		t.Fatalf("shot %v is not a hunting shot: the AI is still working the sunken ship", p)
	}
}

func TestAIHuntsOnParity(t *testing.T) {
	for seed := range uint64(10) {
		p, ok := NewAI(seeded(seed)).Fire(fleetBoard(seed))
		if !ok {
			t.Fatalf("seed %d: no first shot", seed)
		}
		if (p.X+p.Y)%2 != 0 {
			t.Fatalf("seed %d: first shot %v is not on the checkerboard", seed, p)
		}
	}
}

func TestAIDeterministic(t *testing.T) {
	play := func(seed uint64) []Point {
		b := fleetBoard(seed)
		a := NewAI(seeded(seed + 7))
		var out []Point
		for len(out) < 40 {
			p, ok := a.Fire(b)
			if !ok {
				break
			}
			out = append(out, p)
		}
		return out
	}
	first, second := play(11), play(11)
	if len(first) != len(second) {
		t.Fatalf("shot counts differ: %d and %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("shot %d = %v then %v", i, first[i], second[i])
		}
	}
}

func TestAIFullBoard(t *testing.T) {
	b := withShips(t, [4]int{2, 0, 0, 1})
	for y := range Size {
		for x := range Size {
			b.Fire(x, y)
		}
	}
	if _, ok := NewAI(seeded(1)).Fire(b); ok {
		t.Fatal("fired at a full board")
	}
}

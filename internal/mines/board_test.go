package mines

import (
	"math/rand/v2"
	"testing"
)

func planted(w, h int, mines ...[2]int) *Board {
	b := NewBoard(w, h, len(mines), rand.New(rand.NewPCG(1, 2)))
	idx := make([]int, len(mines))
	for i, p := range mines {
		idx[i] = b.index(p[0], p[1])
	}
	b.plant(idx)
	return b
}

func TestFirstRevealIsSafe(t *testing.T) {
	for seed := range uint64(50) {
		b := NewBoard(9, 9, 10, rand.New(rand.NewPCG(seed, seed)))
		b.Reveal(4, 4)
		if b.State() == Lost {
			t.Fatalf("seed %d: first reveal hit a mine", seed)
		}
		if b.Adjacent(4, 4) != 0 {
			t.Fatalf("seed %d: first reveal has %d neighbouring mines", seed, b.Adjacent(4, 4))
		}
		if got := b.countMines(); got != 10 {
			t.Fatalf("seed %d: planted %d mines, want 10", seed, got)
		}
	}
}

func TestFirstRevealSafeOnCrowdedBoard(t *testing.T) {
	b := NewBoard(3, 3, 8, rand.New(rand.NewPCG(3, 4)))
	b.Reveal(1, 1)
	if b.State() != Won {
		t.Fatalf("state = %v, want Won (only safe cell revealed)", b.State())
	}
}

func TestMineCountClamped(t *testing.T) {
	b := NewBoard(2, 2, 10, rand.New(rand.NewPCG(1, 1)))
	if b.Mines() != 3 {
		t.Fatalf("mines = %d, want 3", b.Mines())
	}
}

func TestAdjacency(t *testing.T) {
	b := planted(3, 3, [2]int{0, 0}, [2]int{2, 2})
	want := [][]int{
		{0, 1, 0},
		{1, 2, 1},
		{0, 1, 0},
	}
	for y, row := range want {
		for x, n := range row {
			if b.IsMine(x, y) {
				continue
			}
			if got := b.Adjacent(x, y); got != n {
				t.Errorf("adjacent(%d,%d) = %d, want %d", x, y, got, n)
			}
		}
	}
}

func TestFloodFillStopsAtNumbers(t *testing.T) {
	b := planted(5, 5, [2]int{4, 4})
	b.Reveal(0, 0)
	if b.Revealed(4, 4) {
		t.Fatal("mine revealed by flood fill")
	}
	if b.State() != Won {
		t.Fatalf("state = %v, want Won", b.State())
	}
}

func TestFloodFillPartial(t *testing.T) {
	b := planted(5, 1, [2]int{2, 0})
	b.Reveal(0, 0)
	if !b.Revealed(0, 0) || !b.Revealed(1, 0) {
		t.Fatal("left side not revealed")
	}
	if b.Revealed(3, 0) || b.Revealed(4, 0) {
		t.Fatal("flood fill crossed a number")
	}
}

func TestRevealMineLoses(t *testing.T) {
	b := planted(3, 3, [2]int{0, 0})
	b.Reveal(1, 1)
	b.Reveal(0, 0)
	if b.State() != Lost {
		t.Fatalf("state = %v, want Lost", b.State())
	}
	if x, y, ok := b.Exploded(); !ok || x != 0 || y != 0 {
		t.Fatalf("exploded = %d,%d,%v", x, y, ok)
	}
}

func TestFlagBlocksReveal(t *testing.T) {
	b := planted(3, 3, [2]int{0, 0})
	b.Reveal(1, 1)
	b.ToggleFlag(0, 0)
	b.Reveal(0, 0)
	if b.State() != Playing {
		t.Fatalf("state = %v, want Playing", b.State())
	}
	if b.MinesLeft() != 0 {
		t.Fatalf("mines left = %d, want 0", b.MinesLeft())
	}
	b.ToggleFlag(0, 0)
	if b.Flagged(0, 0) || b.MinesLeft() != 1 {
		t.Fatal("flag not removed")
	}
}

func TestCannotFlagRevealed(t *testing.T) {
	b := planted(4, 1, [2]int{0, 0})
	b.Reveal(1, 0)
	b.ToggleFlag(1, 0)
	if b.Flagged(1, 0) {
		t.Fatal("flagged a revealed cell")
	}
}

func TestFlagBeforeFirstRevealKeepsSafety(t *testing.T) {
	b := NewBoard(9, 9, 10, rand.New(rand.NewPCG(5, 6)))
	b.ToggleFlag(0, 0)
	b.Reveal(4, 4)
	if b.State() == Lost {
		t.Fatal("first reveal after a flag hit a mine")
	}
}

func TestChordRevealsNeighbours(t *testing.T) {
	b := planted(3, 3, [2]int{0, 0})
	b.Reveal(1, 1)
	b.ToggleFlag(0, 0)
	b.Chord(1, 1)
	if b.State() != Won {
		t.Fatalf("state = %v, want Won", b.State())
	}
}

func TestChordNeedsMatchingFlags(t *testing.T) {
	b := planted(3, 3, [2]int{0, 0})
	b.Reveal(1, 1)
	b.Chord(1, 1)
	if b.Revealed(2, 2) {
		t.Fatal("chord without flags revealed neighbours")
	}
}

func TestChordWithWrongFlagLoses(t *testing.T) {
	b := planted(3, 3, [2]int{0, 0})
	b.Reveal(1, 1)
	b.ToggleFlag(2, 2)
	b.Chord(1, 1)
	if b.State() != Lost {
		t.Fatalf("state = %v, want Lost", b.State())
	}
}

func TestWinFlagsAllMines(t *testing.T) {
	b := planted(3, 3, [2]int{0, 0})
	b.Reveal(2, 2)
	if b.State() != Won {
		t.Fatalf("state = %v, want Won", b.State())
	}
	if !b.Flagged(0, 0) || b.MinesLeft() != 0 {
		t.Fatal("mines not flagged on win")
	}
}

func TestNoMovesAfterGameOver(t *testing.T) {
	b := planted(3, 3, [2]int{0, 0})
	b.Reveal(1, 1)
	b.Reveal(0, 0)
	b.Reveal(2, 2)
	if b.Revealed(2, 2) {
		t.Fatal("revealed after losing")
	}
}

func TestOutOfBoundsIgnored(t *testing.T) {
	b := planted(3, 3, [2]int{0, 0})
	b.Reveal(-1, 0)
	b.Reveal(3, 3)
	b.ToggleFlag(5, 5)
	b.Chord(-2, 7)
	if b.State() != Playing {
		t.Fatalf("state = %v, want Playing", b.State())
	}
}

func (b *Board) countMines() int {
	n := 0
	for _, c := range b.cells {
		if c.mine {
			n++
		}
	}
	return n
}

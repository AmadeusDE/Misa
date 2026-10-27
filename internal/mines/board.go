// Package mines is a minesweeper clone: a Board with the rules and a Game
// that plays it in a terminal.
package mines

import "math/rand/v2"

type State int

const (
	Playing State = iota
	Won
	Lost
)

type cell struct {
	mine, revealed, flagged bool
	adjacent                uint8
}

// Board places its mines on the first reveal, away from the revealed cell,
// so the first move never loses.
type Board struct {
	w, h, mines int
	cells       []cell
	placed      bool
	state       State
	revealed    int
	flags       int
	exploded    int
	rng         *rand.Rand
}

func NewBoard(w, h, mines int, rng *rand.Rand) *Board {
	mines = max(0, min(mines, w*h-1))
	return &Board{w: w, h: h, mines: mines, cells: make([]cell, w*h), exploded: -1, rng: rng}
}

func (b *Board) Width() int     { return b.w }
func (b *Board) Height() int    { return b.h }
func (b *Board) Mines() int     { return b.mines }
func (b *Board) State() State   { return b.state }
func (b *Board) Started() bool  { return b.placed }
func (b *Board) MinesLeft() int { return b.mines - b.flags }

func (b *Board) IsMine(x, y int) bool   { return b.in(x, y) && b.cells[b.index(x, y)].mine }
func (b *Board) Revealed(x, y int) bool { return b.in(x, y) && b.cells[b.index(x, y)].revealed }
func (b *Board) Flagged(x, y int) bool  { return b.in(x, y) && b.cells[b.index(x, y)].flagged }

func (b *Board) Adjacent(x, y int) int {
	if !b.in(x, y) {
		return 0
	}
	return int(b.cells[b.index(x, y)].adjacent)
}

// Exploded is the mine whose reveal lost the game.
func (b *Board) Exploded() (x, y int, ok bool) {
	if b.exploded < 0 {
		return 0, 0, false
	}
	return b.exploded % b.w, b.exploded / b.w, true
}

func (b *Board) in(x, y int) bool   { return x >= 0 && y >= 0 && x < b.w && y < b.h }
func (b *Board) index(x, y int) int { return y*b.w + x }

func (b *Board) neighbours(i int, visit func(j int)) {
	x, y := i%b.w, i/b.w
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if (dx != 0 || dy != 0) && b.in(x+dx, y+dy) {
				visit(b.index(x+dx, y+dy))
			}
		}
	}
}

func (b *Board) Reveal(x, y int) {
	if b.state != Playing || !b.in(x, y) {
		return
	}
	i := b.index(x, y)
	if !b.placed {
		b.place(i)
	}
	b.open(i)
}

func (b *Board) ToggleFlag(x, y int) {
	if b.state != Playing || !b.in(x, y) {
		return
	}
	c := &b.cells[b.index(x, y)]
	if c.revealed {
		return
	}
	c.flagged = !c.flagged
	if c.flagged {
		b.flags++
	} else {
		b.flags--
	}
}

// Chord reveals the unflagged neighbours of a revealed number once as many
// flags surround it.
func (b *Board) Chord(x, y int) {
	if b.state != Playing || !b.in(x, y) {
		return
	}
	i := b.index(x, y)
	c := b.cells[i]
	if !c.revealed || c.adjacent == 0 {
		return
	}
	flags := 0
	b.neighbours(i, func(j int) {
		if b.cells[j].flagged {
			flags++
		}
	})
	if flags != int(c.adjacent) {
		return
	}
	b.neighbours(i, func(j int) {
		if b.state == Playing {
			b.open(j)
		}
	})
}

func (b *Board) place(first int) {
	safe := map[int]bool{first: true}
	if b.w*b.h-b.mines >= 9 {
		b.neighbours(first, func(j int) { safe[j] = true })
	}
	candidates := make([]int, 0, len(b.cells))
	for i := range b.cells {
		if !safe[i] {
			candidates = append(candidates, i)
		}
	}
	b.rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	b.plant(candidates[:min(b.mines, len(candidates))])
}

func (b *Board) plant(mines []int) {
	for _, i := range mines {
		b.cells[i].mine = true
		b.neighbours(i, func(j int) { b.cells[j].adjacent++ })
	}
	b.mines = len(mines)
	b.placed = true
}

func (b *Board) open(i int) {
	stack := []int{i}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		c := &b.cells[i]
		if c.revealed || c.flagged {
			continue
		}
		if c.mine {
			b.lose(i)
			return
		}
		c.revealed = true
		b.revealed++
		if c.adjacent == 0 {
			b.neighbours(i, func(j int) { stack = append(stack, j) })
		}
	}
	if b.revealed == len(b.cells)-b.mines {
		b.win()
	}
}

func (b *Board) lose(i int) {
	b.state = Lost
	b.exploded = i
	b.cells[i].revealed = true
}

func (b *Board) win() {
	b.state = Won
	for i := range b.cells {
		if b.cells[i].mine && !b.cells[i].flagged {
			b.cells[i].flagged = true
			b.flags++
		}
	}
}

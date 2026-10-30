// Package battleship is a single-player Battleship game: a Board with the
// rules, an AI that hunts a fleet, and a Game that plays it in a terminal.
package battleship

import "math/rand/v2"

// Size is the standard grid: ten by ten.
const Size = 10

// Fleet is the standard fleet, longest first.
var Fleet = []int{5, 4, 3, 3, 2}

// State is how a game stands.
type State int

const (
	Playing State = iota
	Won           // the player sank the whole enemy fleet
	Lost          // the computer sank the player's fleet
)

// Shot is what a square holds: nothing yet, a miss or a hit.
type Shot int

const (
	Unfired Shot = iota
	Miss
	Hit
)

// Point is a square on a board.
type Point struct{ X, Y int }

// ShipSpec is a ship that has not been placed yet.
type ShipSpec struct {
	Name string
	Len  int
}

// Ship is a vessel placed on a board: its bow, its length and how it lies.
type Ship struct {
	Name       string
	Len        int
	X, Y       int
	Horizontal bool
	hits       int
}

// Cells lists the squares the ship covers, bow first.
func (s *Ship) Cells() []Point {
	out := make([]Point, s.Len)
	for i := range out {
		out[i] = Point{s.X, s.Y}
		if s.Horizontal {
			out[i].X += i
		} else {
			out[i].Y += i
		}
	}
	return out
}

// Sunk reports whether every square of the ship has been hit.
func (s *Ship) Sunk() bool { return s.hits >= s.Len }

// Hits is how many of the ship's squares have been hit.
func (s *Ship) Hits() int { return s.hits }

type cell struct {
	ship int // index into Board.ships, -1 for water
	shot Shot
}

// Board is one fleet on a grid together with the shots fired at it.
type Board struct {
	size    int
	cells   []cell
	ships   []*Ship
	pending []ShipSpec
	rng     *rand.Rand
}

// NewBoard makes an empty size×size board, its ships waiting to be placed.
func NewBoard(size int, fleet []int, rng *rand.Rand) *Board {
	b := &Board{size: size, cells: make([]cell, size*size), pending: fleetSpecs(fleet), rng: rng}
	for i := range b.cells {
		b.cells[i].ship = -1
	}
	return b
}

// shipNames names the ships of the standard fleet; repeated lengths take the
// next name, and any other length is a plain "Ship".
var shipNames = map[int][]string{
	5: {"Carrier"},
	4: {"Battleship"},
	3: {"Cruiser", "Submarine"},
	2: {"Destroyer"},
}

func fleetSpecs(fleet []int) []ShipSpec {
	used := map[int]int{}
	specs := make([]ShipSpec, 0, len(fleet))
	for _, n := range fleet {
		names := shipNames[n]
		name := "Ship"
		if used[n] < len(names) {
			name = names[used[n]]
		}
		used[n]++
		specs = append(specs, ShipSpec{Name: name, Len: n})
	}
	return specs
}

func (b *Board) Size() int { return b.size }
func (b *Board) in(x, y int) bool {
	return x >= 0 && y >= 0 && x < b.size && y < b.size
}

func (b *Board) index(x, y int) int { return y*b.size + x }

// Pending lists the ships still to place, in fleet order.
func (b *Board) Pending() []ShipSpec { return b.pending }

// Ships lists the ships placed so far, in placement order.
func (b *Board) Ships() []*Ship { return b.ships }

// ShipAt is the ship on a square, or nil for water and off-board squares.
func (b *Board) ShipAt(x, y int) *Ship {
	if !b.in(x, y) {
		return nil
	}
	if i := b.cells[b.index(x, y)].ship; i >= 0 {
		return b.ships[i]
	}
	return nil
}

// Clear removes every ship, keeping the shots; a new placement starts here.
func (b *Board) Clear() {
	for i := range b.cells {
		b.cells[i].ship = -1
		b.cells[i].shot = Unfired
	}
	b.pending = fleetSpecs(b.totalFleet())
	b.ships = nil
}

// totalFleet rebuilds the fleet from placed and pending ships.
func (b *Board) totalFleet() []int {
	fleet := make([]int, 0, len(b.ships)+len(b.pending))
	for _, s := range b.ships {
		fleet = append(fleet, s.Len)
	}
	for _, s := range b.pending {
		fleet = append(fleet, s.Len)
	}
	return fleet
}

// CanPlace reports whether a length-long ship fits with its bow at x, y.
func (b *Board) CanPlace(length, x, y int, horizontal bool) bool {
	if length < 1 {
		return false
	}
	w, h := length, 1
	if !horizontal {
		w, h = 1, length
	}
	if x < 0 || y < 0 || x+w > b.size || y+h > b.size {
		return false
	}
	for dy := range h {
		for dx := range w {
			if b.cells[b.index(x+dx, y+dy)].ship >= 0 {
				return false
			}
		}
	}
	return true
}

// Place puts the first pending ship of that length on the board; it reports
// whether it fitted.
func (b *Board) Place(length, x, y int, horizontal bool) bool {
	at := -1
	for i, spec := range b.pending {
		if spec.Len == length {
			at = i
			break
		}
	}
	if at < 0 || !b.CanPlace(length, x, y, horizontal) {
		return false
	}
	spec := b.pending[at]
	b.pending = append(b.pending[:at:at], b.pending[at+1:]...)
	b.ships = append(b.ships, &Ship{Name: spec.Name, Len: spec.Len, X: x, Y: y, Horizontal: horizontal})
	ship := len(b.ships) - 1
	for _, p := range b.ships[ship].Cells() {
		b.cells[b.index(p.X, p.Y)].ship = ship
	}
	return true
}

// PlaceRandom places every pending ship, trying random squares first and
// falling back to a scan so a crowded board still fills.
func (b *Board) PlaceRandom() bool {
	for len(b.pending) > 0 {
		spec := b.pending[0]
		if !b.placeRandomly(spec.Len) {
			return false
		}
	}
	return true
}

func (b *Board) placeRandomly(length int) bool {
	for range 100 {
		x, y := b.rng.IntN(b.size), b.rng.IntN(b.size)
		if b.Place(length, x, y, b.rng.IntN(2) == 0) {
			return true
		}
	}
	for _, horizontal := range []bool{true, false} {
		for y := range b.size {
			for x := range b.size {
				if b.Place(length, x, y, horizontal) {
					return true
				}
			}
		}
	}
	return false
}

// Randomize clears the board and places the whole fleet at random.
func (b *Board) Randomize() bool {
	b.Clear()
	return b.PlaceRandom()
}

// Shot is the result of any shot fired at a square.
func (b *Board) Shot(x, y int) Shot {
	if !b.in(x, y) {
		return Unfired
	}
	return b.cells[b.index(x, y)].shot
}

// Fired reports whether a square has been shot at.
func (b *Board) Fired(x, y int) bool { return b.Shot(x, y) != Unfired }

// Fire shoots at a square and returns the result: Miss or Hit. It returns
// Unfired when the square was already shot at or lies off the board, so a
// caller can tell a fresh shot from a repeated one.
func (b *Board) Fire(x, y int) Shot {
	if !b.in(x, y) {
		return Unfired
	}
	c := &b.cells[b.index(x, y)]
	if c.shot != Unfired {
		return Unfired
	}
	if c.ship >= 0 {
		c.shot = Hit
		b.ships[c.ship].hits++
		return Hit
	}
	c.shot = Miss
	return Miss
}

// SunkAt reports whether the ship on a square has been sunk.
func (b *Board) SunkAt(x, y int) bool {
	s := b.ShipAt(x, y)
	return s != nil && s.Sunk()
}

// AllSunk reports whether every ship on the board is sunk.
func (b *Board) AllSunk() bool {
	if len(b.ships) == 0 || len(b.pending) > 0 {
		return false
	}
	for _, s := range b.ships {
		if !s.Sunk() {
			return false
		}
	}
	return true
}

// Afloat is how many ships still float.
func (b *Board) Afloat() int {
	n := 0
	for _, s := range b.ships {
		if !s.Sunk() {
			n++
		}
	}
	return n
}

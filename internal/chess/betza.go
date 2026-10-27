package chess

import (
	"fmt"
	"math"
	"slices"
	"strconv"
)

// Pieces move by a subset of Betza's funny notation
// (https://www.chessvariants.com/piececlopedia.dir/betzanotation.html).
//
// Atoms (leaps, as file/rank offsets): W (1,0) F (1,1) D (2,0) N (2,1) A (2,2)
// H (3,0) C/L (3,1) Z/J (3,2) G (3,3). Compounds: K = WF, R = WW, B = FF, Q = RB.
// A doubled atom rides (NN is the nightrider); a trailing number limits the
// ride (R4 moves up to four squares).
//
// Prefixes apply to the atom that follows:
//
//	m c     move only / capture only
//	f b l r directions as the owner sees them; s = l or r, v = f or b. A
//	        vertical letter followed by a horizontal one picks a quadrant on
//	        diagonal and oblique atoms (frF is the forward-right diagonal);
//	        otherwise letters add up (fsW is forward or sideways)
//	i       only while the piece has never moved
//	n       lame: the leap is blocked by pieces on its way (nN is the xiangqi
//	        horse, stepping orthogonally first)
//	p g     hops over one piece: p goes on to any square beyond it (cannon),
//	        g lands just behind it (grasshopper)
//	q       circular rider: chains leaps that turn by one step around the
//	        atom's circle each time (qN is the rose)

type hop uint8

const (
	hopNone hop = iota
	hopCannon
	hopGrasshopper
)

// Component is one atom with its modifiers, as White moves it.
type Component struct {
	Vectors  [][2]int // allowed leaps; for a circular rider, the allowed first legs
	Ring     [][2]int // circular riders: every leap of the atom in counterclockwise order
	Range    int      // 1 leaps once, 0 rides without limit, n rides up to n steps
	Move     bool     // may go to an empty square
	Capture  bool     // may take an enemy piece
	Initial  bool
	Lame     bool
	Hop      hop
	Circular bool
}

var atoms = map[byte][2]int{
	'W': {1, 0}, 'F': {1, 1}, 'D': {2, 0}, 'N': {2, 1}, 'A': {2, 2},
	'H': {3, 0}, 'C': {3, 1}, 'L': {3, 1}, 'Z': {3, 2}, 'J': {3, 2}, 'G': {3, 3},
}

var compounds = map[byte]struct {
	atoms string
	rider bool
}{
	'K': {"WF", false},
	'R': {"W", true},
	'B': {"F", true},
	'Q': {"WF", true},
}

type modifiers struct {
	move, capture, initial, lame, circular bool
	hop                                    hop
	dirs                                   string
}

// ParseBetza turns a move description such as "fmWfcFifmW2" into components.
func ParseBetza(s string) ([]Component, error) {
	if s == "" {
		return nil, fmt.Errorf("empty move")
	}
	var comps []Component
	for i := 0; i < len(s); {
		mods, next, err := parseModifiers(s, i)
		if err != nil {
			return nil, err
		}
		if next >= len(s) {
			return nil, fmt.Errorf("%q: modifiers without a piece", s)
		}
		letter := s[next]
		i = next + 1
		rider := false
		bases := string(letter)
		if c, ok := compounds[letter]; ok {
			bases, rider = c.atoms, c.rider
		} else if _, ok := atoms[letter]; !ok {
			return nil, fmt.Errorf("%q: unknown atom %c", s, letter)
		}
		rng := 1
		if rider {
			rng = 0
		}
		switch {
		case i < len(s) && s[i] == letter:
			rng = 0
			i++
		case i < len(s) && isDigit(s[i]):
			j := i
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			n, _ := strconv.Atoi(s[i:j])
			if n < 1 {
				return nil, fmt.Errorf("%q: range must be at least 1", s)
			}
			rng, i = n, j
		}
		for k := range len(bases) {
			comp, err := newComponent(atoms[bases[k]], rng, mods)
			if err != nil {
				return nil, fmt.Errorf("%q: %v", s, err)
			}
			comps = append(comps, comp)
		}
	}
	return comps, nil
}

func parseModifiers(s string, i int) (modifiers, int, error) {
	var m modifiers
	seen := map[byte]bool{}
	for ; i < len(s) && s[i] >= 'a' && s[i] <= 'z'; i++ {
		c := s[i]
		if c != 'f' && c != 'b' && c != 'l' && c != 'r' && seen[c] {
			return m, i, fmt.Errorf("%q: repeated modifier %c", s, c)
		}
		seen[c] = true
		switch c {
		case 'm':
			m.move = true
		case 'c':
			m.capture = true
		case 'i':
			m.initial = true
		case 'n':
			m.lame = true
		case 'q':
			m.circular = true
		case 'p':
			m.hop = hopCannon
		case 'g':
			m.hop = hopGrasshopper
		case 'f', 'b', 'l', 'r', 's', 'v':
			m.dirs += string(c)
		default:
			return m, i, fmt.Errorf("%q: unknown modifier %c", s, c)
		}
	}
	if !m.move && !m.capture {
		m.move, m.capture = true, true
	}
	return m, i, nil
}

func newComponent(atom [2]int, rng int, m modifiers) (Component, error) {
	ring := symmetries(atom)
	allowed := filterDirections(ring, atom, m.dirs)
	if len(allowed) == 0 {
		return Component{}, fmt.Errorf("directions %q leave no moves", m.dirs)
	}
	c := Component{
		Vectors: allowed, Range: rng,
		Move: m.move, Capture: m.capture, Initial: m.initial,
		Lame: m.lame, Hop: m.hop, Circular: m.circular,
	}
	if m.circular {
		c.Ring = ring
		c.Range = 0
	}
	return c, nil
}

// symmetries lists the distinct rotations and reflections of a leap in
// counterclockwise order, starting from the positive file direction.
func symmetries(atom [2]int) [][2]int {
	x, y := atom[0], atom[1]
	var vs [][2]int
	for _, v := range [][2]int{{x, y}, {y, x}, {-y, x}, {-x, y}, {-x, -y}, {-y, -x}, {y, -x}, {x, -y}} {
		if !slices.Contains(vs, v) {
			vs = append(vs, v)
		}
	}
	slices.SortFunc(vs, func(a, b [2]int) int {
		return cmpFloat(angle(a), angle(b))
	})
	return vs
}

func angle(v [2]int) float64 {
	a := math.Atan2(float64(v[1]), float64(v[0]))
	if a < 0 {
		a += 2 * math.Pi
	}
	return a
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func filterDirections(vs [][2]int, atom [2]int, dirs string) [][2]int {
	if dirs == "" {
		return vs
	}
	quadrants := atom[0] != 0 && atom[1] != 0
	var groups []func([2]int) bool
	for i := 0; i < len(dirs); i++ {
		cond := direction(dirs[i])
		if quadrants && isVertical(dirs[i]) && i+1 < len(dirs) && !isVertical(dirs[i+1]) {
			vert, horiz := cond, direction(dirs[i+1])
			cond = func(v [2]int) bool { return vert(v) && horiz(v) }
			i++
		}
		groups = append(groups, cond)
	}
	var out [][2]int
	for _, v := range vs {
		for _, g := range groups {
			if g(v) {
				out = append(out, v)
				break
			}
		}
	}
	return out
}

func isVertical(c byte) bool { return c == 'f' || c == 'b' || c == 'v' }

func direction(c byte) func([2]int) bool {
	switch c {
	case 'f':
		return func(v [2]int) bool { return v[1] > 0 }
	case 'b':
		return func(v [2]int) bool { return v[1] < 0 }
	case 'v':
		return func(v [2]int) bool { return v[1] != 0 }
	case 'l':
		return func(v [2]int) bool { return v[0] < 0 }
	case 'r':
		return func(v [2]int) bool { return v[0] > 0 }
	}
	return func(v [2]int) bool { return v[0] != 0 }
}

// lamePath lists the squares a lame leap crosses before landing: orthogonal
// steps along its longer axis first, then diagonal ones.
func lamePath(v [2]int) [][2]int {
	ax, ay := abs(v[0]), abs(v[1])
	sx, sy := sign(v[0]), sign(v[1])
	var path [][2]int
	cur := [2]int{}
	steps := max(ax, ay)
	for k := range steps - 1 {
		switch {
		case ax >= ay && k < ax-ay:
			cur[0] += sx
		case ay > ax && k < ay-ax:
			cur[1] += sy
		default:
			cur[0] += sx
			cur[1] += sy
		}
		path = append(path, cur)
	}
	return path
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

package chess

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
)

type shuffleDef struct {
	Rank          int    `yaml:"rank"`
	OppositeColor string `yaml:"opposite_color"`
	Between       struct {
		Royal  string `yaml:"royal"`
		Pieces string `yaml:"pieces"`
	} `yaml:"between"`
}
type shuffleSetup struct {
	rank         int
	arrangements [][]uint8
}
type shuffleChoice struct {
	board []uint8
	order []int
}

// Limit setup enumeration to bound both load time and memory for custom variants.
const maxShuffleArrangements = 1000000

func (v *Variant) buildShuffle(d *shuffleDef) error {
	if d.Rank < 1 || d.Rank > v.Ranks || d.Rank-1 == v.Ranks-d.Rank {
		return fmt.Errorf("rank must have a distinct mirrored rank")
	}
	if v.Files > 10 {
		return fmt.Errorf("shuffled ranks are limited to 10 files")
	}
	piece := func(s string) uint8 {
		if len(s) != 1 {
			return 0
		}
		return v.TypeOf(s[0])
	}
	opposite, royal, pair := piece(d.OppositeColor), piece(d.Between.Royal), piece(d.Between.Pieces)
	if opposite == 0 || royal == 0 || pair == 0 || opposite == royal || opposite == pair || royal == pair || !v.Pieces[royal-1].Royal {
		return fmt.Errorf("invalid constraint pieces")
	}
	y := d.Rank - 1
	counts := map[uint8]int{}
	for x := 0; x < v.Files; x++ {
		a, b := v.start[v.Square(x, y)], v.start[v.Square(x, v.Ranks-1-y)]
		if a.Type == 0 || a.Color != White || b.Type != a.Type || b.Color != Black {
			return fmt.Errorf("rank must be full and mirrored by black")
		}
		counts[a.Type]++
	}
	if counts[opposite] != 2 || counts[royal] != 1 || counts[pair] != 2 {
		return fmt.Errorf("constraints require two opposite-color pieces, one royal and two surrounding pieces")
	}
	type group struct {
		typ   uint8
		count int
	}
	var groups []group
	for typ, n := range counts {
		if typ != opposite && typ != royal && typ != pair {
			groups = append(groups, group{typ, n})
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count < groups[j].count
		}
		return groups[i].typ < groups[j].typ
	})
	board := make([]uint8, v.Files)
	var results []shuffleChoice
	overflow := false
	var place func(int, []int)
	place = func(g int, order []int) {
		if overflow {
			return
		}
		if g == len(groups) {
			free := []int{}
			for x, t := range board {
				if t == 0 {
					free = append(free, x)
				}
			}
			if len(free) != 3 {
				return
			}
			board[free[0]], board[free[1]], board[free[2]] = pair, royal, pair
			results = append(results, shuffleChoice{slices.Clone(board), slices.Clone(order)})
			if len(results) > maxShuffleArrangements {
				overflow = true
			}
			for _, x := range free {
				board[x] = 0
			}
			return
		}
		group := groups[g]
		option := 0
		var choose func(int, int)
		choose = func(start, left int) {
			if left == 0 {
				place(g+1, append(order, option))
				option++
				return
			}
			for x := start; x < v.Files; x++ {
				if board[x] == 0 {
					board[x] = group.typ
					choose(x+1, left-1)
					board[x] = 0
				}
			}
		}
		choose(0, group.count)
	}
	// Odd-file bishop is the least significant choice in Scharnagl ordering.
	for odd := 1; odd < v.Files; odd += 2 {
		for even := 0; even < v.Files; even += 2 {
			board[odd], board[even] = opposite, opposite
			place(0, []int{odd / 2, even / 2})
			board[odd], board[even] = 0, 0
		}
	}
	if overflow {
		return fmt.Errorf("more than %d arrangements", maxShuffleArrangements)
	}
	if len(results) == 0 {
		return fmt.Errorf("constraints have no arrangements")
	}
	sort.Slice(results, func(i, j int) bool {
		a, b := results[i].order, results[j].order
		for k := len(a) - 1; k >= 0; k-- {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return false
	})
	s := &shuffleSetup{rank: y}
	for _, r := range results {
		s.arrangements = append(s.arrangements, r.board)
	}
	v.shuffle = s
	return nil
}
func (v *Variant) shuffleCount() int {
	if v.shuffle == nil {
		return 1
	}
	return len(v.shuffle.arrangements)
}
func (v *Variant) shuffleAt(index int) []Piece {
	if index < 0 || index >= v.shuffleCount() {
		return nil
	}
	b := slices.Clone(v.start)
	if v.shuffle == nil {
		return b
	}
	for x, t := range v.shuffle.arrangements[index] {
		b[v.Square(x, v.shuffle.rank)] = Piece{Type: t, Color: White}
		b[v.Square(x, v.Ranks-1-v.shuffle.rank)] = Piece{Type: t, Color: Black}
	}
	return b
}
func (v *Variant) shuffledStart(rng interface{ IntN(int) int }) []Piece {
	if v.shuffle == nil {
		return v.shuffleAt(0)
	}
	n := 0
	if rng == nil {
		n = rand.IntN(v.shuffleCount())
	} else {
		n = rng.IntN(v.shuffleCount())
	}
	return v.shuffleAt(n)
}

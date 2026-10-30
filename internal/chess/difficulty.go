package chess

import (
	"fmt"
	"math/rand/v2"
	"time"
)

// Level selects a search budget and root move-selection policy. Its zero value
// is the original, three-second opponent.
type Level uint8

const (
	Normal Level = iota
	Beginner
	Easy
	Hard
	Expert
)

const LevelNames = "beginner, easy, normal, hard, expert"

func ParseLevel(name string) (Level, error) {
	for _, level := range []Level{Beginner, Easy, Normal, Hard, Expert} {
		if level.String() == name {
			return level, nil
		}
	}
	return Normal, fmt.Errorf("--level must be %s, not %q", LevelNames, name)
}

func (l Level) String() string {
	switch l {
	case Beginner:
		return "beginner"
	case Easy:
		return "easy"
	case Hard:
		return "hard"
	case Expert:
		return "expert"
	default:
		return "normal"
	}
}

func (l Level) next() Level {
	switch l {
	case Beginner:
		return Easy
	case Easy:
		return Normal
	case Normal:
		return Hard
	case Hard:
		return Expert
	default:
		return Beginner
	}
}

type selection struct{ chance, loss int }

func (l Level) profile() (Limits, selection) {
	switch l {
	case Beginner:
		return Limits{Time: time.Second, Depth: 1, Nodes: 4000}, selection{35, 200}
	case Easy:
		return Limits{Time: time.Second, Depth: 2, Nodes: 20000}, selection{15, 75}
	case Hard:
		return Limits{Time: 6 * time.Second}, selection{}
	case Expert:
		return Limits{Time: 12 * time.Second}, selection{}
	default:
		return Limits{Time: DefaultThinkTime}, selection{}
	}
}

// Limits returns a copy of the profile's limits. A positive override changes
// only the wall-clock ceiling, never the depth, node cap or selection policy.
func (l Level) Limits(override time.Duration) Limits {
	lim, _ := l.profile()
	if override > 0 {
		lim.Time = override
	}
	return lim
}

// ThinkLevel applies the profile caps as well as the caller's limits. Weak
// searches use fresh tables and history, so neither a prior expert search nor
// the order in which independent samples run changes their strength.
func (e *Engine) ThinkLevel(root *Position, past []uint64, level Level, lim Limits, seed uint64) Result {
	profile, policy := level.profile()
	if lim.Time <= 0 {
		lim.Time = profile.Time
	}
	if profile.Depth > 0 && (lim.Depth <= 0 || lim.Depth > profile.Depth) {
		lim.Depth = profile.Depth
	}
	if profile.Nodes > 0 && (lim.Nodes <= 0 || lim.Nodes > profile.Nodes) {
		lim.Nodes = profile.Nodes
	}
	if policy.chance == 0 {
		return e.Think(root, past, lim)
	}
	return NewEngine(e.v).think(root, past, lim, policy, seed)
}

type candidate struct {
	Move  Move
	Score int
}

func chooseCandidate(candidates []candidate, policy selection, seed uint64) candidate {
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.Score > best.Score {
			best = c
		}
	}
	// Do not trade a known mate for centipawns, or compare mate distances to cp.
	if IsMateScore(best.Score) {
		return best
	}
	var alternatives []candidate
	for _, c := range candidates {
		if c.Move != best.Move && !IsMateScore(c.Score) && c.Score >= best.Score-policy.loss {
			alternatives = append(alternatives, c)
		}
	}
	rng := rand.New(rand.NewPCG(seed, 0x6d697361))
	if len(alternatives) > 0 && rng.IntN(100) < policy.chance {
		return alternatives[rng.IntN(len(alternatives))]
	}
	return best
}

// scoreRoot gives every candidate a full window at the same depth; PVS bounds
// from the ordinary root search are not suitable for ranking alternatives.
func (e *Engine) scoreRoot(root *Position, legal []Move, depth int) []candidate {
	e.path = append(e.path, root.Hash())
	defer func() { e.path = e.path[:len(e.path)-1] }()
	candidates := make([]candidate, 0, len(legal))
	for _, mv := range legal {
		if e.expired() {
			return nil
		}
		score := -e.negamax(root.Apply(mv), depth-1, 1, -infinity, infinity, true)
		if e.aborted {
			return nil
		}
		candidates = append(candidates, candidate{mv, score})
	}
	return candidates
}

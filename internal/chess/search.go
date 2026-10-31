package chess

import (
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const (
	mateScore = 1_000_000
	infinity  = 2_000_000
	maxPly    = 96
	ttSize    = 1 << 17
)

// IsMateScore reports whether a search score announces a forced mate.
func IsMateScore(score int) bool { return score > mateScore-maxPly || score < -mateScore+maxPly }

// MateIn turns a mate score into moves until mate: positive when the side to
// move mates, negative when it is mated.
func MateIn(score int) int {
	if score > 0 {
		return (mateScore - score + 1) / 2
	}
	return -(mateScore + score) / 2
}

// Limits bound a search; zero fields mean no limit, except that a search with
// neither Time nor Depth stops at maxPly/2 iterations. Nodes counts entries to
// both negamax and quiesce, without overshoot. Cancellation is checked at each
// entry; time every 64 entries and at root moves. Move generation is not
// interruptible, so wall-clock deadlines can overrun between those checks.
type Limits struct {
	Time  time.Duration
	Depth int
	Nodes int
	Stop  *atomic.Bool
}

// Result is the move a search settled on. Score is in centipawns for the side
// to move, and describes Move, not an unselected candidate. Depth zero means
// no iteration completed and Score is unknown. Found is false without moves.
type Result struct {
	Move  Move
	Score int
	Depth int
	Nodes int
	Found bool
}

type ttFlag uint8

const (
	ttExact ttFlag = iota + 1
	ttLower
	ttUpper
)

type ttEntry struct {
	key   uint64
	move  Move
	score int32
	depth int8
	flag  ttFlag
}

// Engine searches positions of one variant. It keeps its table between
// searches; one search runs at a time.
type Engine struct {
	mu      sync.Mutex
	v       *Variant
	eval    *evaluator
	tt      []ttEntry
	killers [maxPly][2]Move
	history []int // by destination square and piece type

	path      []uint64 // hashes of the game so far and the line being searched
	nodes     int
	nodeLimit int
	deadline  time.Time
	stop      *atomic.Bool
	aborted   bool
}

func NewEngine(v *Variant) *Engine {
	return &Engine{
		v:       v,
		eval:    newEvaluator(v),
		tt:      make([]ttEntry, ttSize),
		history: make([]int, v.Size()*(len(v.Pieces)+1)),
	}
}

// Think searches root with iterative deepening and returns the best move of
// the deepest finished iteration. past holds the hashes of the positions
// before root, so lines that repeat them count as draws.
func (e *Engine) Think(root *Position, past []uint64, lim Limits) Result {
	return e.think(root, past, lim, selection{}, 0)
}

func (e *Engine) think(root *Position, past []uint64, lim Limits, policy selection, seed uint64) Result {
	start := time.Now()
	legal := root.Moves()
	if len(legal) == 0 {
		return Result{}
	}
	best := Result{Move: legal[0], Found: true}
	// Obsolete queued work must not wait for a newer search's table lock.
	cancelled := func() bool {
		return (lim.Stop != nil && lim.Stop.Load()) || (lim.Time > 0 && time.Since(start) >= lim.Time)
	}
	for {
		if cancelled() {
			return best
		}
		if e.mu.TryLock() {
			break
		}
		time.Sleep(time.Millisecond)
	}
	defer e.mu.Unlock()
	if cancelled() {
		return best
	}
	e.begin(past, lim)
	if lim.Time > 0 {
		e.deadline = start.Add(lim.Time)
	}
	maxDepth := lim.Depth
	if maxDepth <= 0 {
		maxDepth = maxPly / 2
	}
	var candidates []candidate
	for depth := 1; depth <= maxDepth; depth++ {
		if e.expired() {
			break
		}
		score, move, complete := e.searchRoot(root, legal, depth, best.Move)
		if !complete {
			break
		}
		best.Move = move
		best.Score, best.Depth = score, depth
		candidates = nil
		if policy.chance > 0 && !IsMateScore(score) && len(legal) > 1 {
			candidates = e.scoreRoot(root, legal, depth)
			if e.aborted {
				break
			}
		}
		if len(legal) == 1 || IsMateScore(score) {
			break
		}
		if lim.Time > 0 && time.Since(start) > lim.Time/2 {
			break
		}
	}
	if len(candidates) > 1 {
		chosen := chooseCandidate(candidates, policy, seed)
		best.Move, best.Score = chosen.Move, chosen.Score
	}
	best.Nodes = e.nodes
	return best
}

func (e *Engine) begin(past []uint64, lim Limits) {
	e.path = append(e.path[:0], past...)
	e.nodes, e.aborted, e.stop = 0, false, lim.Stop
	e.nodeLimit = lim.Nodes
	e.deadline = time.Time{}
	if lim.Time > 0 {
		e.deadline = time.Now().Add(lim.Time)
	}
	e.killers = [maxPly][2]Move{}
	for i := range e.history {
		e.history[i] /= 8
	}
}

// searchRoot searches every legal move, the last iteration's best first. Its
// score and move are published only when the whole iteration completes.
func (e *Engine) searchRoot(root *Position, legal []Move, depth int, prev Move) (int, Move, bool) {
	moves := e.order(root, legal, prev, 0)
	alpha, beta := -infinity, infinity
	bestMove := prev
	e.path = append(e.path, root.Hash())
	defer func() { e.path = e.path[:len(e.path)-1] }()
	for i, mv := range moves {
		if e.expired() {
			return alpha, bestMove, false
		}
		child := root.Apply(mv)
		var score int
		if i == 0 {
			score = -e.negamax(child, depth-1, 1, -beta, -alpha, true)
		} else {
			score = -e.negamax(child, depth-1, 1, -alpha-1, -alpha, true)
			if score > alpha && !e.aborted {
				score = -e.negamax(child, depth-1, 1, -beta, -alpha, true)
			}
		}
		if e.aborted {
			return alpha, bestMove, false
		}
		if score > alpha {
			alpha, bestMove = score, mv
		}
	}
	e.store(root.Hash(), depth, alpha, ttExact, bestMove, 0)
	return alpha, bestMove, true
}

func (e *Engine) negamax(p *Position, depth, ply, alpha, beta int, allowNull bool) int {
	if e.tick() {
		return 0
	}
	hash := p.Hash()
	if e.repeated(hash, p.Clock) || p.Clock >= 2*e.v.MoveLimit {
		return 0
	}
	if ply >= maxPly-1 {
		return e.eval.evaluate(p)
	}
	inCheck := p.InCheck()
	if inCheck {
		depth++
	}
	if depth <= 0 {
		return e.quiesce(p, ply, alpha, beta)
	}

	ttMove := Move{Castle: -2}
	if t := &e.tt[hash%ttSize]; t.key == hash {
		ttMove = t.move
		if int(t.depth) >= depth {
			score := fromTT(int(t.score), ply)
			switch {
			case t.flag == ttExact,
				t.flag == ttLower && score >= beta,
				t.flag == ttUpper && score <= alpha:
				return score
			}
		}
	}

	if allowNull && !inCheck && depth >= 3 && beta < mateScore-maxPly && e.hasPieces(p) {
		r := 2
		if depth >= 6 {
			r = 3
		}
		score := -e.negamax(p.null(), depth-1-r, ply+1, -beta, -beta+1, false)
		if e.aborted {
			return 0
		}
		if score >= beta {
			return beta
		}
	}

	e.path = append(e.path, hash)
	defer func() { e.path = e.path[:len(e.path)-1] }()

	origAlpha := alpha
	best, bestMove := -infinity, Move{Castle: -2}
	legal := 0
	for _, mv := range e.order(p, p.pseudoMoves(), ttMove, ply) {
		child := p.Apply(mv)
		if child.royalAttacked(p.Turn) {
			continue
		}
		legal++
		quiet := mv.Take < 0 && mv.Promote == 0
		var score int
		if legal == 1 {
			score = -e.negamax(child, depth-1, ply+1, -beta, -alpha, true)
		} else {
			r := 0
			if depth >= 3 && legal > 3 && quiet && !inCheck && !e.isKiller(mv, ply) && !child.InCheck() {
				r = 1
				if depth >= 6 && legal > 8 {
					r = 2
				}
			}
			score = -e.negamax(child, depth-1-r, ply+1, -alpha-1, -alpha, true)
			if score > alpha && r > 0 && !e.aborted {
				score = -e.negamax(child, depth-1, ply+1, -alpha-1, -alpha, true)
			}
			if score > alpha && score < beta && !e.aborted {
				score = -e.negamax(child, depth-1, ply+1, -beta, -alpha, true)
			}
		}
		if e.aborted {
			return 0
		}
		if score > best {
			best, bestMove = score, mv
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			if quiet {
				e.rememberCutoff(p, mv, depth, ply)
			}
			break
		}
	}
	if legal == 0 {
		if inCheck {
			return -mateScore + ply
		}
		switch e.v.Stalemate {
		case StalemateWin: // the stalemated side to move has lost
			return -mateScore + ply
		case StalemateLoss:
			return mateScore - ply
		}
		return 0
	}
	flag := ttExact
	switch {
	case best <= origAlpha:
		flag = ttUpper
	case best >= beta:
		flag = ttLower
	}
	e.store(hash, depth, best, flag, bestMove, ply)
	return best
}

// quiesce searches captures and promotions until the position is quiet, so
// the evaluation never stands in the middle of an exchange. In check every
// evasion is searched instead.
func (e *Engine) quiesce(p *Position, ply, alpha, beta int) int {
	if e.tick() {
		return 0
	}
	if ply >= maxPly-1 {
		return e.eval.evaluate(p)
	}
	inCheck := p.InCheck()
	stand := -infinity
	if !inCheck {
		stand = e.eval.evaluate(p)
		if stand >= beta {
			return stand
		}
		alpha = max(alpha, stand)
	}
	var moves []Move
	if inCheck {
		moves = p.pseudoMoves()
	} else {
		moves = p.captures()
	}
	legal := 0
	for _, mv := range e.order(p, moves, Move{Castle: -2}, ply) {
		if !inCheck && mv.Promote == 0 && stand+e.victimValue(p, mv)+200 < alpha {
			continue
		}
		child := p.Apply(mv)
		if child.royalAttacked(p.Turn) {
			continue
		}
		legal++
		score := -e.quiesce(child, ply+1, -beta, -alpha)
		if e.aborted {
			return 0
		}
		if score >= beta {
			return score
		}
		alpha = max(alpha, score)
	}
	if inCheck && legal == 0 {
		return -mateScore + ply
	}
	return alpha
}

// tick counts a node and reports whether the search must stop.
func (e *Engine) tick() bool {
	if e.aborted || (e.stop != nil && e.stop.Load()) || (e.nodeLimit > 0 && e.nodes >= e.nodeLimit) {
		e.aborted = true
		return true
	}
	if e.nodes&63 == 0 && e.expired() {
		return true
	}
	e.nodes++
	return e.aborted
}

func (e *Engine) expired() bool {
	if (e.stop != nil && e.stop.Load()) || (!e.deadline.IsZero() && time.Now().After(e.deadline)) {
		e.aborted = true
	}
	return e.aborted
}

// repeated reports whether the position already occurred since the last
// capture or pawn move, in the game or the line searched.
func (e *Engine) repeated(hash uint64, clock int) bool {
	for i := len(e.path) - 1; i >= 0 && i >= len(e.path)-clock; i-- {
		if e.path[i] == hash {
			return true
		}
	}
	return false
}

func (e *Engine) hasPieces(p *Position) bool {
	for _, pc := range p.Board {
		if pc.Type != 0 && pc.Color == p.Turn {
			if t := e.v.Pieces[pc.Type-1]; !t.Royal && !t.Pawn {
				return true
			}
		}
	}
	return false
}

func (p *Position) null() *Position {
	q := p.clone()
	q.Turn = p.Turn.Other()
	q.Clock++
	return q
}

// order sorts moves: the table's move, then captures by most valuable victim
// and least valuable attacker, promotions, killer moves, then history.
func (e *Engine) order(p *Position, moves []Move, first Move, ply int) []Move {
	type scored struct {
		mv    Move
		score int
	}
	list := make([]scored, len(moves))
	for i, mv := range moves {
		s := 0
		switch {
		case mv == first:
			s = 1 << 30
		case mv.Take >= 0 || mv.Promote != 0:
			s = 1<<24 + 16*e.victimValue(p, mv) - e.v.Pieces[p.Board[mv.From].Type-1].Value/16
			if mv.Promote != 0 {
				s += e.v.Pieces[mv.Promote-1].Value
			}
		case ply < maxPly && mv == e.killers[ply][0]:
			s = 1<<22 + 1
		case ply < maxPly && mv == e.killers[ply][1]:
			s = 1 << 22
		default:
			s = e.history[e.historyIndex(p, mv)]
		}
		list[i] = scored{mv, s}
	}
	slices.SortStableFunc(list, func(a, b scored) int { return b.score - a.score })
	for i := range list {
		moves[i] = list[i].mv
	}
	return moves
}

func (e *Engine) victimValue(p *Position, mv Move) int {
	if mv.Take < 0 {
		return 0
	}
	value := e.v.Pieces[p.Board[mv.Take].Type-1].Value
	if p.Board[mv.Take].Color == p.Turn { // a self-capture loses material
		return -value
	}
	return value
}

func (e *Engine) historyIndex(p *Position, mv Move) int {
	return mv.To*(len(e.v.Pieces)+1) + int(p.Board[mv.From].Type)
}

func (e *Engine) isKiller(mv Move, ply int) bool {
	return ply < maxPly && (mv == e.killers[ply][0] || mv == e.killers[ply][1])
}

func (e *Engine) rememberCutoff(p *Position, mv Move, depth, ply int) {
	if ply < maxPly && e.killers[ply][0] != mv {
		e.killers[ply][1], e.killers[ply][0] = e.killers[ply][0], mv
	}
	i := e.historyIndex(p, mv)
	e.history[i] = min(e.history[i]+depth*depth, 1<<20)
}

func (e *Engine) store(hash uint64, depth, score int, flag ttFlag, mv Move, ply int) {
	t := &e.tt[hash%ttSize]
	if t.key == hash && int(t.depth) > depth && flag != ttExact {
		return
	}
	*t = ttEntry{key: hash, move: mv, score: int32(toTT(score, ply)), depth: int8(min(depth, 127)), flag: flag}
}

// Mate scores count from the root; the table keeps them counted from the
// stored node so they stay right when reached by another path.
func toTT(score, ply int) int {
	switch {
	case score > mateScore-maxPly:
		return score + ply
	case score < -mateScore+maxPly:
		return score - ply
	}
	return score
}

func fromTT(score, ply int) int {
	switch {
	case score > mateScore-maxPly:
		return score - ply
	case score < -mateScore+maxPly:
		return score + ply
	}
	return score
}

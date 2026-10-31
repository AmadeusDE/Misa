package chess

import "slices"

// outcome is a game the variant's rules have already ended, and its winner.
// drawn marks the endings the rules leave without a winner: a stalemate under
// the draw rule and a goal arrival the other side answers on a goal square.
// Every other outcome is decisive.
type outcome struct {
	status Status
	winner Color // the winner; meaningless when drawn
	drawn  bool
}

// adjudicate reports the result the rules fix for p without looking at its
// history. A side that has just lost every piece of a type its variant lists
// under extinction loses first (and if both are extinct, the side that just
// moved wins), then a royal that just moved onto a goal square wins, and so
// does a side that has just given the checks its variant's limit asks for, even
// when the side to move is also checkmated or a draw rule applies. Without
// those, a side with no legal move is checkmated when in check and otherwise
// stalemated, and the variant's stalemate rule names the stalemate's winner:
// the side that delivered it under StalemateWin, the stalemated side under
// StalemateLoss, and nobody under StalemateDraw. hasMoves says whether the
// side to move has a legal move; callers that have not generated them pass
// true and adjudicate only the immediate wins, an extinction, a goal arrival
// or a check limit.
func adjudicate(p *Position, hasMoves bool) (outcome, bool) {
	if p.extinct(p.Turn) {
		return outcome{status: ExtinctionWin, winner: p.Turn.Other()}, true
	}
	if p.extinct(p.Turn.Other()) {
		return outcome{status: ExtinctionWin, winner: p.Turn}, true
	}
	if o, over := p.goalOutcome(); over {
		return o, true
	}
	if p.checkWinner() {
		return outcome{status: CheckWin, winner: p.Turn.Other()}, true
	}
	if hasMoves {
		return outcome{}, false
	}
	if p.InCheck() {
		return outcome{status: Checkmate, winner: p.Turn.Other()}, true
	}
	switch p.V.Stalemate {
	case StalemateWin: // the side to move was stalemated and so has lost
		return outcome{status: Stalemate, winner: p.Turn.Other()}, true
	case StalemateLoss: // the side to move stalemated the opponent and so has won
		return outcome{status: Stalemate, winner: p.Turn}, true
	}
	return outcome{status: Stalemate, drawn: true}, true
}

// score turns an outcome into a search score from the side to move's point of
// view, counted from the root. Extinctions, goal arrivals, check limits and
// checkmates are mates, and so is a stalemate under a winning or a losing
// stalemate rule; a drawn ending, a drawn stalemate, a goal arrival the other
// side answers on a goal square or a drawn stalemate scores 0. A decisive
// ending scores as if mate came on this ply, so the root prefers to win sooner
// and lose later.
func (o outcome) score(turn Color, ply int) int {
	switch o.status {
	case ExtinctionWin, GoalWin, CheckWin, Checkmate, Stalemate, DrawGoalReply:
		if o.drawn {
			return 0
		}
		if o.winner == turn {
			return mateScore - ply
		}
		return -mateScore + ply
	}
	return 0
}

// terminalScore scores the ending p has reached from the side to move's point
// of view, for the search's terminal nodes: adjudicate names the winner, and a
// drawn ending scores 0. Callers use it once they know the side to move has no
// legal move, so the search scores every ending the rules fix, not just the
// checkmate its move loop runs into.
func terminalScore(p *Position, ply int) int {
	o, _ := adjudicate(p, false)
	return o.score(p.Turn, ply)
}

// checkWinner reports whether the side that just moved has given as many
// checks as its variant asks for. Every move that leaves the opponent in check
// counts, including the one that delivers the limit-th check.
func (p *Position) checkWinner() bool {
	limit := p.V.CheckLimit
	return limit > 0 && p.Checks[p.Turn.Other()] >= limit
}

// goalOutcome reports what the royals standing on the variant's goal squares
// have settled. Goal squares are absolute and shared by both colours, and a
// royal stands there only because it arrived on the previous move.
//
// With GoalReplyDraw, an arrival by the side that moves first is not yet a win
// while the other side can arrive in reply: answering on a goal square draws,
// and the arrival wins once the other side has no arrival left to play.
func (p *Position) goalOutcome() (outcome, bool) {
	if len(p.V.Goal) == 0 {
		return outcome{}, false
	}
	var arrived [2]bool
	for _, sq := range p.V.Goal {
		if pc := p.Board[sq]; pc.Type != 0 && p.V.Type(pc).Royal {
			arrived[pc.Color] = true
		}
	}
	justMoved := p.Turn.Other()
	switch {
	case p.V.GoalReplyDraw && arrived[White] && arrived[Black]:
		return outcome{status: DrawGoalReply, drawn: true}, true
	case arrived[justMoved]:
		if p.V.GoalReplyDraw && justMoved == White && p.canReachGoal() {
			return outcome{}, false
		}
		return outcome{status: GoalWin, winner: justMoved}, true
	case p.V.GoalReplyDraw && arrived[p.Turn]:
		// The first side's arrival went unanswered.
		return outcome{status: GoalWin, winner: p.Turn}, true
	}
	return outcome{}, false
}

// canReachGoal reports whether the side to move has a legal move that puts one
// of its royals on a goal square.
func (p *Position) canReachGoal() bool {
	for _, m := range p.Moves() {
		if p.V.Type(p.Board[m.From]).Royal && slices.Contains(p.V.Goal, m.To) {
			return true
		}
	}
	return false
}

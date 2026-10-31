package chess

// outcome is a game the variant's rules have already ended, and its winner.
// drawn marks the one ending the rules leave without a winner, a stalemate
// under the draw rule; every other outcome is decisive.
type outcome struct {
	status Status
	winner Color // the winner; meaningless when drawn
	drawn  bool
}

// adjudicate reports the result the rules fix for p without looking at its
// history. A royal that just moved onto a goal square wins, and so does a side
// that has just given the checks its variant's limit asks for, even when the
// side to move is also checkmated or a draw rule applies. Without those, a
// side with no legal move is checkmated when in check and otherwise
// stalemated, and the variant's stalemate rule names the stalemate's winner:
// the side that delivered it under StalemateWin, the stalemated side under
// StalemateLoss, and nobody under StalemateDraw. hasMoves says whether the
// side to move has a legal move; callers that have not generated them pass
// true and adjudicate only the immediate wins, a goal arrival or a check
// limit.
func adjudicate(p *Position, hasMoves bool) (outcome, bool) {
	if p.goalWinner() {
		return outcome{status: GoalWin, winner: p.Turn.Other()}, true
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
// view, counted from the root. Goal arrivals, check limits and checkmates are
// mates, and so is a stalemate under a winning or a losing stalemate rule; a
// drawn stalemate scores 0. A decisive ending scores as if mate came on this
// ply, so the root prefers to win sooner and lose later.
func (o outcome) score(turn Color, ply int) int {
	switch o.status {
	case GoalWin, CheckWin, Checkmate, Stalemate:
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

// checkWinner reports whether the side that just moved has given as many
// checks as its variant asks for. Every move that leaves the opponent in check
// counts, including the one that delivers the limit-th check.
func (p *Position) checkWinner() bool {
	limit := p.V.CheckLimit
	return limit > 0 && p.Checks[p.Turn.Other()] >= limit
}

// goalWinner reports whether the side that just moved has a royal piece on a
// goal square. Goal squares are absolute and shared by both colours. A royal
// standing there could only have arrived on the previous move, so the other
// side cannot also be on one in a live game.
func (p *Position) goalWinner() bool {
	if len(p.V.Goal) == 0 {
		return false
	}
	justMoved := p.Turn.Other()
	for _, sq := range p.V.Goal {
		pc := p.Board[sq]
		if pc.Type != 0 && pc.Color == justMoved && p.V.Type(pc).Royal {
			return true
		}
	}
	return false
}

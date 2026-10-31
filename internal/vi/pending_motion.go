package vi

import "strings"

// resolveMotion is shared by standalone navigation and operator-pending commands.
func (e *Editor) resolveMotion(key string, count int, countGiven bool) (motionResult, bool) {
	if m, ok := e.motionDest(key, count, countGiven); ok {
		return m, true
	}
	return e.complexMotion(key, count, countGiven)
}

func isMotionPrefix(key string) bool {
	switch key {
	case "g", "f", "F", "t", "T", "'", "`":
		return true
	}
	return false
}

func motionAddsJump(key string) bool {
	return key == "G" || key == "gg" || key == "%" || key == "n" || key == "N" || key == "*" || key == "#" || key == "g*" || key == "g#" ||
		strings.HasPrefix(key, "'") || strings.HasPrefix(key, "`")
}

func (e *Editor) commandMotionCount(count int, given bool) (int, bool) {
	if e.opCountGiven {
		return countProduct(e.opCount, count), true
	}
	return count, given
}

func (e *Editor) completePendingMotion(key string) {
	count := max(1, e.count)
	count, given := e.commandMotionCount(count, e.countGiven)
	if e.pendingOp != 0 {
		e.executeOperatorMotion(e.pendingOp, key, count, given)
		return
	}
	if m, ok := e.resolveMotion(key, count, given); ok {
		if motionAddsJump(key) {
			e.addJump()
		}
		e.row, e.col = m.row, m.col
		e.clampCursor()
		e.ensureVisible()
	}
}

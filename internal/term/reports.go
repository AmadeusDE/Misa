package term

import "fmt"

// respond answers a terminal query on the PTY. The caller holds t.mu.
func (t *Terminal) respond(s string) {
	if t.conn != nil {
		t.enqueueReply([]byte(s))
	}
}

func fmtCursorReport(prefix string, row, col int) string {
	return fmt.Sprintf("%s%d;%dR", prefix, row+1, col+1)
}

func fmtSizeReport(rows, cols int) string {
	return fmt.Sprintf("\x1b[8;%d;%dt", rows, cols)
}

// reportMode answers DECRQM (CSI ? Ps $ p): 1 set, 2 reset, 0 unknown.
func (t *Terminal) reportMode() {
	mode := t.p(0, 0)
	state := 0
	if set, known := t.modeState(mode); known {
		state = 2
		if set {
			state = 1
		}
	}
	t.respond(fmt.Sprintf("\x1b[?%d;%d$y", mode, state))
}

func (t *Terminal) modeState(mode int) (set, known bool) {
	switch mode {
	case 1:
		return t.appCursorKeys, true
	case 7:
		return !t.noAutowrap, true
	case 25:
		return !t.cursorHidden, true
	case 47, 1047, 1049:
		return t.altScreen, true
	case 2004:
		return t.bracketedPaste, true
	case 2026:
		return t.synced, true
	case 9, 1000, 1002, 1003:
		return t.mouse.Tracking == mode, true
	case 1006:
		return t.mouse.SGR, true
	}
	return false, false
}

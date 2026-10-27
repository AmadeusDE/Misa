package term

// MouseMode is the mouse reporting a program has asked for.
type MouseMode struct {
	Tracking int  // 0 off, 9 X10, 1000 press/release, 1002 +drag, 1003 +any motion
	SGR      bool // mode 1006 encoding
}

func (t *Terminal) Mouse() MouseMode {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mouse
}

func (t *Terminal) setMouseMode(mode int, set bool) {
	switch {
	case mode == 1006:
		t.mouse.SGR = set
	case set:
		t.mouse.Tracking = mode
	case t.mouse.Tracking == mode:
		t.mouse.Tracking = 0
	}
}

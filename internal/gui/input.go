package gui

import (
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/sys/unix"

	"github.com/AmadeusDE/Misa/internal/ui"
	"github.com/AmadeusDE/Misa/internal/wayland"
	"github.com/AmadeusDE/Misa/internal/xkb"
)

const (
	seatGetPointer        = 0
	seatGetKeyboard       = 1
	seatEventCapabilities = 0
	capPointer            = 1
	capKeyboard           = 2

	keyboardEventKeymap     = 0
	keyboardEventEnter      = 1
	keyboardEventLeave      = 2
	keyboardEventKey        = 3
	keyboardEventModifiers  = 4
	keyboardEventRepeatInfo = 5
	keymapFormatXKBv1       = 1
	keyPressed              = 1

	pointerEventEnter        = 0
	pointerEventMotion       = 2
	pointerEventButton       = 3
	pointerEventAxis         = 4
	pointerEventFrame        = 5
	pointerEventAxisDiscrete = 8
	pointerEventAxisValue120 = 9
	cursorShapeGetPointer    = 1
	cursorShapeSetShape      = 1
	cursorShapeDefault       = 1

	smoothScrollStep = 10.0
)

var pointerButtons = map[uint32]tea.MouseButton{
	0x110: tea.MouseLeft, 0x111: tea.MouseRight, 0x112: tea.MouseMiddle,
	0x113: tea.MouseBackward, 0x114: tea.MouseForward,
}

// seat is keyboard and pointer state.
type seat struct {
	id, keyboard, pointer uint32
	cursorShapes, cursor  uint32
	keymap                *xkb.Keymap
	depressed, latched    uint32
	locked, group         uint32
	repeatRate            int32
	repeatDelay           int32
	repeatKey             uint32
	repeat                *time.Timer
	focused               bool
	serial                uint32
	pointerX, pointerY    float64
	held                  tea.MouseButton
	lastCell              [2]int
	scroll                [2]float64
	discreteFrame         bool
}

func (a *app) bindSeat(reg *wayland.Registry) {
	a.seat.repeatRate, a.seat.repeatDelay = 25, 600
	a.seat.id, _ = reg.Bind("wl_seat", 8, a.seatEvent)
	a.seat.cursorShapes, _ = reg.Bind("wp_cursor_shape_manager_v1", 1, nil)
}

func (a *app) seatEvent(ev *wayland.Event) {
	if ev.Op != seatEventCapabilities {
		return
	}
	s := &a.seat
	caps := ev.Uint()
	if caps&capKeyboard != 0 && s.keyboard == 0 {
		s.keyboard = a.conn.NewObject(a.keyboardEvent)
		a.conn.Send(s.id, seatGetKeyboard, s.keyboard)
	}
	if caps&capPointer != 0 && s.pointer == 0 {
		s.pointer = a.conn.NewObject(a.pointerEvent)
		a.conn.Send(s.id, seatGetPointer, s.pointer)
		if s.cursorShapes != 0 {
			s.cursor = a.conn.NewObject(nil)
			a.conn.Send(s.cursorShapes, cursorShapeGetPointer, s.cursor, s.pointer)
		}
	}
}

func (a *app) keyboardEvent(ev *wayland.Event) {
	s := &a.seat
	switch ev.Op {
	case keyboardEventKeymap:
		format, fd, size := ev.Uint(), ev.FD(), ev.Uint()
		if fd >= 0 {
			if format == keymapFormatXKBv1 {
				s.keymap = readKeymap(fd, int(size))
			}
			unix.Close(fd)
		}
	case keyboardEventEnter:
		s.serial = ev.Uint()
		s.focused = true
		a.requestDraw()
	case keyboardEventLeave:
		s.focused = false
		s.stopRepeat()
		a.requestDraw()
	case keyboardEventKey:
		serial, _, code, state := ev.Uint(), ev.Uint(), ev.Uint(), ev.Uint()
		s.serial = serial
		if state == keyPressed {
			a.pressKey(code)
		} else if code == s.repeatKey {
			s.stopRepeat()
		}
	case keyboardEventModifiers:
		ev.Uint()
		s.depressed, s.latched, s.locked, s.group = ev.Uint(), ev.Uint(), ev.Uint(), ev.Uint()
	case keyboardEventRepeatInfo:
		s.repeatRate, s.repeatDelay = ev.Int(), ev.Int()
	}
}

func readKeymap(fd, size int) *xkb.Keymap {
	data, err := unix.Mmap(fd, 0, size, unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		return nil
	}
	defer unix.Munmap(data)
	km, err := xkb.Parse(string(data))
	if err != nil {
		return nil
	}
	return km
}

func (s *seat) mods() uint32 { return s.depressed | s.latched | s.locked }

func (s *seat) keyMods() keyMods {
	m := s.mods()
	return keyMods{
		shift: m&xkb.ModShift != 0,
		ctrl:  m&xkb.ModControl != 0,
		alt:   s.keymap != nil && m&s.keymap.Mod("Alt") != 0,
		super: s.keymap != nil && m&s.keymap.Mod("Super") != 0,
	}
}

func (a *app) pressKey(code uint32) {
	s := &a.seat
	if s.keymap == nil {
		return
	}
	sym := a.typeKey(code)
	if a.holding && !sym.IsModifier() {
		a.win.closeRequested = true
	}
	if sym.IsModifier() || s.repeatRate <= 0 {
		return
	}
	s.stopRepeat()
	s.repeatKey = code
	s.repeat = time.NewTimer(time.Duration(s.repeatDelay) * time.Millisecond)
}

// typeKey sends what code types with the current modifiers.
func (a *app) typeKey(code uint32) xkb.Keysym {
	s := &a.seat
	xkbCode := code + 8
	sym := s.keymap.Keysym(xkbCode, s.mods(), s.group)
	base := append([]xkb.Keysym{s.keymap.Keysym(xkbCode, 0, s.group)}, s.keymap.BaseKeysyms(xkbCode)...)
	m := s.keyMods()
	switch guiShortcut(sym, base, m) {
	case pasteShortcut:
		a.clip.paste(a)
	case zoomInShortcut:
		a.zoom(1)
	case zoomOutShortcut:
		a.zoom(-1)
	case zoomResetShortcut:
		a.zoom(0)
	default:
		if k, ok := teaKey(sym, base, m); ok && a.term != nil {
			a.term.SendBytes(ui.KeyBytes(k, a.term.AppCursorKeys()))
		}
	}
	return sym
}

func (s *seat) stopRepeat() {
	if s.repeat != nil {
		s.repeat.Stop()
		s.repeat = nil
	}
	s.repeatKey = 0
}

// repeatC fires while a key is held, or never.
func (s *seat) repeatC() <-chan time.Time {
	if s.repeat == nil {
		return nil
	}
	return s.repeat.C
}

func (a *app) repeatKey() {
	s := &a.seat
	if s.repeat == nil || s.keymap == nil {
		return
	}
	a.typeKey(s.repeatKey)
	s.repeat.Reset(time.Second / time.Duration(s.repeatRate))
}

func (a *app) pointerEvent(ev *wayland.Event) {
	s := &a.seat
	switch ev.Op {
	case pointerEventEnter:
		serial := ev.Uint()
		ev.Object()
		s.pointerX, s.pointerY = ev.Fixed().Float(), ev.Fixed().Float()
		s.lastCell = [2]int{-1, -1}
		if s.cursor != 0 {
			a.conn.Send(s.cursor, cursorShapeSetShape, serial, uint32(cursorShapeDefault))
		}
	case pointerEventMotion:
		ev.Uint()
		s.pointerX, s.pointerY = ev.Fixed().Float(), ev.Fixed().Float()
		if col, row := a.cellAt(); [2]int{col, row} != s.lastCell {
			s.lastCell = [2]int{col, row}
			a.sendMouse(tea.MouseMotionMsg{Button: s.held})
		}
	case pointerEventButton:
		serial, _, button, state := ev.Uint(), ev.Uint(), ev.Uint(), ev.Uint()
		s.serial = serial
		b, ok := pointerButtons[button]
		if !ok {
			return
		}
		if state == keyPressed {
			s.held = b
			a.sendMouse(tea.MouseClickMsg{Button: b})
		} else {
			s.held = tea.MouseNone
			a.sendMouse(tea.MouseReleaseMsg{Button: b})
		}
	case pointerEventAxisDiscrete:
		axis, steps := ev.Uint(), ev.Int()
		s.discreteFrame = true
		a.wheel(axis, float64(steps))
	case pointerEventAxisValue120:
		axis, v := ev.Uint(), ev.Int()
		s.discreteFrame = true
		a.scroll(axis, float64(v), 120)
	case pointerEventAxis:
		ev.Uint()
		axis, v := ev.Uint(), ev.Fixed().Float()
		if !s.discreteFrame {
			a.scroll(axis, v, smoothScrollStep)
		}
	case pointerEventFrame:
		s.discreteFrame = false
	}
}

// scroll accumulates axis motion and sends a wheel step per unit of it.
func (a *app) scroll(axis uint32, v, unit float64) {
	if axis > 1 {
		return
	}
	acc := &a.seat.scroll[axis]
	*acc += v
	steps := math.Trunc(*acc / unit)
	*acc -= steps * unit
	a.wheel(axis, steps)
}

func (a *app) wheel(axis uint32, steps float64) {
	up, down := tea.MouseWheelUp, tea.MouseWheelDown
	if axis == 1 {
		up, down = tea.MouseWheelLeft, tea.MouseWheelRight
	}
	b := down
	if steps < 0 {
		b, steps = up, -steps
	}
	for i := 0; i < int(steps); i++ {
		a.sendMouse(tea.MouseWheelMsg{Button: b})
	}
}

// cellAt is the terminal cell under the pointer.
func (a *app) cellAt() (col, row int) {
	scale := float64(a.win.scale)
	fs := a.rend.fonts
	col = int(math.Floor((a.seat.pointerX*scale - float64(a.rend.pad)) / float64(fs.cellW)))
	row = int(math.Floor((a.seat.pointerY*scale - float64(a.rend.pad)) / float64(fs.cellH)))
	return max(0, min(col, a.snap.Cols-1)), max(0, min(row, a.snap.Rows-1))
}

func (a *app) sendMouse(msg tea.MouseMsg) {
	if a.term == nil {
		return
	}
	col, row := a.cellAt()
	m := msg.Mouse()
	m.X, m.Y, m.Mod = col, row, a.seat.keyMods().tea()
	var withPos tea.MouseMsg
	switch msg.(type) {
	case tea.MouseClickMsg:
		withPos = tea.MouseClickMsg(m)
	case tea.MouseReleaseMsg:
		withPos = tea.MouseReleaseMsg(m)
	case tea.MouseMotionMsg:
		withPos = tea.MouseMotionMsg(m)
	default:
		withPos = tea.MouseWheelMsg(m)
	}
	if b := ui.MouseBytes(withPos, col, row, a.term.Mouse()); b != nil {
		a.term.SendBytes(b)
	}
}

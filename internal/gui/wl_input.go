//go:build unix && !android

package gui

import (
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/sys/unix"

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
	scroll                [2]float64
	discreteFrame         bool
}

func (b *wlBackend) bindSeat(reg *wayland.Registry) {
	b.seat.repeatRate, b.seat.repeatDelay = 25, 600
	b.seat.id, _ = reg.Bind("wl_seat", 8, b.seatEvent)
	b.seat.cursorShapes, _ = reg.Bind("wp_cursor_shape_manager_v1", 1, nil)
}

func (b *wlBackend) seatEvent(ev *wayland.Event) {
	if ev.Op != seatEventCapabilities {
		return
	}
	s := &b.seat
	caps := ev.Uint()
	if caps&capKeyboard != 0 && s.keyboard == 0 {
		s.keyboard = b.conn.NewObject(b.keyboardEvent)
		b.conn.Send(s.id, seatGetKeyboard, s.keyboard)
	}
	if caps&capPointer != 0 && s.pointer == 0 {
		s.pointer = b.conn.NewObject(b.pointerEvent)
		b.conn.Send(s.id, seatGetPointer, s.pointer)
		if s.cursorShapes != 0 {
			s.cursor = b.conn.NewObject(nil)
			b.conn.Send(s.cursorShapes, cursorShapeGetPointer, s.cursor, s.pointer)
		}
	}
}

func (b *wlBackend) keyboardEvent(ev *wayland.Event) {
	s := &b.seat
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
		b.emit(Event{Kind: EventFocus, Focused: true})
	case keyboardEventLeave:
		s.focused = false
		s.stopRepeat()
		b.emit(Event{Kind: EventFocus, Focused: false})
	case keyboardEventKey:
		serial, _, code, state := ev.Uint(), ev.Uint(), ev.Uint(), ev.Uint()
		s.serial = serial
		if state == keyPressed {
			b.pressKey(code)
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

func (b *wlBackend) pressKey(code uint32) {
	s := &b.seat
	if s.keymap == nil {
		return
	}
	sym := b.typeKey(code)
	if sym.IsModifier() || s.repeatRate <= 0 {
		return
	}
	s.stopRepeat()
	s.repeatKey = code
	s.repeat = time.NewTimer(time.Duration(s.repeatDelay) * time.Millisecond)
}

// typeKey translates code with the current modifiers into a core key event.
func (b *wlBackend) typeKey(code uint32) xkb.Keysym {
	s := &b.seat
	xkbCode := code + 8
	sym := s.keymap.Keysym(xkbCode, s.mods(), s.group)
	base := append([]xkb.Keysym{s.keymap.Keysym(xkbCode, 0, s.group)}, s.keymap.BaseKeysyms(xkbCode)...)
	m := s.keyMods()
	k, ok := teaKey(sym, base, m)
	ev := Event{Kind: EventKey, DismissHold: !sym.IsModifier()}
	if ok {
		ev.Key = k
	}
	if ok || ev.DismissHold {
		b.emit(ev)
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

func (b *wlBackend) repeatKey() {
	s := &b.seat
	if s.repeat == nil || s.keymap == nil {
		return
	}
	b.typeKey(s.repeatKey)
	s.repeat.Reset(time.Second / time.Duration(s.repeatRate))
}

func (b *wlBackend) pointerEvent(ev *wayland.Event) {
	s := &b.seat
	switch ev.Op {
	case pointerEventEnter:
		serial := ev.Uint()
		ev.Object()
		s.pointerX, s.pointerY = ev.Fixed().Float(), ev.Fixed().Float()
		if s.cursor != 0 {
			b.conn.Send(s.cursor, cursorShapeSetShape, serial, uint32(cursorShapeDefault))
		}
		b.emitPointer(PointerEnter)
	case pointerEventMotion:
		ev.Uint()
		s.pointerX, s.pointerY = ev.Fixed().Float(), ev.Fixed().Float()
		b.emitPointer(PointerMove)
	case pointerEventButton:
		serial, _, button, state := ev.Uint(), ev.Uint(), ev.Uint(), ev.Uint()
		s.serial = serial
		btn, ok := pointerButtons[button]
		if !ok {
			return
		}
		if state == keyPressed {
			s.held = btn
			b.emitPointer(PointerDown)
		} else {
			b.emit(Event{
				Kind:   EventPointer,
				Action: PointerUp,
				X:      s.pointerX,
				Y:      s.pointerY,
				Button: btn,
				Mod:    s.keyMods().tea(),
			})
			s.held = tea.MouseNone
		}
	case pointerEventAxisDiscrete:
		axis, steps := ev.Uint(), ev.Int()
		s.discreteFrame = true
		b.wheel(axis, float64(steps))
	case pointerEventAxisValue120:
		axis, v := ev.Uint(), ev.Int()
		s.discreteFrame = true
		b.scroll(axis, float64(v), 120)
	case pointerEventAxis:
		ev.Uint()
		axis, v := ev.Uint(), ev.Fixed().Float()
		if !s.discreteFrame {
			b.scroll(axis, v, smoothScrollStep)
		}
	case pointerEventFrame:
		s.discreteFrame = false
	}
}

func (b *wlBackend) emitPointer(action PointerAction) {
	s := &b.seat
	b.emit(Event{
		Kind:   EventPointer,
		Action: action,
		X:      s.pointerX,
		Y:      s.pointerY,
		Button: s.held,
		Mod:    s.keyMods().tea(),
	})
}

// scroll accumulates axis motion and emits a wheel step per unit of it.
func (b *wlBackend) scroll(axis uint32, v, unit float64) {
	if axis > 1 {
		return
	}
	acc := &b.seat.scroll[axis]
	*acc += v
	steps := math.Trunc(*acc / unit)
	*acc -= steps * unit
	b.wheel(axis, steps)
}

func (b *wlBackend) wheel(axis uint32, steps float64) {
	if steps == 0 {
		return
	}
	s := &b.seat
	b.emit(Event{
		Kind:  EventScroll,
		Axis:  int(axis),
		Steps: steps,
		X:     s.pointerX,
		Y:     s.pointerY,
		Mod:   s.keyMods().tea(),
	})
}

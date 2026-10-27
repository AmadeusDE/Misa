package android

import (
	"math"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/gui"
)

// MotionEvent actions and button bits. ACTION_SCROLL carries wheel steps in
// x (horizontal, positive right) and y (vertical, positive down), not a
// pointer position. Every other action carries view pixels in x and y.
const (
	ActionDown        = 0
	ActionUp          = 1
	ActionMove        = 2
	ActionCancel      = 3
	ActionPointerDown = 5
	ActionPointerUp   = 6
	ActionScroll      = 8

	ButtonPrimary   = 1
	ButtonSecondary = 2
	ButtonTertiary  = 4
)

const (
	touchSlop  = 8
	scrollStep = 24
	longPress  = 500 * time.Millisecond
)

// Pointer turns a stream of motion events into core pointer, scroll and
// paste events. Coordinates are already in the core's logical space.
//
// A non-zero button state is a mouse or stylus and is forwarded as pointer
// events. A bare touch taps as a left click, drags as wheel steps (finger
// down scrolls toward earlier lines), and a long press is a right click.
// A second finger that taps is paste (ctrl+shift+v).
type Pointer struct {
	down       bool
	x, y       float64
	at         time.Time
	dragged    bool
	scrolling  bool
	horizontal bool
	acc        float64
	long       bool

	second      bool
	secondMoved bool
	secondAt    time.Time
	secondX     float64
	secondY     float64
}

// Feed reports the events produced by one motion. now is the event time,
// so a long press does not depend on the wall clock.
func (p *Pointer) Feed(action int, x, y float64, buttons int, now time.Time) []gui.Event {
	if action == ActionScroll {
		return p.scroll(x, y)
	}
	if buttons != 0 {
		p.reset()
		return p.mouse(action, x, y, buttons)
	}
	return p.touch(action, x, y, now)
}

func (p *Pointer) reset() {
	*p = Pointer{x: p.x, y: p.y}
}

func (p *Pointer) scroll(h, v float64) []gui.Event {
	var ev []gui.Event
	if v != 0 {
		ev = append(ev, gui.Event{Kind: gui.EventScroll, Axis: 0, Steps: v, X: p.x, Y: p.y})
	}
	if h != 0 {
		ev = append(ev, gui.Event{Kind: gui.EventScroll, Axis: 1, Steps: h, X: p.x, Y: p.y})
	}
	return ev
}

func (p *Pointer) mouse(action int, x, y float64, buttons int) []gui.Event {
	var act gui.PointerAction
	switch action {
	case ActionDown, ActionPointerDown:
		act = gui.PointerDown
	case ActionUp, ActionPointerUp, ActionCancel:
		act = gui.PointerUp
	case ActionMove:
		act = gui.PointerMove
	default:
		return nil
	}
	p.x, p.y = x, y
	return []gui.Event{{
		Kind:   gui.EventPointer,
		Action: act,
		X:      x,
		Y:      y,
		Button: buttonOf(buttons),
	}}
}

func buttonOf(state int) tea.MouseButton {
	switch {
	case state&ButtonSecondary != 0:
		return tea.MouseRight
	case state&ButtonTertiary != 0:
		return tea.MouseMiddle
	default:
		return tea.MouseLeft
	}
}

func (p *Pointer) touch(action int, x, y float64, now time.Time) []gui.Event {
	switch action {
	case ActionDown:
		p.down = true
		p.x, p.y = x, y
		p.at = now
		p.dragged = false
		p.scrolling = false
		p.horizontal = false
		p.long = false
		p.acc = 0
		p.second = false
		return nil
	case ActionPointerDown:
		if !p.down {
			return nil
		}
		p.second = true
		p.secondMoved = false
		p.secondAt = now
		p.secondX, p.secondY = x, y
		p.dragged = true
		return nil
	case ActionPointerUp:
		if !p.second {
			return nil
		}
		moved := p.secondMoved || math.Hypot(x-p.secondX, y-p.secondY) > touchSlop
		quick := now.Sub(p.secondAt) < longPress
		p.second = false
		if !moved && quick {
			return []gui.Event{{
				Kind:        gui.EventKey,
				Key:         tea.Key{Code: 'v', Mod: tea.ModCtrl | tea.ModShift},
				DismissHold: true,
			}}
		}
		return nil
	case ActionMove:
		return p.move(x, y, now)
	case ActionUp:
		if !p.down {
			return nil
		}
		ev := p.release(x, y, now)
		p.down = false
		p.second = false
		return ev
	case ActionCancel:
		p.reset()
		return nil
	default:
		return nil
	}
}

func (p *Pointer) move(x, y float64, now time.Time) []gui.Event {
	if !p.down {
		p.x, p.y = x, y
		return []gui.Event{{Kind: gui.EventPointer, Action: gui.PointerMove, X: x, Y: y}}
	}
	if p.second {
		if math.Hypot(x-p.secondX, y-p.secondY) > touchSlop {
			p.secondMoved = true
		}
		return nil
	}
	if p.long {
		p.x, p.y = x, y
		return []gui.Event{{Kind: gui.EventPointer, Action: gui.PointerMove, X: x, Y: y, Button: tea.MouseRight}}
	}
	dx, dy := x-p.x, y-p.y
	if !p.dragged {
		if math.Hypot(dx, dy) < touchSlop {
			if now.Sub(p.at) >= longPress {
				p.long = true
				p.dragged = true
				return []gui.Event{{Kind: gui.EventPointer, Action: gui.PointerDown, X: p.x, Y: p.y, Button: tea.MouseRight}}
			}
			return nil
		}
		p.dragged = true
		p.horizontal = math.Abs(dx) > math.Abs(dy)
		p.scrolling = !p.horizontal
		p.acc = 0
	}
	delta := dy
	axis := 0
	if p.horizontal {
		delta = dx
		axis = 1
	}
	p.acc += delta
	steps := math.Trunc(p.acc / scrollStep)
	p.acc -= steps * scrollStep
	p.x, p.y = x, y
	if steps == 0 {
		return nil
	}
	// A finger moving down reveals earlier lines, which is wheel-up.
	return []gui.Event{{Kind: gui.EventScroll, Axis: axis, Steps: -steps, X: x, Y: y}}
}

func (p *Pointer) release(x, y float64, now time.Time) []gui.Event {
	if p.long {
		p.long = false
		return []gui.Event{{Kind: gui.EventPointer, Action: gui.PointerUp, X: x, Y: y, Button: tea.MouseRight}}
	}
	if p.dragged {
		return nil
	}
	btn := tea.MouseLeft
	if now.Sub(p.at) >= longPress {
		btn = tea.MouseRight
	}
	return []gui.Event{
		{Kind: gui.EventPointer, Action: gui.PointerDown, X: p.x, Y: p.y, Button: btn},
		{Kind: gui.EventPointer, Action: gui.PointerUp, X: x, Y: y, Button: btn},
	}
}

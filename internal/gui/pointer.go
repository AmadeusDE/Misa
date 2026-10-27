package gui

import (
	"math"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ui"
)

func (a *app) onPointer(ev Event) {
	switch ev.Action {
	case PointerEnter:
		a.lastCell = [2]int{-1, -1}
	case PointerMove:
		col, row := a.cellAt(ev.X, ev.Y)
		if [2]int{col, row} == a.lastCell {
			return
		}
		a.lastCell = [2]int{col, row}
		a.sendMouse(tea.MouseMotionMsg{Button: ev.Button}, col, row, ev.Mod)
	case PointerDown:
		col, row := a.cellAt(ev.X, ev.Y)
		a.lastCell = [2]int{col, row}
		a.sendMouse(tea.MouseClickMsg{Button: ev.Button}, col, row, ev.Mod)
	case PointerUp:
		col, row := a.cellAt(ev.X, ev.Y)
		a.sendMouse(tea.MouseReleaseMsg{Button: ev.Button}, col, row, ev.Mod)
	}
}

func (a *app) onScroll(ev Event) {
	steps := ev.Steps
	if steps == 0 {
		return
	}
	up, down := tea.MouseWheelUp, tea.MouseWheelDown
	if ev.Axis == 1 {
		up, down = tea.MouseWheelLeft, tea.MouseWheelRight
	}
	button := down
	if steps < 0 {
		button, steps = up, -steps
	}
	col, row := a.cellAt(ev.X, ev.Y)
	for i := 0; i < int(steps); i++ {
		a.sendMouse(tea.MouseWheelMsg{Button: button}, col, row, ev.Mod)
	}
}

// cellAt is the terminal cell under a logical-pixel pointer.
func (a *app) cellAt(x, y float64) (col, row int) {
	scale := a.scale
	if scale <= 0 {
		scale = 1
	}
	fs := a.rend.fonts
	col = int(math.Floor((x*scale - float64(a.rend.pad)) / float64(fs.cellW)))
	row = int(math.Floor((y*scale - float64(a.rend.pad)) / float64(fs.cellH)))
	return max(0, min(col, a.snap.Cols-1)), max(0, min(row, a.snap.Rows-1))
}

func (a *app) sendMouse(msg tea.MouseMsg, col, row int, mod tea.KeyMod) {
	if a.term == nil {
		return
	}
	m := msg.Mouse()
	m.X, m.Y, m.Mod = col, row, mod
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

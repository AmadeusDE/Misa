package imgview

import (
	tea "charm.land/bubbletea/v2"
	"image"
)

func (m *Viewer) click(p tea.Mouse) {
	if p.Button != tea.MouseLeft || p.X < 0 || p.X >= m.width || p.Y < 0 || p.Y >= m.height-1 || m.help {
		return
	}
	if m.grid {
		start, end := m.visibleRange()
		col := p.X / m.thumb
		row := p.Y / (m.thumb/2 + 2)
		if col >= m.columns() {
			return
		}
		i := start + row*m.columns() + col
		if i >= end {
			return
		}
		if m.clicked && i == m.clickedIndex {
			m.clicked = false
			m.grid = false
			m.pan = image.Point{}
		} else {
			m.selectIndex(i)
			m.clicked = true
			m.clickedIndex = i
		}
		return
	}
	m.drag = true
	m.dragAt = image.Pt(p.X, p.Y)
}
func (m *Viewer) motion(p tea.Mouse) {
	if !m.drag || m.grid {
		return
	}
	c := m.cellSize()
	m.pan.X -= (p.X - m.dragAt.X) * c.X
	m.pan.Y -= (p.Y - m.dragAt.Y) * c.Y
	m.dragAt = image.Pt(p.X, p.Y)
	m.clampPan()
}
func (m *Viewer) wheel(p tea.Mouse) {
	if m.help {
		return
	}
	dir := 0
	if p.Button == tea.MouseWheelUp {
		dir = -1
	}
	if p.Button == tea.MouseWheelDown {
		dir = 1
	}
	if dir == 0 {
		return
	}
	if p.Mod&tea.ModCtrl != 0 {
		s := "+"
		if dir > 0 {
			s = "-"
		}
		m.key(tea.KeyPressMsg{Code: rune(s[0]), Text: s})
		return
	}
	if m.grid {
		m.selectIndex(m.index + dir*m.columns())
	} else {
		m.selectIndex(m.index + dir)
	}
}

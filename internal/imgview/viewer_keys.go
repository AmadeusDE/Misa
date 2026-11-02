package imgview

import (
	tea "charm.land/bubbletea/v2"
	"image"
	"strings"
)

func (m *Viewer) key(k tea.KeyPressMsg) {
	s := k.String()
	if k.Mod == tea.ModShift && len(k.Text) == 1 {
		s = k.Text
	}
	if strings.HasPrefix(s, "shift+") && len(s) == 7 {
		s = strings.ToUpper(s[6:])
	}
	if s == "esc" {
		if m.help {
			m.help = false
		} else if m.prefix != 0 {
			m.prefix = 0
		} else {
			m.quit = true
		}
		return
	}
	if s == "f1" {
		m.help = !m.help
		return
	}
	if m.help {
		return
	}
	if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		m.prefix = min(9999, m.prefix*10+int(s[0]-'0'))
		return
	}
	prefix := m.prefix
	n := max(1, prefix)
	m.prefix = 0
	if s == "q" {
		m.quit = true
		return
	}
	if len(m.items) == 0 {
		return
	}
	it := &m.items[m.index]
	switch s {
	case "enter":
		m.grid = !m.grid
		m.pan = image.Point{}
	case "n", "space":
		m.selectIndex(m.index + n)
	case "p", "backspace":
		m.selectIndex(m.index - n)
	case "g":
		m.selectIndex(0)
	case "G":
		m.selectIndex(len(m.items) - 1)
	case "m":
		it.marked = !it.marked
	case "M":
		for i := range m.items {
			m.items[i].marked = !m.items[i].marked
		}
	case "ctrl+u", "ctrl+m":
		for i := range m.items {
			m.items[i].marked = false
		}
	case "N", "P":
		step := 1
		if s == "P" {
			step = -1
		}
		found := false
		for i := m.index + step; i >= 0 && i < len(m.items); i += step {
			if m.items[i].marked {
				m.selectIndex(i)
				found = true
				break
			}
		}
		if !found {
			m.note = "No further marked image"
		}
	case "h", "left", "l", "right", "j", "down", "k", "up":
		dx, dy := 0, 0
		switch s {
		case "h", "left":
			dx = -1
		case "l", "right":
			dx = 1
		case "j", "down":
			dy = 1
		case "k", "up":
			dy = -1
		}
		if m.grid {
			cols := m.columns()
			m.selectIndex(m.index + dx + dy*cols)
		} else {
			step := m.panStep()
			m.pan.X += dx * step.X
			m.pan.Y += dy * step.Y
			m.clampPan()
		}
	case "+", "=", "-":
		if m.grid {
			if s == "-" {
				m.thumb = max(8, m.thumb-2)
			} else {
				m.thumb = min(40, m.thumb+2)
			}
		} else {
			z := m.scale()
			if s == "-" {
				z /= 1.25
			} else {
				z *= 1.25
			}
			m.zoom = max(0.02, min(32, z))
			m.fit = fitManual
			m.clampPan()
		}
	case "w":
		if !m.grid {
			m.fit = fitWhole
			m.pan = image.Point{}
		}
	case "W":
		if !m.grid {
			m.fit = fitNoUpscale
			m.pan = image.Point{}
		}
	case "e":
		if !m.grid {
			m.fit = fitWidth
			m.pan = image.Point{}
		}
	case "E":
		if !m.grid {
			m.fit = fitHeight
			m.pan = image.Point{}
		}
	case "f":
		if !m.grid {
			if m.fit == fitActual {
				m.fit = fitWhole
			} else {
				m.fit = fitActual
			}
			m.pan = image.Point{}
			m.clampPan()
		}
	case "<":
		it.rotation = (it.rotation + 3) % 4
		it.appearance++
		m.clampPan()
	case ">":
		it.rotation = (it.rotation + 1) % 4
		it.appearance++
		m.clampPan()
	case "?":
		it.flipX = !it.flipX
		it.appearance++
	case "|":
		it.flipY = !it.flipY
		it.appearance++
	case "ctrl+a":
		m.toggleAnimation(prefix)
	case "ctrl+n":
		m.stepAnimation(n)
	case "ctrl+p":
		m.stepAnimation(-n)
	case "r":
		it.loaded = false
		m.cache.Delete(m.cacheKey(m.index))
		it.err = nil
		it.version++
	case "o":
		m.note = "External viewer is not available yet"
	}
}

package aichat

import tea "charm.land/bubbletea/v2"

const wheelLines = 3

// click selects a sidebar row or a model; a click in the main area focuses the input.
func (m *Chat) click(ms tea.Mouse) tea.Cmd {
	if ms.Button != tea.MouseLeft {
		return nil
	}
	switch m.mode {
	case modePicker:
		list := m.picker.matches()
		if i := m.picker.scroll + ms.Y - 1 - pickerHeader; i >= m.picker.scroll && i < len(list) {
			m.picker.cursor = i
			m.choose(list, i)
		}
	case modeChat:
		m.confirmDel = ""
		if sw := m.sideW(); sw > 0 && ms.X < sw {
			if idx := m.sideScroll + ms.Y - 1; ms.Y >= 1 && idx < m.entries() {
				m.focus = focusSidebar
				m.activate(idx)
				m.focus = focusInput
			}
			return nil
		}
		m.focus = focusInput
	}
	return nil
}

// pickerHeader is the rows above the model list: title, filter and a blank line.
const pickerHeader = 3

func (m *Chat) wheel(ms tea.Mouse) {
	step := wheelLines
	switch ms.Button {
	case tea.MouseWheelUp:
		step = -step
	case tea.MouseWheelDown:
	default:
		return
	}
	switch {
	case m.mode == modePicker:
		m.picker.cursor = max(min(m.picker.cursor+step, len(m.picker.matches())-1), 0)
	case m.mode != modeChat:
	case m.sideW() > 0 && ms.X < m.sideW():
		m.sideScroll = max(min(m.sideScroll+step, m.entries()-m.bodyHeight()), 0)
	default:
		m.scrollBy(-step)
	}
}

package aichat

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const modelsTimeout = 12 * time.Second

// picker is the model list: every model of the configured providers, filtered as you type.
type picker struct {
	items, problems []string
	filter          string
	cursor, scroll  int
	loading         bool
	gen             int
}

type modelsMsg struct {
	gen            int
	refs, problems []string
}

// openPicker shows the models already known and fetches the lists again off the Update goroutine.
func (m *Chat) openPicker() tea.Cmd {
	m.mode = modePicker
	m.picker.filter, m.picker.cursor, m.picker.scroll = "", 0, 0
	m.picker.loading = true
	m.picker.gen++
	gen, models := m.picker.gen, m.opt.Models
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), modelsTimeout)
		defer cancel()
		refs, problems := models(ctx)
		return modelsMsg{gen, refs, problems}
	}
}

func (m *Chat) gotModels(msg modelsMsg) {
	if msg.gen != m.picker.gen {
		return
	}
	m.picker.items, m.picker.problems, m.picker.loading = msg.refs, msg.problems, false
}

// matches are the items containing every word of the filter.
func (p *picker) matches() []string {
	words := strings.Fields(strings.ToLower(p.filter))
	var out []string
	for _, item := range p.items {
		low := strings.ToLower(item)
		ok := true
		for _, w := range words {
			ok = ok && strings.Contains(low, w)
		}
		if ok {
			out = append(out, item)
		}
	}
	return out
}

func (m *Chat) pickerKey(msg tea.KeyPressMsg) tea.Cmd {
	list := m.picker.matches()
	switch k := msg.String(); k {
	case "esc", "ctrl+o", "ctrl+c":
		m.mode = modeChat
	case "up":
		m.picker.cursor = max(m.picker.cursor-1, 0)
	case "down":
		m.picker.cursor = min(m.picker.cursor+1, max(len(list)-1, 0))
	case "pgup":
		m.picker.cursor = max(m.picker.cursor-10, 0)
	case "pgdown":
		m.picker.cursor = min(m.picker.cursor+10, max(len(list)-1, 0))
	case "enter":
		m.choose(list, m.picker.cursor)
	case "backspace":
		if r := []rune(m.picker.filter); len(r) > 0 {
			m.picker.filter = string(r[:len(r)-1])
			m.picker.cursor = 0
		}
	case "ctrl+u":
		m.picker.filter, m.picker.cursor = "", 0
	default:
		if msg.Text != "" {
			m.picker.filter += msg.Text
			m.picker.cursor = 0
		}
	}
	return nil
}

// choose takes list[i], or the typed provider/model when nothing in the list matches.
func (m *Chat) choose(list []string, i int) {
	ref := strings.TrimSpace(m.picker.filter)
	if i >= 0 && i < len(list) {
		ref = list[i]
	} else if !strings.Contains(ref, "/") {
		return
	}
	m.mode = modeChat
	m.setModel(ref)
}

func (m *Chat) pickerRows(w, h int) []string {
	rows := []string{accent.Render("model") + dim.Render("  "+m.modelLabel()), "› " + m.picker.filter + "█", ""}
	list := m.picker.matches()
	room := max(h-len(rows)-1, 1)
	m.picker.cursor = min(m.picker.cursor, max(len(list)-1, 0))
	m.picker.scroll = max(min(m.picker.scroll, m.picker.cursor), m.picker.cursor-room+1)
	for i := m.picker.scroll; i < len(list) && i < m.picker.scroll+room; i++ {
		line := "  " + list[i]
		if list[i] == m.conv.Model {
			line = "● " + list[i]
		}
		if i == m.picker.cursor {
			line = accent.Render(fit(line, w))
		}
		rows = append(rows, line)
	}
	switch {
	case m.picker.loading && len(list) == 0:
		rows = append(rows, dim.Render("loading models…"))
	case len(list) == 0 && strings.Contains(m.picker.filter, "/"):
		rows = append(rows, dim.Render("enter uses "+m.picker.filter))
	case len(list) == 0:
		rows = append(rows, dim.Render("no models match; run aiconfig to add providers"))
	}
	for _, p := range m.picker.problems {
		if len(rows) < h {
			rows = append(rows, dim.Render(p))
		}
	}
	if m.picker.loading && len(list) > 0 && len(rows) < h {
		rows = append(rows, dim.Render("refreshing…"))
	}
	return pad(rows, w, h)
}

func (m *Chat) modelLabel() string {
	if m.conv.Model != "" {
		return m.conv.Model
	}
	if ref, err := m.opt.ModelFor(); err == nil {
		return fmt.Sprintf("%s (default)", ref)
	}
	return "none chosen"
}

package ui

import (
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AmadeusDE/Misa/internal/term"
)

type textSel struct {
	active   bool
	hasRange bool
	startRow int
	startCol int
	endRow   int
	endCol   int
}

func (s *textSel) clear() {
	*s = textSel{}
}

func (s *textSel) begin(row, col int) {
	s.active = true
	s.hasRange = false
	s.startRow = row
	s.startCol = col
	s.endRow = row
	s.endCol = col
}

func (s *textSel) extend(row, col int) {
	s.endRow = row
	s.endCol = col
	if row != s.startRow || col != s.startCol {
		s.hasRange = true
	}
}

func (s *textSel) ordered() (r1, c1, r2, c2 int) {
	r1, c1, r2, c2 = s.startRow, s.startCol, s.endRow, s.endCol
	if r1 > r2 || (r1 == r2 && c1 > c2) {
		r1, c1, r2, c2 = r2, c2, r1, c1
	}
	return
}

func (s *textSel) extractFromTerminal(t *term.Terminal) string {
	if !s.hasRange || t == nil {
		return ""
	}
	r1, c1, r2, c2 := s.ordered()
	lines := t.PlainScreen()
	return extractRange(lines, r1, c1, r2, c2)
}

func (s *textSel) extractFromLines(lines []string) string {
	if !s.hasRange {
		return ""
	}
	r1, c1, r2, c2 := s.ordered()
	return extractRange(lines, r1, c1, r2, c2)
}

// extractRange cuts the selection by display columns, so wide and multibyte
// characters are never split and out-of-range bounds are clamped.
func extractRange(lines []string, r1, c1, r2, c2 int) string {
	var parts []string
	for r := max(r1, 0); r <= r2 && r < len(lines); r++ {
		line := lines[r]
		from, to := rowSpan(r, r1, c1, r2, c2, ansi.StringWidth(line))
		part := ""
		if from < to {
			part = ansi.Cut(line, from, to)
		}
		if r != r2 {
			part = strings.TrimRight(part, " ")
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n")
}

// rowSpan is the clamped column range a selection covers on row r.
func rowSpan(r, r1, c1, r2, c2, width int) (from, to int) {
	from, to = 0, width
	if r == r1 {
		from = c1
	}
	if r == r2 {
		to = c2
	}
	return max(0, min(from, width)), max(0, min(to, width))
}

var selHighlight = lipgloss.NewStyle().Reverse(true)

// highlightRows reverses the selected cells of a rendered view whose first
// row is content row offset, keeping the styles around the selection.
func (s *textSel) highlightRows(view string, offset int) string {
	if !s.hasRange {
		return view
	}
	r1, c1, r2, c2 := s.ordered()
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		r := offset + i
		if r < r1 || r > r2 {
			continue
		}
		from, to := rowSpan(r, r1, c1, r2, c2, ansi.StringWidth(line))
		if from >= to {
			continue
		}
		lines[i] = ansi.Truncate(line, from, "") +
			selHighlight.Render(ansi.Strip(ansi.Cut(line, from, to))) +
			ansi.TruncateLeft(line, to, "")
	}
	return strings.Join(lines, "\n")
}

func (s *textSel) copyToClipboard(t *term.Terminal) tea.Cmd {
	return writeClipboard(s.extractFromTerminal(t))
}

func (s *textSel) copyLinesToClipboard(lines []string) tea.Cmd {
	return writeClipboard(s.extractFromLines(lines))
}

func (m *Model) selMouse(msg mouseMsg, t *term.Terminal) (bool, tea.Cmd) {
	if t == nil {
		return false, nil
	}

	switch {
	case msg.Button == tea.MouseLeft && msg.Action == mousePress:
		m.sel.begin(msg.Y, msg.X)
		return true, nil

	case msg.Action == mouseMotion && m.sel.active:
		m.sel.extend(msg.Y, msg.X)
		return true, nil

	case msg.Action == mouseRelease && m.sel.active:
		m.sel.extend(msg.Y, msg.X)
		m.sel.active = false
		if m.sel.hasRange {
			return true, m.sel.copyToClipboard(t)
		}
		return true, nil
	}

	return false, nil
}

func (m *Model) selKey(msg tea.KeyPressMsg, t *term.Terminal) bool {
	if t == nil {
		return false
	}

	key := msg.String()

	switch key {
	case "shift+left":
		if !m.sel.hasRange {
			m.sel.begin(t.CursorRow(), t.CursorCol())
		}
		m.sel.extend(m.sel.endRow, m.sel.endCol-1)
		return true
	case "shift+right":
		if !m.sel.hasRange {
			m.sel.begin(t.CursorRow(), t.CursorCol())
		}
		m.sel.extend(m.sel.endRow, m.sel.endCol+1)
		return true
	case "shift+up":
		if !m.sel.hasRange {
			m.sel.begin(t.CursorRow(), t.CursorCol())
		}
		m.sel.extend(m.sel.endRow-1, m.sel.endCol)
		return true
	case "shift+down":
		if !m.sel.hasRange {
			m.sel.begin(t.CursorRow(), t.CursorCol())
		}
		m.sel.extend(m.sel.endRow+1, m.sel.endCol)
		return true
	case "shift+home":
		if !m.sel.hasRange {
			m.sel.begin(t.CursorRow(), t.CursorCol())
		}
		m.sel.extend(m.sel.endRow, 0)
		return true
	case "shift+end":
		if !m.sel.hasRange {
			m.sel.begin(t.CursorRow(), t.CursorCol())
		}
		m.sel.extend(m.sel.endRow, t.Cols-1)
		return true
	}

	return false
}

func (m *Model) handleVPSelMouse(msg mouseMsg, vpYOffset int, plainLines []string, contentStartY int) (bool, tea.Cmd) {
	row := msg.Y - contentStartY + vpYOffset
	col := msg.X

	switch {
	case msg.Button == tea.MouseLeft && msg.Action == mousePress:
		m.sel.begin(row, col)
		return true, nil

	case msg.Action == mouseMotion && m.sel.active:
		m.sel.extend(row, col)
		return true, nil

	case msg.Action == mouseRelease && m.sel.active:
		m.sel.extend(row, col)
		m.sel.active = false
		if m.sel.hasRange {
			return true, m.sel.copyLinesToClipboard(plainLines)
		}
		return true, nil
	}

	return false, nil
}

func pasteToTerm(t *term.Terminal) {
	if t == nil {
		return
	}
	if text := readClipboard(); text != "" {
		sendPaste(t, text)
	}
}

// insertAtCursor inserts text at the input's cursor, which counts runes.
func insertAtCursor(ti *textinput.Model, text string) {
	val := []rune(ti.Value())
	pos := min(ti.Position(), len(val))
	ti.SetValue(string(val[:pos]) + text + string(val[pos:]))
	ti.SetCursor(pos + utf8.RuneCountInString(text))
}

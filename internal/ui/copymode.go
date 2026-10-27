package ui

import (
	"image/color"

	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"

	"github.com/AmadeusDE/Misa/internal/term"
)

type copyState struct {
	active    bool
	lines     []string
	curRow    int
	curCol    int
	selecting bool
	selRow    int
	selCol    int
	scroll    int
	rows      int
	cols      int
}

func newCopyState(t *term.Terminal, lines []string, rows, cols int) copyState {
	if t != nil && t.InAltScreen() {
		lines = t.PlainScreen()
	} else if t != nil && lines == nil {
		lines = t.PlainLines()
	}
	plain := make([]string, len(lines))
	for i, l := range lines {
		plain[i] = ansi.Strip(l)
	}
	startRow := max(len(plain)-1, 0)
	return copyState{
		active: true,
		lines:  plain,
		curRow: startRow,
		rows:   rows,
		cols:   cols,
		scroll: max(len(plain)-rows, 0),
	}
}

// Columns in copy mode are rune indexes into the line, from 0 to its length.

func (cs *copyState) line(row int) []rune {
	if row < 0 || row >= len(cs.lines) {
		return nil
	}
	return []rune(cs.lines[row])
}

func (cs *copyState) lineLen(row int) int {
	return len(cs.line(row))
}

func (cs *copyState) clampCol() {
	cs.curCol = max(0, min(cs.curCol, cs.lineLen(cs.curRow)))
}

func (cs *copyState) toggleSelect() {
	if cs.selecting {
		cs.selecting = false
	} else {
		cs.selecting = true
		cs.selRow = cs.curRow
		cs.selCol = cs.curCol
	}
}

func (cs *copyState) moveUp(n int) {
	cs.curRow = max(cs.curRow-n, 0)
	cs.clampCol()
	cs.ensureVisible()
}

func (cs *copyState) moveDown(n int) {
	cs.curRow = max(min(cs.curRow+n, len(cs.lines)-1), 0)
	cs.clampCol()
	cs.ensureVisible()
}

func (cs *copyState) moveLeft(n int) {
	cs.curCol = max(cs.curCol-n, 0)
}

func (cs *copyState) moveRight(n int) {
	cs.curCol = max(0, min(cs.curCol+n, max(cs.lineLen(cs.curRow), 1)-1))
}

func (cs *copyState) moveHome() {
	cs.curCol = 0
}

func (cs *copyState) moveEnd() {
	cs.curCol = cs.lineLen(cs.curRow)
}

func (cs *copyState) moveTop() {
	cs.curRow = 0
	cs.clampCol()
	cs.ensureVisible()
}

func (cs *copyState) moveBottom() {
	cs.curRow = max(len(cs.lines)-1, 0)
	cs.clampCol()
	cs.ensureVisible()
}

func (cs *copyState) moveWordForward() {
	line := cs.line(cs.curRow)
	col := min(cs.curCol, len(line))
	for col < len(line) && line[col] != ' ' {
		col++
	}
	for col < len(line) && line[col] == ' ' {
		col++
	}
	cs.curCol = col
}

func (cs *copyState) moveWordBack() {
	line := cs.line(cs.curRow)
	col := min(cs.curCol, len(line))
	if col > 0 {
		col--
	}
	for col > 0 && line[col] == ' ' {
		col--
	}
	for col > 0 && line[col-1] != ' ' {
		col--
	}
	cs.curCol = col
}

func (cs *copyState) ensureVisible() {
	if cs.curRow < cs.scroll {
		cs.scroll = cs.curRow
	}
	if cs.curRow >= cs.scroll+cs.rows {
		cs.scroll = cs.curRow - cs.rows + 1
	}
}

func (cs *copyState) selectionBounds() (r1, c1, r2, c2 int) {
	r1, c1, r2, c2 = cs.selRow, cs.selCol, cs.curRow, cs.curCol
	if r1 > r2 || (r1 == r2 && c1 > c2) {
		r1, c1, r2, c2 = r2, c2, r1, c1
	}
	return
}

func (cs *copyState) selectedText() string {
	if !cs.selecting {
		return ""
	}
	r1, c1, r2, c2 := cs.selectionBounds()
	var parts []string
	for r := r1; r <= r2; r++ {
		line := cs.line(r)
		from, to := 0, len(line)
		if r == r1 {
			from = min(c1, len(line))
		}
		if r == r2 {
			to = min(c2, len(line))
		}
		parts = append(parts, string(line[from:max(from, to)]))
	}
	return strings.Join(parts, "\n")
}

func (cs *copyState) copySelection() string {
	text := cs.selectedText()
	if text == "" {
		if cs.curRow < len(cs.lines) {
			text = strings.TrimRight(cs.lines[cs.curRow], " ")
		}
	}
	if text != "" {
		clipboard.WriteAll(text)
	}
	return text
}

func (cs *copyState) render(ghost color.Color) string {
	if !cs.active || len(cs.lines) == 0 {
		return ""
	}

	selStyle := lipgloss.NewStyle().Reverse(true)
	cursorStyle := lipgloss.NewStyle().Reverse(true).Bold(true)
	r1, c1, r2, c2 := -1, -1, -1, -1
	if cs.selecting {
		r1, c1, r2, c2 = cs.selectionBounds()
	}

	var rendered []string
	end := min(cs.scroll+cs.rows, len(cs.lines))
	for r := cs.scroll; r < end; r++ {
		line := cs.line(r)
		if pad := cs.cols - ansi.StringWidth(string(line)); pad > 0 {
			line = append(line, []rune(strings.Repeat(" ", pad))...)
		}
		if r == cs.curRow && cs.curCol >= len(line) {
			line = append(line, ' ')
		}
		var sb strings.Builder
		for ci, ch := range line {
			inSel := r >= r1 && r <= r2 && (r != r1 || ci >= c1) && (r != r2 || ci < c2)
			switch {
			case r == cs.curRow && ci == cs.curCol:
				sb.WriteString(cursorStyle.Render(string(ch)))
			case inSel:
				sb.WriteString(selStyle.Render(string(ch)))
			default:
				sb.WriteRune(ch)
			}
		}
		rendered = append(rendered, sb.String())
	}

	hint := "COPY: hjkl/arrows:move  v:select  y:copy  q/esc:exit  0/$:home/end  g/G:top/bottom  w/b:word"
	hintLine := lipgloss.NewStyle().Foreground(ghost).Render(hint)

	return strings.Join(rendered, "\n") + "\n" + hintLine
}

func (m *Model) updateCopyMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	cs := &m.copyMode
	key := msg.String()

	switch key {
	case "q", "esc":
		cs.active = false
		return m, nil
	case "h", "left":
		cs.moveLeft(1)
	case "j", "down":
		cs.moveDown(1)
	case "k", "up":
		cs.moveUp(1)
	case "l", "right":
		cs.moveRight(1)
	case "w":
		cs.moveWordForward()
	case "b":
		cs.moveWordBack()
	case "0", "home":
		cs.moveHome()
	case "$", "end":
		cs.moveEnd()
	case "g":
		cs.moveTop()
	case "G", "shift+g":
		cs.moveBottom()
	case "ctrl+u":
		cs.moveUp(cs.rows / 2)
	case "ctrl+d":
		cs.moveDown(cs.rows / 2)
	case "v", "space":
		cs.toggleSelect()
	case "V":
		cs.selecting = true
		cs.selRow = cs.curRow
		cs.selCol = 0
		cs.curCol = cs.lineLen(cs.curRow)
	case "y", "enter":
		text := cs.copySelection()
		cs.active = false
		if text != "" {
			m.status = "copied to clipboard"
		}
		return m, nil
	}

	return m, nil
}

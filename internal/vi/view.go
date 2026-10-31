package vi

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	cursorStyle = lipgloss.NewStyle().Reverse(true)
	statusStyle = lipgloss.NewStyle().Reverse(true)
	tildeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	visualStyle = lipgloss.NewStyle().Reverse(true)
)

func (e *Editor) viewRows() int {
	if e.rows > 0 {
		return e.rows
	}
	return e.height + 2
}

// fit cuts s to w display cells; the pane may be narrower than the text.
func fit(s string, w int) string {
	if w < 1 {
		return ""
	}
	return ansi.Truncate(strings.ReplaceAll(s, "\n", " "), w, "")
}

func (e *Editor) promptLine() (string, bool) {
	var s string
	switch e.mode {
	case ModeConfirm:
		s = e.confirm.prompt()
	case ModeCommand:
		s = ":" + e.cmdBuf
	case ModeSearch:
		s = "/" + e.cmdBuf
		if e.searchDir == -1 {
			s = "?" + e.cmdBuf
		}
	default:
		if e.statusMsg != "" {
			s = e.statusMsg
		} else if e.mode == ModeNormal {
			s = e.pendingDisplay()
		}
	}
	s = strings.ReplaceAll(s, "\n", " ")
	if w := ansi.StringWidth(s); w > e.width && (e.mode == ModeCommand || e.mode == ModeSearch) {
		s = ansi.TruncateLeft(s, w-e.width, "")
	}
	return fit(s, e.width), s != ""
}

// View draws exactly viewRows lines, none wider than the pane. Below three rows
// the text area shrinks first, then the status line gives way to the prompt line.
func (e *Editor) View() string {
	if e.width == 0 || e.height <= 0 {
		return "loading..."
	}
	if e.shellOutput != nil {
		return e.renderShellOutput()
	}
	if e.shell != nil {
		return lipgloss.NewStyle().Reverse(true).Render(fit("Running shell command… Ctrl-C to cancel", e.width))
	}

	e.ensureColumnVisible()
	rows := e.viewRows()
	textRows := max(0, min(e.height, rows-2))
	var lines []string
	for i := 0; i < textRows; i++ {
		lineIdx := e.topLine + i
		if lineIdx < len(e.lines) {
			lines = append(lines, e.renderTextLine(lineIdx))
		} else {
			lines = append(lines, tildeStyle.Render("~"))
		}
	}
	prompt, active := e.promptLine()
	switch {
	case rows >= 3:
		lines = append(lines, e.renderStatusLine(), prompt)
	case active:
		lines = append(lines, prompt)
	default:
		lines = append(lines, e.renderStatusLine())
	}
	return strings.Join(lines, "\n")
}

func (e *Editor) insertLike() bool { return e.mode == ModeInsert || e.mode == ModeReplace }

func (e *Editor) rowLayout(row int) lineLayout {
	if row < 0 || row >= len(e.lines) {
		return lineLayout{}
	}
	return newLayout(e.lines[row], e.tabstop())
}

// cursorVCol is the 1-based display column of the cursor for the ruler: the last
// cell of a tab in normal mode, as Vim shows it.
func (e *Editor) cursorVCol() int {
	start, _ := e.rowLayout(e.row).cursorCells(e.col, e.insertLike())
	return start + 1
}

// ensureColumnVisible scrolls horizontally like Vim's 'nowrap' with sidescroll=0:
// when the cursor leaves the window it is put in the middle of the screen. A cursor
// wider than the pane keeps its first cell visible.
func (e *Editor) ensureColumnVisible() {
	if e.width <= 0 {
		return
	}
	start, n := e.rowLayout(e.row).cursorCells(e.col, e.insertLike())
	end := start + n - 1
	if start >= e.leftCol && end < e.leftCol+e.width {
		return
	}
	e.leftCol = max(0, start-e.width/2)
	if end >= e.leftCol+e.width {
		e.leftCol = end - e.width + 1
	}
	e.leftCol = max(0, min(e.leftCol, start))
}

func (e *Editor) renderTextLine(lineIdx int) string {
	layout := e.rowLayout(lineIdx)

	inVisual := e.visual != visualNone && e.mode == ModeNormal

	var fromR, fromC, toR, toC int
	if inVisual {
		fromR, fromC, toR, toC = e.visualBounds()
	}

	matchFrom, matchTo := 0, 0
	if e.mode == ModeConfirm && lineIdx == e.confirm.row {
		line := e.lines[lineIdx]
		matchFrom = utf8.RuneCountInString(line[:e.confirm.start])
		matchTo = utf8.RuneCountInString(line[:e.confirm.end])
	}
	isSelected := func(col int) bool {
		if col >= matchFrom && col < matchTo {
			return true
		}
		if !inVisual {
			return false
		}
		if e.visual == visualLine {
			return lineIdx >= fromR && lineIdx <= toR
		}
		if lineIdx < fromR || lineIdx > toR {
			return false
		}
		if fromR == toR {
			return col >= fromC && col <= toC
		}
		if lineIdx == fromR {
			return col >= fromC
		}
		if lineIdx == toR {
			return col <= toC
		}
		return true
	}

	marks := lineMarks{selected: isSelected}
	if lineIdx == e.row {
		marks.cursorStart, marks.cursorN = layout.cursorCells(e.col, e.insertLike())
	}
	marks.eolSelected = inVisual && e.visual == visualLine && lineIdx >= fromR && lineIdx <= toR && layout.width == 0

	var b strings.Builder
	for _, run := range layout.window(e.leftCol, e.width, marks) {
		switch run.style {
		case styleCursor:
			b.WriteString(cursorStyle.Render(run.text))
		case styleSelected:
			b.WriteString(visualStyle.Render(run.text))
		default:
			b.WriteString(run.text)
		}
	}
	return b.String()
}

func (e *Editor) renderStatusLine() string {
	modeStr := ""
	switch {
	case e.visual == visualChar:
		modeStr = "-- VISUAL --"
	case e.visual == visualLine:
		modeStr = "-- VISUAL LINE --"
	case e.mode == ModeInsert:
		modeStr = "-- INSERT --"
	case e.mode == ModeReplace || e.mode == ModeReplaceChar:
		modeStr = "-- REPLACE --"
	}

	fname := e.filename
	if fname == "" {
		fname = "[No Name]"
	} else {
		fname = filepath.Base(fname)
	}
	modFlag := ""
	if e.modified {
		modFlag = " [+]"
	}

	left := " " + fname + modFlag
	if modeStr != "" {
		left = modeStr + "  " + left
	}

	right := fmt.Sprintf(" %d,%d ", e.row+1, e.col+1)
	if v := e.cursorVCol(); v != e.col+1 {
		right = fmt.Sprintf(" %d,%d-%d ", e.row+1, e.col+1, v)
	}
	if e.recording {
		right = fmt.Sprintf("recording @%c  ", e.recordReg) + right
	}

	rw := ansi.StringWidth(right)
	if e.width < rw+1 {
		left, right = "", ""
	} else if ansi.StringWidth(left)+rw+1 > e.width {
		left = ansi.Truncate(left, e.width-rw-1, "")
	}
	pad := e.width - ansi.StringWidth(left) - ansi.StringWidth(right)
	return statusStyle.Render(fit(left+strings.Repeat(" ", max(0, pad))+right, e.width))
}

func (e *Editor) pendingDisplay() string {
	var s strings.Builder
	if e.countGiven {
		fmt.Fprintf(&s, "%d", e.count)
	} else if e.opCountGiven && e.opCount > 1 {
		fmt.Fprintf(&s, "%d", e.opCount)
	}
	if e.pendingOp != 0 {
		s.WriteByte(e.pendingOp)
	}
	if e.pendingKey != "" {
		s.WriteString(e.pendingKey)
	}
	return s.String()
}

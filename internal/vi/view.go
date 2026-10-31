package vi

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

var (
	cursorStyle = lipgloss.NewStyle().Reverse(true)
	statusStyle = lipgloss.NewStyle().Reverse(true)
	tildeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	visualStyle = lipgloss.NewStyle().Reverse(true)
)

func (e *Editor) View() string {
	if e.width == 0 || e.height <= 0 {
		return "loading..."
	}

	var b strings.Builder

	for i := 0; i < e.height; i++ {
		lineIdx := e.topLine + i
		if lineIdx < len(e.lines) {
			b.WriteString(e.renderTextLine(lineIdx))
		} else {
			b.WriteString(tildeStyle.Render("~"))
		}
		if i < e.height-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')

	b.WriteString(e.renderStatusLine())
	b.WriteByte('\n')

	switch e.mode {
	case ModeConfirm:
		b.WriteString(e.confirm.prompt())
	case ModeCommand:
		b.WriteString(":" + e.cmdBuf)
	case ModeSearch:
		if e.searchDir == -1 {
			b.WriteString("?" + e.cmdBuf)
		} else {
			b.WriteString("/" + e.cmdBuf)
		}
	default:
		if e.statusMsg != "" {
			b.WriteString(e.statusMsg)
		} else if e.mode == ModeNormal {
			b.WriteString(e.pendingDisplay())
		}
	}

	return b.String()
}

func (e *Editor) renderTextLine(lineIdx int) string {
	runes := []rune(e.lines[lineIdx])
	showCursor := lineIdx == e.row

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
	isSelected := func(row, col int) bool {
		if col >= matchFrom && col < matchTo {
			return true
		}
		if !inVisual {
			return false
		}
		if e.visual == visualLine {
			return row >= fromR && row <= toR
		}
		if row < fromR || row > toR {
			return false
		}
		if fromR == toR {
			return col >= fromC && col <= toC
		}
		if row == fromR {
			return col >= fromC
		}
		if row == toR {
			return col <= toC
		}
		return true
	}

	var b strings.Builder
	dc := 0
	for i, r := range runes {
		if r == '\t' {
			spaces := 8 - (dc % 8)
			if showCursor && i == e.col {
				b.WriteString(cursorStyle.Render(" "))
				for j := 1; j < spaces; j++ {
					if isSelected(lineIdx, i) {
						b.WriteString(visualStyle.Render(" "))
					} else {
						b.WriteByte(' ')
					}
				}
			} else if isSelected(lineIdx, i) {
				for j := 0; j < spaces; j++ {
					b.WriteString(visualStyle.Render(" "))
				}
			} else {
				for j := 0; j < spaces; j++ {
					b.WriteByte(' ')
				}
			}
			dc += spaces
		} else {
			text := string(r)
			width := 1
			if r == '\r' {
				text = "^M"
				width = 2
			}
			if showCursor && i == e.col {
				b.WriteString(cursorStyle.Render(text))
			} else if isSelected(lineIdx, i) {
				b.WriteString(visualStyle.Render(text))
			} else {
				b.WriteString(text)
			}
			dc += width
		}
	}
	if showCursor && e.col >= len(runes) {
		b.WriteString(cursorStyle.Render(" "))
	} else if inVisual && e.visual == visualLine && lineIdx >= fromR && lineIdx <= toR && len(runes) == 0 {
		b.WriteString(visualStyle.Render(" "))
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
	if e.recording {
		right = fmt.Sprintf("recording @%c  ", e.recordReg) + right
	}

	pad := e.width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return statusStyle.Render(left + strings.Repeat(" ", pad) + right)
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

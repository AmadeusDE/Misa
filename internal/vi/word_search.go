package vi

import (
	"strings"
	"unicode"
)

// wordSearchMotion builds a reusable Vim pattern, then uses the ordinary search
// engine so options, highlighting, wrapping and n/N share the same state.
func (e *Editor) wordSearchMotion(key string, count int) (motionResult, bool) {
	line := e.lineRunes(e.row)
	start := max(0, e.col)
	if start >= len(line) {
		e.statusMsg = "No word under cursor"
		return motionResult{}, false
	}
	// Prefer a keyword at or after the cursor; if there is none, use the
	// punctuation run at or after it instead.
	at := start
	for at < len(line) && !isWordChar(line[at]) {
		at++
	}
	keyword := at < len(line)
	if !keyword {
		at = start
		for at < len(line) && unicode.IsSpace(line[at]) {
			at++
		}
		if at == len(line) {
			e.statusMsg = "No word under cursor"
			return motionResult{}, false
		}
	}
	class := func(r rune) bool {
		if keyword {
			return isWordChar(r)
		}
		return isPunct(r)
	}
	for at > 0 && class(line[at-1]) {
		at--
	}
	end := at
	for end < len(line) && class(line[end]) {
		end++
	}
	var b strings.Builder
	if keyword && (key == "*" || key == "#") {
		b.WriteString(`\<`)
	}
	for _, r := range line[at:end] {
		if strings.ContainsRune(`\.*[~^$`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	if keyword && (key == "*" || key == "#") {
		b.WriteString(`\>`)
	}
	e.searchPat = b.String()
	e.searchDir = 1
	if key == "#" || key == "g#" {
		e.searchDir = -1
	}
	e.searchHighlightSuppressed = false
	oldCol := e.col
	e.col = at
	m, ok := e.searchMotion(count, false)
	e.col = oldCol
	return m, ok
}

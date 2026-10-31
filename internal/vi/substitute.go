package vi

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/AmadeusDE/Misa/internal/vi/vipat"
)

func (e *Editor) substituteCmd(from, to int, cmd string, hasRange bool) {
	if !hasRange {
		from, to = e.row, e.row
	}
	pat, repl, flags, ok := parseSubstitute(cmd)
	if !ok {
		e.statusMsg = "Invalid substitute command"
		return
	}

	global := strings.Contains(flags, "g")
	confirm := strings.Contains(flags, "c")
	pat, re, ok := e.usePattern(pat, strings.Contains(flags, "i") || e.ignoreCase)
	if !ok {
		return
	}

	if confirm {
		e.startSubstituteConfirm(from, to, re, pat, repl, global)
		return
	}

	e.saveUndo()
	repl = replacementText(repl)
	total, failed := 0, false
	for i := from; i <= to && i < len(e.lines); i++ {
		newLine, count, first, err := substituteLine(re, e.lines[i], repl, global)
		if err != nil {
			e.statusMsg, failed = err.Error(), true
			break
		}
		if count > 0 {
			if newLine != e.lines[i] {
				e.noteEdit(i, utf8.RuneCountInString(e.lines[i][:first]))
			}
			e.lines[i] = newLine
			total += count
		}
	}
	switch {
	case failed:
	case total > 0:
		e.modified = true
		e.statusMsg = fmt.Sprintf("%d substitution(s)", total)
	default:
		e.statusMsg = "Pattern not found: " + pat
	}
}

func replacementText(repl string) string {
	return strings.ReplaceAll(strings.ReplaceAll(repl, `\n`, "\n"), `\t`, "\t")
}

func parseSubstitute(cmd string) (pattern, replacement, flags string, ok bool) {
	if len(cmd) < 2 {
		return
	}
	parts := splitDelim(cmd[2:], cmd[1])
	if len(parts) < 2 {
		return
	}
	pattern, replacement, ok = parts[0], parts[1], true
	if len(parts) > 2 {
		flags = parts[2]
	}
	return
}

// cutDelim splits s at its first unescaped delimiter; the rest keeps its text.
func cutDelim(s string, delim byte) (before, after string) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case delim:
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

func splitDelim(s string, delim byte) []string {
	var parts []string
	var cur strings.Builder
	escaped := false
	for i := 0; i < len(s); i++ {
		if escaped {
			cur.WriteByte(s[i])
			escaped = false
			continue
		}
		if s[i] == '\\' {
			escaped = true
			cur.WriteByte('\\')
			continue
		}
		if s[i] == delim {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	parts = append(parts, cur.String())
	return parts
}

// nextSubstitution finds the next match to replace at or after from. A line
// that already had a match is exhausted once from reaches its end, and an empty
// match directly after the previous one is skipped, so zero-width patterns
// always make progress (":s/x*/-/g" turns "abc" into "-a-b-c", as in Vim).
func nextSubstitution(re *vipat.Regexp, line string, from, prevEnd int) (start, end int, ok bool, err error) {
	for {
		if prevEnd >= 0 && from >= len(line) {
			return 0, 0, false, nil
		}
		start, end, ok, err = re.Find(line, from)
		if !ok || err != nil {
			return 0, 0, false, err
		}
		if start != end || start != prevEnd {
			return start, end, true, nil
		}
		if start >= len(line) {
			return 0, 0, false, nil
		}
		_, w := utf8.DecodeRuneInString(line[start:])
		from = start + w
	}
}

func substituteLine(re *vipat.Regexp, line, repl string, global bool) (result string, count, first int, err error) {
	var b strings.Builder
	copied, from, prevEnd := 0, 0, -1
	for {
		start, end, ok, err := nextSubstitution(re, line, from, prevEnd)
		if err != nil {
			return line, 0, 0, err
		}
		if !ok {
			break
		}
		if count == 0 {
			first = start
		}
		b.WriteString(line[copied:start])
		b.WriteString(repl)
		copied, prevEnd, from = end, end, end
		count++
		if start == end && end < len(line) {
			_, w := utf8.DecodeRuneInString(line[end:])
			from = end + w
		}
		if !global {
			break
		}
	}
	if count == 0 {
		return line, 0, 0, nil
	}
	b.WriteString(line[copied:])
	return b.String(), count, first, nil
}

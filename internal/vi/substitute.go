package vi

import (
	"fmt"
	"strings"
	"unicode/utf8"
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
	icase := strings.Contains(flags, "i") || e.ignoreCase

	if confirm {
		e.startSubstituteConfirm(from, to, pat, repl, global, icase)
		return
	}

	e.saveUndo()
	total := 0
	for i := from; i <= to && i < len(e.lines); i++ {
		line := e.lines[i]
		var newLine string
		var count int
		if global {
			newLine, count = replaceAll(line, pat, repl, icase)
		} else {
			newLine, count = replaceFirst(line, pat, repl, icase)
		}
		if count > 0 {
			if line != newLine {
				cleanPat := strings.ReplaceAll(strings.ReplaceAll(pat, `\(`, "("), `\)`, ")")
				idx := strings.Index(line, cleanPat)
				if icase {
					idx, _ = foldIndex(line, cleanPat)
				}
				if idx >= 0 {
					e.noteEdit(i, utf8.RuneCountInString(line[:idx]))
				}
			}
			e.lines[i] = newLine
			total += count
		}
	}
	if total > 0 {
		e.modified = true
		e.statusMsg = fmt.Sprintf("%d substitution(s)", total)
	} else {
		e.statusMsg = "Pattern not found: " + pat
	}
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

func replaceFirst(line, pat, repl string, icase bool) (string, int) {
	cleanPat := pat
	if strings.HasPrefix(cleanPat, "\\") {
		cleanPat = strings.ReplaceAll(cleanPat, "\\(", "(")
		cleanPat = strings.ReplaceAll(cleanPat, "\\)", ")")
	}
	cleanRepl := strings.ReplaceAll(repl, "\\n", "\n")
	cleanRepl = strings.ReplaceAll(cleanRepl, "\\t", "\t")

	idx, end := strings.Index(line, cleanPat), 0
	if icase {
		idx, end = foldIndex(line, cleanPat)
	} else if idx >= 0 {
		end = idx + len(cleanPat)
	}
	if idx < 0 {
		return line, 0
	}
	return line[:idx] + cleanRepl + line[end:], 1
}

func replaceAll(line, pat, repl string, icase bool) (string, int) {
	cleanPat := pat
	if strings.HasPrefix(cleanPat, "\\") {
		cleanPat = strings.ReplaceAll(cleanPat, "\\(", "(")
		cleanPat = strings.ReplaceAll(cleanPat, "\\)", ")")
	}
	cleanRepl := strings.ReplaceAll(repl, "\\n", "\n")
	cleanRepl = strings.ReplaceAll(cleanRepl, "\\t", "\t")

	count := 0
	var result strings.Builder
	remaining := line
	for len(remaining) > 0 {
		idx, end := strings.Index(remaining, cleanPat), 0
		if icase {
			idx, end = foldIndex(remaining, cleanPat)
		} else if idx >= 0 {
			end = idx + len(cleanPat)
		}
		if idx < 0 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:idx])
		result.WriteString(cleanRepl)
		remaining = remaining[end:]
		count++
		if cleanPat == "" {
			if len(remaining) == 0 {
				break
			}
			_, n := utf8.DecodeRuneInString(remaining)
			result.WriteString(remaining[:n])
			remaining = remaining[n:]
		}
	}
	return result.String(), count
}

package vi

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (e *Editor) executeCommand(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}

	// Abbreviation definitions
	if strings.HasPrefix(cmd, "iabbrev ") || strings.HasPrefix(cmd, "ia ") {
		e.iabbrevCmd(cmd)
		return
	}
	if cmd == "iabbrev" || cmd == "ia" {
		if len(e.abbrevs) == 0 {
			e.statusMsg = "No abbreviations defined"
			return
		}
		var lines []string
		for k, v := range e.abbrevs {
			lines = append(lines, fmt.Sprintf("  %-20s %s", k, v))
		}
		e.statusMsg = strings.Join(lines, "\n")
		return
	}

	// :set commands
	if strings.HasPrefix(cmd, "set ") || cmd == "set" {
		e.setCmd(cmd)
		return
	}

	// :earlier / :later
	if strings.HasPrefix(cmd, "earlier ") || strings.HasPrefix(cmd, "ea ") {
		e.earlierCmd(cmd)
		return
	}
	if strings.HasPrefix(cmd, "later ") || strings.HasPrefix(cmd, "lat ") {
		e.laterCmd(cmd)
		return
	}

	// :tutor
	if cmd == "tutor" {
		e.lines = strings.Split(tutorText, "\n")
		if len(e.lines) > 1 && e.lines[len(e.lines)-1] == "" {
			e.lines = e.lines[:len(e.lines)-1]
		}
		if len(e.lines) == 0 {
			e.lines = []string{""}
		}
		e.filename = ""
		e.modified = false
		e.row = 0
		e.col = 0
		e.topLine = 0
		e.resetHistory()
		e.statusMsg = "Welcome to the Misa tutor"
		return
	}

	// Parse line range
	from, to, rest, hasRange := e.parseRange(cmd)

	switch {
	case rest == "q":
		if e.modified {
			e.statusMsg = "No write since last change (add ! to override)"
			return
		}
		e.Quit = true
	case rest == "q!":
		e.Quit = true
	case rest == "w":
		if err := e.save(); err != nil {
			e.statusMsg = err.Error()
		} else {
			e.statusMsg = fmt.Sprintf("\"%s\" %dL written", filepath.Base(e.filename), len(e.lines))
		}
	case rest == "wq" || rest == "x":
		if err := e.save(); err != nil {
			e.statusMsg = err.Error()
		} else {
			e.Quit = true
		}
	case strings.HasPrefix(rest, "w "):
		name := strings.TrimSpace(rest[2:])
		if name != "" {
			e.filename = name
			if err := e.save(); err != nil {
				e.statusMsg = err.Error()
			} else {
				e.statusMsg = fmt.Sprintf("\"%s\" %dL written", filepath.Base(e.filename), len(e.lines))
			}
		}
	case strings.HasPrefix(rest, "r "):
		e.readCmd(strings.TrimSpace(rest[2:]))

	case strings.HasPrefix(rest, "!"):
		shellArg := strings.TrimSpace(rest[1:])
		if hasRange {
			e.filterRange(from, to, shellArg)
		} else {
			e.shellCmd(shellArg)
		}

	case strings.HasPrefix(rest, "s") && len(rest) > 1 && !isAlphaNum(rest[1]):
		e.substituteCmd(from, to, rest, hasRange)

	case strings.HasPrefix(rest, "g") && len(rest) > 1 && !isAlphaNum(rest[1]):
		e.globalCmd(from, to, rest[1:], false, hasRange)
	case strings.HasPrefix(rest, "v") && len(rest) > 1 && !isAlphaNum(rest[1]):
		e.globalCmd(from, to, rest[1:], true, hasRange)

	case rest == "d":
		if !hasRange {
			from, to = e.row, e.row
		}
		e.saveUndo()
		e.noteEdit(from, 0)
		deleted := make([]string, 0, to-from+1)
		for i := from; i <= to && i < len(e.lines); i++ {
			deleted = append(deleted, e.lines[i])
		}
		e.deleteToReg(deleted, true)
		e.lines = append(e.lines[:from], e.lines[to+1:]...)
		if len(e.lines) == 0 {
			e.lines = []string{""}
		}
		e.row = from
		if e.row >= len(e.lines) {
			e.row = len(e.lines) - 1
		}
		e.col = e.firstNonBlank(e.row)
		e.statusMsg = fmt.Sprintf("%d lines deleted", to-from+1)

	case rest == "y":
		if !hasRange {
			from, to = e.row, e.row
		}
		yanked := make([]string, 0, to-from+1)
		for i := from; i <= to && i < len(e.lines); i++ {
			yanked = append(yanked, e.lines[i])
		}
		e.yankToReg(yanked, true)
		e.statusMsg = fmt.Sprintf("%d lines yanked", len(yanked))

	case rest == "":
		if hasRange {
			e.row = to
			e.col = e.firstNonBlank(to)
			e.clampCursor()
			e.ensureVisible()
		}

	default:
		// Try as a line number
		if n, err := strconv.Atoi(rest); err == nil {
			r := n - 1
			if r < 0 {
				r = 0
			}
			if r >= len(e.lines) {
				r = len(e.lines) - 1
			}
			e.row = r
			e.col = e.firstNonBlank(r)
			e.clampCursor()
			e.ensureVisible()
		} else {
			e.statusMsg = "Not an editor command: " + rest
		}
	}
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func (e *Editor) parseRange(cmd string) (from, to int, rest string, hasRange bool) {
	from = e.row
	to = e.row
	i := 0

	parseLine := func() (int, bool) {
		if i >= len(cmd) {
			return -1, false
		}
		switch cmd[i] {
		case '.':
			i++
			return e.row, true
		case '$':
			i++
			return len(e.lines) - 1, true
		case '%':
			i++
			from = 0
			to = len(e.lines) - 1
			hasRange = true
			return -1, false
		case '\'':
			if i+1 < len(cmd) {
				i += 2
				ch := cmd[i-1]
				if p, ok := e.marks[ch]; ok {
					return p.row, true
				}
			}
			return -1, false
		default:
			if cmd[i] >= '0' && cmd[i] <= '9' {
				n := 0
				for i < len(cmd) && cmd[i] >= '0' && cmd[i] <= '9' {
					n = n*10 + int(cmd[i]-'0')
					i++
				}
				return n - 1, true
			}
		}
		return -1, false
	}

	if i < len(cmd) && cmd[i] == '%' {
		i++
		from = 0
		to = len(e.lines) - 1
		hasRange = true
		rest = strings.TrimSpace(cmd[i:])
		return
	}

	if line, ok := parseLine(); ok {
		from = line
		to = line
		hasRange = true

		for i < len(cmd) && (cmd[i] == '+' || cmd[i] == '-') {
			sign := 1
			if cmd[i] == '-' {
				sign = -1
			}
			i++
			n := 0
			for i < len(cmd) && cmd[i] >= '0' && cmd[i] <= '9' {
				n = n*10 + int(cmd[i]-'0')
				i++
			}
			if n == 0 {
				n = 1
			}
			from += sign * n
			to = from
		}

		if i < len(cmd) && cmd[i] == ',' {
			i++
			if line2, ok := parseLine(); ok {
				to = line2
			}
			for i < len(cmd) && (cmd[i] == '+' || cmd[i] == '-') {
				sign := 1
				if cmd[i] == '-' {
					sign = -1
				}
				i++
				n := 0
				for i < len(cmd) && cmd[i] >= '0' && cmd[i] <= '9' {
					n = n*10 + int(cmd[i]-'0')
					i++
				}
				if n == 0 {
					n = 1
				}
				to += sign * n
			}
		}
	}

	if from < 0 {
		from = 0
	}
	if to < 0 {
		to = 0
	}
	if from >= len(e.lines) {
		from = len(e.lines) - 1
	}
	if to >= len(e.lines) {
		to = len(e.lines) - 1
	}
	if from > to {
		from, to = to, from
	}

	rest = strings.TrimSpace(cmd[i:])
	return
}

func (e *Editor) substituteCmd(from, to int, cmd string, hasRange bool) {
	if !hasRange {
		from, to = e.row, e.row
	}
	if len(cmd) < 2 {
		return
	}
	delim := cmd[1]
	parts := splitDelim(cmd[2:], delim)
	if len(parts) < 2 {
		e.statusMsg = "Invalid substitute command"
		return
	}
	pat := parts[0]
	repl := parts[1]
	flags := ""
	if len(parts) > 2 {
		flags = parts[2]
	}

	global := strings.Contains(flags, "g")
	confirm := strings.Contains(flags, "c")
	icase := strings.Contains(flags, "i") || e.ignoreCase

	if confirm {
		e.statusMsg = "Confirm not yet supported, applying all"
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

func (e *Editor) globalCmd(from, to int, args string, invert bool, hasRange bool) {
	if !hasRange {
		from = 0
		to = len(e.lines) - 1
	}
	if len(args) < 1 {
		return
	}
	delim := args[0]
	parts := splitDelim(args[1:], delim)
	if len(parts) < 1 {
		return
	}
	pat := parts[0]
	exCmd := ""
	if len(parts) > 1 {
		exCmd = strings.TrimSpace(parts[1])
	}
	if exCmd == "" {
		exCmd = "d"
	}

	e.saveUndo()
	matches := 0
	for i := to; i >= from; i-- {
		if i >= len(e.lines) {
			continue
		}
		found := e.indexOf(e.lines[i], pat) >= 0
		if found != invert {
			e.row = i
			e.executeCommand(fmt.Sprintf("%d%s", i+1, exCmd))
			matches++
		}
	}
	if matches == 0 {
		e.statusMsg = "Pattern not found: " + pat
	} else {
		e.statusMsg = fmt.Sprintf("%d lines affected", matches)
	}
}

func (e *Editor) readCmd(arg string) {
	if strings.HasPrefix(arg, "!") {
		shellCmd := strings.TrimSpace(arg[1:])
		if shellCmd == "" {
			return
		}
		cmd := exec.Command("sh", "-c", shellCmd)
		if e.filename != "" {
			cmd.Dir = filepath.Dir(e.filename)
		}
		out, err := cmd.Output()
		if err != nil {
			e.statusMsg = "Shell error: " + err.Error()
			return
		}
		e.saveUndo()
		lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		for i, line := range lines {
			e.insertLineAfter(e.row+i, line)
		}
		e.row += len(lines)
		e.clampCursor()
		e.statusMsg = fmt.Sprintf("%d lines read", len(lines))
		return
	}

	fname := arg
	if !filepath.IsAbs(fname) && e.filename != "" {
		fname = filepath.Join(filepath.Dir(e.filename), fname)
	}
	data, err := os.ReadFile(fname)
	if err != nil {
		e.statusMsg = err.Error()
		return
	}
	e.saveUndo()
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		e.insertLineAfter(e.row+i, line)
	}
	e.row += len(lines)
	e.clampCursor()
	e.statusMsg = fmt.Sprintf("\"%s\" %d lines read", filepath.Base(fname), len(lines))
}

func (e *Editor) shellCmd(cmd string) {
	if cmd == "" {
		return
	}
	c := exec.Command("sh", "-c", cmd)
	if e.filename != "" {
		c.Dir = filepath.Dir(e.filename)
	}
	out, err := c.CombinedOutput()
	if err != nil && len(out) == 0 {
		e.statusMsg = "Shell error: " + err.Error()
		return
	}
	e.statusMsg = strings.TrimRight(string(out), "\n")
}

func (e *Editor) filterRange(from, to int, cmd string) {
	if cmd == "" {
		return
	}
	input := strings.Join(e.lines[from:to+1], "\n") + "\n"
	c := exec.Command("sh", "-c", cmd)
	c.Stdin = strings.NewReader(input)
	if e.filename != "" {
		c.Dir = filepath.Dir(e.filename)
	}
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	if err != nil && stdout.Len() == 0 {
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		e.statusMsg = "Filter error: " + strings.TrimRight(msg, "\n")
		return
	}

	e.saveUndo()
	result := strings.TrimRight(stdout.String(), "\n")
	newLines := strings.Split(result, "\n")

	replaced := make([]string, 0, from+len(newLines)+len(e.lines)-to-1)
	replaced = append(replaced, e.lines[:from]...)
	replaced = append(replaced, newLines...)
	replaced = append(replaced, e.lines[to+1:]...)
	e.lines = replaced
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	e.modified = true
	e.row = from
	e.clampCursor()
	e.statusMsg = fmt.Sprintf("%d lines filtered", len(newLines))
}

func (e *Editor) setCmd(cmd string) {
	args := strings.TrimSpace(strings.TrimPrefix(cmd, "set"))
	if args == "" {
		var flags []string
		if e.ignoreCase {
			flags = append(flags, "ignorecase")
		} else {
			flags = append(flags, "noignorecase")
		}
		e.statusMsg = strings.Join(flags, " ")
		return
	}
	for _, arg := range strings.Fields(args) {
		switch arg {
		case "ic", "ignorecase":
			e.ignoreCase = true
			e.statusMsg = "ignorecase on"
		case "noic", "noignorecase":
			e.ignoreCase = false
			e.statusMsg = "noignorecase"
		case "hls", "hlsearch":
			e.statusMsg = "hlsearch on"
		case "nohls", "nohlsearch":
			e.statusMsg = "nohlsearch"
		case "is", "incsearch":
			e.statusMsg = "incsearch on"
		case "nois", "noincsearch":
			e.statusMsg = "noincsearch"
		case "nocp", "nocompatible":
			e.statusMsg = "nocompatible"
		default:
			e.statusMsg = "Unknown option: " + arg
		}
	}
}

func (e *Editor) earlierCmd(cmd string) {
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(cmd, "earlier"), "ea"))
	arg = strings.TrimSpace(arg)
	n := 1
	if arg != "" {
		if v, err := strconv.Atoi(arg); err == nil {
			n = v
		}
	}
	for i := 0; i < n; i++ {
		e.undo()
	}
	e.statusMsg = fmt.Sprintf("%d change(s) undone", n)
}

func (e *Editor) laterCmd(cmd string) {
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(cmd, "later"), "lat"))
	arg = strings.TrimSpace(arg)
	n := 1
	if arg != "" {
		if v, err := strconv.Atoi(arg); err == nil {
			n = v
		}
	}
	for i := 0; i < n; i++ {
		e.redo()
	}
	e.statusMsg = fmt.Sprintf("%d change(s) redone", n)
}

func (e *Editor) iabbrevCmd(cmd string) {
	arg := strings.TrimPrefix(cmd, "iabbrev ")
	arg = strings.TrimPrefix(arg, "ia ")
	fields := strings.SplitN(strings.TrimSpace(arg), " ", 2)
	if len(fields) < 2 {
		e.statusMsg = "Usage: :iabbrev <trigger> <expansion>"
		return
	}
	trigger := fields[0]
	expansion := fields[1]
	e.abbrevs[trigger] = expansion
	e.statusMsg = fmt.Sprintf("Abbreviation: %s -> %s", trigger, expansion)
}

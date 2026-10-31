package vi

import (
	"fmt"
	"strconv"
	"strings"
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
		e.file = fileState{eol: true}
		e.modified = false
		e.row = 0
		e.col = 0
		e.topLine = 0
		e.resetHistory()
		e.statusMsg = "Welcome to the Misa tutor"
		return
	}

	// Parse line range
	from, to, rest, hasRange := e.parseRawRange(cmd)
	if e.editCommand(rest, hasRange) {
		return
	}
	if e.fileCommand(rest, from, to, hasRange) {
		return
	}
	if strings.HasPrefix(rest, "r ") || strings.HasPrefix(rest, "r\t") || strings.HasPrefix(rest, "read ") || strings.HasPrefix(rest, "read\t") {
		if hasRange && (from < -1 || to < -1 || from > to || to >= len(e.lines)) {
			e.statusMsg = "E16: Invalid range"
			return
		}
		arg := strings.TrimSpace(rest[strings.IndexAny(rest, " \t"):])
		e.readCmd(arg, to)
		return
	}
	if hasRange && strings.HasPrefix(rest, "!") && (from < 0 || to < from || to >= len(e.lines)) {
		e.statusMsg = "E16: Invalid range"
		return
	}
	from, to = e.normalizeRange(from, to)

	switch {
	case rest == "q":
		if e.modified {
			e.statusMsg = "No write since last change (add ! to override)"
			return
		}
		e.Quit = true
	case rest == "q!":
		e.Quit = true
	case strings.HasPrefix(rest, "!"):
		shellArg := strings.TrimSpace(rest[1:])
		if hasRange {
			e.startShell(shellArg, []byte(e.rangeContent(from, to)), shellFilter, from, to, false)
		} else {
			e.startShell(shellArg, nil, shellDisplay, 0, 0, false)
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

// isCompoundShell examines only command syntax, never a substitute's text.
func isCompoundShell(command string) bool {
	if strings.HasPrefix(command, "!") {
		return true
	}
	i := 0
	for i < len(command) && command[i] >= 'a' && command[i] <= 'z' {
		i++
	}
	verb := command[:i]
	switch verb {
	case "r", "read":
	case "w", "write", "wq", "x", "xit":
		if i < len(command) && command[i] == '!' {
			i++
		}
	default:
		return false
	}
	if i == len(command) || (command[i] != ' ' && command[i] != '\t') {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(command[i:]), "!")
}

func (e *Editor) globalCmd(from, to int, args string, invert bool, hasRange bool) {
	if !hasRange {
		from = 0
		to = len(e.lines) - 1
	}
	if len(args) < 1 {
		return
	}
	pat, exCmd := cutDelim(args[1:], args[0])
	exCmd = strings.TrimSpace(exCmd)
	if exCmd == "" {
		exCmd = "d"
	}

	_, _, nested, _ := e.parseRange(exCmd)
	if _, _, ok := parseEditCommand(nested); ok {
		e.statusMsg = "Edit commands in global are unsupported"
		return
	}
	if isCompoundShell(nested) {
		e.statusMsg = "Shell commands in global are unsupported"
		return
	}
	if strings.HasPrefix(nested, "s") && len(nested) > 1 && !isAlphaNum(nested[1]) {
		_, _, flags, ok := parseSubstitute(nested)
		if ok && strings.Contains(flags, "c") {
			e.statusMsg = confirmationInCompound
			return
		}
	}

	pat, re, ok := e.usePattern(pat, e.ignoreCase)
	if !ok {
		return
	}
	e.saveUndo()
	matches := 0
	for i := to; i >= from; i-- {
		if i >= len(e.lines) {
			continue
		}
		_, _, found, err := re.Find(e.lines[i], 0)
		if err != nil {
			e.statusMsg = err.Error()
			return
		}
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

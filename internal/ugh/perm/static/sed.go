package static

import (
	"regexp"
	"strings"
)

// sedAddr matches a sed address or range prefix: line numbers, $, /regex/,
// first~step, addr,+N, and a trailing ! (negation).
var sedAddr = regexp.MustCompile(`^\s*(?:\d+(?:~\d+)?|\$|/(?:\\.|[^/\\])*/[IM]*)(?:\s*,\s*(?:\d+|\$|/(?:\\.|[^/\\])*/[IM]*|[+~]\d+))?\s*!?\s*`)

// sedOutputCmds only print, delete, branch or shuffle the pattern and hold
// spaces; none touches a file or runs a program.
const sedOutputCmds = "pdqQ=lnNPDgGhHxz{}"

// sedReadOnly reports whether a sed script only transforms its output. It
// splits on ; and newlines without understanding quoting, so anything it
// cannot read is treated as not read-only.
func sedReadOnly(script string) bool {
	for _, piece := range strings.FieldsFunc(script, func(r rune) bool { return r == ';' || r == '\n' }) {
		cmd := strings.TrimSpace(sedAddr.ReplaceAllString(piece, ""))
		switch {
		case cmd == "":
		case len(cmd) == 1 && strings.Contains(sedOutputCmds, cmd):
		case cmd[0] == 'b' || cmd[0] == 't' || cmd[0] == 'T' || cmd[0] == ':':
		case cmd[0] == 's' || cmd[0] == 'y':
			flags, ok := sedSubstFlags(cmd)
			if !ok || cmd[0] == 's' && strings.Trim(flags, "gpiImM0123456789") != "" {
				return false
			}
		default:
			return false // a/i/c text, r/R/w/W files, e, F and anything unknown
		}
	}
	return true
}

// sedSubstFlags returns what follows s/re/replacement/ (or y/src/dst/).
func sedSubstFlags(cmd string) (string, bool) {
	if len(cmd) < 2 || cmd[1] == '\\' || cmd[1] == '\n' || cmd[1] == ' ' {
		return "", false
	}
	delim, seen := cmd[1], 0
	for i := 2; i < len(cmd); i++ {
		switch cmd[i] {
		case '\\':
			i++
		case delim:
			if seen++; seen == 2 {
				return strings.TrimSpace(cmd[i+1:]), true
			}
		}
	}
	return "", false
}

// Behaviour adapted from uutils coreutils csplit (MIT),
// src/uu/csplit/src/patterns.rs at commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663. The regex operands are POSIX
// basic regular expressions from internal/posixre.
package builtins

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

type csplitPatternKind int

const (
	csplitLine csplitPatternKind = iota // INTEGER
	csplitUpTo                          // /REGEX/[OFFSET]
	csplitSkip                          // %REGEX%[OFFSET]
)

type csplitPattern struct {
	kind   csplitPatternKind
	line   int
	re     *posixre.Regexp
	offset int
	// times is how often the pattern runs: 1 without a {N} operand, N+1 with
	// {N}, and 0 for {*} (as often as possible).
	times int
	// text is the operand as csplit names it in diagnostics.
	text string
}

// parseCsplitPatterns parses the pattern operands and validates the line
// numbers, returning the warnings it collected on the way.
func parseCsplitPatterns(args []string) ([]csplitPattern, []string, error) {
	var pats []csplitPattern
	var warnings []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		times := 1
		if i+1 < len(args) {
			if n, always, ok := csplitRepeat(args[i+1]); ok {
				times = n
				if always {
					times = 0
				}
				i++
			}
		}
		p, err := parseCsplitPattern(arg, times)
		if err != nil {
			return nil, nil, err
		}
		pats = append(pats, p)
	}
	var prev int
	for _, p := range pats {
		if p.kind != csplitLine {
			continue
		}
		switch {
		case p.line == 0:
			return nil, nil, fmt.Errorf("0: line number must be greater than zero")
		case prev == p.line:
			warnings = append(warnings, fmt.Sprintf("line number '%d' is the same as preceding line number", p.line))
		case prev > p.line:
			return nil, nil, fmt.Errorf("line number '%d' is smaller than preceding line number, %d", p.line, prev)
		}
		prev = p.line
	}
	return pats, warnings, nil
}

// csplitRepeat reports whether arg is a {N} or {*} repetition operand. For
// {N} it returns N+1, the number of times the pattern runs.
func csplitRepeat(arg string) (n int, always bool, ok bool) {
	if len(arg) < 3 || arg[0] != '{' || arg[len(arg)-1] != '}' {
		return 0, false, false
	}
	body := arg[1 : len(arg)-1]
	if body == "*" {
		return 0, true, true
	}
	v, err := strconv.ParseUint(body, 10, 31)
	if err != nil {
		return 0, false, false
	}
	return int(v) + 1, false, true
}

func parseCsplitPattern(arg string, times int) (csplitPattern, error) {
	invalid := fmt.Errorf("'%s': invalid pattern", arg)
	switch {
	case strings.HasPrefix(arg, "/"):
		body, offset, ok := csplitDelimited(arg, '/')
		if !ok {
			return csplitPattern{}, invalid
		}
		p, err := csplitRegexPattern(csplitUpTo, body, offset, times, invalid)
		if err != nil {
			return csplitPattern{}, err
		}
		return p, nil
	case strings.HasPrefix(arg, "%"):
		body, offset, ok := csplitDelimited(arg, '%')
		if !ok {
			return csplitPattern{}, invalid
		}
		return csplitRegexPattern(csplitSkip, body, offset, times, invalid)
	}
	// A line number is a decimal integer; a leading + is accepted the way an
	// unsigned parse accepts it.
	if isCsplitUint(arg) {
		v, err := strconv.ParseUint(strings.TrimPrefix(arg, "+"), 10, 63)
		if err != nil || v > uint64(1)<<62 {
			return csplitPattern{}, invalid
		}
		return csplitPattern{kind: csplitLine, line: int(v), times: times, text: strconv.FormatUint(v, 10)}, nil
	}
	return csplitPattern{}, invalid
}

func csplitRegexPattern(kind csplitPatternKind, body string, offset, times int, invalid error) (csplitPattern, error) {
	re, err := posixre.Compile(body, posixre.Options{Syntax: posixre.Basic})
	if err != nil {
		return csplitPattern{}, invalid
	}
	delim := "/"
	text := delim + body + delim
	if kind == csplitSkip {
		delim = "%"
		text = delim + body + delim
	}
	if offset != 0 {
		text += fmt.Sprintf("%+d", offset)
	}
	return csplitPattern{kind: kind, re: re, offset: offset, times: times, text: text}, nil
}

// csplitDelimited splits a /REGEX/[OFFSET] or %REGEX%[OFFSET] operand. The
// body is taken up to the last delimiter that leaves a valid offset behind.
func csplitDelimited(arg string, delim byte) (body string, offset int, ok bool) {
	for i := len(arg) - 1; i >= 1; i-- {
		if arg[i] != delim {
			continue
		}
		tail := arg[i+1:]
		if tail == "" {
			return arg[1:i], 0, true
		}
		if isCsplitOffset(tail) {
			v, err := strconv.ParseInt(tail, 10, 64)
			if err != nil {
				return "", 0, false
			}
			return arg[1:i], int(v), true
		}
	}
	return "", 0, false
}

// isCsplitOffset reports whether s is [+-]?[0-9]+.
func isCsplitOffset(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '+' || s[0] == '-' {
		s = s[1:]
	}
	return isCsplitUint(s)
}

func isCsplitUint(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

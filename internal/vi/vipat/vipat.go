// Package vipat parses and matches the supported subset of Vim search patterns.
//
// Patterns are Vim syntax, not Go regexp syntax. Matching is backtracking and
// leftmost-first, as in Vim, and runs against one line of text. The default
// mode is 'magic'; \v, \m, \M and \V switch the mode inside a pattern.
//
// Supported: literals, . * \+ \= \? \{n,m} \{-n,m}, ^ and $ where Vim treats
// them as anchors, \( \), \%( \), \|, collections [...] with ranges, negation
// and [:class:] names, the classes \s \S \d \D \w \W \a \A \l \L \u \U \x \X
// \o \O \h \H, \< \>, \t \e \r \b, and the case atoms \c and \C.
//
// Everything else Vim gives meaning to (\zs, \ze, \&, \@=, \_x, \%[, \%d,
// \1 backreferences, \n, ~, \i \k \f \p ...) is rejected with an
// *UnsupportedError instead of being read as a literal.
//
// Matching is bounded: a pattern that backtracks too long fails with
// ErrTooComplex.
package vipat

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ErrTooComplex reports that matching exceeded its step budget.
var ErrTooComplex = errors.New("E363: Pattern too complex")

// UnsupportedError names a Vim pattern construct this package does not implement.
type UnsupportedError struct{ Construct string }

func (e *UnsupportedError) Error() string {
	return "Unsupported pattern construct: " + e.Construct
}

const (
	attemptSteps = 1 << 20
	totalSteps   = 1 << 26
)

// Regexp is a compiled Vim pattern.
type Regexp struct {
	prog  *node
	fold  bool
	first rune
}

// Compile parses pattern. ignoreCase is the default case handling; \c and \C
// inside the pattern override it.
func Compile(pattern string, ignoreCase bool) (*Regexp, error) {
	toks, caseFlag, err := lex(pattern)
	if err != nil {
		return nil, err
	}
	prog, err := parse(toks)
	if err != nil {
		return nil, err
	}
	re := &Regexp{prog: prog, fold: ignoreCase, first: -1}
	switch caseFlag {
	case caseIgnore:
		re.fold = true
	case caseMatch:
		re.fold = false
	}
	if !re.fold && len(prog.alts) == 1 && len(prog.alts[0]) > 0 && prog.alts[0][0].kind == kLit && prog.alts[0][0].r != utf8.RuneError {
		re.first = prog.alts[0][0].r
	}
	return re, nil
}

// Find returns the leftmost match starting at or after byte offset from, which
// must lie on a rune boundary. The whole line is context for anchors and word
// boundaries.
func (re *Regexp) Find(line string, from int) (start, end int, ok bool, err error) {
	m := matcher{s: line, fold: re.fold}
	for pos := from; pos <= len(line); {
		if re.first >= 0 {
			i := strings.IndexRune(line[pos:], re.first)
			if i < 0 {
				return 0, 0, false, nil
			}
			pos += i
		}
		m.steps = 0
		matched := m.seq([]*node{re.prog}, pos, func(p int) bool { end = p; return true })
		if m.err != nil {
			return 0, 0, false, m.err
		}
		if matched {
			return pos, end, true, nil
		}
		if pos == len(line) {
			break
		}
		_, w := utf8.DecodeRuneInString(line[pos:])
		pos += w
	}
	return 0, 0, false, nil
}

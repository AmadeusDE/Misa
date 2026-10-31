// Package posixre compiles POSIX basic and extended regular expressions
// (BRE/ERE) with leftmost-longest matching, back-references and, on request,
// the GNU operators and escapes that grep and sed users expect. It is the
// shared regex layer for Misa's text utilities.
//
// Matching follows the C locale: character classes and word characters are
// ASCII, ranges compare code points, and input is read as UTF-8 with each
// invalid byte standing for itself. The overall match is leftmost-longest.
// Submatches are those of the highest-priority path to that match
// (leftmost alternative first, greedy repetition), which can differ from
// POSIX's subexpression rule when a pattern is ambiguous.
package posixre

import (
	"errors"
	"regexp"
	"strings"
)

type Syntax uint8

const (
	Basic Syntax = iota
	Extended
)

type Options struct {
	Syntax     Syntax
	IgnoreCase bool
	// GNU enables \| \+ \? in BRE, back-references in ERE, {,n}, and the
	// escapes \< \> \b \B \w \W \s \S \` \' \n.
	GNU bool
	// Line requires the match to span the whole input (grep -x).
	Line bool
	// Word requires the match not to touch a word character on either side
	// (grep -w).
	Word bool
	// Literal treats the pattern as a fixed string (grep -F).
	Literal bool
}

var (
	errUnmatchedOpen     = errors.New(`Unmatched ( or \(`)
	errUnmatchedClose    = errors.New(`Unmatched ) or \)`)
	errUnmatchedBracket  = errors.New("Unmatched [, [^, [:, [., or [=")
	errUnmatchedBrace    = errors.New(`Unmatched \{`)
	errBadClass          = errors.New("Invalid character class name")
	errBadCollation      = errors.New("Invalid collation character")
	errBadRange          = errors.New("Invalid range end")
	errBadBackref        = errors.New("Invalid back reference")
	errBadInterval       = errors.New(`Invalid content of \{\}`)
	errBadPreceding      = errors.New("Invalid preceding regular expression")
	errTrailingBackslash = errors.New("Trailing backslash")
	errTooBig            = errors.New("Regular expression too big")
)

// ErrTooComplex reports that a pattern with back-references needed more
// search steps than the matcher allows for one call.
var ErrTooComplex = errors.New("regular expression too complex for this input")

type Regexp struct {
	prog     []inst
	groups   int
	loops    int
	backrefs bool
	prefix   []byte
	fast     *regexp.Regexp
	// fastAnywhere is false when the pattern looks at the text before the
	// search position (^, \b, \B), which the fast path cannot see.
	fastAnywhere bool
}

func Compile(pattern string, opt Options) (*Regexp, error) {
	return CompileAll([]string{pattern}, opt)
}

// CompileAll matches any of the patterns, choosing the leftmost-longest
// match among them. Groups are numbered across patterns in order, and each
// pattern's back-references refer to its own groups.
func CompileAll(patterns []string, opt Options) (*Regexp, error) {
	alt := &node{kind: nAlt}
	groups := 0
	for _, pat := range patterns {
		n, count, err := parse(pat, opt, groups)
		if err != nil {
			return nil, err
		}
		groups += count
		alt.subs = append(alt.subs, n)
	}
	root := alt
	if len(alt.subs) == 1 {
		root = alt.subs[0]
	}
	root = constrain(root, opt)
	prog, loops, err := compileProgram(root, opt.IgnoreCase)
	if err != nil {
		return nil, err
	}
	re := &Regexp{prog: prog, groups: groups, loops: loops}
	scan(root, re)
	if !opt.IgnoreCase {
		prefix, _ := literalPrefix(root)
		re.prefix = []byte(prefix)
	}
	re.fast = fastPath(root, opt.IgnoreCase)
	return re, nil
}

func MustCompile(pattern string, opt Options) *Regexp {
	re, err := Compile(pattern, opt)
	if err != nil {
		panic("posixre: " + err.Error())
	}
	return re
}

func MustCompileAll(patterns []string, opt Options) *Regexp {
	re, err := CompileAll(patterns, opt)
	if err != nil {
		panic("posixre: " + err.Error())
	}
	return re
}

func constrain(root *node, opt Options) *node {
	switch {
	case opt.Line:
		return &node{kind: nCat, subs: []*node{{kind: nAssert, assert: aBegin}, root, {kind: nAssert, assert: aEnd}}}
	case opt.Word:
		return &node{kind: nCat, subs: []*node{{kind: nAssert, assert: aNoWordBefore}, root, {kind: nAssert, assert: aNoWordAfter}}}
	}
	return root
}

func scan(n *node, re *Regexp) {
	re.fastAnywhere = true
	walk(n, func(n *node) {
		switch {
		case n.kind == nBackref:
			re.backrefs = true
		case n.kind == nAssert && n.assert != aEnd:
			re.fastAnywhere = false
		}
	})
}

// literalPrefix returns the text every match starts with, and whether n
// matches exactly that text.
func literalPrefix(n *node) (string, bool) {
	switch n.kind {
	case nLit:
		if n.r >= invalidMin {
			return "", false
		}
		return string(n.r), true
	case nAssert:
		return "", true
	case nGroup:
		return literalPrefix(n.subs[0])
	case nCat:
		var b strings.Builder
		for _, sub := range n.subs {
			p, whole := literalPrefix(sub)
			b.WriteString(p)
			if !whole {
				return b.String(), false
			}
		}
		return b.String(), true
	}
	return "", false
}

func walk(n *node, fn func(*node)) {
	fn(n)
	for _, sub := range n.subs {
		walk(sub, fn)
	}
}

func (re *Regexp) NumSubexp() int { return re.groups }

// Find returns the leftmost-longest match starting at or after pos as
// byte offsets: the match, then each group, with -1 for unset groups.
// Anchors and word boundaries see the text before pos. It returns nil when
// nothing matches.
func (re *Regexp) Find(b []byte, pos int) ([]int, error) {
	if re.fast != nil && (pos == 0 || re.fastAnywhere) {
		m := re.fast.FindSubmatchIndex(b[pos:])
		if m == nil {
			return nil, nil
		}
		for i := range m {
			if m[i] >= 0 {
				m[i] += pos
			}
		}
		return m, nil
	}
	return re.newMachine(b, pos).find(pos, re.prefix)
}

func (re *Regexp) Match(b []byte) (bool, error) {
	if re.fast != nil {
		return re.fast.Match(b), nil
	}
	m, err := re.Find(b, 0)
	return m != nil, err
}

// FindAll returns successive non-overlapping matches, at most n when n is
// not negative. Like Go's regexp, it skips an empty match that directly
// follows the previous match.
func (re *Regexp) FindAll(b []byte, n int) ([][]int, error) {
	var out [][]int
	prevEnd := -1
	for pos := 0; pos <= len(b) && (n < 0 || len(out) < n); {
		m, err := re.Find(b, pos)
		if err != nil || m == nil {
			return out, err
		}
		accept := true
		if m[1] == m[0] {
			accept = m[0] != prevEnd
			if m[1] < len(b) {
				_, w := decodeBytes(b, m[1])
				pos = m[1] + w
			} else {
				pos = len(b) + 1
			}
		} else {
			pos = m[1]
		}
		prevEnd = m[1]
		if accept {
			out = append(out, m)
		}
	}
	return out, nil
}

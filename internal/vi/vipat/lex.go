package vipat

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type tokKind uint8

const (
	tLit tokKind = iota
	tAny
	tSet
	tStar
	tPlus
	tQuest
	tBrace
	tOpen
	tClose
	tAlt
	tBol
	tEol
	tBow
	tEow
)

type token struct {
	kind     tokKind
	r        rune
	set      *charSet
	min, max int
	greedy   bool
}

type caseMode int8

const (
	caseDefault caseMode = iota
	caseIgnore
	caseMatch
)

// toggleChars are the characters whose meaning flips with a backslash; which
// side is special depends on the magic mode.
const toggleChars = ".*[~+=?{()|&<>@%"

type lexer struct {
	s    string
	i    int
	mode byte
	cas  caseMode
	toks []token
}

func lex(s string) ([]token, caseMode, error) {
	l := &lexer{s: s, mode: 'm'}
	for l.more() {
		if err := l.next(); err != nil {
			return nil, 0, err
		}
	}
	return l.toks, l.cas, nil
}

func unsupported(construct string) error { return &UnsupportedError{construct} }

func (l *lexer) emit(t token)   { l.toks = append(l.toks, t) }
func (l *lexer) lit(r rune)     { l.emit(token{kind: tLit, r: r}) }
func (l *lexer) more() bool     { return l.i < len(l.s) }
func (l *lexer) peekRune() rune { r, _ := utf8.DecodeRuneInString(l.s[l.i:]); return r }
func (l *lexer) read() rune {
	r, n := utf8.DecodeRuneInString(l.s[l.i:])
	l.i += n
	return r
}

func (l *lexer) unescapedSpecial(c rune) bool {
	switch l.mode {
	case 'v':
		return true
	case 'm':
		return strings.ContainsRune(".*[~", c)
	}
	return false
}

func (l *lexer) next() error {
	c := l.read()
	escaped := false
	if c == '\\' {
		if !l.more() {
			l.lit('\\')
			return nil
		}
		c, escaped = l.read(), true
	}
	if strings.ContainsRune(toggleChars, c) {
		if l.unescapedSpecial(c) != escaped {
			return l.operator(c)
		}
		l.lit(c)
		return nil
	}
	if !escaped {
		switch c {
		case '^':
			l.emit(token{kind: tBol})
		case '$':
			l.emit(token{kind: tEol})
		default:
			l.lit(c)
		}
		return nil
	}
	return l.escape(c)
}

func (l *lexer) operator(c rune) error {
	simple := map[rune]tokKind{'.': tAny, '*': tStar, '+': tPlus, '=': tQuest, '?': tQuest,
		'(': tOpen, ')': tClose, '|': tAlt, '<': tBow, '>': tEow}
	if kind, ok := simple[c]; ok {
		l.emit(token{kind: kind})
		return nil
	}
	switch c {
	case '[':
		set, ok, err := l.collection()
		if err != nil {
			return err
		}
		if !ok {
			l.lit('[')
			return nil
		}
		l.emit(token{kind: tSet, set: set})
	case '{':
		return l.brace()
	case '~':
		return unsupported(`~ (write \~ for a literal tilde)`)
	case '&':
		return unsupported(`\&`)
	case '@':
		return unsupported(`\@` + l.takeWhile("0123456789<=!>", 3))
	case '%':
		if !l.more() {
			return unsupported(`\%`)
		}
		if l.peekRune() == '(' {
			l.read()
			l.emit(token{kind: tOpen})
			return nil
		}
		return unsupported(`\%` + string(l.read()))
	}
	return nil
}

func (l *lexer) takeWhile(set string, limit int) string {
	start := l.i
	for n := 0; n < limit && l.more() && strings.ContainsRune(set, l.peekRune()); n++ {
		l.read()
	}
	return l.s[start:l.i]
}

func (l *lexer) escape(c rune) error {
	if set, ok := escapeClass(c); ok {
		l.emit(token{kind: tSet, set: set})
		return nil
	}
	switch c {
	case 't':
		l.lit('\t')
	case 'e':
		l.lit(0x1b)
	case 'r':
		l.lit('\r')
	case 'b':
		l.lit('\b')
	case 'n':
		return unsupported(`\n (multiline patterns)`)
	case 'c':
		l.cas = caseIgnore
	case 'C':
		l.cas = caseMatch
	case 'v', 'm', 'M', 'V':
		l.mode = byte(c)
	case 'z', '_':
		if !l.more() {
			return unsupported(`\` + string(c))
		}
		return unsupported(`\` + string(c) + string(l.read()))
	default:
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			return unsupported(`\` + string(c))
		}
		l.lit(c)
	}
	return nil
}

func (l *lexer) brace() error {
	syntax := fmt.Errorf(`E554: Syntax error in \{...}`)
	end := strings.IndexByte(l.s[l.i:], '}')
	if end < 0 {
		return syntax
	}
	body := strings.TrimSuffix(l.s[l.i:l.i+end], `\`)
	l.i += end + 1
	t := token{kind: tBrace, greedy: true, max: -1}
	if rest, lazy := strings.CutPrefix(body, "-"); lazy {
		t.greedy, body = false, rest
	}
	lo, hi, comma := strings.Cut(body, ",")
	var ok bool
	if lo != "" {
		if t.min, ok = atoiLimit(lo); !ok {
			return syntax
		}
	}
	switch {
	case !comma && lo != "":
		t.max = t.min
	case hi != "":
		if t.max, ok = atoiLimit(hi); !ok {
			return syntax
		}
	}
	if t.max >= 0 && t.min > t.max {
		t.min, t.max = t.max, t.min
	}
	l.emit(t)
	return nil
}

func atoiLimit(s string) (int, bool) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = min(n*10+int(c-'0'), 1<<24)
	}
	return n, true
}

// collection parses the part of a [...] after the opening bracket. ok is
// false when there is no closing bracket, in which case Vim reads a literal [.
func (l *lexer) collection() (set *charSet, ok bool, err error) {
	rest := l.s[l.i:]
	i := 0
	set = &charSet{}
	if strings.HasPrefix(rest, "^") {
		set.neg = true
		i++
	}
	for first := true; i < len(rest); first = false {
		c, _ := utf8.DecodeRuneInString(rest[i:])
		if c == ']' && !first {
			l.i += i + 1
			return set, true, nil
		}
		if c == '[' && i+1 < len(rest) && strings.IndexByte(":=.", rest[i+1]) >= 0 {
			delim := rest[i+1]
			if end := strings.Index(rest[i+2:], string(delim)+"]"); end >= 0 {
				name := rest[i+2 : i+2+end]
				if delim != ':' {
					return nil, false, unsupported(fmt.Sprintf("[%c%s%c]", delim, name, delim))
				}
				if pred, known := bracketClasses[name]; known {
					set.preds = append(set.preds, pred)
					i += 2 + end + 2
					continue
				}
			}
		}
		lo, w, err := collectionRune(rest[i:])
		if err != nil {
			return nil, false, err
		}
		i += w
		hi := lo
		if i+1 < len(rest) && rest[i] == '-' && rest[i+1] != ']' {
			var w2 int
			if hi, w2, err = collectionRune(rest[i+1:]); err != nil {
				return nil, false, err
			}
			if hi < lo {
				return nil, false, fmt.Errorf("E944: Reverse range in character class")
			}
			i += 1 + w2
		}
		set.ranges = append(set.ranges, [2]rune{lo, hi})
	}
	return nil, false, nil
}

// collectionRune reads one possibly escaped member and its byte width.
func collectionRune(s string) (rune, int, error) {
	c, n := utf8.DecodeRuneInString(s)
	if c != '\\' || n >= len(s) {
		return c, n, nil
	}
	d, dn := utf8.DecodeRuneInString(s[n:])
	switch d {
	case 'e':
		return 0x1b, n + dn, nil
	case 't':
		return '\t', n + dn, nil
	case 'r':
		return '\r', n + dn, nil
	case 'b':
		return '\b', n + dn, nil
	case '\\', ']', '^', '-':
		return d, n + dn, nil
	case 'n':
		return 0, 0, unsupported(`\n in a collection`)
	case 'd', 'o', 'x', 'u', 'U':
		if n+dn < len(s) && strings.IndexByte("0123456789abcdefABCDEF", s[n+dn]) >= 0 {
			return 0, 0, unsupported(`\` + string(d) + ` character code in a collection`)
		}
	}
	return '\\', n, nil
}

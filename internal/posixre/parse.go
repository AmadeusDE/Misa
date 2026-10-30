package posixre

import (
	"strings"
	"unicode/utf8"
)

type nodeKind uint8

const (
	nEmpty nodeKind = iota
	nLit
	nAny
	nClass
	nCat
	nAlt
	nRepeat
	nGroup
	nBackref
	nAssert
)

type assertion uint8

const (
	aBegin assertion = iota
	aEnd
	aWordBoundary
	aNotWordBoundary
	aWordStart
	aWordEnd
	aNoWordBefore
	aNoWordAfter
)

type node struct {
	kind     nodeKind
	r        rune
	class    *charClass
	subs     []*node
	min, max int
	index    int
	assert   assertion
}

const (
	repeatMax  = 32767
	unbounded  = -1
	invalidMin = 0x110000
)

// decodeRune maps each byte of an invalid UTF-8 sequence to its own rune
// above the Unicode range, so invalid input still matches byte for byte.
func decodeRune(s string, i int) (rune, int) {
	r, w := utf8.DecodeRuneInString(s[i:])
	if r == utf8.RuneError && w == 1 {
		return invalidMin + rune(s[i]), 1
	}
	return r, w
}

func decodeBytes(b []byte, i int) (rune, int) {
	r, w := utf8.DecodeRune(b[i:])
	if r == utf8.RuneError && w == 1 {
		return invalidMin + rune(b[i]), 1
	}
	return r, w
}

type parser struct {
	src    string
	pos    int
	ere    bool
	gnu    bool
	base   int
	groups int
	closed []bool
}

func parse(pattern string, opt Options, groupBase int) (*node, int, error) {
	if opt.Literal {
		return literal(pattern), 0, nil
	}
	p := &parser{src: pattern, ere: opt.Syntax == Extended, gnu: opt.GNU, base: groupBase, groups: groupBase}
	p.closed = make([]bool, groupBase+1)
	n, err := p.alternation(0)
	if err != nil {
		return nil, 0, err
	}
	return n, p.groups - groupBase, nil
}

func literal(s string) *node {
	cat := &node{kind: nCat}
	for i := 0; i < len(s); {
		r, w := decodeRune(s, i)
		cat.subs = append(cat.subs, &node{kind: nLit, r: r})
		i += w
	}
	return cat
}

func (p *parser) eof() bool { return p.pos >= len(p.src) }

func (p *parser) has(s string) bool { return strings.HasPrefix(p.src[p.pos:], s) }

func (p *parser) atAlt() bool {
	if p.ere {
		return p.has("|")
	}
	return p.gnu && p.has(`\|`)
}

func (p *parser) closeOperator() string {
	if p.ere {
		return ")"
	}
	return `\)`
}

func (p *parser) alternation(depth int) (*node, error) {
	alt := &node{kind: nAlt}
	for {
		branch, err := p.branch(depth)
		if err != nil {
			return nil, err
		}
		alt.subs = append(alt.subs, branch)
		if !p.atAlt() {
			break
		}
		p.pos += len(p.token("|"))
	}
	if len(alt.subs) == 1 {
		return alt.subs[0], nil
	}
	return alt, nil
}

func (p *parser) branchEnds(depth int) bool {
	if p.eof() || p.atAlt() {
		return true
	}
	return depth > 0 && p.has(p.closeOperator())
}

func (p *parser) branch(depth int) (*node, error) {
	cat := &node{kind: nCat}
	atStart, sawCaret := true, false
	for !p.branchEnds(depth) {
		if atStart && !sawCaret && p.src[p.pos] == '^' {
			p.pos++
			sawCaret = true
			cat.subs = append(cat.subs, &node{kind: nAssert, assert: aBegin})
			continue
		}
		atom, err := p.atom(depth, atStart)
		if err != nil {
			return nil, err
		}
		atStart = false
		if atom, err = p.quantifiers(atom); err != nil {
			return nil, err
		}
		cat.subs = append(cat.subs, atom)
	}
	if len(cat.subs) == 1 {
		return cat.subs[0], nil
	}
	if len(cat.subs) == 0 {
		return &node{kind: nEmpty}, nil
	}
	return cat, nil
}

func (p *parser) quantifiers(atom *node) (*node, error) {
	for !p.eof() {
		min, max, ok, err := p.quantifier()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		atom = &node{kind: nRepeat, subs: []*node{atom}, min: min, max: max}
	}
	return atom, nil
}

func (p *parser) quantifier() (min, max int, ok bool, err error) {
	c := p.src[p.pos]
	switch {
	case c == '*':
		p.pos++
		return 0, unbounded, true, nil
	case p.ere && c == '+', !p.ere && p.gnu && p.has(`\+`):
		p.pos += len(p.token("+"))
		return 1, unbounded, true, nil
	case p.ere && c == '?', !p.ere && p.gnu && p.has(`\?`):
		p.pos += len(p.token("?"))
		return 0, 1, true, nil
	case p.ere && c == '{' && p.intervalFollows(), !p.ere && p.has(`\{`):
		p.pos += len(p.token("{"))
		min, max, err = p.interval()
		return min, max, err == nil, err
	}
	return 0, 0, false, nil
}

// token spells an operator for the syntax: bare in ERE,
// backslash-escaped in BRE.
func (p *parser) token(op string) string {
	if p.ere {
		return op
	}
	return `\` + op
}

func (p *parser) intervalFollows() bool {
	if p.pos+1 >= len(p.src) {
		return false
	}
	c := p.src[p.pos+1]
	return isDigit(c) || (p.gnu && c == ',')
}

func (p *parser) interval() (int, int, error) {
	closer := p.token("}")
	end := strings.Index(p.src[p.pos:], closer)
	if end < 0 {
		return 0, 0, errUnmatchedBrace
	}
	body := p.src[p.pos : p.pos+end]
	p.pos += end + len(closer)
	lo, hi, comma := strings.Cut(body, ",")
	if lo == "" && !(p.gnu && comma) {
		return 0, 0, errBadInterval
	}
	min, ok := parseCount(lo, 0)
	if !ok {
		return 0, 0, errBadInterval
	}
	max := min
	if comma {
		if max, ok = parseCount(hi, unbounded); !ok {
			return 0, 0, errBadInterval
		}
	}
	if min > repeatMax || max > repeatMax {
		return 0, 0, errTooBig
	}
	if max != unbounded && max < min {
		return 0, 0, errBadInterval
	}
	return min, max, nil
}

func parseCount(s string, empty int) (int, bool) {
	if s == "" {
		return empty, true
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return 0, false
		}
		if n <= repeatMax {
			n = n*10 + int(s[i]-'0')
		}
	}
	return n, true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func (p *parser) atom(depth int, atStart bool) (*node, error) {
	c := p.src[p.pos]
	if atStart {
		if lit, ok := p.leadingLiteral(); ok {
			return lit, nil
		}
		if !p.ere && p.has(`\{`) {
			return nil, errBadPreceding
		}
	}
	switch {
	case c == '.':
		p.pos++
		return &node{kind: nAny}, nil
	case c == '[':
		p.pos++
		cls, err := p.bracket()
		return &node{kind: nClass, class: cls}, err
	case p.ere && c == '^':
		p.pos++
		return &node{kind: nAssert, assert: aBegin}, nil
	case c == '$' && p.dollarIsAnchor(depth):
		p.pos++
		return &node{kind: nAssert, assert: aEnd}, nil
	case p.ere && c == '(':
		p.pos++
		return p.group(depth)
	case p.ere && c == ')':
		p.pos++
		return &node{kind: nLit, r: ')'}, nil
	case c == '\\':
		return p.escape(depth)
	}
	r, w := decodeRune(p.src, p.pos)
	p.pos += w
	return &node{kind: nLit, r: r}, nil
}

// leadingLiteral treats a repetition operator with nothing to repeat as an
// ordinary character, as POSIX requires for BRE and GNU does for ERE.
func (p *parser) leadingLiteral() (*node, bool) {
	c := p.src[p.pos]
	switch {
	case c == '*', p.ere && (c == '+' || c == '?' || c == '{'):
		p.pos++
		return &node{kind: nLit, r: rune(c)}, true
	case !p.ere && p.gnu && (p.has(`\+`) || p.has(`\?`)):
		p.pos += 2
		return &node{kind: nLit, r: rune(p.src[p.pos-1])}, true
	}
	return nil, false
}

func (p *parser) dollarIsAnchor(depth int) bool {
	if p.ere {
		return true
	}
	rest := p.src[p.pos+1:]
	return rest == "" || (depth > 0 && strings.HasPrefix(rest, `\)`)) || (p.gnu && strings.HasPrefix(rest, `\|`))
}

func (p *parser) group(depth int) (*node, error) {
	p.groups++
	index := p.groups
	p.closed = append(p.closed, false)
	sub, err := p.alternation(depth + 1)
	if err != nil {
		return nil, err
	}
	if !p.has(p.closeOperator()) {
		return nil, errUnmatchedOpen
	}
	p.pos += len(p.closeOperator())
	p.closed[index] = true
	return &node{kind: nGroup, subs: []*node{sub}, index: index}, nil
}

func (p *parser) escape(depth int) (*node, error) {
	if p.pos+1 >= len(p.src) {
		return nil, errTrailingBackslash
	}
	c := p.src[p.pos+1]
	p.pos += 2
	switch {
	case !p.ere && c == '(':
		return p.group(depth)
	case !p.ere && c == ')':
		return nil, errUnmatchedClose
	case c >= '1' && c <= '9' && (!p.ere || p.gnu):
		return p.backref(int(c - '0'))
	}
	if p.gnu {
		if n, ok := gnuEscape(c); ok {
			return n, nil
		}
	}
	r, w := decodeRune(p.src, p.pos-1)
	p.pos += w - 1
	return &node{kind: nLit, r: r}, nil
}

func (p *parser) backref(n int) (*node, error) {
	index := p.base + n
	if index >= len(p.closed) || !p.closed[index] {
		return nil, errBadBackref
	}
	return &node{kind: nBackref, index: index}, nil
}

func gnuEscape(c byte) (*node, bool) {
	switch c {
	case '<':
		return &node{kind: nAssert, assert: aWordStart}, true
	case '>':
		return &node{kind: nAssert, assert: aWordEnd}, true
	case 'b':
		return &node{kind: nAssert, assert: aWordBoundary}, true
	case 'B':
		return &node{kind: nAssert, assert: aNotWordBoundary}, true
	case '`':
		return &node{kind: nAssert, assert: aBegin}, true
	case '\'':
		return &node{kind: nAssert, assert: aEnd}, true
	case 'w', 'W':
		return &node{kind: nClass, class: &charClass{ranges: wordRanges, negate: c == 'W'}}, true
	case 's', 'S':
		return &node{kind: nClass, class: &charClass{ranges: spaceRanges, negate: c == 'S'}}, true
	case 'n':
		return &node{kind: nLit, r: '\n'}, true
	}
	return nil, false
}

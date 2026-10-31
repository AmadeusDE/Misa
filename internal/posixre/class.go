package posixre

import (
	"strings"
	"unicode"
)

type charClass struct {
	ranges []rune // lo, hi pairs
	negate bool
}

var (
	wordRanges  = []rune{'0', '9', 'A', 'Z', '_', '_', 'a', 'z'}
	spaceRanges = []rune{'\t', '\r', ' ', ' '}
)

// namedClasses follow the C locale: membership is ASCII only.
var namedClasses = map[string][]rune{
	"alnum":  {'0', '9', 'A', 'Z', 'a', 'z'},
	"alpha":  {'A', 'Z', 'a', 'z'},
	"blank":  {'\t', '\t', ' ', ' '},
	"cntrl":  {0, 0x1f, 0x7f, 0x7f},
	"digit":  {'0', '9'},
	"graph":  {'!', '~'},
	"lower":  {'a', 'z'},
	"print":  {' ', '~'},
	"punct":  {'!', '/', ':', '@', '[', '`', '{', '~'},
	"space":  spaceRanges,
	"upper":  {'A', 'Z'},
	"xdigit": {'0', '9', 'A', 'F', 'a', 'f'},
}

func (c *charClass) contains(r rune) bool {
	for i := 0; i < len(c.ranges); i += 2 {
		if r >= c.ranges[i] && r <= c.ranges[i+1] {
			return true
		}
	}
	return false
}

func (c *charClass) matches(r rune, fold bool) bool {
	in := c.contains(r)
	if !in && fold && r < invalidMin {
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if c.contains(f) {
				in = true
				break
			}
		}
	}
	return in != c.negate
}

// bracket parses a bracket expression; p.pos is just past the opening '['.
func (p *parser) bracket() (*charClass, error) {
	cls := &charClass{}
	if p.has("^") {
		cls.negate = true
		p.pos++
	}
	first := true
	for {
		if p.eof() {
			return nil, errUnmatchedBracket
		}
		if p.src[p.pos] == ']' && !first {
			p.pos++
			return cls, nil
		}
		first = false
		if p.has("[:") {
			ranges, err := p.namedClass()
			if err != nil {
				return nil, err
			}
			cls.ranges = append(cls.ranges, ranges...)
			continue
		}
		lo, err := p.bracketChar()
		if err != nil {
			return nil, err
		}
		hi := lo
		if p.has("-") && p.pos+1 < len(p.src) && p.src[p.pos+1] != ']' {
			p.pos++
			if p.has("[:") {
				return nil, errBadRange
			}
			if hi, err = p.bracketChar(); err != nil {
				return nil, err
			}
			if hi < lo {
				return nil, errBadRange
			}
		}
		cls.ranges = append(cls.ranges, lo, hi)
	}
}

func (p *parser) namedClass() ([]rune, error) {
	end := strings.Index(p.src[p.pos+2:], ":]")
	if end < 0 {
		return nil, errUnmatchedBracket
	}
	name := p.src[p.pos+2 : p.pos+2+end]
	p.pos += end + 4
	ranges, ok := namedClasses[name]
	if !ok {
		return nil, errBadClass
	}
	return ranges, nil
}

// bracketChar reads one character, including [.c.] and [=c=] forms, which
// name a single character in the C locale.
func (p *parser) bracketChar() (rune, error) {
	if p.has("[.") || p.has("[=") {
		delim := p.src[p.pos+1 : p.pos+2]
		end := strings.Index(p.src[p.pos+2:], delim+"]")
		if end < 0 {
			return 0, errUnmatchedBracket
		}
		name := p.src[p.pos+2 : p.pos+2+end]
		p.pos += end + 4
		if name == "" {
			return 0, errBadCollation
		}
		r, w := decodeRune(name, 0)
		if w != len(name) {
			return 0, errBadCollation
		}
		return r, nil
	}
	r, w := decodeRune(p.src, p.pos)
	p.pos += w
	return r, nil
}

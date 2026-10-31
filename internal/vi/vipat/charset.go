package vipat

import "unicode"

type charSet struct {
	neg, noFold bool
	ranges      [][2]rune
	preds       []func(rune) bool
}

func (s *charSet) contains(r rune) bool {
	for _, rg := range s.ranges {
		if r >= rg[0] && r <= rg[1] {
			return true
		}
	}
	for _, p := range s.preds {
		if p(r) {
			return true
		}
	}
	return false
}

func (s *charSet) match(r rune, fold bool) bool {
	in := s.contains(r)
	if !in && fold && !s.noFold {
		for f := unicode.SimpleFold(r); f != r && !in; f = unicode.SimpleFold(f) {
			in = s.contains(f)
		}
	}
	return in != s.neg
}

func foldEqual(a, b rune) bool {
	if a == b {
		return true
	}
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}

func isWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func rangePred(pairs ...rune) func(rune) bool {
	return func(r rune) bool {
		for i := 0; i < len(pairs); i += 2 {
			if r >= pairs[i] && r <= pairs[i+1] {
				return true
			}
		}
		return false
	}
}

var escapeClasses = map[rune]func(rune) bool{
	's': rangePred(' ', ' ', '\t', '\t'),
	'd': rangePred('0', '9'),
	'w': rangePred('0', '9', 'a', 'z', 'A', 'Z', '_', '_'),
	'a': rangePred('a', 'z', 'A', 'Z'),
	'l': rangePred('a', 'z'),
	'u': rangePred('A', 'Z'),
	'x': rangePred('0', '9', 'a', 'f', 'A', 'F'),
	'o': rangePred('0', '7'),
	'h': rangePred('a', 'z', 'A', 'Z', '_', '_'),
}

func escapeClass(c rune) (*charSet, bool) {
	lower := unicode.ToLower(c)
	pred, ok := escapeClasses[lower]
	if !ok {
		return nil, false
	}
	return &charSet{preds: []func(rune) bool{pred}, neg: lower != c, noFold: true}, true
}

var bracketClasses = map[string]func(rune) bool{
	"alpha":     unicode.IsLetter,
	"digit":     rangePred('0', '9'),
	"alnum":     func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) },
	"lower":     unicode.IsLower,
	"upper":     unicode.IsUpper,
	"space":     rangePred(' ', ' ', '\t', '\r', '\v', '\f'),
	"blank":     rangePred(' ', ' ', '\t', '\t'),
	"punct":     func(r rune) bool { return r < 128 && (unicode.IsPunct(r) || unicode.IsSymbol(r)) },
	"cntrl":     unicode.IsControl,
	"graph":     func(r rune) bool { return unicode.IsGraphic(r) && !unicode.IsSpace(r) },
	"print":     unicode.IsGraphic,
	"xdigit":    rangePred('0', '9', 'a', 'f', 'A', 'F'),
	"return":    rangePred('\r', '\r'),
	"tab":       rangePred('\t', '\t'),
	"escape":    rangePred(0x1b, 0x1b),
	"backspace": rangePred('\b', '\b'),
}

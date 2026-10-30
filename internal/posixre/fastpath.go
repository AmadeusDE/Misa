package posixre

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// fastPath translates patterns Go's regexp can express exactly into its
// syntax, so common searches run on the standard library engine. It returns
// nil for back-references, \< \> and grep -w, and for anything that names
// U+FFFD or a raw invalid byte, which Go would conflate.
func fastPath(root *node, icase bool) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("(?s)")
	if icase {
		b.WriteString("(?i)")
	}
	if !emitGo(&b, root) {
		return nil
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	re.Longest()
	return re
}

func emitGo(b *strings.Builder, n *node) bool {
	switch n.kind {
	case nEmpty:
		b.WriteString("(?:)")
	case nLit:
		if !goSafe(n.r, n.r) {
			return false
		}
		fmt.Fprintf(b, `\x{%x}`, n.r)
	case nAny:
		b.WriteByte('.')
	case nClass:
		return emitGoClass(b, n.class)
	case nAssert:
		return emitGoAssert(b, n.assert)
	case nBackref:
		return false
	case nGroup:
		b.WriteByte('(')
		ok := emitGo(b, n.subs[0])
		b.WriteByte(')')
		return ok
	case nCat, nAlt:
		b.WriteString("(?:")
		for i, sub := range n.subs {
			if i > 0 && n.kind == nAlt {
				b.WriteByte('|')
			}
			if !emitGo(b, sub) {
				return false
			}
		}
		b.WriteByte(')')
	case nRepeat:
		b.WriteString("(?:")
		if !emitGo(b, n.subs[0]) {
			return false
		}
		b.WriteByte(')')
		emitGoRepeat(b, n.min, n.max)
	}
	return true
}

func goSafe(lo, hi rune) bool {
	return hi < invalidMin && (hi < utf8.RuneError || lo > utf8.RuneError)
}

func emitGoClass(b *strings.Builder, c *charClass) bool {
	b.WriteByte('[')
	if c.negate {
		b.WriteByte('^')
	}
	for i := 0; i < len(c.ranges); i += 2 {
		if !goSafe(c.ranges[i], c.ranges[i+1]) {
			return false
		}
		fmt.Fprintf(b, `\x{%x}-\x{%x}`, c.ranges[i], c.ranges[i+1])
	}
	b.WriteByte(']')
	return true
}

func emitGoAssert(b *strings.Builder, a assertion) bool {
	switch a {
	case aBegin:
		b.WriteByte('^')
	case aEnd:
		b.WriteByte('$')
	case aWordBoundary:
		b.WriteString(`\b`)
	case aNotWordBoundary:
		b.WriteString(`\B`)
	default:
		return false
	}
	return true
}

func emitGoRepeat(b *strings.Builder, min, max int) {
	switch {
	case min == 0 && max == unbounded:
		b.WriteByte('*')
	case min == 1 && max == unbounded:
		b.WriteByte('+')
	case min == 0 && max == 1:
		b.WriteByte('?')
	case max == unbounded:
		fmt.Fprintf(b, "{%d,}", min)
	default:
		fmt.Fprintf(b, "{%d,%d}", min, max)
	}
}

package vipat

import "fmt"

type kind uint8

const (
	kLit kind = iota
	kAny
	kSet
	kBol
	kEol
	kBow
	kEow
	kGroup
	kRepeat
)

type node struct {
	kind     kind
	r        rune
	set      *charSet
	alts     [][]*node
	sub      *node
	min, max int
	greedy   bool
}

func (n *node) single() bool { return n.kind == kLit || n.kind == kAny || n.kind == kSet }

type parser struct {
	toks []token
	i    int
}

func parse(toks []token) (*node, error) {
	p := &parser{toks: toks}
	g, err := p.alternation(0)
	if err != nil {
		return nil, err
	}
	if p.i < len(p.toks) {
		return nil, fmt.Errorf(`E55: Unmatched \)`)
	}
	return g, nil
}

func (p *parser) alternation(depth int) (*node, error) {
	g := &node{kind: kGroup}
	for {
		seq, err := p.sequence()
		if err != nil {
			return nil, err
		}
		g.alts = append(g.alts, seq)
		if p.i < len(p.toks) && p.toks[p.i].kind == tAlt {
			p.i++
			continue
		}
		break
	}
	if depth > 0 && (p.i >= len(p.toks) || p.toks[p.i].kind != tClose) {
		return nil, fmt.Errorf(`E54: Unmatched \(`)
	}
	return g, nil
}

func (p *parser) endsBranch() bool {
	return p.i >= len(p.toks) || p.toks[p.i].kind == tAlt || p.toks[p.i].kind == tClose
}

func (p *parser) sequence() ([]*node, error) {
	var seq []*node
	for p.i < len(p.toks) {
		t := p.toks[p.i]
		if t.kind == tAlt || t.kind == tClose {
			break
		}
		p.i++
		switch t.kind {
		case tStar, tPlus, tQuest, tBrace:
			var err error
			if seq, err = repeat(seq, t); err != nil {
				return nil, err
			}
		case tBol:
			if len(seq) == 0 {
				seq = append(seq, &node{kind: kBol})
			} else {
				seq = append(seq, &node{kind: kLit, r: '^'})
			}
		case tEol:
			if p.endsBranch() {
				seq = append(seq, &node{kind: kEol})
			} else {
				seq = append(seq, &node{kind: kLit, r: '$'})
			}
		case tOpen:
			g, err := p.alternation(1)
			if err != nil {
				return nil, err
			}
			p.i++
			seq = append(seq, g)
		case tLit:
			seq = append(seq, &node{kind: kLit, r: t.r})
		case tAny:
			seq = append(seq, &node{kind: kAny})
		case tSet:
			seq = append(seq, &node{kind: kSet, set: t.set})
		case tBow:
			seq = append(seq, &node{kind: kBow})
		case tEow:
			seq = append(seq, &node{kind: kEow})
		}
	}
	return seq, nil
}

// repeat applies a repetition operator to the last atom. A * with no atom, or
// directly after ^, is a literal, as in Vim.
func repeat(seq []*node, t token) ([]*node, error) {
	if len(seq) == 0 || seq[len(seq)-1].kind == kBol && t.kind == tStar {
		if t.kind != tStar {
			return nil, fmt.Errorf("E64: %s follows nothing", repeatName(t))
		}
		return append(seq, &node{kind: kLit, r: '*'}), nil
	}
	last := seq[len(seq)-1]
	if last.kind == kRepeat {
		return nil, fmt.Errorf("E61: Nested %s", repeatName(t))
	}
	rep := &node{kind: kRepeat, sub: last, greedy: true, max: -1}
	switch t.kind {
	case tPlus:
		rep.min = 1
	case tQuest:
		rep.max = 1
	case tBrace:
		rep.min, rep.max, rep.greedy = t.min, t.max, t.greedy
	}
	seq[len(seq)-1] = rep
	return seq, nil
}

func repeatName(t token) string {
	switch t.kind {
	case tStar:
		return "*"
	case tPlus:
		return `\+`
	case tQuest:
		return `\=`
	}
	return `\{`
}

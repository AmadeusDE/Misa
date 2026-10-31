package vipat

import "unicode/utf8"

type matcher struct {
	s     string
	fold  bool
	steps int
	total int
	err   error
	stack []int
}

func (m *matcher) tick() bool {
	m.steps++
	m.total++
	if m.steps > attemptSteps || m.total > totalSteps {
		m.err = ErrTooComplex
	}
	return m.err != nil
}

func (m *matcher) step(n *node, pos int) (int, bool) {
	if pos >= len(m.s) {
		return 0, false
	}
	r, w := utf8.DecodeRuneInString(m.s[pos:])
	switch n.kind {
	case kLit:
		if r != n.r && !(m.fold && foldEqual(r, n.r)) {
			return 0, false
		}
	case kSet:
		if !n.set.match(r, m.fold) {
			return 0, false
		}
	}
	return pos + w, true
}

func (m *matcher) wordBefore(pos int) bool {
	if pos == 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(m.s[:pos])
	return isWord(r)
}

func (m *matcher) wordAfter(pos int) bool {
	if pos >= len(m.s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(m.s[pos:])
	return isWord(r)
}

// seq matches items at pos, then calls k with the end position.
func (m *matcher) seq(items []*node, pos int, k func(int) bool) bool {
	for len(items) > 0 {
		if m.tick() {
			return false
		}
		n := items[0]
		switch n.kind {
		case kLit, kAny, kSet:
			p, ok := m.step(n, pos)
			if !ok {
				return false
			}
			pos = p
		case kBol:
			if pos != 0 {
				return false
			}
		case kEol:
			if pos != len(m.s) {
				return false
			}
		case kBow:
			if !m.wordAfter(pos) || m.wordBefore(pos) {
				return false
			}
		case kEow:
			if !m.wordBefore(pos) || m.wordAfter(pos) {
				return false
			}
		case kGroup:
			rest := items[1:]
			for _, alt := range n.alts {
				if m.seq(alt, pos, func(p int) bool { return m.seq(rest, p, k) }) {
					return true
				}
				if m.err != nil {
					return false
				}
			}
			return false
		case kRepeat:
			if n.sub.single() {
				return m.repeatSingle(n, items[1:], pos, k)
			}
			return m.repeatGroup(n, items[1:], 0, pos, k)
		}
		items = items[1:]
	}
	return k(pos)
}

func (m *matcher) repeatSingle(n *node, rest []*node, pos int, k func(int) bool) bool {
	if !n.greedy {
		return m.lazySingle(n, rest, pos, k)
	}
	base := len(m.stack)
	defer func() { m.stack = m.stack[:base] }()
	m.stack = append(m.stack, pos)
	for p := pos; n.max < 0 || len(m.stack)-base-1 < n.max; {
		if m.tick() {
			return false
		}
		np, ok := m.step(n.sub, p)
		if !ok {
			break
		}
		m.stack = append(m.stack, np)
		p = np
	}
	for i := len(m.stack) - 1; i-base >= n.min; i-- {
		if m.seq(rest, m.stack[i], k) {
			return true
		}
		if m.err != nil {
			return false
		}
	}
	return false
}

func (m *matcher) lazySingle(n *node, rest []*node, pos int, k func(int) bool) bool {
	for count := 0; ; count++ {
		if count >= n.min {
			if m.seq(rest, pos, k) {
				return true
			}
			if m.err != nil {
				return false
			}
		}
		if n.max >= 0 && count >= n.max || m.tick() {
			return false
		}
		np, ok := m.step(n.sub, pos)
		if !ok {
			return false
		}
		pos = np
	}
}

func (m *matcher) repeatGroup(n *node, rest []*node, count, pos int, k func(int) bool) bool {
	more := func() bool {
		if n.max >= 0 && count >= n.max {
			return false
		}
		return m.seq([]*node{n.sub}, pos, func(p int) bool {
			if p == pos && count >= n.min {
				return false
			}
			return m.repeatGroup(n, rest, count+1, p, k)
		})
	}
	done := func() bool { return count >= n.min && m.seq(rest, pos, k) }
	if m.tick() {
		return false
	}
	if n.greedy {
		return more() || (m.err == nil && done())
	}
	return done() || (m.err == nil && more())
}

package posixre

import (
	"bytes"
	"unicode"
)

const (
	maxVisitedBits = 1 << 26
	maxSteps       = 1 << 24
)

type job struct {
	pc, pos int
	restore int // slot+1 to restore, 0 for a thread
	value   int
}

// machine runs a depth-first search over the program in priority order and
// keeps the first thread that reaches the longest end. With a visited set
// keyed by (pc, pos) this is the same choice Go's NFA makes with Longest,
// and it runs in linear time. Back-references make a thread's future depend
// on its captures, so that search drops the visited set and is bounded by
// a step budget instead.
type machine struct {
	prog    []inst
	input   []byte
	caps    []int
	best    []int
	loopAt  []int
	stack   []job
	visited []uint32
	base    int
	steps   int
}

func foldOrbit(r rune) []rune {
	var orbit []rune
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		orbit = append(orbit, f)
	}
	return orbit
}

func (re *Regexp) newMachine(input []byte, pos int) *machine {
	m := &machine{prog: re.prog, input: input, base: pos}
	m.caps = make([]int, 2*(re.groups+1))
	m.best = make([]int, len(m.caps))
	if bits := len(re.prog) * (len(input) - pos + 1); !re.backrefs && bits <= maxVisitedBits {
		m.visited = make([]uint32, (bits+31)/32)
	} else {
		m.loopAt = make([]int, re.loops)
	}
	return m
}

func (m *machine) find(start int, prefix []byte) ([]int, error) {
	for {
		if len(prefix) > 0 {
			i := bytes.Index(m.input[start:], prefix)
			if i < 0 {
				return nil, nil
			}
			start += i
		}
		matched, err := m.tryAt(start)
		if err != nil || matched {
			return m.best, err
		}
		if start >= len(m.input) {
			return nil, nil
		}
		_, w := decodeBytes(m.input, start)
		start += w
	}
}

func (m *machine) tryAt(start int) (bool, error) {
	for i := range m.caps {
		m.caps[i] = -1
	}
	for i := range m.loopAt {
		m.loopAt[i] = -1
	}
	m.best[1] = -1
	m.stack = append(m.stack[:0], job{pc: 0, pos: start})
	for len(m.stack) > 0 {
		j := m.stack[len(m.stack)-1]
		m.stack = m.stack[:len(m.stack)-1]
		if j.restore > 0 {
			m.restore(j.restore-1, j.value)
			continue
		}
		if err := m.run(j.pc, j.pos); err != nil {
			return false, err
		}
	}
	return m.best[1] >= 0, nil
}

func (m *machine) restore(slot, value int) {
	if slot < len(m.caps) {
		m.caps[slot] = value
	} else {
		m.loopAt[slot-len(m.caps)] = value
	}
}

func (m *machine) seen(pc, pos int) bool {
	if m.visited == nil {
		return false
	}
	k := pc*(len(m.input)-m.base+1) + pos - m.base
	bit := uint32(1) << (k & 31)
	if m.visited[k>>5]&bit != 0 {
		return true
	}
	m.visited[k>>5] |= bit
	return false
}

func (m *machine) run(pc, pos int) error {
	for {
		if m.seen(pc, pos) {
			return nil
		}
		if m.visited == nil {
			if m.steps++; m.steps > maxSteps {
				return ErrTooComplex
			}
		}
		in := &m.prog[pc]
		switch in.op {
		case opRune, opAny, opClass:
			if pos >= len(m.input) {
				return nil
			}
			r, w := decodeBytes(m.input, pos)
			if !in.matchRune(r) {
				return nil
			}
			pos += w
			pc = in.out
		case opNop:
			pc = in.out
		case opLoopReset:
			if m.loopAt != nil {
				m.stack = append(m.stack, job{restore: len(m.caps) + in.n + 1, value: m.loopAt[in.n]})
				m.loopAt[in.n] = -1
			}
			pc = in.out
		case opSplit:
			if in.loop >= 0 && m.loopAt != nil {
				if m.loopAt[in.loop] == pos {
					pc = in.alt
					continue
				}
				m.stack = append(m.stack, job{restore: len(m.caps) + in.loop + 1, value: m.loopAt[in.loop]})
				m.loopAt[in.loop] = pos
			}
			m.stack = append(m.stack, job{pc: in.alt, pos: pos})
			pc = in.out
		case opSave:
			if in.n < len(m.caps) {
				m.stack = append(m.stack, job{restore: in.n + 1, value: m.caps[in.n]})
				m.caps[in.n] = pos
			}
			pc = in.out
		case opAssert:
			if !m.holds(in.assert, pos) {
				return nil
			}
			pc = in.out
		case opBackref:
			w, ok := m.backref(in, pos)
			if !ok {
				return nil
			}
			pos += w
			pc = in.out
		case opMatch:
			if m.best[1] < 0 || pos > m.best[1] {
				copy(m.best, m.caps)
			}
			return nil
		}
	}
}

func (in *inst) matchRune(r rune) bool {
	switch in.op {
	case opAny:
		return true
	case opClass:
		return in.class.matches(r, in.icase)
	}
	if r == in.r {
		return true
	}
	for _, f := range in.fold {
		if r == f {
			return true
		}
	}
	return false
}

func (m *machine) backref(in *inst, pos int) (int, bool) {
	lo, hi := m.caps[2*in.n], m.caps[2*in.n+1]
	if lo < 0 || hi < 0 {
		return 0, false
	}
	want := m.input[lo:hi]
	rest := m.input[pos:]
	if in.icase {
		if len(rest) < len(want) || !bytes.EqualFold(rest[:len(want)], want) {
			return 0, false
		}
		return len(want), true
	}
	if !bytes.HasPrefix(rest, want) {
		return 0, false
	}
	return len(want), true
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func (m *machine) holds(a assertion, pos int) bool {
	before := pos > 0 && isWordByte(m.input[pos-1])
	after := pos < len(m.input) && isWordByte(m.input[pos])
	switch a {
	case aBegin:
		return pos == 0
	case aEnd:
		return pos == len(m.input)
	case aWordBoundary:
		return before != after
	case aNotWordBoundary:
		return before == after
	case aWordStart:
		return !before && after
	case aWordEnd:
		return before && !after
	case aNoWordBefore:
		return !before
	case aNoWordAfter:
		return !after
	}
	return false
}

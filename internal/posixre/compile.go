// Program layout follows Go's regexp/syntax compiler (BSD-3-Clause); see docs/CREDITS.md.
package posixre

type opcode uint8

const (
	opRune opcode = iota
	opAny
	opClass
	opSplit
	opSave
	opAssert
	opBackref
	opMatch
	opNop
	opLoopReset
)

// inst is one program instruction. For opSplit, out is preferred over alt;
// loop numbers the split when it jumps back into a repetition.
type inst struct {
	op     opcode
	r      rune
	icase  bool
	fold   []rune
	class  *charClass
	assert assertion
	n      int
	out    int
	alt    int
	loop   int
}

const maxProgram = 1 << 16

type compiler struct {
	prog  []inst
	fold  bool
	loops int
	err   error
}

// frag is a compiled fragment: its entry and the outgoing edges still to be
// connected, each encoded as pc<<1 | (1 if it is a split's alt edge).
type frag struct {
	start    int
	holes    []int
	nullable bool
}

func compileProgram(root *node, fold bool) ([]inst, int, error) {
	c := &compiler{fold: fold}
	f := c.cat(c.save(0), c.cat(c.node(root), c.save(1)))
	if c.err != nil {
		return nil, 0, c.err
	}
	c.patch(f, c.emit(inst{op: opMatch}))
	if c.err != nil {
		return nil, 0, c.err
	}
	return c.prog, c.loops, nil
}

func (c *compiler) emit(i inst) int {
	if len(c.prog) >= maxProgram {
		c.err = errTooBig
		return 0
	}
	i.loop = -1
	c.prog = append(c.prog, i)
	return len(c.prog) - 1
}

func (c *compiler) patch(f frag, target int) {
	if c.err != nil {
		return
	}
	for _, h := range f.holes {
		if h&1 == 1 {
			c.prog[h>>1].alt = target
		} else {
			c.prog[h>>1].out = target
		}
	}
}

func (c *compiler) single(i inst, nullable bool) frag {
	pc := c.emit(i)
	return frag{start: pc, holes: []int{pc << 1}, nullable: nullable}
}

func (c *compiler) save(n int) frag { return c.single(inst{op: opSave, n: n}, true) }

func (c *compiler) nop() frag { return c.single(inst{op: opNop}, true) }

func (c *compiler) cat(a, b frag) frag {
	c.patch(a, b.start)
	return frag{start: a.start, holes: b.holes, nullable: a.nullable && b.nullable}
}

func (c *compiler) alt(a, b frag) frag {
	pc := c.emit(inst{op: opSplit, out: a.start, alt: b.start})
	holes := append(append([]int{}, a.holes...), b.holes...)
	return frag{start: pc, holes: holes, nullable: a.nullable || b.nullable}
}

func (c *compiler) quest(a frag) frag {
	pc := c.emit(inst{op: opSplit, out: a.start})
	return frag{start: pc, holes: append([]int{pc<<1 | 1}, a.holes...), nullable: true}
}

// loop mirrors Go's regexp/syntax compiler so that both engines pick the
// same submatches.
func (c *compiler) loop(a frag) frag {
	pc := c.emit(inst{op: opSplit, out: a.start})
	c.patch(a, pc)
	return frag{start: pc, holes: []int{pc<<1 | 1}, nullable: a.nullable}
}

// plus of a body that can match empty records where each iteration starts,
// so the back-reference search can refuse an iteration that consumed
// nothing instead of looping forever.
func (c *compiler) plus(a frag) frag {
	if !a.nullable {
		return frag{start: a.start, holes: c.loop(a).holes}
	}
	reset := c.emit(inst{op: opLoopReset, n: c.loops, out: a.start})
	l := c.loop(a)
	if c.err == nil {
		c.prog[l.start].loop = c.loops
	}
	c.loops++
	return frag{start: reset, holes: l.holes, nullable: true}
}

func (c *compiler) star(a frag) frag {
	if a.nullable {
		return c.quest(c.plus(a))
	}
	f := c.loop(a)
	f.nullable = true
	return f
}

func (c *compiler) node(n *node) frag {
	if c.err != nil {
		return frag{}
	}
	switch n.kind {
	case nLit:
		return c.single(c.literal(n.r), false)
	case nAny:
		return c.single(inst{op: opAny}, false)
	case nClass:
		return c.single(inst{op: opClass, class: n.class, icase: c.fold}, false)
	case nAssert:
		return c.single(inst{op: opAssert, assert: n.assert}, true)
	case nBackref:
		return c.single(inst{op: opBackref, n: n.index, icase: c.fold}, true)
	case nGroup:
		return c.cat(c.save(2*n.index), c.cat(c.node(n.subs[0]), c.save(2*n.index+1)))
	case nCat:
		f := c.nop()
		for _, sub := range n.subs {
			f = c.cat(f, c.node(sub))
		}
		return f
	case nAlt:
		f := c.node(n.subs[0])
		for _, sub := range n.subs[1:] {
			f = c.alt(f, c.node(sub))
		}
		return f
	case nRepeat:
		return c.repeat(n)
	}
	return c.nop()
}

// repeat expands counted repetition the way Go's syntax.Simplify does:
// x{2,} is xx+ and x{2,4} is xx(x(x)?)?.
func (c *compiler) repeat(n *node) frag {
	sub := n.subs[0]
	switch {
	case n.min == 0 && n.max == unbounded:
		return c.star(c.node(sub))
	case n.max == unbounded:
		f := c.nop()
		for i := 0; i < n.min-1 && c.err == nil; i++ {
			f = c.cat(f, c.node(sub))
		}
		return c.cat(f, c.plus(c.node(sub)))
	case n.max == 0:
		return c.nop()
	}
	f := c.nop()
	for i := 0; i < n.min && c.err == nil; i++ {
		f = c.cat(f, c.node(sub))
	}
	if n.max > n.min {
		f = c.cat(f, c.optionalChain(sub, n.max-n.min))
	}
	return f
}

func (c *compiler) optionalChain(sub *node, count int) frag {
	if count == 1 || c.err != nil {
		return c.quest(c.node(sub))
	}
	return c.quest(c.cat(c.node(sub), c.optionalChain(sub, count-1)))
}

func (c *compiler) literal(r rune) inst {
	i := inst{op: opRune, r: r}
	if c.fold {
		i.fold = foldOrbit(r)
	}
	return i
}

package builtins

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

type sedRun struct {
	ctx      *Context
	cmds     []*sedCmd
	in       *sedInput
	out      *sedOutput
	delim    byte
	quiet    bool
	separate bool

	ps       []byte
	psNL     bool
	lineNo   int64
	lastRe   *posixre.Regexp
	quitting bool
	exitCode int
	fatal    error

	hold      []byte
	replaced  bool
	appends   []sedAppend
	lineWidth int64
	files     *sedFiles

	stdout     *sedOutput
	inPlace    *sedInPlace
	readFailed bool
	editFailed bool
}

var errSedNoPrevious = errors.New("no previous regular expression")

func newSedRun(ctx *Context, cmds []*sedCmd, files []string) *sedRun {
	out := &sedOutput{w: bufio.NewWriter(ctx.Stdout), delim: '\n'}
	return &sedRun{
		ctx:       ctx,
		cmds:      cmds,
		in:        &sedInput{ctx: ctx, names: files, delim: '\n'},
		out:       out,
		stdout:    out,
		delim:     '\n',
		lineWidth: 70,
	}
}

func (r *sedRun) run() int {
	r.in.delim = r.delim
	r.in.separate = r.separate
	if r.files, r.fatal = openSedFiles(r.ctx, r.cmds, r.out); r.fatal != nil {
		fmt.Fprintf(r.ctx.Stderr, "sed: %v\n", r.fatal)
		return 4
	}
	defer r.files.close()
	if r.inPlace != nil {
		r.editAll()
	} else {
		r.in.fill(true)
		r.process()
		r.readFailed = r.in.failed
	}
	if err := r.out.w.Flush(); err != nil && r.out.err == nil {
		r.out.err = err
	}
	if err := r.files.flush(); err != nil && r.fatal == nil {
		r.fatal = err
	}
	switch {
	case r.fatal != nil:
		fmt.Fprintf(r.ctx.Stderr, "sed: %v\n", r.fatal)
		return 4
	case r.out.err != nil:
		fmt.Fprintf(r.ctx.Stderr, "sed: couldn't write: %s\n", grepErrText(r.out.err))
		return 4
	case r.exitCode != 0:
		return r.exitCode
	case r.editFailed:
		return 4
	case r.readFailed:
		return 2
	}
	return 0
}

// process runs cycles until the input ends or a command quits.
func (r *sedRun) process() {
	for !r.quitting && r.fatal == nil {
		rec, nl, ok := r.in.read()
		if !ok {
			return
		}
		if r.in.restarted && r.separate {
			r.lineNo = 0
		}
		r.in.restarted = false
		r.ps, r.psNL = append(r.ps[:0], rec...), nl
		r.lineNo++
		r.replaced = false
		r.cycle()
		if r.out.unbuffered {
			r.out.w.Flush()
		}
	}
}

type sedAction uint8

const (
	sedEndCycle sedAction = iota // autoprint unless -n
	sedDeleted                   // d, c: no autoprint
	sedRestart                   // D: run again on what is left, without reading
)

// cycle runs the script over the pattern space, then prints it and the
// text queued by a, r and R.
func (r *sedRun) cycle() {
	for {
		action := r.execute()
		if action == sedEndCycle && !r.quiet {
			r.out.record(r.ps, r.psNL)
		}
		r.flushAppends()
		if action != sedRestart || r.quitting || r.fatal != nil {
			return
		}
	}
}

func (r *sedRun) execute() sedAction {
	for pc := 0; pc < len(r.cmds) && r.fatal == nil; {
		c := r.cmds[pc]
		if r.selects(c) == c.negate {
			if c.name == '{' {
				pc = c.jump
			} else {
				pc++
			}
			continue
		}
		pc++
		switch c.name {
		case 'b':
			pc = c.jump
		case 't', 'T':
			if r.replaced == (c.name == 't') {
				pc = c.jump
			}
			r.replaced = false
		case 'd':
			return sedDeleted
		case 'D':
			i := bytes.IndexByte(r.ps, r.delim)
			if i < 0 {
				return sedDeleted
			}
			r.ps = append(r.ps[:0], r.ps[i+1:]...)
			return sedRestart
		case 'c':
			if c.a2.kind == sedNoAddr || !c.active || c.negate {
				r.out.text(c.text)
			}
			return sedDeleted
		case 'n':
			if !r.in.hasNext && !r.in.more() {
				r.quitting = true
				return sedEndCycle
			}
			if !r.quiet {
				r.out.record(r.ps, r.psNL)
			}
			r.flushAppends()
			r.readLine(false)
		case 'N':
			if !r.in.hasNext && !r.in.more() {
				r.quitting = true
				return sedEndCycle
			}
			r.flushAppends()
			r.readLine(true)
		case 'q', 'Q':
			if c.n >= 0 {
				r.exitCode = int(c.n)
			}
			r.quitting = true
			if c.name == 'Q' {
				return sedDeleted
			}
			return sedEndCycle
		case 's':
			r.substitute(c)
		default:
			r.simple(c)
		}
	}
	return sedEndCycle
}

// readLine replaces the pattern space with the next record, or appends it
// after a delimiter for N.
func (r *sedRun) readLine(join bool) {
	rec, nl, _ := r.in.read()
	if r.in.restarted && r.separate {
		r.lineNo = 0
	}
	r.in.restarted = false
	if join {
		r.ps = append(append(r.ps, r.delim), rec...)
	} else {
		r.ps = append(r.ps[:0], rec...)
	}
	r.psNL = nl
	r.lineNo++
	r.replaced = false
}

// simple runs the commands that only touch the buffers or the output.
func (r *sedRun) simple(c *sedCmd) {
	switch c.name {
	case 'p':
		r.out.record(r.ps, r.psNL)
	case 'P':
		first, _, _ := bytes.Cut(r.ps, []byte{r.delim})
		r.out.record(first, true)
	case '=':
		r.out.text(strconv.FormatInt(r.lineNo, 10) + "\n")
	case 'a':
		r.appends = append(r.appends, sedAppend{text: c.text})
	case 'i':
		r.out.text(c.text)
	case 'r':
		r.appends = append(r.appends, sedAppend{file: c.file})
	case 'R':
		if line, ok := r.readFileLine(c.file); ok {
			r.appends = append(r.appends, sedAppend{text: line})
		}
	case 'w':
		r.writeFile(c.file, r.ps)
	case 'W':
		first, _, _ := bytes.Cut(r.ps, []byte{r.delim})
		r.writeFile(c.file, first)
	case 'g':
		r.ps = append(r.ps[:0], r.hold...)
	case 'G':
		r.ps = append(append(r.ps, r.delim), r.hold...)
	case 'h':
		r.hold = append(r.hold[:0], r.ps...)
	case 'H':
		r.hold = append(append(r.hold, r.delim), r.ps...)
	case 'x':
		r.ps, r.hold = r.hold, r.ps
	case 'z':
		r.ps = r.ps[:0]
	case 'y':
		r.ps = sedTransliterate(r.ps, c.from, c.to)
	case 'l':
		width := c.n
		if width < 0 {
			width = r.lineWidth
		}
		r.out.text(sedList(r.ps, int(width), r.delim) + string(r.delim))
	case 'F':
		r.out.text(r.in.name + "\n")
	}
}

func (r *sedRun) selects(c *sedCmd) bool {
	if c.a1.kind == sedNoAddr {
		return true
	}
	if c.a2.kind == sedNoAddr {
		return r.matchAddr(&c.a1)
	}
	if !c.active {
		if !r.startRange(c) {
			return false
		}
		if c.a1.kind != sedZero {
			return true
		}
	}
	switch c.a2.kind {
	case sedLine:
		c.active = r.lineNo < c.a2.n
	case sedPlus, sedMult:
		c.active = r.lineNo < c.end
	case sedLast:
		c.active = !r.in.last()
	case sedRegex:
		c.active = !r.matchAddr(&c.a2)
	}
	return true
}

// startRange reports whether addr1 starts the range on this line and works
// out where a numeric end lands. A regex end is first tried on the next
// line, except for 0,/re/, which may end on line 1.
func (r *sedRun) startRange(c *sedCmd) bool {
	if c.a1.kind == sedZero {
		if c.zeroDone {
			return false
		}
		c.zeroDone, c.active = true, true
		return true
	}
	if !r.matchAddr(&c.a1) {
		return false
	}
	switch c.a2.kind {
	case sedLine:
		c.active = c.a2.n > r.lineNo
	case sedPlus:
		c.end = r.lineNo + c.a2.n
		c.active = c.a2.n > 0
	case sedMult:
		c.active = c.a2.n > 0 && r.lineNo%c.a2.n != 0
		if c.active {
			c.end = (r.lineNo/c.a2.n + 1) * c.a2.n
		}
	default:
		c.active = !(c.a2.kind == sedLast && r.in.last())
	}
	return true
}

func (r *sedRun) matchAddr(a *sedAddr) bool {
	switch a.kind {
	case sedLine:
		return r.lineNo == a.n
	case sedLast:
		return r.in.last()
	case sedStep:
		if a.step <= 0 {
			return r.lineNo == a.n
		}
		return r.lineNo >= a.n && (r.lineNo-a.n)%a.step == 0
	case sedRegex:
		re := r.regex(a.re)
		if re == nil {
			return false
		}
		ok, err := re.Match(r.ps)
		if err != nil {
			r.fatal = err
		}
		return ok
	}
	return false
}

func (r *sedRun) regex(re *posixre.Regexp) *posixre.Regexp {
	if re == nil {
		if r.lastRe == nil {
			r.fatal = errSedNoPrevious
		}
		return r.lastRe
	}
	r.lastRe = re
	return re
}

func (r *sedRun) substitute(c *sedCmd) {
	re := r.regex(c.re)
	if re == nil {
		return
	}
	limit := c.nth
	if c.global {
		limit = -1
	}
	locs, err := re.FindAll(r.ps, limit)
	if err != nil {
		r.fatal = err
		return
	}
	if len(locs) < c.nth {
		return
	}
	var out []byte
	last := 0
	for _, loc := range locs[c.nth-1:] {
		out = append(out, r.ps[last:loc[0]]...)
		out = sedAppendReplacement(out, c.repl, r.ps, loc)
		last = loc[1]
	}
	r.ps = append(out, r.ps[last:]...)
	r.replaced = true
	if c.print {
		r.out.record(r.ps, r.psNL)
	}
	if c.wfile != "" {
		r.writeFile(c.wfile, r.ps)
	}
}

// sedAppendReplacement expands & and \N, applying GNU's \L \U \E (until
// changed) and \l \u (next character only) case conversions.
func sedAppendReplacement(out []byte, parts []sedReplPart, src []byte, loc []int) []byte {
	var mode, once byte
	emit := func(s []byte) {
		if mode == 0 && once == 0 {
			out = append(out, s...)
			return
		}
		for len(s) > 0 {
			ch, w := utf8.DecodeRune(s)
			switch {
			case once == 'u':
				ch = unicode.ToUpper(ch)
			case once == 'l':
				ch = unicode.ToLower(ch)
			case mode == 'U':
				ch = unicode.ToUpper(ch)
			case mode == 'L':
				ch = unicode.ToLower(ch)
			}
			once = 0
			if ch == utf8.RuneError && w == 1 {
				out = append(out, s[0])
			} else {
				out = utf8.AppendRune(out, ch)
			}
			s = s[w:]
		}
	}
	for _, p := range parts {
		switch {
		case p.caseOp == 'L' || p.caseOp == 'U':
			mode, once = p.caseOp, 0
		case p.caseOp == 'E':
			mode, once = 0, 0
		case p.caseOp != 0:
			once = p.caseOp
		case p.group < 0:
			emit([]byte(p.literal))
		case 2*p.group+1 < len(loc) && loc[2*p.group] >= 0:
			emit(src[loc[2*p.group]:loc[2*p.group+1]])
		}
	}
	return out
}

// csplit splits FILE into sections according to PATTERN operands, writing
// each section to a numbered file and printing the byte counts.
//
// Behaviour adapted from uutils coreutils csplit (MIT),
// src/uu/csplit/src/csplit.rs and src/uu/csplit/src/csplit_error.rs at commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663, and from toybox toys/pending/csplit.c
// (0BSD) at commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. Regex operands
// are POSIX basic regular expressions from internal/posixre.
//
// Deliberate differences from both references: the input is read up front
// instead of streamed, so a read error (a directory operand, say) is reported
// before any output file exists; and negative offsets are supported, which
// toybox declines ("no negative offsets").
package builtins

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

const csplitSpec = "b(suffix-format):f(prefix):k(keep-files)q(quiet)sn(digits):z(elide-empty-files)(silent)(suppress-matched)"

var csplitSpecParsed *Spec

func init() {
	var err error
	csplitSpecParsed, err = ParseSpec(csplitSpec)
	if err != nil {
		panic("builtins: bad csplit spec: " + err.Error())
	}
	Register(Command{Name: "csplit", Run: runCsplit})
}

func runCsplit(ctx *Context) int {
	flags, err := csplitSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "csplit:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, csplitSpecParsed) {
		return 0
	}

	prefix := "xx"
	if v, ok := flags.ExplicitString("f"); ok {
		prefix = v
	}
	digits := 2
	if v, ok := flags.ExplicitString("n"); ok {
		d, err := strconv.Atoi(v)
		if err != nil || d < 0 {
			fmt.Fprintf(ctx.Stderr, "csplit: invalid number: '%s'\n", v)
			return 1
		}
		digits = d
	}
	suffix := ""
	if v, ok := flags.ExplicitString("b"); ok {
		suffix = v
	}
	names, err := newCsplitName(prefix, suffix, digits)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "csplit: %s\n", err)
		return 1
	}

	operands := flags.Operands
	if len(operands) < 2 {
		fmt.Fprintln(ctx.Stderr, "csplit: missing operand")
		return 1
	}
	input, patternArgs := operands[0], operands[1:]

	// uutils opens the input before it validates the patterns, so a bad file
	// wins over a bad pattern.
	src, code := csplitOpen(ctx, input)
	if code != 0 {
		return code
	}

	pats, warnings, err := parseCsplitPatterns(patternArgs)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "csplit: %s\n", err)
		return 1
	}
	for _, w := range warnings {
		fmt.Fprintf(ctx.Stderr, "csplit: warning: %s\n", w)
	}

	lines, rerr := csplitReadLines(src)
	if rerr != nil {
		fmt.Fprintf(ctx.Stderr, "csplit: read error: %s\n", fmtOsErr(rerr))
		return 1
	}

	e := &csplitEngine{
		ctx:      ctx,
		names:    names,
		lines:    lines,
		quiet:    flags.Bool("q") || flags.Bool("s") || flags.Bool("silent"),
		elide:    flags.Bool("z"),
		keep:     flags.Bool("k"),
		suppress: flags.Bool("suppress-matched"),
	}
	if msg := e.run(pats); msg != "" {
		fmt.Fprintf(ctx.Stderr, "csplit: %s\n", msg)
		if !e.keep {
			e.removeSplits()
		}
		return 1
	}
	return 0
}

// csplitOpen returns the input stream and an exit code.
func csplitOpen(ctx *Context, name string) (io.Reader, int) {
	if name == "-" {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "csplit: stdin not available")
			return nil, 1
		}
		return ctx.Stdin, 0
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(ctx.Dir, path)
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "csplit: cannot open '%s' for reading: %s\n", name, fmtOsErr(err))
		return nil, 1
	}
	return f, 0
}

// csplitReadLines reads every line, keeping the line terminator.
func csplitReadLines(r io.Reader) ([][]byte, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var lines [][]byte
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if err != nil {
			if err == io.EOF {
				return lines, nil
			}
			return lines, err
		}
	}
}

// csplitFailure is a pattern failure; text is the pattern operand, or the
// finished message when kind is csplitFailed.
type csplitFailure struct {
	kind csplitErrKind
	text string
}

type csplitErrKind int

const (
	csplitFailed csplitErrKind = iota
	csplitMatchNotFound
	csplitLineOutOfRange
)

func csplitIOFailure(msg string) *csplitFailure {
	return &csplitFailure{kind: csplitFailed, text: msg}
}

type csplitEngine struct {
	ctx      *Context
	names    *csplitName
	lines    [][]byte
	quiet    bool
	elide    bool
	keep     bool
	suppress bool

	files   []string // created files, in order
	counter int
	cur     *os.File
	curName string
	curSize int64
	curOpen bool
}

// path resolves an output file name against the invocation directory.
func (e *csplitEngine) path(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(e.ctx.Dir, name)
}

func (e *csplitEngine) newSplit() string {
	name := e.names.get(e.counter)
	f, err := os.OpenFile(e.path(name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return fmt.Sprintf("%s: %s", name, fmtOsErr(err))
	}
	e.cur = f
	e.curName = name
	e.curSize = 0
	e.curOpen = true
	e.counter++
	e.files = append(e.files, name)
	return ""
}

func (e *csplitEngine) write(from, to int) string {
	if !e.curOpen {
		return ""
	}
	for i := from; i < to; i++ {
		n, err := e.cur.Write(e.lines[i])
		e.curSize += int64(n)
		if err != nil {
			return fmt.Sprintf("%s: %s", e.curName, fmtOsErr(err))
		}
	}
	return ""
}

// finishSplit closes the current split, dropping it when empty and -z was
// given, and otherwise reporting its size.
func (e *csplitEngine) finishSplit() string {
	if !e.curOpen {
		return ""
	}
	err := e.cur.Close()
	e.curOpen = false
	if err != nil {
		return fmt.Sprintf("%s: %s", e.curName, fmtOsErr(err))
	}
	if e.elide && e.curSize == 0 {
		os.Remove(e.path(e.curName))
		e.files = e.files[:len(e.files)-1]
		e.counter--
		return ""
	}
	if !e.quiet {
		fmt.Fprintf(e.ctx.Stdout, "%d\n", e.curSize)
	}
	return ""
}

func (e *csplitEngine) removeSplits() {
	for _, name := range e.files {
		os.Remove(e.path(name))
	}
	e.files = nil
}

// run applies the patterns and returns a diagnostic message on failure.
func (e *csplitEngine) run(pats []csplitPattern) string {
	n := len(e.lines)
	pos, carry := 0, 0
	stopped := false
	for _, p := range pats {
		if stopped {
			break
		}
		hits := p.times
		if hits == 0 {
			hits = math.MaxInt
		}
		for ith := 1; ith <= hits; ith++ {
			var f *csplitFailure
			if p.kind == csplitLine {
				f = e.doLine(p, csplitTarget(p.line, ith), &pos, &carry)
			} else {
				f = e.doMatch(p, &pos, &carry)
			}
			if f == nil {
				continue
			}
			switch f.kind {
			case csplitFailed:
				return f.text
			case csplitMatchNotFound:
				if p.times == 0 {
					// {*} stops at the first repetition that matches
					// nothing, silently; the rest of the input follows.
					stopped = true
				} else if ith > 1 {
					return fmt.Sprintf("'%s': match not found on repetition %d", f.text, ith-1)
				} else {
					return fmt.Sprintf("'%s': match not found", f.text)
				}
			case csplitLineOutOfRange:
				if ith > 1 {
					return fmt.Sprintf("'%s': line number out of range on repetition %d", f.text, ith-1)
				}
				return fmt.Sprintf("'%s': line number out of range", f.text)
			}
			if stopped {
				break
			}
		}
	}
	// A {*} that ran out of matches already wrote everything to the final
	// split, so no empty trailing split is created.
	if pos < n || !stopped {
		if msg := e.newSplit(); msg != "" {
			return msg
		}
		if pos < n {
			if msg := e.write(pos, n); msg != "" {
				return msg
			}
		}
		if msg := e.finishSplit(); msg != "" {
			return msg
		}
	}
	return ""
}

// csplitTarget is the line number of the ith execution of a line pattern;
// repetitions count on from the first number.
func csplitTarget(line, ith int) int {
	if ith <= 1 {
		return line
	}
	if line > math.MaxInt/ith {
		return math.MaxInt
	}
	return line * ith
}

// doLine runs one INTEGER pattern execution.
//
// pos is the first line of the split; carry is how many lines from pos on are
// already carried over from an earlier pattern and so are not up for
// matching. Lines the pattern does not consume stay carried for the next one.
func (e *csplitEngine) doLine(p csplitPattern, target int, pos, carry *int) *csplitFailure {
	n := len(e.lines)
	if msg := e.newSplit(); msg != "" {
		return csplitIOFailure(msg)
	}
	end := target - 1
	if *pos >= n {
		// no input left for the split the pattern asks for
		if msg := e.finishSplit(); msg != "" {
			return csplitIOFailure(msg)
		}
		return &csplitFailure{kind: csplitLineOutOfRange, text: p.text}
	}
	if end >= n {
		// The target line is at or past the end of the input. Everything
		// left goes to the split; the pattern then fails, unless the line
		// it names would have been suppressed anyway.
		if msg := e.write(*pos, n); msg != "" {
			return csplitIOFailure(msg)
		}
		*pos, *carry = n, 0
		if msg := e.finishSplit(); msg != "" {
			return csplitIOFailure(msg)
		}
		if e.suppress && end == n {
			return nil
		}
		return &csplitFailure{kind: csplitLineOutOfRange, text: p.text}
	}
	// The split ends at line end; everything before it is written out.
	breakAt := end
	if breakAt < *pos {
		breakAt = *pos
	}
	carried := *pos + *carry // end of the carried-over range
	if msg := e.write(*pos, breakAt); msg != "" {
		return csplitIOFailure(msg)
	}
	if e.suppress {
		// the line the pattern named is dropped, together with the carried
		// lines before it
		*pos = breakAt + 1
		*carry = carried - breakAt - 1
	} else {
		*pos = breakAt
		*carry = carried - breakAt
		if *carry < 1 {
			*carry = 1
		}
	}
	if *carry < 0 {
		*carry = 0
	}
	if *pos+*carry > n {
		*carry = n - *pos
	}
	if msg := e.finishSplit(); msg != "" {
		return csplitIOFailure(msg)
	}
	return nil
}

// doMatch runs one /REGEX/[OFFSET] or %REGEX%[OFFSET] execution.
func (e *csplitEngine) doMatch(p csplitPattern, pos, carry *int) *csplitFailure {
	n := len(e.lines)
	skip := p.kind == csplitSkip
	if !skip {
		if msg := e.newSplit(); msg != "" {
			return csplitIOFailure(msg)
		}
	}
	end := func() *csplitFailure {
		if msg := e.finishSplit(); msg != "" {
			return csplitIOFailure(msg)
		}
		return nil
	}
	// Lines carried over from the previous pattern are part of this split
	// and are not searched again.
	from := *pos + *carry
	found := -1
	for i := from; i < n; i++ {
		line := e.lines[i]
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		ok, err := p.re.Match(line)
		if err != nil {
			if f := end(); f != nil {
				return f
			}
			return csplitIOFailure(fmt.Sprintf("'%s': %s", p.text, err))
		}
		if ok {
			found = i
			break
		}
	}
	if found < 0 {
		// Everything left belongs to this split, matched line or not.
		if !skip {
			if msg := e.write(*pos, n); msg != "" {
				return csplitIOFailure(msg)
			}
			if f := end(); f != nil {
				return f
			}
		}
		*pos, *carry = n, 0
		return &csplitFailure{kind: csplitMatchNotFound, text: p.text}
	}
	target := found + p.offset
	writeTo, overflow := target, false
	switch {
	case target < *pos:
		// a negative offset may not reach back past the start of the split
		writeTo = *pos
	case target > n:
		writeTo, overflow = n, true
	}
	if !skip {
		if msg := e.write(*pos, writeTo); msg != "" {
			return csplitIOFailure(msg)
		}
		if f := end(); f != nil {
			return f
		}
	}
	if overflow {
		*pos, *carry = n, 0
		return &csplitFailure{kind: csplitLineOutOfRange, text: p.text}
	}
	if target < *pos {
		return &csplitFailure{kind: csplitLineOutOfRange, text: p.text}
	}
	*pos = target
	keep := 0
	switch {
	case p.offset < 0:
		keep = -p.offset
		if !e.suppress {
			keep++
		}
	case p.offset == 0 && !e.suppress:
		keep = 1
	}
	if e.suppress && *pos < n {
		*pos++
	}
	room := n - *pos
	if room < 0 {
		room = 0
	}
	*carry = csplitMin(keep, room)
	return nil
}

func csplitMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

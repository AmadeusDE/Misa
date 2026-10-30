// Behaviour follows toybox sed.c (0BSD) and its tests; uutils/sed (MIT) is a secondary reference.
package builtins

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

type sedAddrKind uint8

const (
	sedNoAddr sedAddrKind = iota
	sedLine
	sedLast
	sedRegex
	sedStep // first~step
	sedPlus // addr1,+N
	sedMult // addr1,~N
	sedZero // 0,/re/
)

type sedAddr struct {
	kind    sedAddrKind
	n, step int64
	re      *posixre.Regexp // nil for an empty regex: reuse the last one
}

type sedReplPart struct {
	literal string
	group   int // -1 for a literal, else the group to insert (0 is &)
	caseOp  byte
}

type sedCmd struct {
	a1, a2 sedAddr
	negate bool
	name   byte
	pos    int

	re     *posixre.Regexp
	repl   []sedReplPart
	global bool
	nth    int
	print  bool
	wfile  string

	n     int64 // q/Q exit code, l width
	label string
	text  string
	file  string
	from  []rune
	to    []rune
	jump  int // '{': index after the matching '}'; b/t/T: target

	active, zeroDone bool
	end              int64
}

type sedScriptError struct {
	pos int
	msg string
}

func (e *sedScriptError) Error() string { return e.msg }

type sedParser struct {
	src    string
	pos    int
	ere    bool
	gnu    bool
	cmds   []*sedCmd
	blocks []int
	quiet  bool
}

func (p *sedParser) fail(format string, args ...any) error {
	return &sedScriptError{pos: p.pos, msg: fmt.Sprintf(format, args...)}
}

func (p *sedParser) eof() bool  { return p.pos >= len(p.src) }
func (p *sedParser) peek() byte { return p.src[p.pos] }

func (p *sedParser) skipSpace() {
	for !p.eof() && (p.peek() == ' ' || p.peek() == '\t') {
		p.pos++
	}
}

func parseSedScript(src string, ere, gnu bool) ([]*sedCmd, bool, error) {
	p := &sedParser{src: src, ere: ere, gnu: gnu}
	if src == "#n" || strings.HasPrefix(src, "#n\n") {
		p.quiet = true
	}
	if err := p.script(); err != nil {
		return nil, false, err
	}
	if len(p.blocks) > 0 {
		p.pos = p.cmds[p.blocks[len(p.blocks)-1]].pos
		return nil, false, p.fail("unmatched `{'")
	}
	if err := p.resolveLabels(); err != nil {
		return nil, false, err
	}
	return p.cmds, p.quiet, nil
}

func (p *sedParser) script() error {
	for {
		for !p.eof() && strings.IndexByte(" \t\n;", p.peek()) >= 0 {
			p.pos++
		}
		if p.eof() {
			return nil
		}
		if p.peek() == '#' {
			p.skipLine()
			continue
		}
		if err := p.command(); err != nil {
			return err
		}
	}
}

func (p *sedParser) skipLine() {
	for !p.eof() && p.peek() != '\n' {
		p.pos++
	}
}

func (p *sedParser) command() error {
	c := &sedCmd{pos: p.pos}
	var err error
	if c.a1, err = p.address(); err != nil {
		return err
	}
	if c.a1.kind != sedNoAddr {
		p.skipSpace()
		if !p.eof() && p.peek() == ',' {
			p.pos++
			p.skipSpace()
			if c.a2, err = p.address2(); err != nil {
				return err
			}
		}
	}
	if c.a1.kind == sedZero && (c.a2.kind != sedRegex) {
		return p.fail("invalid usage of line address 0")
	}
	p.skipSpace()
	for !p.eof() && p.peek() == '!' {
		c.negate = true
		p.pos++
		p.skipSpace()
	}
	if p.eof() {
		return p.fail("missing command")
	}
	c.name = p.peek()
	p.pos++
	if err := p.arguments(c); err != nil {
		return err
	}
	p.cmds = append(p.cmds, c)
	return nil
}

func (p *sedParser) address() (sedAddr, error) {
	if p.eof() {
		return sedAddr{}, nil
	}
	switch c := p.peek(); {
	case c >= '0' && c <= '9':
		n := p.number()
		if !p.eof() && p.peek() == '~' {
			p.pos++
			step := p.number()
			return sedAddr{kind: sedStep, n: n, step: step}, nil
		}
		if n == 0 {
			return sedAddr{kind: sedZero}, nil
		}
		return sedAddr{kind: sedLine, n: n}, nil
	case c == '$':
		p.pos++
		return sedAddr{kind: sedLast}, nil
	case c == '/', c == '\\':
		return p.regexAddress()
	}
	return sedAddr{}, nil
}

func (p *sedParser) address2() (sedAddr, error) {
	if !p.eof() && (p.peek() == '+' || p.peek() == '~') {
		kind := sedPlus
		if p.peek() == '~' {
			kind = sedMult
		}
		p.pos++
		if p.eof() || !isDigit(p.peek()) {
			return sedAddr{}, p.fail("expected number after `%c'", p.src[p.pos-1])
		}
		return sedAddr{kind: kind, n: p.number()}, nil
	}
	a, err := p.address()
	if err != nil {
		return a, err
	}
	switch a.kind {
	case sedNoAddr:
		return a, p.fail("unexpected `,'")
	case sedZero:
		return a, p.fail("invalid usage of line address 0")
	case sedStep:
		return sedAddr{kind: sedLine, n: a.n}, nil
	}
	return a, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func (p *sedParser) number() int64 {
	start := p.pos
	for !p.eof() && isDigit(p.peek()) {
		p.pos++
	}
	n, err := strconv.ParseInt(p.src[start:p.pos], 10, 64)
	if err != nil {
		return 1<<63 - 1
	}
	return n
}

func (p *sedParser) regexAddress() (sedAddr, error) {
	delim := byte('/')
	if p.peek() == '\\' {
		p.pos++
		if p.eof() {
			return sedAddr{}, p.fail("unexpected end of script")
		}
		delim = p.peek()
	}
	p.pos++
	raw, err := p.delimited(delim, true, "unterminated address regex")
	if err != nil {
		return sedAddr{}, err
	}
	icase := false
	for !p.eof() {
		switch p.peek() {
		case 'I':
			icase = true
		case 'M':
			return sedAddr{}, p.fail("the M modifier is not supported")
		default:
			re, err := p.compile(raw, icase)
			return sedAddr{kind: sedRegex, re: re}, err
		}
		p.pos++
	}
	re, err := p.compile(raw, icase)
	return sedAddr{kind: sedRegex, re: re}, err
}

func (p *sedParser) compile(raw string, icase bool) (*posixre.Regexp, error) {
	if raw == "" {
		return nil, nil
	}
	syntax := posixre.Basic
	if p.ere {
		syntax = posixre.Extended
	}
	re, err := posixre.Compile(raw, posixre.Options{Syntax: syntax, GNU: p.gnu, IgnoreCase: icase})
	if err != nil {
		return nil, p.fail("%v", err)
	}
	return re, nil
}

// delimited reads up to an unescaped delim. In a regex, bracket expressions
// are copied as they are (the delimiter does not end them), \delim is the
// delimiter itself, and \n and \t are a newline and a tab unless the
// delimiter is that letter.
func (p *sedParser) delimited(delim byte, regex bool, unterminated string) (string, error) {
	var b strings.Builder
	for {
		if p.eof() || p.peek() == '\n' {
			return "", p.fail("%s", unterminated)
		}
		c := p.peek()
		switch {
		case c == delim:
			p.pos++
			return b.String(), nil
		case c == '\\':
			if p.pos+1 >= len(p.src) {
				return "", p.fail("%s", unterminated)
			}
			n := p.src[p.pos+1]
			p.pos += 2
			switch {
			case n == delim:
				b.WriteByte(delim)
			case n == '\n':
				b.WriteByte('\n')
			case regex && n == 'n':
				b.WriteByte('\n')
			case regex && n == 't':
				b.WriteByte('\t')
			default:
				b.WriteByte('\\')
				b.WriteByte(n)
			}
		case regex && c == '[':
			end := sedBracketEnd(p.src, p.pos)
			if end < 0 {
				return "", p.fail("%s", unterminated)
			}
			b.WriteString(p.src[p.pos:end])
			p.pos = end
		default:
			b.WriteByte(c)
			p.pos++
		}
	}
}

// sedBracketEnd returns the offset just past the bracket expression at i,
// or -1 when it is not closed on this line.
func sedBracketEnd(s string, i int) int {
	j := i + 1
	if j < len(s) && s[j] == '^' {
		j++
	}
	if j < len(s) && s[j] == ']' {
		j++
	}
	for j < len(s) && s[j] != '\n' {
		switch {
		case s[j] == ']':
			return j + 1
		case s[j] == '[' && j+1 < len(s) && strings.IndexByte(":.=", s[j+1]) >= 0:
			end := strings.Index(s[j+2:], string(s[j+1])+"]")
			if end < 0 {
				return -1
			}
			j += end + 4
		default:
			j++
		}
	}
	return -1
}

func (p *sedParser) arguments(c *sedCmd) error {
	switch c.name {
	case '{':
		p.blocks = append(p.blocks, len(p.cmds))
		return nil
	case '}':
		if c.a1.kind != sedNoAddr || c.negate {
			return p.fail("} doesn't want any addresses")
		}
		if len(p.blocks) == 0 {
			return p.fail("unexpected `}'")
		}
		open := p.blocks[len(p.blocks)-1]
		p.blocks = p.blocks[:len(p.blocks)-1]
		p.cmds[open].jump = len(p.cmds) + 1
	case '=', 'd', 'D', 'g', 'G', 'h', 'H', 'n', 'N', 'p', 'P', 'x', 'z', 'F':
	case 'q', 'Q', 'l', 'L':
		if c.name != 'l' && c.a2.kind != sedNoAddr {
			return p.fail("command only uses one address")
		}
		p.skipSpace()
		c.n = -1
		if !p.eof() && isDigit(p.peek()) {
			c.n = p.number()
		}
	case 'a', 'i', 'c':
		c.text = p.text()
		return nil
	case ':':
		if c.a1.kind != sedNoAddr {
			return p.fail(": doesn't want any addresses")
		}
		p.skipSpace()
		if c.label = p.label(";\n"); c.label == "" {
			return p.fail("\":\" lacks a label")
		}
	case 'b', 't', 'T':
		p.skipSpace()
		c.label = p.label("; \t\n}")
	case 'r', 'R', 'w', 'W':
		c.file = p.filename()
		return nil
	case 's':
		return p.substitute(c)
	case 'y':
		return p.transliterate(c)
	case 'e':
		return p.fail("e command is not supported")
	case 'v':
		p.label("; \t\n}")
	default:
		p.pos--
		return p.fail("unknown command: `%c'", c.name)
	}
	return p.endCommand()
}

func (p *sedParser) endCommand() error {
	p.skipSpace()
	if p.eof() || strings.IndexByte(";\n}#", p.peek()) >= 0 {
		if !p.eof() && p.peek() == ';' {
			p.pos++
		}
		return nil
	}
	return p.fail("extra characters after command")
}

func (p *sedParser) label(stop string) string {
	start := p.pos
	for !p.eof() && strings.IndexByte(stop, p.peek()) < 0 {
		p.pos++
	}
	return strings.TrimRight(p.src[start:p.pos], " \t")
}

func (p *sedParser) filename() string {
	p.skipSpace()
	start := p.pos
	p.skipLine()
	return p.src[start:p.pos]
}

// text reads a/i/c text: either "a text" on one line (GNU) or "a\" with the
// text on following lines. A backslash keeps the next character, and one
// at the end of a line continues the text on the next line.
func (p *sedParser) text() string {
	p.skipSpace()
	if p.pos+1 < len(p.src) && p.src[p.pos] == '\\' && p.src[p.pos+1] == '\n' {
		p.pos += 2
	} else if p.pos+1 == len(p.src) && p.src[p.pos] == '\\' {
		p.pos++
		return ""
	}
	var b strings.Builder
	for !p.eof() {
		c := p.peek()
		p.pos++
		if c == '\n' {
			break
		}
		if c == '\\' && !p.eof() {
			c = p.peek()
			p.pos++
		}
		b.WriteByte(c)
	}
	return b.String() + "\n"
}

func (p *sedParser) substitute(c *sedCmd) error {
	if p.eof() || p.peek() == '\n' || p.peek() == '\\' {
		return p.fail("unterminated `s' command")
	}
	delim := p.peek()
	p.pos++
	raw, err := p.delimited(delim, true, "unterminated `s' command")
	if err != nil {
		return err
	}
	if c.repl, err = p.replacement(delim); err != nil {
		return err
	}
	icase := false
	for !p.eof() && !strings.ContainsRune(" \t\n;}#", rune(p.peek())) {
		switch f := p.peek(); {
		case f == 'g':
			if c.global {
				return p.fail("multiple `g' options to `s' command")
			}
			c.global = true
		case f == 'p':
			if c.print {
				return p.fail("multiple `p' options to `s' command")
			}
			c.print = true
		case f == 'i' || f == 'I':
			icase = true
		case isDigit(f):
			if c.nth != 0 {
				return p.fail("multiple number options to `s' command")
			}
			if c.nth = int(p.number()); c.nth == 0 {
				return p.fail("number option to `s' command may not be zero")
			}
			continue
		case f == 'w':
			p.pos++
			c.wfile = p.filename()
			return p.checkReplacement(c, raw, icase)
		case f == 'm' || f == 'M' || f == 'e':
			return p.fail("the `%c' option to `s' is not supported", f)
		default:
			return p.fail("unknown option to `s'")
		}
		p.pos++
	}
	if c.nth == 0 {
		c.nth = 1
	}
	if err := p.checkReplacement(c, raw, icase); err != nil {
		return err
	}
	return p.endCommand()
}

func (p *sedParser) checkReplacement(c *sedCmd, raw string, icase bool) error {
	if c.nth == 0 {
		c.nth = 1
	}
	re, err := p.compile(raw, icase)
	if err != nil {
		return err
	}
	c.re = re
	if re == nil {
		return nil
	}
	for _, part := range c.repl {
		if part.group > re.NumSubexp() {
			return p.fail("invalid reference \\%d on `s' command's RHS", part.group)
		}
	}
	return nil
}

func (p *sedParser) replacement(delim byte) ([]sedReplPart, error) {
	var parts []sedReplPart
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, sedReplPart{literal: lit.String(), group: -1})
			lit.Reset()
		}
	}
	for {
		if p.eof() {
			return nil, p.fail("unterminated `s' command")
		}
		c := p.peek()
		p.pos++
		switch {
		case c == delim:
			flush()
			return parts, nil
		case c == '\n':
			return nil, p.fail("unterminated `s' command")
		case c == '&':
			flush()
			parts = append(parts, sedReplPart{group: 0})
		case c == '\\':
			if p.eof() {
				return nil, p.fail("unterminated `s' command")
			}
			n := p.peek()
			p.pos++
			switch {
			case n == delim:
				lit.WriteByte(n)
			case n >= '0' && n <= '9':
				flush()
				parts = append(parts, sedReplPart{group: int(n - '0')})
			case strings.IndexByte("LUluE", n) >= 0 && p.gnu:
				flush()
				parts = append(parts, sedReplPart{group: -1, caseOp: n})
			case n == 'n':
				lit.WriteByte('\n')
			case n == 't':
				lit.WriteByte('\t')
			case n == 'r':
				lit.WriteByte('\r')
			case n == 'a':
				lit.WriteByte('\a')
			case n == 'f':
				lit.WriteByte('\f')
			case n == 'v':
				lit.WriteByte('\v')
			default:
				lit.WriteByte(n)
			}
		default:
			lit.WriteByte(c)
		}
	}
}

func (p *sedParser) transliterate(c *sedCmd) error {
	if p.eof() || p.peek() == '\n' || p.peek() == '\\' {
		return p.fail("unterminated `y' command")
	}
	delim := p.peek()
	p.pos++
	from, err := p.delimited(delim, false, "unterminated `y' command")
	if err != nil {
		return err
	}
	to, err := p.delimited(delim, false, "unterminated `y' command")
	if err != nil {
		return err
	}
	c.from, c.to = []rune(sedUnescapeY(from)), []rune(sedUnescapeY(to))
	if len(c.from) != len(c.to) {
		return p.fail("strings for `y' command are different lengths")
	}
	return p.endCommand()
}

func sedUnescapeY(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func (p *sedParser) resolveLabels() error {
	labels := map[string]int{}
	for i, c := range p.cmds {
		if c.name == ':' {
			if _, dup := labels[c.label]; dup {
				p.pos = c.pos
				return p.fail("duplicate label `%s'", c.label)
			}
			labels[c.label] = i
		}
	}
	for _, c := range p.cmds {
		if c.name != 'b' && c.name != 't' && c.name != 'T' {
			continue
		}
		if c.label == "" {
			c.jump = len(p.cmds)
			continue
		}
		target, ok := labels[c.label]
		if !ok {
			p.pos = c.pos
			return p.fail("can't find label for jump to `%s'", c.label)
		}
		c.jump = target
	}
	return nil
}

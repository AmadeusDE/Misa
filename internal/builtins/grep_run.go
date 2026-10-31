package builtins

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const (
	grepColorMatch = "01;31"
	grepColorFile  = "35"
	grepColorLine  = "32"
	grepColorSep   = "36"
)

type grepRun struct {
	ctx        *Context
	o          *grepOptions
	m          grepMatcher
	filter     *grepFilter
	out        *bufio.Writer
	delim      byte
	failed     bool
	matched    bool
	listed     bool
	stop       bool
	printedAny bool
	writeErr   error
}

type grepRecord struct {
	text       []byte
	no, offset int64
}

// grepFile is the per-file scan state.
type grepFile struct {
	name        string
	selected    int64
	afterLeft   int
	lastPrinted int64
	before      []grepRecord
}

func newGrepRun(ctx *Context, o *grepOptions, m grepMatcher) *grepRun {
	g := &grepRun{ctx: ctx, o: o, m: m, out: bufio.NewWriter(ctx.Stdout), delim: '\n'}
	if o.nullData {
		g.delim = 0
	}
	if o.onlyMatching || o.count || o.filesWith || o.filesWithout || o.quiet {
		o.after, o.before, o.contextSet = 0, 0, false
	}
	return g
}

func (g *grepRun) files(names []string) int {
	for _, name := range names {
		switch {
		case g.stop:
		case name == "-":
			g.file(name)
		case g.o.recursive:
			g.walk(name)
		case !g.filter.skipFile(name, true):
			g.file(name)
		}
	}
	if err := g.out.Flush(); err != nil && g.writeErr == nil {
		g.writeErr = err
	}
	switch {
	case g.o.quiet && g.matched:
		return 0
	case g.writeErr != nil:
		fmt.Fprintf(g.ctx.Stderr, "grep: write error: %s\n", grepErrText(g.writeErr))
		return 2
	case g.failed:
		return 2
	case g.o.filesWithout:
		if g.listed {
			return 0
		}
		return 1
	case g.matched:
		return 0
	}
	return 1
}

func (g *grepRun) fail(name string, err error) {
	g.failed = true
	if !g.o.noMessages {
		fmt.Fprintf(g.ctx.Stderr, "grep: %s: %s\n", name, grepErrText(err))
	}
}

func (g *grepRun) open(name string) (io.Reader, func(), bool) {
	if name == "-" {
		if g.ctx.Stdin == nil {
			return strings.NewReader(""), func() {}, true
		}
		return g.ctx.Stdin, func() {}, true
	}
	f, err := os.Open(resolvePath(g.ctx, name))
	if err != nil {
		g.fail(name, err)
		return nil, nil, false
	}
	if st, err := f.Stat(); err == nil && st.IsDir() {
		f.Close()
		g.fail(name, errors.New("Is a directory"))
		return nil, nil, false
	}
	return f, func() { f.Close() }, true
}

func (g *grepRun) file(name string) {
	r, closeFn, ok := g.open(name)
	if !ok {
		return
	}
	defer closeFn()
	st := &grepFile{name: name}
	if name == "-" {
		st.name = g.o.label
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), math.MaxInt)
	sc.Split(grepSplit(g.delim))
	var no, offset int64
	for !g.stop && sc.Scan() {
		if g.o.maxCount >= 0 && st.selected >= g.o.maxCount && st.afterLeft == 0 {
			break
		}
		rec := sc.Bytes()
		no++
		r := grepRecord{text: rec, no: no, offset: offset}
		offset += int64(len(rec)) + 1
		if g.o.maxCount >= 0 && st.selected >= g.o.maxCount {
			g.context(st, r)
			continue
		}
		if !g.selects(st, rec) {
			g.context(st, r)
			continue
		}
		st.selected++
		g.matched = true
		if g.o.quiet {
			g.stop = true
			return
		}
		if g.o.filesWith || g.o.filesWithout {
			break
		}
		if !g.o.count {
			g.emitSelected(st, r)
		}
	}
	if err := sc.Err(); err != nil {
		g.fail(name, err)
	}
	g.summary(st)
}

func grepSplit(delim byte) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, delim); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
}

func (g *grepRun) selects(st *grepFile, rec []byte) bool {
	if g.m == nil {
		return g.o.invert
	}
	ok, err := g.m.Match(rec)
	if err != nil {
		g.fail(st.name, err)
	}
	return ok != g.o.invert
}

func (g *grepRun) context(st *grepFile, r grepRecord) {
	switch {
	case st.afterLeft > 0:
		st.afterLeft--
		g.emit(st, r, '-')
	case g.o.before > 0:
		if len(st.before) == g.o.before {
			st.before = st.before[1:]
		}
		r.text = bytes.Clone(r.text)
		st.before = append(st.before, r)
	}
}

func (g *grepRun) emitSelected(st *grepFile, r grepRecord) {
	for _, b := range st.before {
		g.emit(st, b, '-')
	}
	st.before = st.before[:0]
	g.emit(st, r, ':')
	st.afterLeft = g.o.after
}

func (g *grepRun) emit(st *grepFile, r grepRecord, sep byte) {
	g.groupBreak(st, r.no)
	st.lastPrinted = r.no
	g.printedAny = true
	switch {
	case g.o.onlyMatching && sep == ':':
		g.emitMatches(st, r)
	case g.o.color && sep == ':' && !g.o.invert:
		g.prefix(st.name, r.no, r.offset, sep)
		g.highlight(r.text)
		g.write([]byte{g.delim})
	default:
		g.prefix(st.name, r.no, r.offset, sep)
		g.write(r.text)
		g.write([]byte{g.delim})
	}
	if g.o.lineBuffered {
		g.out.Flush()
	}
}

func (g *grepRun) groupBreak(st *grepFile, no int64) {
	if !g.o.contextSet || g.o.groupSep == nil || !g.printedAny {
		return
	}
	if st.lastPrinted == 0 || no > st.lastPrinted+1 {
		g.paint(grepColorSep, *g.o.groupSep)
		g.write([]byte{'\n'})
	}
}

func (g *grepRun) emitMatches(st *grepFile, r grepRecord) {
	if g.o.invert || g.m == nil {
		return
	}
	locs, _ := grepFindAll(g.m, r.text)
	for _, loc := range locs {
		if loc[0] == loc[1] {
			continue
		}
		g.prefix(st.name, r.no, r.offset+int64(loc[0]), ':')
		if g.o.color {
			g.paint(grepColorMatch, string(r.text[loc[0]:loc[1]]))
		} else {
			g.write(r.text[loc[0]:loc[1]])
		}
		g.write([]byte{g.delim})
	}
}

func (g *grepRun) highlight(text []byte) {
	locs, _ := grepFindAll(g.m, text)
	last := 0
	for _, loc := range locs {
		if loc[0] == loc[1] {
			continue
		}
		g.write(text[last:loc[0]])
		g.paint(grepColorMatch, string(text[loc[0]:loc[1]]))
		last = loc[1]
	}
	g.write(text[last:])
}

func (g *grepRun) prefix(name string, no, offset int64, sep byte) {
	if g.o.withFilename {
		g.paint(grepColorFile, name)
		if g.o.nullName {
			g.write([]byte{0})
		} else {
			g.paint(grepColorSep, string(sep))
		}
	}
	if g.o.lineNumber {
		g.paint(grepColorLine, strconv.FormatInt(no, 10))
		g.paint(grepColorSep, string(sep))
	}
	if g.o.byteOffset {
		g.paint(grepColorLine, strconv.FormatInt(offset, 10))
		g.paint(grepColorSep, string(sep))
	}
}

func (g *grepRun) summary(st *grepFile) {
	switch {
	case g.o.quiet:
	case g.o.filesWith && st.selected > 0, g.o.filesWithout && st.selected == 0:
		g.listed = true
		g.paint(grepColorFile, st.name)
		g.write([]byte{g.nameEnd()})
	case g.o.count:
		if g.o.withFilename {
			g.paint(grepColorFile, st.name)
			if g.o.nullName {
				g.write([]byte{0})
			} else {
				g.paint(grepColorSep, ":")
			}
		}
		g.write([]byte(strconv.FormatInt(st.selected, 10) + "\n"))
	}
}

func (g *grepRun) nameEnd() byte {
	if g.o.nullName {
		return 0
	}
	return '\n'
}

// paint writes s in GNU grep's colour format when --color is on.
func (g *grepRun) paint(code, s string) {
	if g.o.color && s != "" {
		g.write([]byte("\x1b[" + code + "m\x1b[K" + s + "\x1b[m\x1b[K"))
		return
	}
	g.write([]byte(s))
}

func (g *grepRun) write(b []byte) {
	if g.writeErr != nil {
		return
	}
	if _, err := g.out.Write(b); err != nil {
		g.writeErr = err
		g.stop = true
	}
}

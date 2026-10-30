package builtins

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

type joinSepMode int

const (
	joinSepWhitespace joinSepMode = iota
	joinSepLine
	joinSepBytes
)

// joinSep splits one input line into fields. Whitespace mode merges runs of
// blanks and drops leading ones; line mode makes the whole line one field.
type joinSep struct {
	mode joinSepMode
	sep  []byte
}

func (s *joinSep) fieldRanges(line []byte) [][2]int {
	switch s.mode {
	case joinSepLine:
		return [][2]int{{0, len(line)}}
	case joinSepWhitespace:
		ranges := make([][2]int, 0, 8)
		last := 0
		for i := 0; i < len(line); i++ {
			if line[i] != ' ' && line[i] != '\t' && line[i] != '\n' {
				continue
			}
			if i > last {
				ranges = append(ranges, [2]int{last, i})
			}
			last = i + 1
		}
		return append(ranges, [2]int{last, len(line)})
	default:
		ranges := make([][2]int, 0, 8)
		last := 0
		for {
			i := bytes.Index(line[last:], s.sep)
			if i < 0 {
				break
			}
			ranges = append(ranges, [2]int{last, last + i})
			last += i + len(s.sep)
		}
		return append(ranges, [2]int{last, len(line)})
	}
}

func (s *joinSep) output() []byte {
	if s.mode == joinSepWhitespace {
		return []byte(" ")
	}
	return s.sep
}

type joinLine struct {
	text   []byte
	ranges [][2]int
}

func joinMakeLine(sep *joinSep, text []byte) joinLine {
	return joinLine{text: text, ranges: sep.fieldRanges(text)}
}

func (l *joinLine) field(index uint64) ([]byte, bool) {
	if index >= uint64(len(l.ranges)) {
		return nil, false
	}
	r := l.ranges[index]
	return l.text[r[0]:r[1]], true
}

// joinLineReader streams records terminated by one byte, without the
// terminator. A final unterminated record is yielded; an empty input yields
// none.
type joinLineReader struct {
	r     *bufio.Reader
	c     io.Closer
	delim byte
}

func (lr *joinLineReader) next() ([]byte, bool, error) {
	line, err := lr.r.ReadBytes(lr.delim)
	if len(line) > 0 {
		if line[len(line)-1] == lr.delim {
			line = line[:len(line)-1]
		}
		return line, true, nil
	}
	if err == nil || err == io.EOF {
		return nil, false, nil
	}
	return nil, false, err
}

func (lr *joinLineReader) close() {
	if lr.c != nil {
		lr.c.Close()
	}
}

type joinInput struct {
	ignoreCase bool
	checkOrder joinCheckOrder
	stderr     io.Writer
}

func (in *joinInput) compare(a, b []byte) int {
	if in.ignoreCase {
		return joinCompareFolded(a, b)
	}
	return bytes.Compare(a, b)
}

// joinCompareFolded orders like `sort -f`: compare byte-wise with ASCII letters
// folded to upper case, then by length.
func joinCompareFolded(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		ca, cb := joinUpper(a[i]), joinUpper(b[i])
		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

func joinUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}

type joinOrderError struct{ msg string }

func (e *joinOrderError) Error() string { return e.msg }

type joinState struct {
	key           uint64
	name          string
	fileNum       joinFileNum
	printUnpaired bool
	reader        *joinLineReader
	sep           *joinSep
	seq           []joinLine
	lineNum       int
	hasFailed     bool
	hasUnpaired   bool
}

func joinOpenState(ctx *Context, name string, fileNum joinFileNum, st *joinSettings) (*joinState, error) {
	delim := byte('\n')
	if st.zeroTerminated {
		delim = 0
	}
	var reader *bufio.Reader
	var closer io.Closer
	if name == "-" {
		if ctx.Stdin == nil {
			return nil, errors.New("standard input not available")
		}
		reader = bufio.NewReader(ctx.Stdin)
	} else {
		f, err := os.Open(resolvePath(ctx, name))
		if err != nil {
			return nil, err
		}
		reader = bufio.NewReader(f)
		closer = f
	}
	printUnpaired := st.printUnpaired1
	if fileNum == joinFile2 {
		printUnpaired = st.printUnpaired2
	}
	key := st.key1
	if fileNum == joinFile2 {
		key = st.key2
	}
	return &joinState{
		key:           key,
		name:          name,
		fileNum:       fileNum,
		printUnpaired: printUnpaired,
		reader:        &joinLineReader{r: reader, c: closer, delim: delim},
		sep:           &st.separator,
	}, nil
}

func (s *joinState) close() { s.reader.close() }

func (s *joinState) hasLine() bool { return len(s.seq) > 0 }

func (s *joinState) currentKey() []byte {
	key, _ := s.seq[0].field(s.key)
	return key
}

func (s *joinState) readLine() (joinLine, bool, error) {
	text, ok, err := s.reader.next()
	if err != nil || !ok {
		return joinLine{}, false, err
	}
	s.lineNum++
	return joinMakeLine(s.sep, text), true, nil
}

// nextLine reads the next line, enforcing --check-order. In the default mode an
// out-of-order line is reported once per file and the run continues; with
// --check-order it is fatal.
func (s *joinState) nextLine(in *joinInput) (joinLine, bool, error) {
	line, ok, err := s.readLine()
	if err != nil || !ok {
		return line, ok, err
	}
	if in.checkOrder == joinCheckOff {
		return line, true, nil
	}
	if in.checkOrder == joinCheckDefault && (!s.hasUnpaired || s.hasFailed) {
		return line, true, nil
	}
	key, _ := line.field(s.key)
	if in.compare(s.currentKey(), key) <= 0 {
		return line, true, nil
	}
	if in.checkOrder == joinCheckDefault && !(s.hasUnpaired && !s.hasFailed) {
		return line, true, nil
	}
	msg := fmt.Sprintf("%s:%d: is not sorted: %s", s.name, s.lineNum, line.text)
	if in.checkOrder == joinCheckOn {
		return joinLine{}, false, &joinOrderError{msg: msg}
	}
	fmt.Fprintf(in.stderr, "join: %s\n", msg)
	s.hasFailed = true
	return line, true, nil
}

func (s *joinState) reset(line joinLine, ok bool) {
	s.seq = s.seq[:0]
	if ok {
		s.seq = append(s.seq, line)
	}
}

func (s *joinState) resetReadLine() error {
	line, ok, err := s.readLine()
	if err != nil {
		return err
	}
	s.reset(line, ok)
	return nil
}

func (s *joinState) resetNextLine(in *joinInput) error {
	line, ok, err := s.nextLine(in)
	if err != nil {
		return err
	}
	s.reset(line, ok)
	return nil
}

// initialize buffers the first line and, for -o auto, reports its field count.
func (s *joinState) initialize(autoFormat bool) (int, error) {
	line, ok, err := s.readLine()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	s.seq = append(s.seq, line)
	if autoFormat {
		return len(line.ranges), nil
	}
	return 0, nil
}

// extend keeps the lines whose key equals the current one and returns the first
// line with a different key.
func (s *joinState) extend(in *joinInput) (joinLine, bool, error) {
	for {
		line, ok, err := s.nextLine(in)
		if err != nil || !ok {
			return joinLine{}, false, err
		}
		key, _ := line.field(s.key)
		if in.compare(s.currentKey(), key) != 0 {
			return line, true, nil
		}
		s.seq = append(s.seq, line)
	}
}

func (s *joinState) skipLine(w *bufio.Writer, in *joinInput, repr *joinRepr) error {
	if s.printUnpaired {
		s.writeFirstLine(w, repr)
	}
	return s.resetNextLine(in)
}

func (s *joinState) finalize(w *bufio.Writer, in *joinInput, repr *joinRepr) error {
	if !s.hasLine() {
		return nil
	}
	if s.printUnpaired {
		s.writeFirstLine(w, repr)
	}
	line, ok, err := s.nextLine(in)
	for ok && err == nil {
		if s.printUnpaired {
			s.writeLine(w, &line, repr)
		}
		s.reset(line, true)
		line, ok, err = s.nextLine(in)
	}
	return err
}

func (s *joinState) writeFirstLine(w *bufio.Writer, repr *joinRepr) {
	s.writeLine(w, &s.seq[0], repr)
}

func (s *joinState) writeLine(w *bufio.Writer, line *joinLine, repr *joinRepr) {
	key, _ := line.field(s.key)
	if repr.usesFormat() {
		repr.writeFormat(w, func(spec joinOutField) ([]byte, bool) {
			switch {
			case spec.key:
				return key, true
			case spec.file == s.fileNum:
				return line.field(spec.field)
			default:
				return nil, false
			}
		})
	} else {
		repr.writeField(w, key, true)
		repr.writeFields(w, line, s.key)
	}
	w.WriteByte(repr.delim)
}

// combine writes every pair of the two buffered key groups.
func (s *joinState) combine(w *bufio.Writer, other *joinState, repr *joinRepr) {
	for i := range s.seq {
		line1 := &s.seq[i]
		key, _ := line1.field(s.key)
		for j := range other.seq {
			line2 := &other.seq[j]
			if repr.usesFormat() {
				repr.writeFormat(w, func(spec joinOutField) ([]byte, bool) {
					switch {
					case spec.key:
						return key, true
					case spec.file == s.fileNum:
						return line1.field(spec.field)
					case spec.file == other.fileNum:
						return line2.field(spec.field)
					default:
						return nil, false
					}
				})
			} else {
				repr.writeField(w, key, true)
				repr.writeFields(w, line1, s.key)
				repr.writeFields(w, line2, other.key)
			}
			w.WriteByte(repr.delim)
		}
	}
}

// writeHeaders prints the first line of each input as a header pair and leaves
// the states positioned on the first data line.
func (s *joinState) writeHeaders(w *bufio.Writer, other *joinState, repr *joinRepr) {
	switch {
	case s.hasLine() && other.hasLine():
		s.combine(w, other, repr)
	case s.hasLine():
		s.writeFirstLine(w, repr)
	case other.hasLine():
		other.writeFirstLine(w, repr)
	}
}

type joinRepr struct {
	delim  byte
	sep    *joinSep
	format []joinOutField
	empty  []byte
}

func (r *joinRepr) usesFormat() bool { return len(r.format) > 0 }

// fieldOrEmpty returns the filler for fields that are missing or zero length.
func (r *joinRepr) fieldOrEmpty(field []byte) []byte {
	if len(field) == 0 {
		return r.empty
	}
	return field
}

func (r *joinRepr) writeField(w *bufio.Writer, field []byte, ok bool) {
	if !ok {
		w.Write(r.empty)
		return
	}
	w.Write(r.fieldOrEmpty(field))
}

// writeFields writes every field of the line except the join field.
func (r *joinRepr) writeFields(w *bufio.Writer, line *joinLine, skip uint64) {
	for i := range line.ranges {
		if uint64(i) == skip {
			continue
		}
		w.Write(r.sep.output())
		field, _ := line.field(uint64(i))
		w.Write(r.fieldOrEmpty(field))
	}
}

func (r *joinRepr) writeFormat(w *bufio.Writer, get func(joinOutField) ([]byte, bool)) {
	for i, spec := range r.format {
		if i > 0 {
			w.Write(r.sep.output())
		}
		field, ok := get(spec)
		r.writeField(w, field, ok)
	}
}

func joinExec(ctx *Context, file1, file2 string, st *joinSettings) int {
	state1, err := joinOpenState(ctx, file1, joinFile1, st)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "join: %s: %s\n", file1, fmtOsErr(err))
		return 1
	}
	defer state1.close()
	state2, err := joinOpenState(ctx, file2, joinFile2, st)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "join: %s: %s\n", file2, fmtOsErr(err))
		return 1
	}
	defer state2.close()

	input := &joinInput{
		ignoreCase: st.ignoreCase,
		checkOrder: st.checkOrder,
		stderr:     ctx.Stderr,
	}

	format := st.format
	if st.autoFormat {
		format = []joinOutField{{key: true}}
		for _, state := range []*joinState{state1, state2} {
			fields, err := state.initialize(true)
			if err != nil {
				return joinReadError(ctx, state, err)
			}
			for i := uint64(0); i < uint64(fields); i++ {
				if i != state.key {
					format = append(format, joinOutField{file: state.fileNum, field: i})
				}
			}
		}
	} else {
		for _, state := range []*joinState{state1, state2} {
			if _, err := state.initialize(false); err != nil {
				return joinReadError(ctx, state, err)
			}
		}
	}

	repr := &joinRepr{delim: '\n', sep: &st.separator, format: format, empty: st.empty}
	if st.zeroTerminated {
		repr.delim = 0
	}

	w := bufio.NewWriterSize(ctx.Stdout, 64<<10)
	status := 0
	if st.header {
		state1.writeHeaders(w, state2, repr)
		if err := state1.resetReadLine(); err != nil {
			status = joinReadError(ctx, state1, err)
		}
		if err := state2.resetReadLine(); err != nil && status == 0 {
			status = joinReadError(ctx, state2, err)
		}
	}

	for status == 0 && state1.hasLine() && state2.hasLine() {
		diff := input.compare(state1.currentKey(), state2.currentKey())
		switch {
		case diff < 0:
			if err := state1.skipLine(w, input, repr); err != nil {
				status = joinReadError(ctx, state1, err)
			}
			state1.hasUnpaired, state2.hasUnpaired = true, true
		case diff > 0:
			if err := state2.skipLine(w, input, repr); err != nil {
				status = joinReadError(ctx, state2, err)
			}
			state1.hasUnpaired, state2.hasUnpaired = true, true
		default:
			next1, ok1, err1 := state1.extend(input)
			if err1 != nil {
				status = joinReadError(ctx, state1, err1)
				break
			}
			next2, ok2, err2 := state2.extend(input)
			if err2 != nil {
				status = joinReadError(ctx, state2, err2)
				break
			}
			if st.printJoined {
				state1.combine(w, state2, repr)
			}
			state1.reset(next1, ok1)
			state2.reset(next2, ok2)
		}
	}

	if status == 0 {
		if err := state1.finalize(w, input, repr); err != nil {
			status = joinReadError(ctx, state1, err)
		}
	}
	if status == 0 {
		if err := state2.finalize(w, input, repr); err != nil {
			status = joinReadError(ctx, state2, err)
		}
	}

	if err := w.Flush(); err != nil && status == 0 {
		fmt.Fprintf(ctx.Stderr, "join: %s\n", fmtOsErr(err))
		return 1
	}
	if status != 0 {
		return status
	}
	if state1.hasFailed || state2.hasFailed {
		fmt.Fprintln(ctx.Stderr, "join: input is not in sorted order")
		return 1
	}
	return 0
}

// joinReadError reports a fatal read or order error and returns the status for
// the run.
func joinReadError(ctx *Context, state *joinState, err error) int {
	var orderErr *joinOrderError
	if errors.As(err, &orderErr) {
		fmt.Fprintf(ctx.Stderr, "join: %s\n", orderErr.msg)
		return 1
	}
	fmt.Fprintf(ctx.Stderr, "join: %s: %s\n", state.name, fmtOsErr(err))
	return 1
}

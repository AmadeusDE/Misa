package awk

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// reader splits one input stream into records. Regular-expression RS reads
// the rest of the stream into memory first.
type reader struct {
	br     *bufio.Reader
	mem    []byte
	slurp  bool
	off    int
	closer io.Closer
	err    error
}

func newReader(r io.Reader, closer io.Closer) *reader {
	return &reader{br: bufio.NewReaderSize(r, 64<<10), closer: closer}
}

func (r *reader) close() {
	if r.closer != nil {
		r.closer.Close()
	}
}

// until returns the text up to the next delimiter byte. found reports the
// delimiter; ok is false at the end of input with nothing read.
func (r *reader) until(d byte) (s string, found, ok bool) {
	if r.slurp {
		rest := r.mem[r.off:]
		if len(rest) == 0 {
			return "", false, false
		}
		if i := bytes.IndexByte(rest, d); i >= 0 {
			r.off += i + 1
			return string(rest[:i]), true, true
		}
		r.off = len(r.mem)
		return string(rest), false, true
	}
	var buf []byte
	for {
		chunk, err := r.br.ReadSlice(d)
		if err == bufio.ErrBufferFull {
			buf = append(buf, chunk...)
			continue
		}
		if err != nil && err != io.EOF {
			r.err = err
		}
		if err == nil {
			chunk = chunk[:len(chunk)-1]
			if buf == nil {
				return string(chunk), true, true
			}
			return string(append(buf, chunk...)), true, true
		}
		buf = append(buf, chunk...)
		return string(buf), false, len(buf) > 0
	}
}

func (r *reader) peek() (byte, bool) {
	if r.slurp {
		if r.off < len(r.mem) {
			return r.mem[r.off], true
		}
		return 0, false
	}
	b, err := r.br.Peek(1)
	if err != nil {
		return 0, false
	}
	return b[0], true
}

func (r *reader) skipNewlines() {
	for {
		if b, ok := r.peek(); !ok || b != '\n' {
			return
		}
		if r.slurp {
			r.off++
		} else {
			r.br.ReadByte()
		}
	}
}

func (r *reader) slurpAll() {
	if r.slurp {
		return
	}
	data, err := io.ReadAll(r.br)
	if err != nil {
		r.err = err
	}
	r.mem, r.off, r.slurp = data, 0, true
}

func (r *reader) paragraph() (string, bool) {
	r.skipNewlines()
	var parts []string
	for {
		line, found, ok := r.until('\n')
		if !ok {
			break
		}
		parts = append(parts, line)
		if !found {
			break
		}
		if b, ok := r.peek(); !ok || b == '\n' {
			r.skipNewlines()
			break
		}
	}
	if parts == nil {
		return "", false
	}
	return strings.Join(parts, "\n"), true
}

func (in *interp) regexRecord(r *reader) (string, bool) {
	r.slurpAll()
	rest := r.mem[r.off:]
	if len(rest) == 0 {
		return "", false
	}
	re := in.regex(in.rs)
	for pos := 0; pos <= len(rest); {
		m, err := re.Find(rest, pos)
		if err != nil {
			runtimeErrorf("regular expression error: %v", err)
		}
		if m == nil {
			break
		}
		if m[1] > m[0] {
			r.off += m[1]
			return string(rest[:m[0]]), true
		}
		pos = m[1] + 1
	}
	r.off = len(r.mem)
	return string(rest), true
}

func (in *interp) readRecord(r *reader) (string, bool) {
	switch {
	case len(in.rs) == 1:
		s, _, ok := r.until(in.rs[0])
		return s, ok
	case in.rs == "":
		return r.paragraph()
	}
	return in.regexRecord(r)
}

// flushReader flushes pending output before a read that may block.
type flushReader struct {
	r io.Reader
	w *bufio.Writer
}

func (f flushReader) Read(p []byte) (int, error) {
	f.w.Flush()
	return f.r.Read(p)
}

// inputs walks the file operands, applying var=value operands as it
// reaches them, and falls back to standard input when there are no files.
type inputs struct {
	operands []string
	next     int
	cur      *reader
	sawFile  bool
	stdinUse bool
}

func isAssignment(arg string) (name, value string, ok bool) {
	i := strings.IndexByte(arg, '=')
	if i <= 0 || !isLetter(arg[0]) {
		return "", "", false
	}
	for _, c := range []byte(arg[:i]) {
		if !isLetter(c) && !isDigit(c) {
			return "", "", false
		}
	}
	return arg[:i], arg[i+1:], true
}

func (in *interp) openStdin() *reader {
	return newReader(flushReader{in.cfg.Stdin, in.out}, nil)
}

// openNext opens the next input file, returning false when none remain.
func (in *interp) openNext() bool {
	ip := &in.in
	for ip.next < len(ip.operands) {
		arg := ip.operands[ip.next]
		ip.next++
		if name, value, ok := isAssignment(arg); ok {
			in.assignOperand(name, value)
			continue
		}
		ip.sawFile = true
		in.vars[vFILENAME] = str(arg)
		in.vars[vFNR] = num(0)
		if arg == "-" {
			ip.cur = in.openStdin()
			return true
		}
		path := arg
		if !filepath.IsAbs(path) {
			path = filepath.Join(in.cfg.Dir, path)
		}
		f, err := os.Open(path)
		if err != nil {
			in.warnf("can't open file %s: %s", arg, errors.Unwrap(err))
			in.status = 2
			continue
		}
		ip.cur = newReader(f, f)
		return true
	}
	if !ip.sawFile && !ip.stdinUse {
		ip.stdinUse = true
		in.vars[vFNR] = num(0)
		ip.cur = in.openStdin()
		return true
	}
	return false
}

func (in *interp) skipFile() {
	if in.in.cur != nil {
		in.in.cur.close()
		in.in.cur = nil
	}
}

// nextRecord reads the next record into $0 and bumps NR and FNR.
func (in *interp) nextRecord() bool {
	for {
		if in.in.cur == nil && !in.openNext() {
			return false
		}
		cur := in.in.cur
		s, ok := in.readRecord(cur)
		if ok {
			in.setRecord(s)
			in.vars[vNR] = num(in.vars[vNR].float() + 1)
			in.vars[vFNR] = num(in.vars[vFNR].float() + 1)
			return true
		}
		if cur.err != nil {
			in.warnf("read error: %v", cur.err)
			in.status = 2
		}
		in.skipFile()
	}
}

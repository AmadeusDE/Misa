package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// sedAppend is output queued for the end of the cycle: a/R text, or the
// contents of an r file, read when it is written out.
type sedAppend struct {
	text string
	file string
}

// sedFiles holds the w targets, opened (and truncated) before any input is
// read and shared by every command naming the same file, and the R sources,
// read one line per command.
type sedFiles struct {
	ctx     *Context
	writers map[string]*bufio.Writer
	closers []io.Closer
	readers map[string]*bufio.Reader
}

func openSedFiles(ctx *Context, cmds []*sedCmd, out *sedOutput) (*sedFiles, error) {
	f := &sedFiles{ctx: ctx, writers: map[string]*bufio.Writer{}, readers: map[string]*bufio.Reader{}}
	for _, c := range cmds {
		name := c.wfile
		if c.name == 'w' || c.name == 'W' {
			name = c.file
		}
		if name == "" || name == "/dev/stdout" || f.writers[name] != nil {
			continue
		}
		if name == "/dev/stderr" {
			f.writers[name] = bufio.NewWriter(ctx.Stderr)
			continue
		}
		file, err := os.Create(resolvePath(ctx, name))
		if err != nil {
			f.close()
			return nil, fmt.Errorf("couldn't open file %s: %s", name, grepErrText(err))
		}
		f.closers = append(f.closers, file)
		f.writers[name] = bufio.NewWriter(file)
	}
	return f, nil
}

func (f *sedFiles) flush() error {
	for name, w := range f.writers {
		if err := w.Flush(); err != nil {
			return fmt.Errorf("couldn't write to %s: %s", name, grepErrText(err))
		}
	}
	return nil
}

func (f *sedFiles) close() {
	for _, c := range f.closers {
		c.Close()
	}
}

func (r *sedRun) writeFile(name string, b []byte) {
	if name == "/dev/stdout" {
		r.out.record(b, true)
		return
	}
	w := r.files.writers[name]
	w.Write(b)
	w.WriteByte(r.delim)
}

func (r *sedRun) readFileLine(name string) (string, bool) {
	rd, ok := r.files.readers[name]
	if !ok {
		if file, err := os.Open(resolvePath(r.ctx, name)); err == nil {
			r.files.closers = append(r.files.closers, file)
			rd = bufio.NewReader(file)
		}
		r.files.readers[name] = rd
	}
	if rd == nil {
		return "", false
	}
	line, _ := rd.ReadString('\n')
	if line == "" {
		return "", false
	}
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	return line, true
}

func (r *sedRun) flushAppends() {
	for _, a := range r.appends {
		if a.file == "" {
			r.out.text(a.text)
			continue
		}
		var data []byte
		var err error
		if a.file == "/dev/stdin" {
			data, err = io.ReadAll(r.ctx.Stdin)
		} else {
			data, err = os.ReadFile(resolvePath(r.ctx, a.file))
		}
		if err == nil && len(data) > 0 {
			r.out.text(string(data))
		}
	}
	r.appends = r.appends[:0]
}

func sedTransliterate(ps []byte, from, to []rune) []byte {
	out := make([]byte, 0, len(ps))
	for len(ps) > 0 {
		ch, w := utf8.DecodeRune(ps)
		mapped := false
		if ch != utf8.RuneError || w != 1 {
			for i, f := range from {
				if f == ch {
					out = utf8.AppendRune(out, to[i])
					mapped = true
					break
				}
			}
		}
		if !mapped {
			out = append(out, ps[:w]...)
		}
		ps = ps[w:]
	}
	return out
}

// sedList renders the pattern space unambiguously for l: C escapes, octal
// for other non-printable bytes (C locale), lines folded with a trailing
// backslash so none is longer than width, and a final $.
func sedList(ps []byte, width int, delim byte) string {
	var b strings.Builder
	col := 0
	for _, c := range ps {
		tok := sedListByte(c)
		if width > 1 && col+len(tok) > width-1 {
			b.WriteString("\\")
			b.WriteByte(delim)
			col = 0
		}
		b.WriteString(tok)
		col += len(tok)
	}
	return b.String() + "$"
}

func sedListByte(c byte) string {
	if i := strings.IndexByte("\\\a\b\f\n\r\t\v", c); i >= 0 {
		return `\` + string(`\abfnrtv`[i])
	}
	if c >= 0x20 && c < 0x7f {
		return string(c)
	}
	return fmt.Sprintf("\\%03o", c)
}

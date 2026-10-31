package builtins

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const sedSpec = "n(quiet)(silent)e(expression):f(file):E(regexp-extended)rs(separate)z(null-data)u(unbuffered)" +
	"(posix)i(in-place):?l#(line-length)(debug)(sandbox)(follow-symlinks)"

var sedSpecParsed = mustParseSpec("sed", sedSpec)

func init() {
	Register(Command{Name: "sed", Run: runSed})
}

// sedUnsupported names commands that parse but have no implementation:
// GNU's deprecated L (fmt).
const sedUnsupported = "L"

type sedPiece struct {
	label string
	start int
}

func runSed(ctx *Context) int {
	flags, err := sedSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "sed: %v\n", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, sedSpecParsed) {
		return 0
	}
	for _, opt := range [][2]string{{"debug", "--debug"}, {"sandbox", "--sandbox"}} {
		if _, ok := flags.String(opt[0]); ok {
			fmt.Fprintf(ctx.Stderr, "sed: %s is not supported yet\n", opt[1])
			return 1
		}
	}
	src, pieces, files, err := sedScript(ctx, flags)
	if err != nil {
		if err == errSedUsage {
			fmt.Fprintln(ctx.Stderr, "Usage: sed [OPTION]... {script-only-if-no-other-script} [input-file]...")
		} else {
			fmt.Fprintf(ctx.Stderr, "sed: %v\n", err)
		}
		return 1
	}
	ere := flags.Bool("E") || flags.Bool("r")
	cmds, quiet, err := parseSedScript(src, ere, !flags.Bool("posix"))
	if err == nil {
		err = sedCheckUnsupported(cmds)
	}
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "sed: %s\n", sedLocate(err, src, pieces))
		return 1
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	r := newSedRun(ctx, cmds, files)
	r.quiet = quiet || flags.Bool("n")
	r.separate = flags.Bool("s")
	r.out.unbuffered = flags.Bool("u")
	if n, ok := flags.Num("l"); ok {
		r.lineWidth = n
	}
	if flags.Bool("z") {
		r.delim, r.out.delim = 0, 0
	}
	if suffix, ok := flags.String("i"); ok {
		r.inPlace = &sedInPlace{suffix: suffix, follow: flags.Bool("follow-symlinks")}
		r.separate = true
	}
	return r.run()
}

var errSedUsage = errors.New("usage")

// sedScript joins -e and -f sources in command-line order, each on its own
// line, so "a\" at the end of one -e continues into the next.
func sedScript(ctx *Context, flags *ParseResult) (string, []sedPiece, []string, error) {
	exprs, files := flags.AllStrings("e"), flags.AllStrings("f")
	var parts []string
	var pieces []sedPiece
	start := 0
	add := func(label, text string) {
		pieces = append(pieces, sedPiece{label: label, start: start})
		parts = append(parts, text)
		start += len(text) + 1
	}
	ei, fi := 0, 0
	for _, key := range flags.Order() {
		switch key {
		case "e":
			add(fmt.Sprintf("-e expression #%d", ei+1), exprs[ei])
			ei++
		case "f":
			name := files[fi]
			fi++
			data, err := sedReadScript(ctx, name)
			if err != nil {
				return "", nil, nil, fmt.Errorf("couldn't open file %s: %s", name, grepErrText(err))
			}
			add("file "+name, strings.TrimSuffix(string(data), "\n"))
		}
	}
	operands := flags.Operands
	if len(parts) == 0 {
		if len(operands) == 0 {
			return "", nil, nil, errSedUsage
		}
		add("-e expression #1", operands[0])
		operands = operands[1:]
	}
	return strings.Join(parts, "\n"), pieces, operands, nil
}

func sedReadScript(ctx *Context, name string) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(ctx.Stdin)
	}
	return os.ReadFile(resolvePath(ctx, name))
}

// sedLocate formats a script error as GNU sed does, naming the -e
// expression or file and the 1-based character offset within it.
func sedLocate(err error, src string, pieces []sedPiece) string {
	var se *sedScriptError
	if !errors.As(err, &se) {
		return err.Error()
	}
	piece := pieces[0]
	for _, p := range pieces {
		if p.start <= se.pos {
			piece = p
		}
	}
	return fmt.Sprintf("%s, char %d: %s", piece.label, min(se.pos, len(src))-piece.start+1, se.msg)
}

func sedCheckUnsupported(cmds []*sedCmd) error {
	for _, c := range cmds {
		if strings.IndexByte(sedUnsupported, c.name) >= 0 {
			return &sedScriptError{pos: c.pos, msg: fmt.Sprintf("`%c' command is not supported", c.name)}
		}
	}
	return nil
}

// sedOutput writes records, holding back the newline of a last line that had
// none until something else is written after it.
type sedOutput struct {
	w          *bufio.Writer
	delim      byte
	missing    bool
	unbuffered bool
	err        error
}

func (o *sedOutput) record(b []byte, newline bool) {
	o.pending()
	o.write(b)
	if newline {
		o.write([]byte{o.delim})
	} else {
		o.missing = true
	}
}

func (o *sedOutput) text(s string) {
	o.pending()
	o.write([]byte(s))
}

func (o *sedOutput) pending() {
	if o.missing {
		o.missing = false
		o.write([]byte{o.delim})
	}
}

func (o *sedOutput) write(b []byte) {
	if o.err == nil {
		_, o.err = o.w.Write(b)
	}
}

// sedInput reads records from the input files in turn, one record ahead,
// so that "$" knows whether the current line is the last.
type sedInput struct {
	ctx       *Context
	names     []string
	delim     byte
	separate  bool
	cur       *bufio.Reader
	closer    io.Closer
	name      string
	next      []byte
	nextNL    bool
	hasNext   bool
	failed    bool
	restarted bool
}

func (in *sedInput) open() bool {
	for len(in.names) > 0 {
		name := in.names[0]
		in.names = in.names[1:]
		if name == "-" {
			in.cur, in.closer, in.name = bufio.NewReader(in.ctx.Stdin), nil, "-"
			return true
		}
		f, err := os.Open(resolvePath(in.ctx, name))
		if err == nil {
			if st, serr := f.Stat(); serr == nil && st.IsDir() {
				f.Close()
				err = fmt.Errorf("read error on %s: Is a directory", name)
				fmt.Fprintf(in.ctx.Stderr, "sed: %v\n", err)
				in.failed = true
				continue
			}
			in.cur, in.closer, in.name = bufio.NewReader(f), f, name
			return true
		}
		fmt.Fprintf(in.ctx.Stderr, "sed: can't read %s: %s\n", name, grepErrText(err))
		in.failed = true
	}
	return false
}

// fill loads the lookahead record. Unless crossFiles is false it moves on
// to later files when the current one is exhausted.
func (in *sedInput) fill(crossFiles bool) {
	in.hasNext = false
	for {
		if in.cur != nil {
			rec, err := in.cur.ReadBytes(in.delim)
			if len(rec) > 0 {
				in.nextNL = rec[len(rec)-1] == in.delim && err == nil
				if in.nextNL {
					rec = rec[:len(rec)-1]
				}
				in.next, in.hasNext = rec, true
				return
			}
			if in.closer != nil {
				in.closer.Close()
			}
			in.cur = nil
			if !crossFiles {
				return
			}
		}
		if !in.open() {
			return
		}
	}
}

// read returns the next record and whether it ended with the delimiter.
// With -s the lookahead stops at the end of each file, and restarted
// reports that read had to move on to the next one.
func (in *sedInput) read() ([]byte, bool, bool) {
	if !in.hasNext {
		in.fill(true)
		if !in.hasNext {
			return nil, false, false
		}
		in.restarted = true
	}
	rec, nl := in.next, in.nextNL
	in.fill(!in.separate)
	return rec, nl, true
}

func (in *sedInput) last() bool { return !in.hasNext }

// more reports whether a later file still has records, for n and N at the
// end of a file read with -s.
func (in *sedInput) more() bool {
	in.fill(true)
	if in.hasNext {
		in.restarted = true
	}
	return in.hasNext
}

// bunzip2 and bzcat: bzip2 decompression through Go's compress/bzip2.
//
// Behaviour (option set, suffix handling, -f "replace original", the temp file
// that is only renamed once a stream is whole) follows toybox
// toys/other/bzcat.c, 0BSD, at commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb.
// No external bzip2 is ever run; the decoder is the standard library's.
//
// Differences from toybox, all deliberate:
//   - there is no -v: a successful file prints nothing.
//   - -k keeps the input and still writes the output, like gzip -k; toybox's -k
//     throws the decompressed data away.
//   - an existing output is never replaced without -f, and never interactively:
//     stdin may not be a tty, so there is no prompt at all.
//   - .bz2, .tbz2, .tbz and .bz are recognised; toybox only trims .bz and .bz2.
//     A name with no known suffix is refused with gzip's wording unless -f is
//     given, which replaces the original file.
//   - every stream of a concatenated file is decoded, and trailing bytes after
//     the last stream are an error instead of being ignored: toybox stops after
//     the first stream.
//
// The plumbing shared with gzip (-c/-f/-k/-t, stdin, the temp file and the
// rename) lives in decompress.go.
package builtins

import (
	"bufio"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"strings"
)

const bunzip2Spec = "c(stdout)f(force)k(keep)t(test)"

var (
	bunzip2SpecParsed *Spec
	bzcatSpecParsed   *Spec
)

func init() {
	var err error
	if bunzip2SpecParsed, err = ParseSpec(bunzip2Spec); err != nil {
		panic("builtins: bad bunzip2 spec: " + err.Error())
	}
	// bzcat is bunzip2 -c and takes no options of its own.
	if bzcatSpecParsed, err = ParseSpec(""); err != nil {
		panic("builtins: bad bzcat spec: " + err.Error())
	}
	Register(Command{Name: "bunzip2", Run: func(ctx *Context) int { return runBunzip2(ctx, "bunzip2") }})
	Register(Command{Name: "bzcat", Run: func(ctx *Context) int { return runBunzip2(ctx, "bzcat") }})
}

const bzip2NotBzip2 = "not bzip2 data"

// bzip2Suffixes name the output of a decompression: .tbz2 and .tbz are tarballs.
var bzip2Suffixes = []suffixRule{{".bz2", ""}, {".tbz2", ".tar"}, {".tbz", ".tar"}, {".bz", ""}}

type bzip2Options struct {
	codec codec
	flags streamFlags
}

func runBunzip2(ctx *Context, name string) int {
	spec := bunzip2SpecParsed
	cat := name == "bzcat"
	if cat {
		spec = bzcatSpecParsed
	}

	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %v\n", name, err)
		return 1
	}
	// Usage/version text names the registered command rather than ctx.CmdName,
	// which is not always set (tests, in-process panes).
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, spec.GenerateUsage(name))
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", name)
		return 0
	}

	o := &bzip2Options{
		codec: codec{name: name, transform: bunzip2Transform, errText: bzip2ErrText, suffixes: bzip2Suffixes},
		flags: streamFlags{
			toStdout: cat || flags.Bool("c"),
			force:    flags.Bool("f"),
			keep:     flags.Bool("k"),
			test:     flags.Bool("t"),
		},
	}

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}

	status := 0
	for _, operand := range files {
		if !o.runOne(ctx, operand) {
			status = 1
		}
	}
	return status
}

func (o *bzip2Options) runOne(ctx *Context, operand string) bool {
	src, ok := openInput(ctx, o.codec.name, operand, o.flags.force)
	if !ok {
		return false
	}
	defer src.close()

	if src.isStdin() || o.flags.toStdout || o.flags.test {
		return o.codec.toStream(ctx, o.flags, src, false)
	}
	outDisplay, ok := o.outputName(src.display)
	if !ok {
		fmt.Fprintf(ctx.Stderr, "%s: %s: unknown suffix -- ignored (use -c to write to stdout)\n", o.codec.name, src.display)
		return false
	}
	return o.codec.toFile(ctx, o.flags, src, resolvePath(ctx, operand), resolvePath(ctx, outDisplay), src.modTime)
}

// outputName maps an operand to the decompressed name. With -f a name with no
// known suffix replaces the original file, like toybox's "replace original".
func (o *bzip2Options) outputName(display string) (string, bool) {
	if out, ok := o.codec.outputName(display); ok {
		return out, true
	}
	return display, o.flags.force
}

func bunzip2Transform(src *input, out io.Writer) error {
	return bunzip2Stream(src.reader, out)
}

// bunzip2Stream decodes every stream of a concatenated bzip2 input. An empty
// input is not bzip2 data, and bytes after the last stream are an error rather
// than silently ignored.
func bunzip2Stream(in io.Reader, out io.Writer) error {
	br := bufio.NewReader(in)
	for streams := 0; ; streams++ {
		// compress/bzip2.Reader follows a concatenated stream itself and
		// reports io.EOF at the end of the last one, so this loop usually runs
		// once; it is here so a decoder that stops at a stream boundary still
		// decodes the rest.
		_, err := br.Peek(1)
		if errors.Is(err, io.EOF) {
			if streams > 0 {
				return nil
			}
			return errors.New(bzip2NotBzip2)
		}
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, bzip2.NewReader(br)); err != nil {
			return err
		}
	}
}

func bzip2ErrText(err error) string {
	var structural bzip2.StructuralError
	if errors.As(err, &structural) {
		switch msg := string(structural); {
		case msg == "bad magic value":
			return bzip2NotBzip2
		case strings.Contains(msg, "continuation"):
			return "trailing data after the last bzip2 stream"
		case strings.Contains(msg, "checksum"):
			return "invalid checksum (corrupt data)"
		default:
			return msg
		}
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return "unexpected end of file"
	}
	return err.Error()
}

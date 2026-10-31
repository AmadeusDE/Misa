// xzcat: .xz decompression to stdout through internal/xz, Misa's own decoder.
//
// Behaviour (option set, stdin handling, one operand after another) follows
// toybox toys/pending/xzcat.c, 0BSD, at commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb; the decoder itself is the port in
// internal/xz, which is derived from the same file. No external xz is ever run.
//
// Differences from toybox, all deliberate:
//   - concatenated streams and stream padding are decoded; toybox stops at the
//     end of the first stream.
//   - the LZMA2 dictionary is capped at 64 MiB (the same limit toybox uses) and
//     a stream asking for more is refused before anything is allocated.
//   - truncation is reported as "unexpected end of file" rather than as
//     corruption, and trailing garbage is its own error.
//
// The plumbing shared with gzip and bunzip2 (operands, stdin, "-", the
// diagnostics) lives in decompress.go. xzcat has no -c/-f/-k/-t of its own: it
// always writes to stdout, exactly like toybox's xzcat. Reading compressed data
// from a terminal is allowed, because there is no -f to force it with and
// toybox's xzcat does not check either.
package builtins

import (
	"errors"
	"fmt"
	"io"

	"github.com/AmadeusDE/Misa/internal/xz"
)

const xzcatSpec = ""

var xzcatSpecParsed *Spec

func init() {
	var err error
	if xzcatSpecParsed, err = ParseSpec(xzcatSpec); err != nil {
		panic("builtins: bad xzcat spec: " + err.Error())
	}
	Register(Command{Name: "xzcat", Run: runXzcat})
}

func runXzcat(ctx *Context) int {
	flags, err := xzcatSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "xzcat: %v\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, xzcatSpecParsed.GenerateUsage("xzcat"))
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "xzcat (Misa) 0.1.0\n")
		return 0
	}

	c := codec{name: "xzcat", transform: xzcatTransform, errText: xzErrText}
	f := streamFlags{toStdout: true}

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}

	status := 0
	for _, operand := range files {
		// force is set: there is no -f, so a terminal is not refused.
		src, ok := openInput(ctx, "xzcat", operand, true)
		if !ok {
			status = 1
			continue
		}
		if !c.toStream(ctx, f, src, false) {
			status = 1
		}
		src.close()
	}
	return status
}

func xzcatTransform(src *input, out io.Writer) error {
	_, err := io.Copy(out, xz.NewReader(src.reader))
	return err
}

// xzErrText turns the decoder's errors into the wording the other decompression
// builtins use.
func xzErrText(err error) string {
	switch {
	case errors.Is(err, xz.ErrNotXZ):
		return "not in xz format"
	case errors.Is(err, xz.ErrCorrupt):
		return "file is corrupt"
	case errors.Is(err, xz.ErrOptions):
		return "unsupported options in the .xz headers"
	case errors.Is(err, xz.ErrMemoryLimit):
		return "memory limit exceeded: LZMA2 dictionary is too big"
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return "unexpected end of file"
	}
	return err.Error()
}

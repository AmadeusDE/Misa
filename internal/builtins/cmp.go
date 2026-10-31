// cmp compares two files byte for byte. Behavior follows uutils
// diffutils cmp (https://github.com/uutils/diffutils, src/cmp.rs, MIT
// licensed), including GNU-compatible message formats: exit status 0 for
// identical, 1 for different, 2 for error. The spec string below is kept
// only for --help usage text; parsing is done by cmpParseArgs.
package builtins

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const cmpSpec = `^b(print-bytes)s(silent)l(verbose)i(ignore-initial):n(bytes):`

var cmpSpecParsed *Spec

func init() {
	var err error
	cmpSpecParsed, err = ParseSpec(cmpSpec)
	if err != nil {
		panic("builtins: bad cmp spec: " + err.Error())
	}
	Register(Command{Name: "cmp", Run: runCmp})
}

const cmpBufSize = 4096

func runCmp(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	params, err := cmpParseArgs(ctx.CmdName, ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, err)
		return 2
	}
	if params.help || params.version {
		flags := &ParseResult{}
		flags.HelpRequested = params.help
		flags.VersionRequested = params.version
		if HandleHelpVersion(ctx, flags, cmpSpecParsed) {
			return 0
		}
	}
	if (params.from == "-" && params.to == "-") || cmpSameFile(ctx, params.from, params.to) {
		return 0
	}
	equal, err := cmpCompare(ctx, &params)
	if err != nil {
		if !params.quiet {
			fmt.Fprintln(ctx.Stderr, err)
		}
		return 2
	}
	if equal {
		return 0
	}
	return 1
}

func cmpSameFile(ctx *Context, from, to string) bool {
	if from == "-" || to == "-" {
		return false
	}
	a, err := os.Stat(resolvePath(ctx, from))
	if err != nil {
		return false
	}
	b, err := os.Stat(resolvePath(ctx, to))
	if err != nil {
		return false
	}
	return os.SameFile(a, b)
}

func cmpCompare(ctx *Context, p *cmpParams) (bool, error) {
	from, err := cmpOpenReader(ctx, p.from, p.skipA)
	if err != nil {
		return false, err
	}
	defer from.close()
	to, err := cmpOpenReader(ctx, p.to, p.skipB)
	if err != nil {
		return false, err
	}
	defer to.close()

	if p.quiet {
		// uutils and GNU skip all comparison I/O when only the sizes
		// differ and silent mode was requested.
		if sa, e1 := os.Stat(resolvePath(ctx, p.from)); e1 == nil {
			if sb, e2 := os.Stat(resolvePath(ctx, p.to)); e2 == nil && sa.Size() != sb.Size() {
				return false, nil
			}
		}
	}

	width := cmpOffsetWidth(ctx, p)

	different := false
	atByte := uint64(1)
	atLine := uint64(1)
	startOfLine := true
	for {
		bufA, err := from.peek(ctx)
		if err != nil {
			return false, err
		}
		bufB, err := to.peek(ctx)
		if err != nil {
			return false, err
		}
		if len(bufA) == 0 && len(bufB) == 0 {
			break
		}
		if len(bufA) == 0 || len(bufB) == 0 {
			eofOn := p.to
			if len(bufA) == 0 {
				eofOn = p.from
			}
			cmpReportEOF(ctx, p, atByte, atLine, startOfLine, eofOn)
			return false, nil
		}
		consumed := min(len(bufA), len(bufB))
		if bytes.Equal(bufA[:consumed], bufB[:consumed]) {
			atByte += uint64(consumed)
			atLine += uint64(bytes.Count(bufA[:consumed], []byte{'\n'}))
			startOfLine = bufA[consumed-1] == '\n'
			if p.maxBytes != nil && atByte > *p.maxBytes {
				break
			}
			from.skip(consumed)
			to.skip(consumed)
			continue
		}
		for i := 0; i < consumed; i++ {
			fa, fb := bufA[i], bufB[i]
			if fa != fb {
				different = true
				if p.verbose {
					if err := cmpVerboseLine(ctx, p, fa, fb, atByte, width); err != nil {
						return false, err
					}
				} else {
					cmpReportDifference(ctx, p, fa, fb, atByte, atLine)
					return false, nil
				}
			}
			startOfLine = fa == '\n'
			if startOfLine {
				atLine++
			}
			atByte++
			if p.maxBytes != nil && atByte > *p.maxBytes {
				break
			}
		}
		from.skip(consumed)
		to.skip(consumed)
	}
	return !different, nil
}

// cmpReader is a buffered input stream with the skip offset already
// consumed; peek returns the buffered bytes, or nil after EOF.
type cmpReader struct {
	r    *bufio.Reader
	name string
	cls  io.Closer
}

func cmpOpenReader(ctx *Context, name string, skip uint64) (*cmpReader, error) {
	var rd cmpReader
	rd.name = name
	if name == "-" {
		if ctx.Stdin == nil {
			return nil, fmt.Errorf("cmp: %s: stdin not available", name)
		}
		rd.r = bufio.NewReaderSize(ctx.Stdin, cmpBufSize)
	} else {
		f, err := os.Open(resolvePath(ctx, name))
		if err != nil {
			return nil, fmt.Errorf("cmp: %s: %s", name, fmtOsErr(err))
		}
		rd.cls = f
		rd.r = bufio.NewReaderSize(ContextReader(ctx.CancellationContext(), f), cmpBufSize)
	}
	if skip > 0 {
		if _, err := io.CopyN(io.Discard, rd.r, int64(skip)); err != nil && err != io.EOF {
			rd.close()
			return nil, fmt.Errorf("cmp: %s: %s", name, fmtOsErr(err))
		}
	}
	return &rd, nil
}

func (r *cmpReader) peek(ctx *Context) ([]byte, error) {
	buf, err := r.r.Peek(cmpBufSize)
	if err == io.EOF {
		return buf, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cmp: %s: %s", r.name, fmtOsErr(err))
	}
	return buf, nil
}

func (r *cmpReader) skip(n int) {
	_, _ = r.r.Discard(n)
}

func (r *cmpReader) close() {
	if r.cls != nil {
		r.cls.Close()
	}
}

// cmpOffsetWidth is the decimal width used to pad -l byte offsets: the
// smaller file size, clamped by --bytes, mirrored from offset_width in
// uutils cmp.
func cmpOffsetWidth(ctx *Context, p *cmpParams) int {
	limit := uint64(math.MaxUint64)
	if p.maxBytes != nil {
		limit = *p.maxBytes
	}
	if sa, e1 := os.Stat(resolvePath(ctx, p.from)); e1 == nil {
		if sb, e2 := os.Stat(resolvePath(ctx, p.to)); e2 == nil {
			smaller := min(sa.Size(), sb.Size())
			if smaller >= 0 && uint64(smaller) < limit {
				limit = uint64(smaller)
			}
		}
	}
	if limit == 0 {
		return 2
	}
	return len(strconv.FormatUint(limit, 10))
}

func cmpReportDifference(ctx *Context, p *cmpParams, fromByte, toByte byte, atByte, atLine uint64) {
	if p.quiet {
		return
	}
	term := "byte"
	if !p.printBytes && cmpPosixLocale(ctx) {
		term = "char"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s differ: %s %d, line %d", p.from, p.to, term, atByte, atLine)
	if p.printBytes {
		fmt.Fprintf(&b, " is %3o %s %3o %s", fromByte, cmpVisibleByte(fromByte), toByte, cmpVisibleByte(toByte))
	}
	b.WriteByte('\n')
	io.WriteString(ctx.Stdout, b.String())
}

func cmpReportEOF(ctx *Context, p *cmpParams, atByte, atLine uint64, startOfLine bool, eofOn string) {
	if p.quiet {
		return
	}
	switch {
	case atByte == 1:
		fmt.Fprintf(ctx.Stderr, "cmp: EOF on %s which is empty\n", eofOn)
	case p.verbose:
		fmt.Fprintf(ctx.Stderr, "cmp: EOF on %s after byte %d\n", eofOn, atByte-1)
	case startOfLine:
		fmt.Fprintf(ctx.Stderr, "cmp: EOF on %s after byte %d, line %d\n", eofOn, atByte-1, atLine-1)
	default:
		fmt.Fprintf(ctx.Stderr, "cmp: EOF on %s after byte %d, in line %d\n", eofOn, atByte-1, atLine)
	}
}

func cmpVerboseLine(ctx *Context, p *cmpParams, fromByte, toByte byte, atByte uint64, width int) error {
	if p.printBytes {
		_, err := fmt.Fprintf(ctx.Stdout, "%*d %3o %-4s %3o %s\n", width, atByte, fromByte, cmpVisibleByte(fromByte), toByte, cmpVisibleByte(toByte))
		return err
	}
	_, err := fmt.Fprintf(ctx.Stdout, "%*d %3o %3o\n", width, atByte, fromByte, toByte)
	return err
}

// cmpPosixLocale reports whether the message locale is C/POSIX, which
// makes GNU-style messages say "char" instead of "byte". An explicitly
// configured non-C locale (or byte-for-byte aliases configured for
// Misa) keeps "byte".
func cmpPosixLocale(ctx *Context) bool {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		prefix := key + "="
		for _, kv := range ctx.Env {
			if strings.HasPrefix(kv, prefix) {
				v := kv[len(prefix):]
				return v == "C" || v == "POSIX"
			}
		}
	}
	return false
}

// cmpVisibleByte renders a byte the way GNU cmp prints it with -b:
// control characters as ^X, DEL as ^?, high bytes as M- escapes.
func cmpVisibleByte(b byte) string {
	switch {
	case b < 32:
		return "^" + string(b+64)
	case b < 127:
		return string(b)
	case b == 127:
		return "^?"
	case b < 160:
		return "M-^" + string(b-64)
	case b < 255:
		return "M-" + string(b-128)
	default:
		return "M-^?"
	}
}

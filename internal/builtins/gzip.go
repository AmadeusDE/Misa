// gzip, gunzip and zcat: RFC 1952 streams through Go's compress/gzip.
//
// Behaviour (option set, suffix handling, overwrite refusal, metadata copy,
// concatenated members) follows toybox toys/lsb/gzip.c, 0BSD, at commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. No external gzip is ever run.
//
// Differences from toybox, all deliberate:
//   - the original name and mtime are stored in the header by default, -n omits
//     them and -N restores them when decompressing; toybox always writes
//     MTIME=0 and no FNAME.
//   - an existing output is never replaced without -f, and never interactively:
//     stdin may not be a tty, so there is no prompt at all (toybox simply opens
//     the output with O_EXCL and reports "File exists").
//   - -c -t still discards the output, where toybox writes -t -c output to
//     stdout.
//   - -l (list) does not exist, in toybox or here.
//
// The plumbing shared with bunzip2 (-c/-f/-k/-t, stdin, the temp file and the
// rename) lives in decompress.go.
package builtins

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// gzipMode is the name the family was invoked under. gunzip always decompresses
// and zcat also writes to stdout, exactly like toybox's gunzip_main/zcat_main
// setting the -d/-c flags before running gzip.
type gzipMode int

const (
	gzipCompress gzipMode = iota
	gzipDecompress
	gzipCat
)

func (m gzipMode) name() string {
	switch m {
	case gzipDecompress:
		return "gunzip"
	case gzipCat:
		return "zcat"
	}
	return "gzip"
}

// The -1..-9 group is exclusive in toybox; the option parser here has no group
// syntax, so gzipLevel rejects a second level after parsing.
const (
	gzipPackSpec   = "c(stdout)d(decompress)f(force)k(keep)n(no-name)N(name)t(test)123456789"
	gzipUnpackSpec = "c(stdout)d(decompress)f(force)k(keep)N(name)t(test)123456789"
)

var (
	gzipPackSpecParsed   *Spec
	gzipUnpackSpecParsed *Spec
)

func init() {
	var err error
	if gzipPackSpecParsed, err = ParseSpec(gzipPackSpec); err != nil {
		panic("builtins: bad gzip spec: " + err.Error())
	}
	if gzipUnpackSpecParsed, err = ParseSpec(gzipUnpackSpec); err != nil {
		panic("builtins: bad gunzip spec: " + err.Error())
	}
	Register(Command{Name: "gzip", Run: func(ctx *Context) int { return runGzip(ctx, gzipCompress) }})
	Register(Command{Name: "gunzip", Run: func(ctx *Context) int { return runGzip(ctx, gzipDecompress) }})
	Register(Command{Name: "zcat", Run: func(ctx *Context) int { return runGzip(ctx, gzipCat) }})
}

const gzipNotGzip = "not in gzip format"

// gzipSuffixes name the output of a decompression: the suffix is dropped, or
// replaced (.tgz is a tarball).
var gzipSuffixes = []suffixRule{{".gz", ""}, {".tgz", ".tar"}, {".z", ""}}

type gzipOptions struct {
	codec      codec
	flags      streamFlags
	decompress bool
	noName     bool
	restore    bool
	level      int
}

func runGzip(ctx *Context, mode gzipMode) int {
	name := mode.name()
	spec := gzipUnpackSpecParsed
	if mode == gzipCompress {
		spec = gzipPackSpecParsed
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

	level, err := gzipLevel(flags)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %v\n", name, err)
		return 1
	}

	o := &gzipOptions{
		flags: streamFlags{
			toStdout: mode == gzipCat || flags.Bool("c"),
			force:    flags.Bool("f"),
			keep:     flags.Bool("k"),
			test:     flags.Bool("t"),
		},
		decompress: mode != gzipCompress || flags.Bool("d") || flags.Bool("t"),
		noName:     flags.Bool("n"),
		restore:    flags.Bool("N"),
		level:      level,
	}
	o.codec = codec{name: name, transform: o.transform, errText: gzipErrText, suffixes: gzipSuffixes}

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

// gzipLevel returns the compression level, rejecting a second -# (toybox's
// [-123456789] option group). The default matches toybox: 6.
func gzipLevel(flags *ParseResult) (int, error) {
	level := 0
	for d := 1; d <= 9; d++ {
		if !flags.Bool(strconv.Itoa(d)) {
			continue
		}
		if level != 0 {
			return 0, fmt.Errorf("only one compression level may be given")
		}
		level = d
	}
	if level == 0 {
		level = gzip.DefaultCompression
	}
	return level, nil
}

func (o *gzipOptions) runOne(ctx *Context, operand string) bool {
	src, ok := openInput(ctx, o.codec.name, operand, o.flags.force)
	if !ok {
		return false
	}
	defer src.close()

	if src.isStdin() || o.flags.toStdout || o.flags.test {
		return o.codec.toStream(ctx, o.flags, src, !o.decompress)
	}
	inPath := resolvePath(ctx, operand)
	if o.decompress {
		return o.decompressToFile(ctx, src, inPath)
	}
	return o.codec.toFile(ctx, o.flags, src, inPath, resolvePath(ctx, operand+".gz"), src.modTime)
}

func (o *gzipOptions) decompressToFile(ctx *Context, src *input, inPath string) bool {
	var header gzip.Header
	if o.restore {
		if h, err := peekGzipHeader(src.file); err == nil {
			header = h
		}
	}

	outDisplay, ok := o.decompressedName(src.display, header.Name)
	if !ok {
		fmt.Fprintf(ctx.Stderr, "%s: %s: unknown suffix -- ignored (use -c to write to stdout)\n", o.codec.name, src.display)
		return false
	}

	modTime := src.modTime
	if o.restore && !header.ModTime.IsZero() {
		modTime = header.ModTime
	}
	return o.codec.toFile(ctx, o.flags, src, inPath, resolvePath(ctx, outDisplay), modTime)
}

// transform runs the codec for one source into out.
func (o *gzipOptions) transform(src *input, out io.Writer) error {
	if o.decompress {
		return gunzipStream(src.reader, out)
	}
	return o.compress(src, out)
}

func (o *gzipOptions) compress(src *input, out io.Writer) error {
	zw, err := gzip.NewWriterLevel(out, o.level)
	if err != nil {
		return err
	}
	if !o.noName {
		// RFC 1952 requires FNAME to be Latin-1; Go refuses anything else.
		if src.base != "" && isLatin1(src.base) {
			zw.Name = src.base
		}
		if !src.modTime.IsZero() {
			zw.ModTime = src.modTime
		}
	}
	zw.OS = 255 // unknown, like toybox
	if _, err := io.Copy(zw, src.reader); err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
}

// gunzipStream decompresses every member of a concatenated stream. An empty
// input is not a gzip stream (toybox: "not gzip"), and trailing bytes after the
// last member are an error rather than silently ignored.
func gunzipStream(in io.Reader, out io.Writer) error {
	br := bufio.NewReader(in)
	members := 0
	for {
		zr, err := gzip.NewReader(br)
		if err != nil {
			if members > 0 && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) {
				return nil
			}
			if members == 0 && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) {
				return errors.New(gzipNotGzip)
			}
			return err
		}
		if _, err := io.Copy(out, zr); err != nil {
			zr.Close()
			return err
		}
		if err := zr.Close(); err != nil {
			return err
		}
		members++
		if _, err := br.Peek(1); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// peekGzipHeader reads the first member's header and rewinds f, for -N.
func peekGzipHeader(f *os.File) (gzip.Header, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return gzip.Header{}, err
	}
	zr, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		f.Seek(0, io.SeekStart)
		return gzip.Header{}, err
	}
	h := zr.Header
	zr.Close()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return gzip.Header{}, err
	}
	return h, nil
}

// decompressedName turns an operand into the output name. The suffix decides,
// like toybox (.gz dropped, .tgz becomes .tar, .z dropped); with -N the name
// stored in the header is used when the suffix says nothing, reduced to its
// base name so it can never leave the input's directory.
func (o *gzipOptions) decompressedName(display, headerName string) (string, bool) {
	if out, ok := o.codec.outputName(display); ok {
		return out, true
	}
	if o.restore && headerName != "" {
		base := filepath.Base(headerName)
		if base != "" && base != "." && base != ".." && base != string(filepath.Separator) {
			return filepath.Join(filepath.Dir(display), base), true
		}
	}
	return "", false
}

func isLatin1(s string) bool {
	for _, r := range s {
		if r > 0xff {
			return false
		}
	}
	return true
}

func gzipErrText(err error) string {
	switch {
	case errors.Is(err, gzip.ErrHeader):
		return gzipNotGzip
	case errors.Is(err, gzip.ErrChecksum):
		return "invalid checksum (corrupt data)"
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return "unexpected end of file"
	}
	return strings.TrimPrefix(err.Error(), "gzip: ")
}

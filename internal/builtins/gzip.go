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
package builtins

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
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

type gzipOptions struct {
	name       string // diagnostic prefix: gzip, gunzip or zcat
	decompress bool
	toStdout   bool
	force      bool
	keep       bool
	noName     bool
	restore    bool
	test       bool
	level      int
}

// gzipSource is one input stream: a file operand or stdin ("-").
type gzipSource struct {
	display string // operand as typed, for diagnostics
	base    string // base name stored in the header ("" for stdin)
	reader  io.Reader
	file    *os.File // nil for stdin
	info    os.FileInfo
	modTime time.Time
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
		name:       name,
		decompress: mode != gzipCompress || flags.Bool("d") || flags.Bool("t"),
		toStdout:   mode == gzipCat || flags.Bool("c"),
		force:      flags.Bool("f"),
		keep:       flags.Bool("k"),
		noName:     flags.Bool("n"),
		restore:    flags.Bool("N"),
		test:       flags.Bool("t"),
		level:      level,
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
	if operand == "-" {
		if ctx.Stdin == nil {
			fmt.Fprintf(ctx.Stderr, "%s: -: stdin not available\n", o.name)
			return false
		}
		if isTerminal(ctx.Stdin) && !o.force {
			fmt.Fprintf(ctx.Stderr, "%s: -: need -f to read from a terminal\n", o.name)
			return false
		}
		return o.toStream(ctx, &gzipSource{display: "-", reader: ctx.Stdin})
	}

	path := resolvePath(ctx, operand)
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, operand, fmtOsErr(err))
		return false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, operand, fmtOsErr(err))
		return false
	}
	if info.IsDir() {
		fmt.Fprintf(ctx.Stderr, "%s: %s: Is a directory\n", o.name, operand)
		return false
	}

	src := &gzipSource{
		display: operand,
		base:    filepath.Base(operand),
		reader:  f,
		file:    f,
		info:    info,
		modTime: info.ModTime(),
	}
	if o.toStdout || o.test {
		return o.toStream(ctx, src)
	}
	if o.decompress {
		return o.decompressToFile(ctx, src, path)
	}
	return o.compressToFile(ctx, src, path)
}

// toStream writes to stdout (or nowhere with -t) and leaves every input alone.
func (o *gzipOptions) toStream(ctx *Context, src *gzipSource) bool {
	if o.test {
		if err := o.transform(src, io.Discard); err != nil {
			fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, src.display, gzipErrText(err))
			return false
		}
		return true
	}
	if !o.decompress && !o.force && writerIsTerminal(ctx.Stdout) {
		fmt.Fprintf(ctx.Stderr, "%s: %s: compressed data not written to a terminal; use -f to force\n", o.name, src.display)
		return false
	}
	bw := bufio.NewWriter(ctx.Stdout)
	if err := o.transform(src, bw); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, src.display, gzipErrText(err))
		return false
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, src.display, fmtOsErr(err))
		return false
	}
	return true
}

func (o *gzipOptions) compressToFile(ctx *Context, src *gzipSource, inPath string) bool {
	return o.replaceFile(ctx, src, inPath, resolvePath(ctx, src.display+".gz"), src.info.Mode(), src.modTime)
}

func (o *gzipOptions) decompressToFile(ctx *Context, src *gzipSource, inPath string) bool {
	var header gzip.Header
	if o.restore {
		if h, err := peekGzipHeader(src.file); err == nil {
			header = h
		}
	}

	outDisplay, ok := decompressedName(src.display, header.Name, o.restore)
	if !ok {
		fmt.Fprintf(ctx.Stderr, "%s: %s: unknown suffix -- ignored (use -c to write to stdout)\n", o.name, src.display)
		return false
	}

	modTime := src.modTime
	if o.restore && !header.ModTime.IsZero() {
		modTime = header.ModTime
	}
	return o.replaceFile(ctx, src, inPath, resolvePath(ctx, outDisplay), src.info.Mode(), modTime)
}

// replaceFile streams src into a temporary file next to outPath and renames it
// into place, so a damaged stream never leaves a half-written output. The
// input is removed unless -k. There is no interactive prompt: stdin may not be
// a tty, so an existing output is always refused without -f.
func (o *gzipOptions) replaceFile(ctx *Context, src *gzipSource, inPath, outPath string, mode os.FileMode, modTime time.Time) bool {
	if !o.force {
		if _, err := os.Lstat(outPath); err == nil {
			fmt.Fprintf(ctx.Stderr, "%s: %s: already exists; use -f to overwrite\n", o.name, outPath)
			return false
		} else if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, outPath, fmtOsErr(err))
			return false
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(outPath), ".gzip-*")
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, outPath, fmtOsErr(err))
		return false
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if err := o.transform(src, tmp); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, src.display, gzipErrText(err))
		return false
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, outPath, fmtOsErr(err))
		return false
	}

	perm := mode & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	if err := os.Chmod(tmpName, perm); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, outPath, fmtOsErr(err))
		return false
	}
	if err := os.Chtimes(tmpName, modTime, modTime); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, outPath, fmtOsErr(err))
		return false
	}
	if err := os.Rename(tmpName, outPath); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, outPath, fmtOsErr(err))
		return false
	}
	renamed = true

	if !o.keep {
		if err := os.Remove(inPath); err != nil {
			fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", o.name, src.display, fmtOsErr(err))
			return false
		}
	}
	return true
}

// transform runs the codec for one source into out.
func (o *gzipOptions) transform(src *gzipSource, out io.Writer) error {
	if o.decompress {
		return gunzipStream(src.reader, out)
	}
	return o.compress(src, out)
}

func (o *gzipOptions) compress(src *gzipSource, out io.Writer) error {
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
func decompressedName(display, headerName string, restore bool) (string, bool) {
	for _, s := range []struct{ suffix, replace string }{{".gz", ""}, {".tgz", ".tar"}, {".z", ""}} {
		if len(display) > len(s.suffix) && strings.HasSuffix(display, s.suffix) {
			return display[:len(display)-len(s.suffix)] + s.replace, true
		}
	}
	if restore && headerName != "" {
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

func writerIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(f.Fd())
}

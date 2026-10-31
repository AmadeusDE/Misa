// Shared plumbing for the compression builtins: gzip's family (gzip.go) and
// bunzip2/bzcat (bunzip2.go).
//
// Everything that is the same for a compressor and a decompressor lives here:
// enumerating operands, reading stdin ("-") and refusing to read compressed
// data from a terminal, writing to stdout versus a file, the temp-file-and-
// rename output path, and the "name: operand: message" diagnostics. The codec,
// its error text and the suffix table stay with the command.
package builtins

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

// input is one operand's worth of bytes: a file, or stdin for "-".
type input struct {
	display string // operand as typed, for diagnostics
	base    string // base name ("" for stdin)
	reader  io.Reader
	file    *os.File // nil for stdin
	info    os.FileInfo
	modTime time.Time
}

// openInput resolves one operand. "-" is stdin, refused when it is a terminal
// unless force is set; files are opened relative to ctx.Dir, and a directory is
// never an input.
func openInput(ctx *Context, name, operand string, force bool) (*input, bool) {
	if operand == "-" {
		if ctx.Stdin == nil {
			fmt.Fprintf(ctx.Stderr, "%s: -: stdin not available\n", name)
			return nil, false
		}
		if isTerminal(ctx.Stdin) && !force {
			fmt.Fprintf(ctx.Stderr, "%s: -: need -f to read from a terminal\n", name)
			return nil, false
		}
		return &input{display: "-", reader: ctx.Stdin}, true
	}

	f, err := os.Open(resolvePath(ctx, operand))
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", name, operand, fmtOsErr(err))
		return nil, false
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", name, operand, fmtOsErr(err))
		return nil, false
	}
	if info.IsDir() {
		f.Close()
		fmt.Fprintf(ctx.Stderr, "%s: %s: Is a directory\n", name, operand)
		return nil, false
	}
	return &input{
		display: operand,
		base:    filepath.Base(operand),
		reader:  f,
		file:    f,
		info:    info,
		modTime: info.ModTime(),
	}, true
}

// isStdin reports whether the input is the standard input rather than a file;
// stdin is never written to a file, only to stdout.
func (in *input) isStdin() bool { return in.file == nil }

func (in *input) close() {
	if in.file != nil {
		in.file.Close()
	}
}

// streamFlags is the option set the compression builtins share.
type streamFlags struct {
	toStdout bool // -c
	force    bool // -f
	keep     bool // -k
	test     bool // -t
}

// codec is one command's half of the plumbing: the transform that turns a whole
// stream into another, the text for its failures (the "not <format>" message
// included) and the suffixes that name a decompressed output. The source is
// passed on because a compressor may store its name and mtime.
type codec struct {
	name      string
	transform func(src *input, out io.Writer) error
	errText   func(error) string
	suffixes  []suffixRule
}

// outputName maps an operand to the name of the decompressed file using the
// suffix table; ok is false when no suffix matches.
func (c *codec) outputName(display string) (string, bool) {
	return stripKnownSuffix(display, c.suffixes)
}

// toStream writes one source to stdout, or nowhere with -t, and leaves every
// input alone. refuseTerminal keeps compressed data off a terminal.
func (c *codec) toStream(ctx *Context, f streamFlags, src *input, refuseTerminal bool) bool {
	if f.test {
		if err := c.transform(src, io.Discard); err != nil {
			c.report(ctx, src, err)
			return false
		}
		return true
	}
	if refuseTerminal && !f.force && writerIsTerminal(ctx.Stdout) {
		fmt.Fprintf(ctx.Stderr, "%s: %s: compressed data not written to a terminal; use -f to force\n", c.name, src.display)
		return false
	}
	bw := bufio.NewWriter(ctx.Stdout)
	if err := c.transform(src, bw); err != nil {
		c.report(ctx, src, err)
		return false
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", c.name, src.display, fmtOsErr(err))
		return false
	}
	return true
}

func (c *codec) report(ctx *Context, src *input, err error) {
	fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", c.name, src.display, c.errText(err))
}

// toFile streams src into a temporary file next to outPath and renames it into
// place, so a damaged stream never leaves a half-written output. The mode and
// mtime of the input are copied over, and the input is removed unless -k --
// unless the output replaces it (-f on a name with no known suffix), in which
// case nothing is removed. There is no interactive prompt: stdin may not be a
// tty, so an existing output is always refused without -f.
func (c *codec) toFile(ctx *Context, f streamFlags, src *input, inPath, outPath string, modTime time.Time) bool {
	if !f.force {
		if _, err := os.Lstat(outPath); err == nil {
			fmt.Fprintf(ctx.Stderr, "%s: %s: already exists; use -f to overwrite\n", c.name, outPath)
			return false
		} else if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", c.name, outPath, fmtOsErr(err))
			return false
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(outPath), "."+c.name+"-*")
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", c.name, outPath, fmtOsErr(err))
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

	if err := c.transform(src, tmp); err != nil {
		c.report(ctx, src, err)
		return false
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", c.name, outPath, fmtOsErr(err))
		return false
	}

	perm := src.info.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	if err := os.Chmod(tmpName, perm); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", c.name, outPath, fmtOsErr(err))
		return false
	}
	if err := os.Chtimes(tmpName, modTime, modTime); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", c.name, outPath, fmtOsErr(err))
		return false
	}
	if err := os.Rename(tmpName, outPath); err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", c.name, outPath, fmtOsErr(err))
		return false
	}
	renamed = true

	if !f.keep && inPath != outPath {
		if err := os.Remove(inPath); err != nil {
			fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", c.name, src.display, fmtOsErr(err))
			return false
		}
	}
	return true
}

// suffixRule maps the suffix of an operand to the name of the decompressed file.
type suffixRule struct{ suffix, replace string }

// stripKnownSuffix applies the first matching rule, like toybox dropping ".gz"
// for nothing and turning ".tgz" into ".tar".
func stripKnownSuffix(display string, rules []suffixRule) (string, bool) {
	for _, r := range rules {
		if len(display) > len(r.suffix) && strings.HasSuffix(display, r.suffix) {
			return display[:len(display)-len(r.suffix)] + r.replace, true
		}
	}
	return "", false
}

func writerIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(f.Fd())
}

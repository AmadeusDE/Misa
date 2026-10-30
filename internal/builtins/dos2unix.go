// Behaviour adapted from toybox dos2unix.c (0BSD).
//
// dos2unix drops the CR of every CRLF pair, unix2dos turns every LF that is not
// already preceded by a CR into CRLF. A CR that is not followed by LF is passed
// through unchanged in both directions (toybox does the same). Input is handled
// as bytes: NULs, invalid UTF-8 and a byte order mark come out unchanged, since
// ignoring a BOM is what toybox does.
//
// With no operand (or the operand "-") the conversion is a filter from stdin to
// stdout. With file operands each file is converted in place: the output goes to
// a temporary file next to the real target first, which is renamed over the
// target only after it was written and closed completely. The original mode is
// restored on the temporary file, so a failed run leaves the original untouched
// and removes the temporary file. Symlinks are followed, so converting a link
// rewrites its target and leaves the link in place.
package builtins

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// newlineChunk is the streaming read size; the output buffer of twice that size
// covers the worst case where every byte grows into "\r\n".
const newlineChunk = 32 << 10

var dos2unixSpec = mustSpec("")

func init() {
	Register(Command{Name: "dos2unix", Run: func(ctx *Context) int { return runDosUnix(ctx, false) }})
	Register(Command{Name: "unix2dos", Run: func(ctx *Context) int { return runDosUnix(ctx, true) }})
}

func runDosUnix(ctx *Context, toDOS bool) int {
	name := "dos2unix"
	if toDOS {
		name = "unix2dos"
	}

	flags, err := dos2unixSpec.ParseArgs(ctx.Args)
	if err != nil {
		// toybox has no options here, so anything else is rejected, not ignored.
		fmt.Fprintf(ctx.Stderr, "%s: %v\nTry '%s --help' for more information.\n", name, err, name)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, dos2unixSpec.GenerateUsage(name))
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", name)
		return 0
	}

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}

	status := 0
	for _, arg := range files {
		if arg == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintf(ctx.Stderr, "%s: -: stdin not available\n", name)
				status = 1
				continue
			}
			if err := convertNewlines(ctx.Stdout, ctx.Stdin, toDOS); err != nil {
				fmt.Fprintf(ctx.Stderr, "%s: -: %s\n", name, fmtOsErr(err))
				status = 1
			}
			continue
		}
		path := arg
		if !filepath.IsAbs(path) {
			path = filepath.Join(ctx.Dir, path)
		}
		if err := convertFileNewlines(path, toDOS); err != nil {
			fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", name, arg, fmtOsErr(err))
			status = 1
		}
	}
	return status
}

// convertNewlines streams src into dst, converting newlines as it goes. Memory
// use is bounded by the chunk size, no matter how large the input is.
func convertNewlines(dst io.Writer, src io.Reader, toDOS bool) error {
	buf := make([]byte, newlineChunk)
	out := make([]byte, 0, 2*len(buf))
	pendingCR := false // a CR was seen; what follows decides whether it stays

	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			out = out[:0]
			for _, c := range buf[:n] {
				if pendingCR {
					// dos2unix drops the CR of a CRLF pair; unix2dos keeps it.
					if toDOS || c != '\n' {
						out = append(out, '\r')
					}
					pendingCR = false
				} else if toDOS && c == '\n' {
					out = append(out, '\r')
				}
				if c == '\r' {
					pendingCR = true
				} else {
					out = append(out, c)
				}
			}
			if _, werr := dst.Write(out); werr != nil {
				return werr
			}
		}
		if rerr == io.EOF {
			if pendingCR {
				// A CR at end of input is not a CRLF pair, so it survives.
				_, werr := dst.Write([]byte{'\r'})
				return werr
			}
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

// convertFileNewlines converts path in place through a temporary file in the
// same directory. On any failure the original file and its mode are untouched
// and the temporary file is removed.
func convertFileNewlines(path string, toDOS bool) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	fi, err := in.Stat()
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return &os.PathError{Op: "read", Path: path, Err: errIsDir{}}
	}

	// Rewrite the symlink target, never replace the link with a regular file.
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	dir, base := filepath.Split(target)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, "."+base+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	discard := func(err error) error {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	if err := convertNewlines(tmp, in, toDOS); err != nil {
		return discard(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, fi.Mode().Perm()|(fi.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky))); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// errIsDir reports a directory operand the way the OS reports a read on one.
type errIsDir struct{}

func (errIsDir) Error() string { return "is a directory" }

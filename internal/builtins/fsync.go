// Behaviour adapted from toybox fsync.c (0BSD).
package builtins

import (
	"fmt"
	"os"
	"syscall"
)

const fsyncSpec = "d(data-only)"

var fsyncSpecParsed *Spec

func init() {
	var err error
	fsyncSpecParsed, err = ParseSpec(fsyncSpec)
	if err != nil {
		panic("builtins: bad fsync spec: " + err.Error())
	}
	Register(Command{Name: "fsync", Run: runFsync})
}

func runFsync(ctx *Context) int {
	flags, err := fsyncSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "fsync:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, fsyncSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "fsync: missing operand")
		return 1
	}
	dataOnly := flags.Bool("d")
	code := 0
	for _, name := range flags.Operands {
		if ctx.CancellationContext().Err() != nil {
			return ExitStatus(ctx.CancellationContext().Err())
		}
		if err := fsyncPath(ctx, name, dataOnly); err != nil {
			fmt.Fprintf(ctx.Stderr, "fsync: can't sync '%s': %s\n", name, fmtOsErr(err))
			code = 1
		}
	}
	return code
}

// fsyncPath flushes one operand. Directories open read-only and sync fine;
// O_NONBLOCK keeps a FIFO from blocking the open instead of failing.
func fsyncPath(ctx *Context, name string, dataOnly bool) error {
	file, err := os.OpenFile(resolvePath(ctx, name), os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	if dataOnly {
		return fdatasync(file)
	}
	return file.Sync()
}

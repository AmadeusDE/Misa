// Behaviour adapted from toybox unlink.c (0BSD) and uutils coreutils unlink (MIT).
package builtins

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

func init() {
	Register(Command{Name: "unlink", Run: runUnlink})
}

func runUnlink(ctx *Context) int {
	spec, _ := ParseSpec("")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "unlink:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	if len(flags.Operands) != 1 {
		fmt.Fprintln(ctx.Stderr, "unlink: 1 value required\nUsage: unlink FILE")
		return 1
	}
	name := flags.Operands[0]
	path := name
	if name != "" {
		path = resolvePath(ctx, name)
	}
	if runtime.GOOS == "windows" {
		if info, lerr := os.Lstat(path); lerr == nil && info.IsDir() {
			fmt.Fprintf(ctx.Stderr, "unlink: cannot unlink '%s': Is a directory\n", name)
			return 1
		}
	}
	if err := syscall.Unlink(path); err != nil {
		fmt.Fprintf(ctx.Stderr, "unlink: cannot unlink '%s': %s\n", name, fmtOsErr(&os.PathError{Op: "unlink", Path: path, Err: err}))
		return 1
	}
	return 0
}

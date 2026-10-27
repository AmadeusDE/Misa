//go:build linux

// Behaviour adapted from toybox sync.c (0BSD) and uutils coreutils sync (MIT).
package builtins

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func init() {
	Register(Command{Name: "sync", Run: runSync})
}

func runSync(ctx *Context) int {
	spec, _ := ParseSpec("d(data)f(file-system)")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "sync:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	data, fs := flags.Bool("d"), flags.Bool("f")
	if data && fs {
		fmt.Fprintln(ctx.Stderr, "sync: --data and --file-system are mutually exclusive")
		return 1
	}
	if data && len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "sync: --data needs at least one argument")
		return 1
	}
	code := 0
	for _, name := range flags.Operands {
		file, err := openSyncFile(ctx, name)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "sync: error opening '%s': %s\n", name, fmtOsErr(err))
			code = 1
		} else {
			file.Close()
		}
	}
	if code != 0 {
		return code
	}
	if (!data && !fs) || len(flags.Operands) == 0 {
		syscall.Sync()
		return 0
	}
	for _, name := range flags.Operands {
		file, err := openSyncFile(ctx, name)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "sync: error opening '%s': %s\n", name, fmtOsErr(err))
			code = 1
			continue
		}
		fd := int(file.Fd())
		err = unix.SetNonblock(fd, false)
		if err == nil {
			if fs {
				err = unix.Syncfs(fd)
			} else {
				err = unix.Fdatasync(fd)
			}
		}
		file.Close()
		if err != nil {
			reason := fmtOsErr(&os.PathError{Op: "sync", Path: name, Err: err})
			if err == syscall.EINVAL {
				reason = "Invalid input"
			}
			fmt.Fprintf(ctx.Stderr, "sync: error syncing '%s': %s\n", name, reason)
			code = 1
		}
	}
	return code
}

func openSyncFile(ctx *Context, name string) (*os.File, error) {
	path := name
	if name != "" {
		path = resolvePath(ctx, name)
	}
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

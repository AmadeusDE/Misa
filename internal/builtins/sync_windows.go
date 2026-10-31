//go:build windows

package builtins

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

func init() {
	Register(Command{Name: "sync", Run: runSync})
}

// runSync flushes the named files with FlushFileBuffers. Windows has no call
// that flushes every volume without administrator rights, so a bare sync, and
// --file-system, report that instead of pretending.
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
	if flags.Bool("f") {
		l, _ := platformLimit("sync", "--file-system")
		return failUnsupported(ctx, l)
	}
	if len(flags.Operands) == 0 {
		l, _ := platformLimit("sync", "no operands")
		return failUnsupported(ctx, l)
	}
	code := 0
	for _, name := range flags.Operands {
		if err := flushFile(resolvePath(ctx, name)); err != nil {
			fmt.Fprintf(ctx.Stderr, "sync: error syncing '%s': %s\n", name, fmtOsErr(err))
			code = 1
		}
	}
	return code
}

// flushFile opens path for writing (FlushFileBuffers needs GENERIC_WRITE;
// directories need backup semantics) and flushes it.
func flushFile(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer windows.CloseHandle(h)
	if err := windows.FlushFileBuffers(h); err != nil {
		return &os.PathError{Op: "sync", Path: path, Err: err}
	}
	return nil
}

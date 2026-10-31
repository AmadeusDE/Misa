//go:build !(linux || darwin || dragonfly || freebsd || netbsd || openbsd || solaris)

package builtins

import "fmt"

// flock needs flock(2), which this platform does not provide (or Go does not
// bind). The command still exists so scripts get an honest error and status
// rather than "command not found".
func init() {
	Register(Command{Name: "flock", Run: runFlockUnsupported})
}

func runFlockUnsupported(ctx *Context) int {
	for _, arg := range ctx.Args {
		switch arg {
		case "--help", "-h":
			fmt.Fprintln(ctx.Stdout, "Usage: flock [-sxun] [-w SECS] FILE COMMAND [ARG]...\nManage advisory file locks (not supported on this platform).")
			return 0
		case "--version":
			fmt.Fprintln(ctx.Stdout, "flock (Misa) 0.1.0")
			return 0
		}
	}
	fmt.Fprintln(ctx.Stderr, "flock: not supported on this platform (needs flock(2))")
	return 1
}

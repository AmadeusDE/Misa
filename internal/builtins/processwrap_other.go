//go:build !unix

package builtins

import (
	"errors"
	"fmt"
)

// errUnsupportedPlatform is returned by process wrappers on hosts without
// POSIX process control.
var errUnsupportedPlatform = errors.New("unsupported on this platform")

func init() {
	// The names exist on every platform so a script gets an honest error
	// instead of "command not found".
	for _, name := range []string{"nohup", "setsid", "setpgid", "renice"} {
		Register(Command{Name: name, Run: runUnsupportedPlatform})
	}
}

func runUnsupportedPlatform(ctx *Context) int {
	fmt.Fprintf(ctx.Stderr, "%s: %v\n", ctx.CmdName, errUnsupportedPlatform)
	return ExitInternal
}

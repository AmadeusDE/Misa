//go:build unix

package builtins

import (
	"fmt"
	"os"
	"syscall"
)

func currentNice() (int, error) { return syscall.Getpriority(syscall.PRIO_PROCESS, 0) }

// niceRequestFor changes the priority of this process before the command
// runs: the command inherits it, and a multicall builtin is its own process.
func niceRequestFor(ctx *Context, adj int) niceRequest {
	curPrio, err := currentNice()
	if err == nil {
		newPrio := curPrio + adj
		if err := syscall.Setpriority(syscall.PRIO_PROCESS, 0, newPrio); err != nil || (adj < 0 && os.Geteuid() != 0) {
			fmt.Fprintln(ctx.Stderr, "nice: warning: setpriority: Permission denied")
		}
	}
	return niceRequest{}
}

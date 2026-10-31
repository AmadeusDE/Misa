//go:build windows

package builtins

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func currentNice() (int, error) {
	class, err := windows.GetPriorityClass(windows.CurrentProcess())
	if err != nil {
		return 0, err
	}
	return niceForPriorityClass(class), nil
}

// niceRequestFor sets the priority class of the new process only, through its
// creation flags. Misa's own priority never changes. A builtin that runs inside
// this process cannot be reprioritised, so the user is told.
func niceRequestFor(ctx *Context, adj int) niceRequest {
	cur, err := currentNice()
	if err != nil {
		cur = 0
	}
	class := priorityClassForNice(cur + adj)
	return niceRequest{
		prep: func(cmd *exec.Cmd) {
			if cmd.SysProcAttr == nil {
				cmd.SysProcAttr = &syscall.SysProcAttr{}
			}
			cmd.SysProcAttr.CreationFlags |= class
		},
		builtinNote: "nice: warning: a Misa builtin runs inside the shell process, so its priority is not changed",
	}
}

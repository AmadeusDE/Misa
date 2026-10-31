package builtins

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// awkShell runs the command lines of awk's system(), pipes and getline with
// `sh -c`, the way Misa runs input that needs a shell. It moves to the
// internal shell engine when that lands.
func awkShell(ctx *Context) func(context.Context, string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(c context.Context, line string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		cmd := exec.CommandContext(c, "sh", "-c", line)
		cmd.Dir = ctx.Dir
		cmd.Env = ctx.Env
		cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
		cmd.WaitDelay = 100 * time.Millisecond
		err := cmd.Run()
		var exit *exec.ExitError
		switch {
		case err == nil, errors.Is(err, exec.ErrWaitDelay):
			return 0, nil
		case errors.As(err, &exit):
			if st, ok := exit.Sys().(syscall.WaitStatus); ok && st.Signaled() {
				return 256 + int(st.Signal()), nil
			}
			return exit.ExitCode(), nil
		}
		return 127, err
	}
}

// awkEnviron is the environment behind ENVIRON: the context's, or the
// process's when the context does not carry one.
func awkEnviron(ctx *Context) []string {
	if ctx.Env != nil {
		return ctx.Env
	}
	return os.Environ()
}

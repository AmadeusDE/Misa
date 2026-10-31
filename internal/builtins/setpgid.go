//go:build unix

// setpgid is a Misa-specific wrapper: toybox has no setpgid utility, so the
// contract here is Misa's own (see docs/AGENTS.md and docs/CREDITS.md).
package builtins

import (
	"fmt"

	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

func init() {
	Register(Command{Name: "setpgid", Run: runSetpgid})
}

func runSetpgid(ctx *Context) int {
	argv, status := setpgidOperands(ctx)
	if argv == nil {
		return status
	}
	// The launcher owns an explicit process group for every child it starts, so
	// the command and its descendants can be signalled as one group. The
	// calling process keeps its own group: only the child moves.
	inv := proceffect.Invocation{Dir: ctx.Dir, Env: ctx.Environ(), Stdin: ctx.Stdin, Stdout: ctx.Stdout, Stderr: ctx.Stderr}
	proc, err := launchCommand(ctx, argv, proceffect.Request{}, inv, commandexec.Options{})
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "setpgid: failed to run command '%s': %s\n", argv[0], fmtOsErr(err))
		return launchStatus(ctx, err)
	}
	return proc.Wait()
}

func setpgidOperands(ctx *Context) ([]string, int) {
	for i, a := range ctx.Args {
		switch {
		case a == "--":
			if i+1 < len(ctx.Args) {
				return ctx.Args[i+1:], 0
			}
			return nil, setpgidUsageError(ctx, "missing operand")
		case a == "--help":
			fmt.Fprint(ctx.Stdout, "Usage: setpgid [--] COMMAND [ARG]...\n\nRun COMMAND in its own process group and wait for it.\n")
			return nil, 0
		case a == "--version":
			fmt.Fprintln(ctx.Stdout, "setpgid (Misa) 0.1.0")
			return nil, 0
		case len(a) > 1 && a[0] == '-':
			return nil, setpgidUsageError(ctx, fmt.Sprintf("unrecognized option '%s'", a))
		default:
			return ctx.Args[i:], 0
		}
	}
	return nil, setpgidUsageError(ctx, "missing operand")
}

func setpgidUsageError(ctx *Context, message string) int {
	fmt.Fprintf(ctx.Stderr, "setpgid: %s\nTry 'setpgid --help' for more information.\n", message)
	return ExitInternal
}

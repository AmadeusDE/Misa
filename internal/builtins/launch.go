//go:build unix

package builtins

import (
	"errors"
	"strings"

	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

// launchCommand starts argv through the shared launcher. A bare registered
// builtin name runs in the isolated child's dispatcher; anything else is an
// external program resolved against the invocation's PATH. Requests that must
// reach the child (nohup's ignored SIGHUP, a nice adjustment) always take the
// helper.
func launchCommand(ctx *Context, argv []string, req proceffect.Request, inv proceffect.Invocation, opts commandexec.Options) (*commandexec.Process, error) {
	builtin := false
	if _, ok := Lookup(argv[0]); ok {
		builtin = !strings.ContainsAny(argv[0], `/\`)
	}
	if builtin || req.IgnoreHup || req.Adjustment != 0 {
		req.DispatchOnly = req.DispatchOnly || builtin
		req.Argv = argv
		return proceffect.StartOptions(ctx.CancellationContext(), req, inv, opts)
	}
	return commandexec.Start(ctx.CancellationContext(), argv[0], commandexec.Invocation{
		Dir: inv.Dir, Env: inv.Env, Args: argv[1:],
		Stdin: inv.Stdin, Stdout: inv.Stdout, Stderr: inv.Stderr,
	}, opts)
}

// launchStatus maps a launch failure to the status a process wrapper exits
// with: 125 when isolation is unavailable, otherwise the launcher's 126/127
// convention.
func launchStatus(ctx *Context, err error) int {
	if errors.Is(err, proceffect.ErrUnavailable) {
		return ExitInternal
	}
	if e := ctx.CancellationContext().Err(); e != nil {
		return commandexec.ContextStatus(e)
	}
	return commandexec.ErrorStatus(err)
}

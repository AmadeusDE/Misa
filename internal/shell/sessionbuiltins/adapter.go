// Package sessionbuiltins adapts shell session invocations to the F02 builtin
// and external command dispatcher.
//
// This adapter does not make all builtins invocation-safe: env's default nested
// execution still uses the process-environment bridge (F03), nice can change
// process/thread priority (F04), and many streaming builtins do not yet honor
// cancellation (F05). timeout retains its legacy signal handling (F06).
// Other unaudited builtins may also use process-global state. See
// docs/plans/invocation.md for the current boundaries.
//
// Configure additionally supplies the S04 resolution hooks, so a session's
// `type`, `command -v`, and `hash` see the same Misa registry utilities and
// PATH executables that Run executes.
package sessionbuiltins

import (
	"context"

	"github.com/AmadeusDE/Misa/internal/builtinexec"
	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/shell/session"
)

// Run dispatches an invocation using builtinexec's builtin and external lookup.
func Run(ctx context.Context, inv session.Invocation) int {
	name := inv.Argv0
	if name == "" {
		name = inv.Name
	}
	return builtinexec.Run(ctx, inv.Name, &builtins.Context{
		CmdName: name, Args: inv.Args, Dir: inv.Dir, Env: inv.Env,
		Stdin: inv.Stdin, Stdout: inv.Stdout, Stderr: inv.Stderr,
	})
}

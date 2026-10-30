// Package builtinexec composes the builtin registry with external execution.
package builtinexec

import (
	"context"
	"strings"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/commandexec"
)

// For selects the default nested executor for a builtin.
func For(name string) builtins.Executor { return Run }

// Run executes exact bare builtin names first; explicit paths are external.
func Run(ctx context.Context, name string, inv *builtins.Context) int {
	if ctx == nil {
		ctx = inv.CancellationContext()
	}
	if err := ctx.Err(); err != nil {
		return builtins.ExitStatus(err)
	}
	if inv == nil {
		inv = &builtins.Context{}
	}
	local := *inv
	local.Exec = For(name)
	if _, ok := builtins.Lookup(name); ok && !strings.ContainsAny(name, `/\`) {
		return builtins.Run(ctx, name, &local)
	}
	return commandexec.Run(ctx, name, commandexec.Invocation{
		Dir: local.Dir, Env: local.Env, Args: local.Args, Argv0: local.CmdName,
		Stdin: local.Stdin, Stdout: local.Stdout, Stderr: local.Stderr,
	})
}

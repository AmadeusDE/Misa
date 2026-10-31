// Package proceffect isolates process-wide effects in a re-executed child.
package proceffect

import (
	"context"
	"io"

	"github.com/AmadeusDE/Misa/internal/commandexec"
)

// Flag is reserved for the private child protocol. Call Handle with os.Args[2:].
const Flag = "--misa-process-effect"

type Request struct {
	Adjustment int
	// DispatchOnly skips priority adjustment and launches the default dispatcher.
	DispatchOnly bool
	// IgnoreHup makes the child ignore SIGHUP. The disposition survives exec,
	// which is how nohup's command outlives a closed terminal.
	IgnoreHup bool
	// Renice changes the priority of other processes from the child, so the
	// calling process is never reniced by accident.
	Renice *ReniceRequest
	Argv   []string
}

// ReniceRequest is one nice-value change for explicit targets.
type ReniceRequest struct {
	Priority int
	// Which is syscall.PRIO_PROCESS, PRIO_PGRP or PRIO_USER.
	Which int
	IDs   []int
}

// Renice applies req in an isolated child and returns its exit status: zero
// when every target was changed, one when any setpriority failed (the child
// reports each failure on stderr).
func Renice(ctx context.Context, req ReniceRequest, inv Invocation) int {
	return run(ctx, Request{Renice: &req}, inv)
}

// Invocation describes the child's exact environment and working directory. A nil
// Env denotes an empty environment. Non-file streams become pipes and are not TTYs.
type Invocation struct {
	Dir            string
	Env            []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// Run executes req in a child without changing the calling process. Nested
// calls share the outer group; inner cancellation kills only the direct child,
// so nested descendants may survive until outer cancellation.
func Run(ctx context.Context, req Request, inv Invocation) int { return run(ctx, req, inv) }

// Umask returns the current process mask without changing it.
func Umask() (int, error) { return umask() }

// Niceness returns the calling thread's current nice value.
func Niceness() (int, error) { return niceness() }

// Handle runs the private child protocol; it must only be called in a child.
func Handle(args []string, dispatch func([]string) int) int {
	if dispatch == nil {
		return HandleContext(args, nil)
	}
	return HandleContext(args, func(_ context.Context, argv []string) int { return dispatch(argv) })
}

// HandleContext supplies cancellation for child builtin dispatch. Only the
// isolated helper subscribes to process signals; library callers never do.
func HandleContext(args []string, dispatch func(context.Context, []string) int) int {
	return handle(args, dispatch)
}

// Start launches a process-effect helper supervised by the shared launcher.
func Start(ctx context.Context, req Request, inv Invocation) (*commandexec.Process, error) {
	return start(ctx, req, inv, commandexec.Options{ShareGroup: sharesGroup()})
}

// StartOptions launches a helper with explicit group ownership (used by timeout).
func StartOptions(ctx context.Context, req Request, inv Invocation, opts commandexec.Options) (*commandexec.Process, error) {
	return start(ctx, req, inv, opts)
}

// Exec replaces the child with an external command resolved using its PATH.
// An unset PATH searches nowhere; an empty PATH searches the current directory.
func Exec(argv []string) int { return execTarget(argv) }

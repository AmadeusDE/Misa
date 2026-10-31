package builtins

import (
	"context"
	"time"
)

// Now captures time from the invocation's clock, or the system clock.
func (ctx *Context) Now() time.Time {
	if ctx.Clock != nil {
		return ctx.Clock()
	}
	return time.Now()
}

// DoneContext supplies cancellation to filesystem operations.
func (ctx *Context) DoneContext() context.Context {
	return ctx.CancellationContext()
}

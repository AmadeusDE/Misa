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

// Location resolves TZ from this invocation without changing process state.
func (ctx *Context) Location() *time.Location {
	if tz, ok := lookupEnv(ctx, "TZ"); ok {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return getLocation(ctx, ctx.Now())
}

// DoneContext supplies cancellation to filesystem operations.
func (ctx *Context) DoneContext() context.Context {
	if ctx.Cancellation != nil {
		return ctx.Cancellation
	}
	return context.Background()
}

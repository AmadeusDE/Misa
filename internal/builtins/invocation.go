package builtins

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CancellationContext returns the invocation context, or Background for legacy callers.
func (c *Context) CancellationContext() context.Context {
	if c != nil && c.Ctx != nil {
		return c.Ctx
	}
	return context.Background()
}

// Environ returns a snapshot of the invocation environment. A nil Env inherits
// the process environment; a non-nil empty Env is deliberately empty.
func (c *Context) Environ() []string {
	if c == nil || c.Env == nil {
		return os.Environ()
	}
	return append([]string{}, c.Env...)
}

func (c *Context) LookupEnv(key string) (string, bool) {
	if c == nil || c.Env == nil {
		return os.LookupEnv(key)
	}
	prefix := key + "="
	for _, entry := range c.Env {
		if strings.HasPrefix(entry, prefix) {
			return entry[len(prefix):], true
		}
	}
	return "", false
}
func (c *Context) Getenv(key string) string { value, _ := c.LookupEnv(key); return value }

// Run dispatches a builtin with normalized invocation-local state. It never
// mutates inv or its argument/environment slices.
func Run(ctx context.Context, name string, inv *Context) int {
	command, ok := Lookup(name)
	if !ok {
		return ExitNotFound
	}
	var c Context
	if inv != nil {
		c = *inv
		if inv.Args != nil {
			c.Args = append([]string{}, inv.Args...)
		}
		if inv.Env != nil {
			c.Env = append([]string{}, inv.Env...)
		}
	}
	if ctx == nil {
		ctx = c.CancellationContext()
	}
	c.Ctx = ctx
	if c.Stdin == nil {
		c.Stdin = strings.NewReader("")
	}
	if c.Stdout == nil {
		c.Stdout = io.Discard
	}
	if c.Stderr == nil {
		c.Stderr = io.Discard
	}
	dir, err := filepath.Abs(c.Dir)
	if err != nil {
		return ExitInternal
	}
	c.Dir = dir
	if c.CmdName == "" {
		c.CmdName = name
	}
	return command.Run(&c)
}

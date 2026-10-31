package builtins

import (
	"context"
	"io"
	"sort"
	"time"
)

// Context is an invocation's state; Run supplies defaults without changing it.
type Context struct {
	Dir     string
	Args    []string
	CmdName string
	Width   int
	Height  int
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader

	// Optional invocation-local services. Nil preserves existing callers.
	Clock func() time.Time
	Env   []string // nil inherits; a non-nil empty slice is deliberately empty.

	Ctx                            context.Context
	StdinTTY, StdoutTTY, StderrTTY bool
	Exec                           Executor
}

// Executor runs a nested command without imposing a dependency on a shell.
type Executor func(context.Context, string, *Context) int

type Command struct {
	Name string
	Run  func(ctx *Context) int
}

var registry = map[string]Command{}

func Register(c Command) {
	registry[c.Name] = c
}

func Lookup(name string) (Command, bool) {
	c, ok := registry[name]
	return c, ok
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

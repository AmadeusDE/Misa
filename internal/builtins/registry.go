package builtins

import (
	"context"
	"io"
	"sort"
	"time"
)

type Context struct {
	Dir     string
	Args    []string
	CmdName string
	Width   int
	Height  int
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader
	Env     []string

	// Optional invocation-local services. Nil preserves existing callers.
	Clock        func() time.Time
	Cancellation context.Context
}

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

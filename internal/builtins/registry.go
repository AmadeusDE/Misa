package builtins

import (
	"io"
	"sort"
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
	// Cancel, when non-nil, is closed to ask a long-running builtin to stop.
	// Only builtins that can stop cooperatively (bc loops) observe it.
	Cancel <-chan struct{}
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

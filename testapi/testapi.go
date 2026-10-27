// Package testapi exposes builtins for external test suites (e.g., misagnutest).
package testapi

import (
	"io"
	"os"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

type Context = builtins.Context

type Command struct {
	Name string
	Run  func(ctx *Context) int
}

func Lookup(name string) (Command, bool) {
	cmd, ok := builtins.Lookup(name)
	if !ok {
		return Command{}, false
	}
	return Command{Name: cmd.Name, Run: cmd.Run}, true
}

func Names() []string {
	return builtins.Names()
}

func NewContext(dir string, args []string, cmdName string, w, h int, stdout, stderr io.Writer, stdin io.Reader) *Context {
	return &Context{
		Dir:     dir,
		Args:    args,
		CmdName: cmdName,
		Width:   w,
		Height:  h,
		Stdout:  stdout,
		Stderr:  stderr,
		Stdin:   stdin,
		Env:     os.Environ(),
	}
}

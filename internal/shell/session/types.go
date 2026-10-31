package session

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Invocation contains the expanded command and its invocation-local state.
// Args excludes Name. Env contains exported name=value pairs.
type Invocation struct {
	Name           string
	Args           []string
	Dir            string
	Env            []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	// Argv0, when non-empty, is the command name presented to the executed
	// program; it defaults to Name. It never affects lookup.
	Argv0 string
}

// ExecHandler executes a command without invoking a shell implicitly.
type ExecHandler func(context.Context, Invocation) int

// Options configures a session. Nil Env snapshots the process environment; an
// explicitly empty Env stays empty. Nil streams mean empty input and discarded output.
type Options struct {
	Dir string
	Env []string
	// Arg0, when non-nil, sets $0 independently of source names, even if empty.
	// Nil preserves the interpreter's source-name/default behavior.
	Arg0 *string
	// Args are positional parameters; ShellOptions are interpreter option words.
	// Both slices are copied during construction.
	Args           []string
	ShellOptions   []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Exec           ExecHandler
	// MisaBuiltin reports whether a bare name is a Misa registry utility. Nil
	// means no registry is known, so only the interpreter's keywords,
	// functions, and builtins resolve before PATH.
	MisaBuiltin func(name string) bool
	// LookPath resolves an executable name through env's PATH, relative to dir.
	// Nil uses the interpreter's PATH search. It should match the Exec handler,
	// so that `type`, `command -v`, and `hash` agree with execution.
	LookPath func(name, dir string, env []string) (string, bool)
	// DefaultPath is the PATH that `command -p` searches. Empty means
	// DefaultCommandPath.
	DefaultPath string
}

// Result describes one submission. Ordinary nonzero shell statuses are not errors.
type Result struct {
	Status     int
	Exited     bool
	ParseError *SyntaxErr
	Err        error
}

// SyntaxErr describes a parse error. Offset is zero-based in bytes; Line and
// Column are one-based (zero if unavailable). Message is the parser diagnostic.
type SyntaxErr struct {
	Offset, Line, Column uint
	Message              string
	Name                 string
}

func (e *SyntaxErr) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("%s:%d:%d: %s", e.Name, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Column, e.Message)
}

// ErrExited indicates that the session has exited or been closed.
var ErrExited = errors.New("shell session exited")

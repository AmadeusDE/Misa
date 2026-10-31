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
}

// ExecHandler executes a command without invoking a shell implicitly.
type ExecHandler func(context.Context, Invocation) int

// Options configures a session. Nil Env snapshots the process environment; an
// explicitly empty Env stays empty. Nil streams mean empty input and discarded output.
type Options struct {
	Dir            string
	Env            []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Exec           ExecHandler
	// OnCwdChange, if non-nil, is called for every directory change the
	// session makes, after the submission that made it. See Session.Run.
	OnCwdChange func(oldDir, newDir string)
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
}

func (e *SyntaxErr) Error() string { return fmt.Sprintf("%d:%d: %s", e.Line, e.Column, e.Message) }

// ErrExited indicates that the session has exited or been closed.
var ErrExited = errors.New("shell session exited")

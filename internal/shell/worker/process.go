package worker

import (
	"errors"
	"io"
	"time"
)

// ProcessOptions specifies the worker executable and its inherited process state.
// Nil Args selects misa's multicall entry point; a non-nil slice replaces it.
// Nil Env inherits the parent environment; a non-nil empty slice clears it.
// Nil streams are empty/discarded. Streams remain caller-owned. Non-file streams
// use asynchronous pumps: a Run result can precede delivery of command output.
// A caller-blocked reader or writer may leave its pump goroutine blocked even
// after the child has been reaped; S06 must drain output to quiescence.
type ProcessOptions struct {
	Executable      string
	Args            []string
	Dir             string
	Env             []string
	Stdin           io.Reader
	Stdout, Stderr  io.Writer
	Version         int
	ShutdownTimeout time.Duration
}

// ErrProcessUnsupported reports that child workers are unavailable on this OS.
var ErrProcessUnsupported = errors.New("shell worker child process unsupported on this platform")

package term

import (
	"errors"
	"io"
)

// ErrUnsupported is returned by Start calls on platforms without a terminal
// transport. Callers fall back to running the program as a multicall process.
var ErrUnsupported = errors.New("terminal transport unavailable on this platform")

// conn is the program's side of the terminal as the core sees it: the byte
// stream both ways plus the window size. The core owns the single reader
// goroutine (StartReader) and the ordered writer queue (input.go); a conn only
// moves bytes. Close must unblock a pending Read.
type conn interface {
	io.ReadWriteCloser
	Resize(rows, cols int) error
}

// proc is the started process. Status delivers the exit status once, in the
// shell's convention (128+signal), after the process is gone. Kill ends the
// whole tree or session, not just the leader, and may be called repeatedly.
type proc interface {
	ID() int
	Status() <-chan int
	Kill()
}

// continuer is the optional job-control capability: a platform whose proc can
// be suspended and resumed (Unix) implements it, and reports suspension through
// startSpec.OnStop. It is not part of the portable minimum.
type continuer interface {
	Continue() error
}

// startSpec is everything a transport needs to launch a program: explicit
// argv, working directory and environment, and the initial window size.
type startSpec struct {
	Argv       []string
	Dir        string
	Env        []string
	Rows, Cols int
	OnStop     func() // called when the process is suspended; may be nil
}

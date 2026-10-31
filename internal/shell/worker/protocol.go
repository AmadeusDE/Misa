// Package worker runs a persistent session over length-prefixed JSON control
// frames, limited to 1 MiB each. The hello exchanges protocol version and
// capabilities; command IDs increase monotonically. Caller streams are never
// closed. Control Close must unblock pending reads and writes.
//
// Canceling while a control write is blocked seals the connection, since a
// partially written frame cannot safely be resumed.
// In-process handlers and caller-provided streams must cooperate with cancellation;
// Go cannot stop a handler or a stream operation that blocks forever.
package worker

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/AmadeusDE/Misa/internal/shell/session"
)

const Version = 1
const maxFrame = 1 << 20

var ErrProtocol = errors.New("worker protocol error")
var ErrCrash = errors.New("worker crashed")

// ErrRuntime identifies a non-syntax interpreter failure.
var ErrRuntime = errors.New("shell runtime error")

// ExitError describes an abnormal worker process exit or crash.
type ExitError struct {
	Status  int
	Message string
	Crash   bool
	Err     error
}

func (e *ExitError) Unwrap() error {
	if e.Crash {
		return ErrCrash
	}
	return e.Err
}

// ExitCode returns the process status; -1 denotes termination by a signal.
func (e *ExitError) ExitCode() int { return e.Status }
func (e *ExitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("worker exited with status %d", e.Status)
}

type Options struct {
	Session session.Options
	Version int
}
type frame struct {
	Type         string             `json:"type"`
	Version      int                `json:"version,omitempty"`
	Capabilities []string           `json:"capabilities,omitempty"`
	ID           uint64             `json:"id,omitempty"`
	Script       string             `json:"script,omitempty"`
	Status       int                `json:"status,omitempty"`
	Exited       bool               `json:"exited,omitempty"`
	Error        string             `json:"error,omitempty"`
	Message      string             `json:"message,omitempty"`
	Syntax       *session.SyntaxErr `json:"syntax,omitempty"`
}

func writeFrame(w io.Writer, f frame) error {
	b, e := json.Marshal(f)
	if e != nil {
		return e
	}
	if len(b) > maxFrame {
		return fmt.Errorf("%w: frame too large", ErrProtocol)
	}
	var h [4]byte
	binary.BigEndian.PutUint32(h[:], uint32(len(b)))
	for _, p := range [][]byte{h[:], b} {
		for len(p) > 0 {
			n, e := w.Write(p)
			if e != nil {
				return e
			}
			if n <= 0 {
				return io.ErrShortWrite
			}
			p = p[n:]
		}
	}
	return nil
}
func readFrame(r io.Reader) (frame, error) {
	var f frame
	var h [4]byte
	if _, e := io.ReadFull(r, h[:]); e != nil {
		return f, e
	}
	n := binary.BigEndian.Uint32(h[:])
	if n == 0 || n > maxFrame {
		return f, fmt.Errorf("%w: invalid frame length", ErrProtocol)
	}
	b := make([]byte, n)
	if _, e := io.ReadFull(r, b); e != nil {
		return f, e
	}
	if e := json.Unmarshal(b, &f); e != nil {
		return f, fmt.Errorf("%w: invalid JSON", ErrProtocol)
	}
	return f, nil
}
func resultFrame(id uint64, r session.Result) frame {
	f := frame{Type: "result", ID: id, Status: r.Status, Exited: r.Exited, Syntax: r.ParseError}
	switch {
	case r.ParseError != nil:
		f.Error = "syntax"
	case errors.Is(r.Err, session.ErrExited):
		f.Error = "exited"
	case errors.Is(r.Err, context.Canceled):
		f.Error = "canceled"
	case errors.Is(r.Err, context.DeadlineExceeded):
		f.Error = "deadline"
	case r.Err != nil:
		f.Error = "runtime"
	}
	return f
}

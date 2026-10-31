// Package commandexec launches external commands without shell fallback.
package commandexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Invocation struct {
	Dir            string
	Env            []string
	Args           []string
	Argv0          string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// Options controls process-group ownership and cancellation. By default Start
// creates and owns a process group; ShareGroup inherits the caller's group,
// and cancellation and group signals then target only the direct child.
type Options struct {
	ShareGroup bool
	// CancelSignal is sent on cancellation when CancelGrace is positive. A nil
	// signal defaults to SIGTERM. With zero or negative grace, SIGKILL is immediate.
	CancelSignal os.Signal
	// CancelGrace bounds the wait for the leader after the graceful signal.
	CancelGrace time.Duration
	// WaitDelay bounds how long Wait keeps reading output after the direct
	// child exits, for descendants that inherited the output pipes. The bound
	// restarts whenever output arrives, so a slow writer never truncates
	// output. Zero waits for EOF.
	WaitDelay time.Duration
}

// Resolve resolves name against the invocation's working directory and PATH.
// An unset PATH never implies the current directory; an empty PATH does.
func Resolve(name string, inv Invocation) (string, error) {
	dir := inv.Dir
	if dir == "" {
		dir = "."
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	check := func(p string) (string, error) {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		p = filepath.Clean(p)
		if err := executable(p); err != nil {
			return "", err
		}
		return p, nil
	}
	if name == "" {
		return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	if strings.ContainsAny(name, `/\`) || filepath.IsAbs(name) {
		return check(name)
	}
	env := inv.Env
	if env == nil {
		env = os.Environ()
	}
	path, found := "", false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			path = e[5:]
			found = true
			break
		}
	}
	if !found {
		return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	var denied error
	for _, entry := range filepath.SplitList(path) {
		if entry == "" {
			entry = "."
		}
		p, e := check(filepath.Join(entry, name))
		if e == nil {
			return p, nil
		}
		if !errors.Is(e, os.ErrNotExist) && denied == nil {
			denied = e
		}
	}
	// SplitList("") returns nil, but an empty PATH means the working directory.
	if path == "" {
		p, e := check(name)
		if e == nil {
			return p, nil
		}
		if !errors.Is(e, os.ErrNotExist) {
			denied = e
		}
	}
	if denied != nil {
		return "", denied
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}
func ContextStatus(err error) int {
	if errors.Is(err, context.DeadlineExceeded) {
		return 124
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	return 125
}
func Run(ctx context.Context, name string, inv Invocation) int {
	p, err := Start(ctx, name, inv, Options{})
	if err != nil {
		if inv.Stderr != nil {
			fmt.Fprintf(inv.Stderr, "%s: %v\n", name, err)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ContextStatus(err)
		}
		return ErrorStatus(err)
	}
	return p.Wait()
}

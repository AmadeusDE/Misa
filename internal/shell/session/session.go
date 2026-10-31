// Package session provides isolated, serialized shell interpreter sessions.
package session

import (
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/AmadeusDE/Misa/internal/sh/expand"
	"github.com/AmadeusDE/Misa/internal/sh/interp"
	"github.com/AmadeusDE/Misa/internal/sh/syntax"
)

type Session struct {
	runMu  sync.Mutex
	mu     sync.Mutex
	closed bool
	cancel context.CancelFunc
	runner *interp.Runner
	parser *syntax.Parser
	input  *os.File // owned pipe read end, if any
}

// New constructs a session without changing the process environment or cwd.
func New(o Options) (*Session, error) {
	env := o.Env
	if env == nil {
		env = os.Environ()
	} else {
		env = slices.Clone(env)
	}
	s := &Session{parser: syntax.NewParser(syntax.Variant(syntax.LangPOSIX))}
	// Dir is validated before attaching input, so constructor errors do not
	// consume a caller-owned reader.
	stdio := func(r *interp.Runner) error {
		in := o.Stdin
		if in != nil {
			if _, ok := in.(*os.File); !ok {
				pr, pw, err := os.Pipe()
				if err != nil {
					return err
				}
				s.input = pr
				go func(source io.Reader) { _, _ = io.Copy(pw, source); _ = pw.Close() }(in)
				in = pr
			}
		}
		return interp.StdIO(in, o.Stdout, o.Stderr)(r)
	}
	handler := o.Exec
	if handler == nil {
		handler = func(context.Context, Invocation) int { return 127 }
	}
	r, err := interp.New(interp.Dir(o.Dir), interp.Env(expand.ListEnviron(env...)), stdio, interp.ExecHandler(func(ctx context.Context, args []string) error {
		hc := interp.HandlerCtx(ctx)
		if hc.Stdin == nil {
			hc.Stdin = strings.NewReader("")
		}
		code := handler(ctx, Invocation{Name: args[0], Args: slices.Clone(args[1:]), Dir: hc.Dir, Env: exported(hc.Env), Stdin: hc.Stdin, Stdout: hc.Stdout, Stderr: hc.Stderr})
		if code == 0 {
			return nil
		}
		return interp.NewExitStatus(uint8(code))
	}))
	if err != nil {
		if s.input != nil {
			_ = s.input.Close()
		}
		return nil, err
	}
	s.runner = r
	r.Reset()
	// Populate the interpreter’s public Vars snapshot, including bootstrap
	// variables, without executing a command.
	_ = r.Run(context.Background(), &syntax.File{})
	return s, nil
}

func exported(env expand.Environ) []string {
	vars := map[string]expand.Variable{}
	env.Each(func(n string, v expand.Variable) bool { vars[n] = v; return true })
	result := make([]string, 0, len(vars))
	for n, v := range vars {
		if v.IsSet() && v.Exported && v.Kind == expand.String {
			result = append(result, n+"="+v.Str)
		}
	}
	sort.Strings(result)
	return result
}

// Run parses all source before executing it. Runs on the same session serialize.
func (s *Session) Run(ctx context.Context, src string) Result {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Result{Err: ErrExited}
	}
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()
	defer func() { cancel(); s.mu.Lock(); s.cancel = nil; s.mu.Unlock() }()
	if err := ctx.Err(); err != nil {
		return s.canceled(err)
	}
	file, err := s.parser.Parse(strings.NewReader(src), "")
	if err != nil {
		se := syntaxError(err)
		return Result{Status: 2, ParseError: se, Err: se}
	}
	if err := ctx.Err(); err != nil {
		return s.canceled(err)
	}
	err = s.runner.Run(ctx, file)
	if ctx.Err() != nil {
		return s.canceled(ctx.Err())
	}
	result := Result{Exited: s.runner.Exited()}
	var status interp.ExitStatus
	if errors.As(err, &status) {
		result.Status = int(status)
	} else if err != nil {
		result.Status = 1
		result.Err = err
	}
	if result.Exited {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
	}
	return result
}
func (s *Session) canceled(err error) Result {
	status := 130
	if errors.Is(err, context.DeadlineExceeded) {
		status = 124
	}
	s.runner.SetLastStatus(uint8(status))
	return Result{Status: status, Err: err}
}
func syntaxError(err error) *SyntaxErr {
	var p syntax.ParseError
	if errors.As(err, &p) {
		return &SyntaxErr{p.Pos.Offset(), p.Pos.Line(), p.Pos.Col(), p.Text}
	}
	var l syntax.LangError
	if errors.As(err, &l) {
		return &SyntaxErr{l.Pos.Offset(), l.Pos.Line(), l.Pos.Col(), l.Error()}
	}
	return &SyntaxErr{Message: err.Error()}
}

// Close seals the session and cancels an active run; it never closes caller streams.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	if s.input != nil {
		_ = s.input.Close()
		s.input = nil
	}
	return nil
}

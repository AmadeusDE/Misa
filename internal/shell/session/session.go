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
	// handler executes commands the interpreter does not own; introspection
	// builtins dispatch through it so execution and reporting cannot disagree.
	handler     ExecHandler
	misaBuiltin func(string) bool
	lookPath    func(name, dir string, env []string) (string, bool)
	defaultPath string
	// hash is the `hash` table; hashPath is the PATH value it was built from.
	hash     map[string]string
	hashPath string

	onCwdChange func(oldDir, newDir string)
	notifyMu    sync.Mutex
	cwdEvents   []cwdEvent // pending changes of the running submission
}

// cwdEvent is one successful directory change reported by the interpreter.
type cwdEvent struct{ oldDir, newDir string }

// New constructs a session without changing the process environment or cwd.
func New(o Options) (*Session, error) {
	env := o.Env
	if env == nil {
		env = os.Environ()
	} else {
		env = slices.Clone(env)
	}
	s := &Session{
		parser:      syntax.NewParser(syntax.Variant(syntax.LangPOSIX)),
		misaBuiltin: o.MisaBuiltin,
		lookPath:    o.LookPath,
		defaultPath: o.DefaultPath,
		onCwdChange: o.OnCwdChange,
	}
	if s.misaBuiltin == nil {
		s.misaBuiltin = func(string) bool { return false }
	}
	if s.lookPath == nil {
		s.lookPath = defaultLookPath
	}
	if s.defaultPath == "" {
		s.defaultPath = DefaultCommandPath
	}
	arg0 := func(r *interp.Runner) error {
		if o.Arg0 != nil {
			return interp.Arg0(*o.Arg0)(r)
		}
		return nil
	}
	params := append(slices.Clone(o.ShellOptions), "--")
	params = append(params, o.Args...)
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
	s.handler = handler
	r, err := interp.New(interp.Dir(o.Dir), interp.Env(expand.ListEnviron(env...)), arg0, interp.Params(params...), stdio, interp.CallHandler(s.rewriteCall), interp.ExecHandler(s.execHandler))
	if err != nil {
		if s.input != nil {
			_ = s.input.Close()
		}
		return nil, err
	}
	s.runner = r
	r.SetOnChangeDir(func(oldDir, newDir string) {
		// Runs while Run holds runMu, so the slice needs no extra lock.
		s.cwdEvents = append(s.cwdEvents, cwdEvent{oldDir, newDir})
	})
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
//
// Options' OnCwdChange is called for every successful directory change a
// submission made, in order, with the old and new logical directories. The
// events are delivered after the submission finishes and outside the run lock,
// so the callback may call Dir; it must not call Run. Changes made inside a
// subshell or process substitution are not reported, and neither is a change
// that lands on the directory the shell is already in.
func (s *Session) Run(ctx context.Context, src string) Result {
	return s.RunFile(ctx, "", src)
}

// RunFile parses source under name for diagnostics, then executes it with the
// session's independent Arg0. Runs on the same session serialize.
func (s *Session) RunFile(ctx context.Context, name, src string) Result {
	s.runMu.Lock()
	s.cwdEvents = nil
	result := s.runLocked(ctx, name, src)
	events := s.cwdEvents
	s.cwdEvents = nil
	s.runMu.Unlock()
	if s.onCwdChange != nil && len(events) > 0 {
		s.notifyMu.Lock()
		for _, e := range events {
			s.onCwdChange(e.oldDir, e.newDir)
		}
		s.notifyMu.Unlock()
	}
	return result
}

func (s *Session) runLocked(ctx context.Context, name, src string) Result {
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
	file, err := s.parser.Parse(strings.NewReader(src), name)
	if err != nil {
		se := syntaxError(err)
		se.Name = name
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
		return &SyntaxErr{Offset: p.Pos.Offset(), Line: p.Pos.Line(), Column: p.Pos.Col(), Message: p.Text}
	}
	var l syntax.LangError
	if errors.As(err, &l) {
		message := l.Error()
		if l.Filename != "" {
			message = strings.TrimPrefix(message, l.Filename+":")
		}
		message = strings.TrimPrefix(message, l.Pos.String()+": ")
		return &SyntaxErr{Offset: l.Pos.Offset(), Line: l.Pos.Line(), Column: l.Pos.Col(), Message: message}
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

// execHandler dispatches commands the interpreter does not own. The names
// rewritten by rewriteCall are implemented here, so introspection sees the
// Misa registry and this session's resolver instead of the interpreter's own
// builtins. The plain names are accepted too, because `exec type` and
// `exec command` reach the exec handler without passing the call handler.
func (s *Session) execHandler(ctx context.Context, args []string) error {
	hc := interp.HandlerCtx(ctx)
	if hc.Stdin == nil {
		hc.Stdin = strings.NewReader("")
	}
	switch args[0] {
	case internalType, "type":
		return s.runType(ctx, hc, args[1:])
	case internalHash, "hash":
		return s.runHash(ctx, hc, args[1:])
	case internalCommand, "command":
		return s.runCommand(ctx, hc, args[1:])
	}
	return s.runExternal(ctx, hc, args[0], args[1:])
}

// runExternal delegates to the injected handler, recording a PATH resolution in
// the hash table the way a shell remembers lookups.
func (s *Session) runExternal(ctx context.Context, hc interp.HandlerContext, name string, args []string) error {
	s.remember(name, hc.Dir, hc.Env)
	return exitErr(s.handler(ctx, Invocation{
		Name: name, Args: slices.Clone(args), Dir: hc.Dir, Env: exported(hc.Env),
		Stdin: hc.Stdin, Stdout: hc.Stdout, Stderr: hc.Stderr,
	}))
}

// defaultLookPath uses the interpreter's PATH search when the caller injects no
// resolver.
func defaultLookPath(name, dir string, env []string) (string, bool) {
	path, err := interp.LookPathDir(dir, expand.ListEnviron(env...), name)
	if err != nil {
		return "", false
	}
	return path, true
}

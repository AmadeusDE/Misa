package session

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/AmadeusDE/Misa/internal/sh/expand"
	"github.com/AmadeusDE/Misa/internal/sh/interp"
	"github.com/AmadeusDE/Misa/internal/sh/syntax"
)

// DefaultCommandPath is the fixed path `command -p` searches. It is the POSIX
// confstr(_CS_PATH) value rather than the session's own PATH, so scripts can
// select the standard utilities even when Misa overrides PATH.
const DefaultCommandPath = "/bin:/usr/bin"

// The interpreter owns `type`, `hash` and `command`, and its builtins cannot
// see Misa utilities or this session's resolver. The call handler rewrites
// those three names to the private names below, which the exec handler
// implements. The names are reserved: a shell function of that exact name is
// never rewritten, so functions still take precedence over the builtins.
const (
	internalType    = "__misa_type"
	internalHash    = "__misa_hash"
	internalCommand = "__misa_command"
)

var introspectNames = map[string]string{
	"type":    internalType,
	"hash":    internalHash,
	"command": internalCommand,
}

// rewriteCall sends the interpreter-owned introspection builtins to this
// session's implementations. It runs for every simple command, after expansion
// and before function/builtin dispatch, so a function of the same name still
// wins.
func (s *Session) rewriteCall(_ context.Context, args []string) ([]string, error) {
	if len(args) == 0 || introspectNames[args[0]] == "" || s.function(args[0]) {
		return args, nil
	}
	rewritten := slices.Clone(args)
	rewritten[0] = introspectNames[args[0]]
	return rewritten, nil
}

func (s *Session) function(name string) bool {
	if s.runner == nil {
		return false
	}
	_, ok := s.runner.Funcs[name]
	return ok
}

// commandClass is a name's resolution result, in shell precedence order.
type commandClass int

const (
	classNotFound commandClass = iota
	classKeyword
	classFunction
	classShellBuiltin
	classMisaBuiltin
	classPath
)

func classToken(c commandClass) string {
	switch c {
	case classKeyword:
		return "keyword"
	case classFunction:
		return "function"
	case classShellBuiltin:
		return "builtin"
	case classMisaBuiltin:
		return "misa"
	}
	return "file"
}

func classDescription(c commandClass, name, path string) string {
	switch c {
	case classKeyword:
		return name + " is a shell keyword"
	case classFunction:
		return name + " is a function"
	case classShellBuiltin:
		return name + " is a shell builtin"
	case classMisaBuiltin:
		return name + " is a misa builtin"
	}
	return name + " is " + path
}

// classify resolves name in the order the session executes it: keywords,
// functions, interpreter builtins, Misa registry utilities, then PATH.
func (s *Session) classify(hc interp.HandlerContext, name string) (commandClass, string) {
	switch {
	case syntax.IsKeyword(name):
		return classKeyword, ""
	case s.function(name):
		return classFunction, ""
	case interp.IsBuiltin(name):
		return classShellBuiltin, ""
	case !strings.ContainsAny(name, `/\`) && s.misaBuiltin(name):
		return classMisaBuiltin, ""
	}
	if path, ok := s.lookPath(name, hc.Dir, exported(hc.Env)); ok {
		return classPath, path
	}
	return classNotFound, ""
}

func exitErr(code int) error {
	if code == 0 {
		return nil
	}
	return interp.NewExitStatus(uint8(code))
}

// flagWords splits leading option words from operands. It stops at the first
// operand or after "--".
func flagWords(args []string) (flags, operands []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
		if len(arg) < 2 || arg[0] != '-' {
			return args[:i], args[i:]
		}
	}
	return args, nil
}

// runType implements `type`, including the -p/-P and -t forms the interpreter
// previously provided. -a and -f are unsupported and fail rather than
// pretending to list locations.
func (s *Session) runType(_ context.Context, hc interp.HandlerContext, args []string) error {
	flags, names := flagWords(args)
	mode := ""
	for _, flag := range flags {
		switch flag {
		case "-p", "-P":
			mode = "path"
		case "-t":
			mode = "token"
		case "-a", "-f":
			fmt.Fprintf(hc.Stderr, "type: %s: unsupported option\n", flag)
			return exitErr(2)
		default:
			fmt.Fprintf(hc.Stderr, "type: invalid option %q\n", flag)
			return exitErr(2)
		}
	}
	code := 0
	for _, name := range names {
		if mode == "path" {
			path, ok := s.lookPath(name, hc.Dir, exported(hc.Env))
			if !ok {
				code = 1
				continue
			}
			fmt.Fprintln(hc.Stdout, path)
			continue
		}
		class, path := s.classify(hc, name)
		if class == classNotFound {
			fmt.Fprintf(hc.Stderr, "type: %s: not found\n", name)
			code = 1
			continue
		}
		if mode == "token" {
			fmt.Fprintln(hc.Stdout, classToken(class))
			continue
		}
		fmt.Fprintln(hc.Stdout, classDescription(class, name, path))
	}
	return exitErr(code)
}

// runCommand implements `command`. Without -v/-V it executes the name while
// bypassing functions; -p additionally forces the external utility from
// DefaultCommandPath, which is the escape from a shadowing Misa utility.
func (s *Session) runCommand(ctx context.Context, hc interp.HandlerContext, args []string) error {
	flags, operands := flagWords(args)
	show, verbose, defaultPath := false, false, false
	for _, flag := range flags {
		switch flag {
		case "-v":
			show = true
		case "-V":
			verbose = true
		case "-p":
			defaultPath = true
		default:
			fmt.Fprintf(hc.Stderr, "command: invalid option %q\n", flag)
			return exitErr(2)
		}
	}
	if len(operands) == 0 {
		return nil
	}
	if show || verbose {
		code := 0
		for _, name := range operands {
			class, path := s.classify(hc, name)
			if class == classNotFound {
				code = 1
				continue
			}
			switch {
			case verbose:
				fmt.Fprintln(hc.Stdout, classDescription(class, name, path))
			case class == classPath:
				fmt.Fprintln(hc.Stdout, path)
			default:
				fmt.Fprintln(hc.Stdout, name)
			}
		}
		return exitErr(code)
	}
	name, rest := operands[0], operands[1:]
	if defaultPath && !strings.ContainsAny(name, `/\`) {
		path, ok := s.lookPath(name, hc.Dir, withPath(exported(hc.Env), s.defaultPath))
		if !ok {
			fmt.Fprintf(hc.Stderr, "command: %s: not found\n", name)
			return exitErr(127)
		}
		return s.runAt(ctx, hc, name, path, rest)
	}
	return s.runDirect(ctx, hc, name, rest)
}

// runDirect executes a name for `command`, bypassing functions but not
// builtins; introspection builtins stay ours so `command type` agrees with
// `type`.
func (s *Session) runDirect(ctx context.Context, hc interp.HandlerContext, name string, args []string) error {
	switch name {
	case "type":
		return s.runType(ctx, hc, args)
	case "hash":
		return s.runHash(ctx, hc, args)
	case "command":
		return s.runCommand(ctx, hc, args)
	}
	if interp.IsBuiltin(name) {
		return hc.Builtin(ctx, append([]string{name}, args...))
	}
	return s.runExternal(ctx, hc, name, args)
}

// runAt executes an already resolved executable path under the name the caller
// typed.
func (s *Session) runAt(ctx context.Context, hc interp.HandlerContext, name, path string, args []string) error {
	return exitErr(s.handler(ctx, Invocation{
		Name: path, Argv0: name, Args: slices.Clone(args), Dir: hc.Dir,
		Env: exported(hc.Env), Stdin: hc.Stdin, Stdout: hc.Stdout, Stderr: hc.Stderr,
	}))
}

// runHash implements `hash`: a table of PATH resolutions. `hash -r` clears it,
// a PATH change clears it, and execution still re-resolves through the
// injected handler, so a stale entry can never run the wrong file.
func (s *Session) runHash(_ context.Context, hc interp.HandlerContext, args []string) error {
	flags, names := flagWords(args)
	reset := false
	for _, flag := range flags {
		if flag != "-r" {
			fmt.Fprintf(hc.Stderr, "hash: invalid option %q\n", flag)
			return exitErr(2)
		}
		reset = true
	}
	env := exported(hc.Env)
	pathName := pathValue(env)
	table := s.hashTable(pathName)
	if reset {
		table = s.clearHash(pathName)
	}
	if len(names) == 0 {
		sorted := make([]string, 0, len(table))
		for name := range table {
			sorted = append(sorted, name)
		}
		sort.Strings(sorted)
		for _, name := range sorted {
			fmt.Fprintf(hc.Stdout, "%s=%s\n", name, table[name])
		}
		return nil
	}
	code := 0
	for _, name := range names {
		if strings.ContainsAny(name, `/\`) {
			// The table holds bare command names, not paths.
			fmt.Fprintf(hc.Stderr, "hash: %s: not a command name\n", name)
			code = 1
			continue
		}
		if interp.IsBuiltin(name) || s.function(name) || s.misaBuiltin(name) {
			// Only PATH executables have a location to remember.
			continue
		}
		path, ok := s.lookPath(name, hc.Dir, env)
		if !ok {
			fmt.Fprintf(hc.Stderr, "hash: %s: not found\n", name)
			code = 1
			continue
		}
		table[name] = path
	}
	return exitErr(code)
}

// remember records the PATH location of a command the interpreter did not own.
// Misa utilities and shell builtins have no path and are not recorded.
func (s *Session) remember(name, dir string, env expand.Environ) {
	if strings.ContainsAny(name, `/\`) || interp.IsBuiltin(name) || s.function(name) || s.misaBuiltin(name) {
		return
	}
	exportedEnv := exported(env)
	table := s.hashTable(pathValue(exportedEnv))
	if _, ok := table[name]; ok {
		return
	}
	if path, ok := s.lookPath(name, dir, exportedEnv); ok {
		table[name] = path
	}
}

// hashTable returns the live table, clearing it when PATH changed since it was
// built.
func (s *Session) hashTable(path string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hash == nil || s.hashPath != path {
		s.hash = map[string]string{}
		s.hashPath = path
	}
	return s.hash
}

func (s *Session) clearHash(path string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hash = map[string]string{}
	s.hashPath = path
	return s.hash
}

// pathValue reads PATH from an exported environment list.
func pathValue(env []string) string {
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			return value
		}
	}
	return ""
}

// withPath returns env with PATH replaced by path.
func withPath(env []string, path string) []string {
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, "PATH=") {
			continue
		}
		out = append(out, entry)
	}
	return append(out, "PATH="+path)
}

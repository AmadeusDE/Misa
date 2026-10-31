// Package shcli provides the noninteractive shell command-line entry point.
package shcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/shell/session"
	"github.com/AmadeusDE/Misa/internal/shell/sessionbuiltins"
)

// Options supplies invocation-local state and streams. Nil Env inherits the
// process environment; a non-nil empty Env does not. Streams remain caller-owned.
type Options struct {
	Dir            string
	Env            []string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	StdinTTY       bool
}

// readSource reads standard input while observing cancellation. A caller-owned
// reader cannot be interrupted, so a blocked read is abandoned, not closed.
func readSource(ctx context.Context, in io.Reader) (string, error) {
	if in == nil {
		return "", nil
	}
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := io.ReadAll(in)
		done <- result{data, err}
	}()
	select {
	case r := <-done:
		return string(r.data), r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func canceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func readStatus(err error) int {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return 124
	case errors.Is(err, context.Canceled):
		return 130
	}
	return 1
}

// Run executes a command string, script file, or noninteractive standard input.
// It never loads startup files or invokes a host shell as a fallback.
func Run(ctx context.Context, name string, args []string, opts Options) int {
	if name == "" {
		name = "sh"
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	usage := func(message string) int { fmt.Fprintf(stderr, "%s: %s\n", name, message); return 1 }
	var flags []string
	command, fromStdin := false, false
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if len(arg) < 2 || arg[0] != '-' || arg == "-" {
			break
		}
		if strings.HasPrefix(arg, "--") {
			return usage("unsupported option " + arg)
		}
		for _, flag := range arg[1:] {
			switch flag {
			case 'e', 'u', 'x':
				flags = append(flags, "-"+string(flag))
			case 'c':
				command = true
			case 's':
				fromStdin = true
			default:
				return usage(fmt.Sprintf("unsupported option -%c", flag))
			}
		}
		i++
	}
	if command && fromStdin {
		return usage("-c and -s cannot be combined")
	}
	var source, label, arg0 string
	arg0, label = name, name
	positional := args[i:]
	switch {
	case command:
		if len(positional) == 0 {
			return usage("-c requires a command string")
		}
		source = positional[0]
		positional = positional[1:]
		if len(positional) > 0 {
			arg0 = positional[0]
			positional = positional[1:]
		}
	case !fromStdin && len(positional) > 0 && positional[0] != "-":
		label = positional[0]
		arg0 = label
		positional = positional[1:]
		path := label
		if !filepath.IsAbs(path) && opts.Dir != "" {
			path = filepath.Join(opts.Dir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s: %v\n", name, label, err)
			if errors.Is(err, os.ErrNotExist) {
				return 127
			}
			return 126
		}
		source = string(data)
	default:
		if !fromStdin && len(positional) > 0 {
			positional = positional[1:]
		}
		if opts.StdinTTY {
			return usage("interactive shell is not supported")
		}
		data, err := readSource(ctx, opts.Stdin)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return readStatus(err)
		}
		source = data
	}
	s, err := session.New(session.Options{Dir: opts.Dir, Env: opts.Env, Stdin: opts.Stdin, Stdout: opts.Stdout, Stderr: opts.Stderr, Exec: sessionbuiltins.Run, Arg0: &arg0, Args: positional, ShellOptions: flags})
	if err != nil {
		return usage(err.Error())
	}
	defer s.Close()
	result := s.RunFile(ctx, label, source)
	if result.ParseError != nil {
		fmt.Fprintf(stderr, "%s:%d:%d: %s\n", label, result.ParseError.Line, result.ParseError.Column, result.ParseError.Message)
	} else if result.Err != nil && !canceled(result.Err) {
		fmt.Fprintf(stderr, "%s: %v\n", name, result.Err)
	}
	return result.Status
}

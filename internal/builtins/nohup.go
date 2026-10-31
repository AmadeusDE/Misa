//go:build unix

// Behaviour adapted from toybox toys/posix/nohup.c (0BSD, commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb) and uutils coreutils src/uu/nohup
// (MIT, commit fee61ff5d4f87e0f912a52e639942b5f7ad58663); see docs/CREDITS.md.
package builtins

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
	"golang.org/x/term"
)

const (
	nohupOutput   = "nohup.out"
	nohupFailFile = 125 // uutils: EXIT_CANCELED when nohup.out cannot be opened
)

func init() {
	Register(Command{Name: "nohup", Run: runNohup})
}

func runNohup(ctx *Context) int {
	argv, status := nohupOperands(ctx)
	if argv == nil {
		return status
	}
	stdin, stdout, stderr := ctx.Stdin, ctx.Stdout, ctx.Stderr
	var opened []*os.File
	defer func() {
		for _, f := range opened {
			_ = f.Close()
		}
	}()
	if isTerminal(stdin) {
		// toybox opens /dev/null read-only, so a program that reads it sees EOF
		// instead of an error. uutils opens it write-only; toybox is the reference.
		devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "nohup: cannot replace stdin: %s\n", fmtOsErr(err))
			return nohupFailFile
		}
		opened = append(opened, devNull)
		stdin = devNull
	}
	if isTerminalWriter(stdout) {
		name, file, err := openNohupOutput(ctx)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "nohup: %v\n", err)
			return nohupFailFile
		}
		opened = append(opened, file)
		stdout = file
		fmt.Fprintf(ctx.Stderr, "nohup: ignoring input and appending output to '%s'\n", name)
	}
	if isTerminalWriter(stderr) {
		// The hint above went to the original stderr, so a caller still sees it.
		stderr = stdout
	}
	if !proceffect.Available() {
		fmt.Fprintln(ctx.Stderr, "nohup: process isolation unavailable in this host")
		return ExitInternal
	}
	env := ctx.Environ()
	if _, builtin := Lookup(argv[0]); !builtin || strings.ContainsAny(argv[0], `/\`) {
		// Report an unresolvable command the way the launcher does, before the
		// child starts and its diagnostics lose the nohup prefix.
		if _, err := commandexec.Resolve(argv[0], commandexec.Invocation{Dir: ctx.Dir, Env: env}); err != nil {
			fmt.Fprintf(ctx.Stderr, "nohup: failed to run command '%s': %s\n", argv[0], fmtOsErr(err))
			return commandexec.ErrorStatus(err)
		}
	}
	req := proceffect.Request{DispatchOnly: true, IgnoreHup: true, Argv: argv}
	return proceffect.Run(ctx.CancellationContext(), req, proceffect.Invocation{
		Dir: ctx.Dir, Env: env, Stdin: stdin, Stdout: stdout, Stderr: stderr,
	})
}

// nohupOperands splits the command from the leading options. It returns nil and
// a status when the invocation is complete without a child.
func nohupOperands(ctx *Context) ([]string, int) {
	for i, a := range ctx.Args {
		switch {
		case a == "--":
			if i+1 < len(ctx.Args) {
				return ctx.Args[i+1:], 0
			}
			return nil, nohupUsageError(ctx, "missing operand")
		case a == "--help":
			fmt.Fprint(ctx.Stdout, "Usage: nohup COMMAND [ARG]...\n\nRun COMMAND, ignoring hangup signals.\nStandard output is appended to 'nohup.out' when it is a terminal.\n")
			return nil, 0
		case a == "--version":
			fmt.Fprintln(ctx.Stdout, "nohup (Misa) 0.1.0")
			return nil, 0
		case len(a) > 1 && a[0] == '-':
			// toybox's nohup takes no options: TOYFLAG_ARGFAIL(125).
			return nil, nohupUsageError(ctx, fmt.Sprintf("unrecognized option '%s'", a))
		default:
			return ctx.Args[i:], 0
		}
	}
	return nil, nohupUsageError(ctx, "missing operand")
}

// isTerminalWriter reports whether w is a terminal, so nohup only replaces the
// streams it has to.
func isTerminalWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

func nohupUsageError(ctx *Context, message string) int {
	fmt.Fprintf(ctx.Stderr, "nohup: %s\nTry 'nohup --help' for more information.\n", message)
	return nohupFailFile
}

// openNohupOutput appends to nohup.out in the invocation's directory, falling
// back to $HOME/nohup.out. A new file is created 0600 like toybox and uutils so
// other users cannot read a detached job's log.
func openNohupOutput(ctx *Context) (string, *os.File, error) {
	flags := os.O_CREATE | os.O_APPEND | os.O_WRONLY
	file, err := os.OpenFile(filepath.Join(ctx.Dir, nohupOutput), flags, 0600)
	if err == nil {
		return nohupOutput, file, nil
	}
	home, ok := ctx.LookupEnv("HOME")
	if !ok || home == "" {
		return nohupOutput, nil, fmt.Errorf("cannot open '%s': %s", nohupOutput, fmtOsErr(err))
	}
	homeOut := filepath.Join(home, nohupOutput)
	file, homeErr := os.OpenFile(homeOut, flags, 0600)
	if homeErr != nil {
		return homeOut, nil, fmt.Errorf("cannot open '%s': %s; cannot open '%s': %s",
			nohupOutput, fmtOsErr(err), homeOut, fmtOsErr(homeErr))
	}
	return homeOut, file, nil
}

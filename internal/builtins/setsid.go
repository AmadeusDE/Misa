//go:build unix

// Behaviour adapted from toybox toys/other/setsid.c (0BSD, commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb); see docs/CREDITS.md.
package builtins

import (
	"fmt"
	"time"

	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

const setsidSpec = "^cwd"

var setsidSpecParsed *Spec

func init() {
	var err error
	setsidSpecParsed, err = ParseSpec(setsidSpec)
	if err != nil {
		panic("builtins: bad setsid spec: " + err.Error())
	}
	Register(Command{Name: "setsid", Run: runSetsid})
}

func runSetsid(ctx *Context) int {
	flags, err := setsidSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "setsid: %v\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, "Usage: setsid [-cdw] COMMAND [ARG]...\n\nRun COMMAND in a new session, detached from this terminal.\n"+
			"  -c  make stdin the new session's controlling terminal\n"+
			"  -d  detach from the terminal (a new session already has no controlling terminal)\n"+
			"  -w  wait for the command and exit with its status (always on in Misa)\n")
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintln(ctx.Stdout, "setsid (Misa) 0.1.0")
		return 0
	}
	if flags.Bool("c") && flags.Bool("d") {
		fmt.Fprintln(ctx.Stderr, "setsid: -c and -d are mutually exclusive")
		return 1
	}
	argv := flags.Operands
	if len(argv) == 0 {
		fmt.Fprintln(ctx.Stderr, "setsid: missing operand\nTry 'setsid --help' for more information.")
		return 1
	}
	// Misa always waits for the command and forwards its status: it has no job
	// supervision yet, so a detached-and-forgotten child would be unreaped.
	opts := commandexec.Options{
		Setsid: true,
		// A stolen controlling terminal needs a terminal on stdin; without one
		// the kernel call would fail the launch, while toybox ignores the error.
		Setctty:   flags.Bool("c") && isTerminal(ctx.Stdin),
		WaitDelay: 100 * time.Millisecond,
	}
	inv := proceffect.Invocation{Dir: ctx.Dir, Env: ctx.Environ(), Stdin: ctx.Stdin, Stdout: ctx.Stdout, Stderr: ctx.Stderr}
	proc, err := launchCommand(ctx, argv, proceffect.Request{}, inv, opts)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "setsid: failed to run command '%s': %s\n", argv[0], fmtOsErr(err))
		return launchStatus(ctx, err)
	}
	return proc.Wait()
}

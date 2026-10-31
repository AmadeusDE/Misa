//go:build unix

// Behaviour adapted from toybox toys/posix/renice.c (0BSD, commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb); see docs/CREDITS.md.
package builtins

import (
	"fmt"
	"os/user"
	"strconv"
	"syscall"

	"github.com/AmadeusDE/Misa/internal/proceffect"
)

const reniceSpec = "g(group)p(pid)n(adjustment):u(user)"

var reniceSpecParsed *Spec

func init() {
	var err error
	reniceSpecParsed, err = ParseSpec(reniceSpec)
	if err != nil {
		panic("builtins: bad renice spec: " + err.Error())
	}
	Register(Command{Name: "renice", Run: runRenice})
}

func runRenice(ctx *Context) int {
	flags, err := reniceSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "renice: %v\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, "Usage: renice [-gpu] -n INCREMENT ID...\n\nChange the nice value of running processes.\n"+
			"  -g, --group   interpret IDs as process group ids\n"+
			"  -p, --pid     interpret IDs as process ids (default)\n"+
			"  -u, --user    interpret IDs as user names or ids\n"+
			"  -n, --adjustment=INCREMENT  add INCREMENT to each target's nice value\n")
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintln(ctx.Stdout, "renice (Misa) 0.1.0")
		return 0
	}
	value, ok := flags.ExplicitString("n")
	if !ok {
		fmt.Fprintln(ctx.Stderr, "renice: missing priority increment (-n)\nTry 'renice --help' for more information.")
		return 1
	}
	increment, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "renice: invalid increment '%s'\n", value)
		return 1
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "renice: missing operand\nTry 'renice --help' for more information.")
		return 1
	}
	// toybox gives -g precedence over -u, and -u over the default -p.
	which := syscall.PRIO_PROCESS
	switch {
	case flags.Bool("g"):
		which = syscall.PRIO_PGRP
	case flags.Bool("u"):
		which = syscall.PRIO_USER
	}
	status := 0
	var ids []int
	for _, operand := range flags.Operands {
		id, ok := reniceTarget(operand, which)
		if !ok {
			fmt.Fprintf(ctx.Stderr, "renice: bad '%s'\n", operand)
			status = 1
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return status
	}
	if !proceffect.Available() {
		fmt.Fprintln(ctx.Stderr, "renice: process isolation unavailable in this host")
		return ExitInternal
	}
	// The change happens in an isolated child: renice's own process (the Misa
	// host) must never inherit a priority change meant for the command line.
	req := proceffect.ReniceRequest{Priority: int(increment), Which: which, IDs: ids}
	if proceffect.Renice(ctx.CancellationContext(), req, proceffect.Invocation{
		Dir: ctx.Dir, Env: ctx.Environ(), Stdin: ctx.Stdin, Stdout: ctx.Stdout, Stderr: ctx.Stderr,
	}) != 0 {
		status = 1
	}
	return status
}

// reniceTarget resolves one operand for which. toybox accepts user names only
// for -u; Misa also accepts a numeric user id.
func reniceTarget(operand string, which int) (int, bool) {
	if which == syscall.PRIO_USER {
		if id, err := strconv.Atoi(operand); err == nil && id >= 0 {
			return id, true
		}
		entry, err := user.Lookup(operand)
		if err != nil {
			return 0, false
		}
		id, err := strconv.Atoi(entry.Uid)
		if err != nil || id < 0 {
			return 0, false
		}
		return id, true
	}
	id, err := strconv.ParseInt(operand, 10, 32)
	if err != nil || id < 0 {
		return 0, false
	}
	return int(id), true
}

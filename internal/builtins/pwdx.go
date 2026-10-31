// pwdx — report the current working directory of a process.
//
// Behaviour follows toybox toys/other/pwdx.c (0BSD, commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb): read the /proc/PID/cwd symlink
// and print "PID: path", or "PID: strerror" when the link cannot be read,
// continuing with the remaining operands. uutils/procps pwdx (MIT, commit
// e697667b3f758803e38d8ab4c8f912171beca602) was consulted for the
// invalid-process-id wording only.
//
// Deliberate divergences from those references: per-pid error lines go to
// stdout in argument order, exactly like Toybox (uutils/procps writes them to
// stderr); Toybox accepts and ignores -a, which is rejected here as an unknown
// option because the flag does nothing; a non-numeric operand is reported as
// "Invalid process id" instead of being resolved through atoi as pid 0. This
// is a native Go implementation; see docs/CREDITS.md.

package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// pwdx takes no options: only the parser's implicit -h/--help and --version.
const pwdxSpec = ""

var pwdxSpecParsed *Spec

// pwdxProcRoot is the procfs root; "" means /proc. Tests point it at a fixture.
var pwdxProcRoot = ""

func init() {
	var err error
	pwdxSpecParsed, err = ParseSpec(pwdxSpec)
	if err != nil {
		panic("builtins: bad pwdx spec: " + err.Error())
	}
	Register(Command{Name: "pwdx", Run: runPwdx})
}

func pwdxUsage() string {
	return "usage: pwdx PID...\n\n" +
		"Print working directory of processes listed on command line.\n" +
		"pwdx takes no options; -h/--help and --version are accepted.\n"
}

func runPwdx(ctx *Context) int {
	if err := ctx.CancellationContext().Err(); err != nil {
		return ExitStatus(err)
	}

	flags, err := pwdxSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "pwdx: %v\nTry 'pwdx --help' for more information.\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, pwdxUsage())
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprint(ctx.Stdout, "pwdx (Misa) 0.1.0\n")
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "pwdx: missing operand")
		fmt.Fprintln(ctx.Stderr, "Try 'pwdx --help' for more information.")
		return 1
	}

	root := pwdxProcRoot
	if root == "" {
		root = "/proc"
	}
	if _, err := os.Stat(root); err != nil {
		fmt.Fprintf(ctx.Stderr, "pwdx: %s: %s (process directories are not available on this platform)\n",
			root, fmtOsErr(err))
		return 1
	}

	status := 0
	for _, arg := range flags.Operands {
		if err := ctx.CancellationContext().Err(); err != nil {
			return ExitStatus(err)
		}
		pid, ok := pwdxParsePID(arg)
		if !ok {
			fmt.Fprintf(ctx.Stdout, "%s: Invalid process id\n", arg)
			status = 1
			continue
		}
		path, err := os.Readlink(filepath.Join(root, strconv.Itoa(pid), "cwd"))
		if err != nil {
			fmt.Fprintf(ctx.Stdout, "%s: %s\n", arg, fmtOsErr(err))
			status = 1
			continue
		}
		fmt.Fprintf(ctx.Stdout, "%s: %s\n", arg, path)
	}
	return status
}

func pwdxParsePID(arg string) (int, bool) {
	if arg == "" {
		return 0, false
	}
	for i := 0; i < len(arg); i++ {
		if arg[i] < '0' || arg[i] > '9' {
			return 0, false
		}
	}
	pid, err := strconv.Atoi(arg)
	if err != nil {
		return 0, false
	}
	return pid, true
}

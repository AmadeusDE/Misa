// killall — send a signal to every process with the given name.
//
// The option set, the -SIGNAL operand, the "no name" and "No such process"
// wording, the -i prompt and the -w polling follow toybox toys/lsb/killall.c,
// with the name matching of lib/lib.c's names_to_pid(names, cb, 1) — that is,
// pidof -x: comm for names that fit it, the executable's inode for path
// operands, and the basenames of argv[0]/argv[1] otherwise (0BSD, commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb). The per-name error reporting and
// the signal-name parsing follow uutils/procps pgrep/pkill and its
// process_matcher.rs (MIT, commit e697667b3f758803e38d8ab4c8f912171beca602).
// This is a native Go implementation; see docs/CREDITS.md.

package builtins

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

const killallSpec = "s(signal):l(list)q(quiet)v(verbose)w(wait)i(interactive)"

var killallSpecParsed *Spec

// Seams for tests: the procfs root, the -w poll interval and the process
// killall must never signal.
var (
	killallProcRoot     = ""
	killallPollInterval = 100 * time.Millisecond
	killallGetpid       = os.Getpid
)

func init() {
	var err error
	killallSpecParsed, err = ParseSpec(killallSpec)
	if err != nil {
		panic("builtins: bad killall spec: " + err.Error())
	}
	Register(Command{Name: "killall", Run: runKillall})
}

func runKillall(ctx *Context) int {
	sigArg, args := killallSplitSignal(ctx.Args)

	flags, err := killallSpecParsed.ParseArgs(args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "killall: %v\nTry 'killall --help' for more information.\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, killallUsage())
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "killall (Misa) 0.1.0\n")
		return 0
	}
	if flags.Bool("l") {
		fmt.Fprintln(ctx.Stdout, killallSignalList())
		return 0
	}

	quiet := flags.Bool("q")
	interactive := flags.Bool("i")
	verbose := flags.Bool("v")
	wait := flags.Bool("w")

	sigName := "TERM"
	if sigArg != "" {
		sigName = sigArg
	} else if s, ok := flags.String("signal"); ok {
		sigName = s
	}
	sig, ok := parseSignalArg(sigName)
	if !ok {
		if quiet {
			return 1
		}
		fmt.Fprintf(ctx.Stderr, "killall: invalid signal %q\n", sigName)
		return 1
	}

	names := flags.Operands
	if len(names) == 0 {
		fmt.Fprintf(ctx.Stderr, "killall: no name\nTry 'killall --help' for more information.\n")
		return 1
	}

	waitCtx := ctx.CancellationContext()
	procs, snapErrs := proc.New(killallProcRoot).Snapshot(waitCtx, proc.Options{Argv: true, Exe: true})
	if len(snapErrs) > 0 {
		if !quiet {
			fmt.Fprintf(ctx.Stderr, "killall: %v\n", snapErrs[0])
		}
		return 1
	}

	self := killallGetpid()
	prompt := bufio.NewReader(ctx.Stdin)
	status := 0
	var signalled []int

	for _, name := range names {
		matched := killallMatchAll(procs, name, ctx.Dir, self)
		if len(matched) == 0 {
			status = 1
			if !quiet {
				fmt.Fprintf(ctx.Stderr, "killall: %s: No such process\n", name)
			}
			continue
		}
		for _, p := range matched {
			if interactive && !killallConfirm(ctx, prompt, name, p.PID) {
				continue
			}
			if err := syscall.Kill(p.PID, sig); err != nil {
				status = 1
				if !quiet {
					fmt.Fprintf(ctx.Stderr, "killall: pid %d: %v\n", p.PID, err)
				}
				continue
			}
			if wait {
				signalled = append(signalled, p.PID)
			}
			if verbose {
				fmt.Fprintf(ctx.Stdout, "Killed %s(%d) with signal %d\n", name, p.PID, int(sig))
			}
		}
	}

	if len(signalled) > 0 {
		if err := killallWaitGone(waitCtx, signalled, killallPollInterval); err != nil {
			return ExitStatus(err)
		}
	}
	return status
}

// killallSplitSignal removes the first -SIGNAL operand (a single dash, before
// any --) whose text names a signal and returns it, leaving the rest for the
// option parser. -s and its value are untouched: "s" is not a signal.
func killallSplitSignal(argv []string) (string, []string) {
	rest := make([]string, 0, len(argv))
	for i, a := range argv {
		if a == "--" {
			return "", append(rest, argv[i:]...)
		}
		if len(a) > 1 && a[0] == '-' && a[1] != '-' {
			if _, ok := parseSignalArg(a[1:]); ok {
				return a[1:], append(rest, argv[i+1:]...)
			}
		}
		rest = append(rest, a)
	}
	return "", rest
}

// killallSignalList is the list `kill -l` prints: the names of 0..31 in
// numeric order, then the realtime bounds.
func killallSignalList() string {
	names := make([]string, 0, 34)
	for i := 0; i <= 31; i++ {
		if name, ok := sigNumberMap[i]; ok {
			names = append(names, name)
		}
	}
	names = append(names, "RTMIN", "RTMAX")
	return strings.Join(names, " ")
}

func killallUsage() string {
	return "Usage: killall [-l] [-iqvw] [-SIGNAL|-s SIGNAL] PROCESS_NAME...\n" +
		"\n" +
		"Send a signal (default: TERM) to all processes with the given names.\n" +
		"\n" +
		"  -i  Ask for confirmation before killing\n" +
		"  -l  Print list of all available signals\n" +
		"  -q  Don't print any warnings or error messages\n" +
		"  -s  Send SIGNAL instead of SIGTERM\n" +
		"  -v  Report if the signal was successfully sent\n" +
		"  -w  Wait until all signalled processes are dead\n"
}

// killallMatchAll returns the processes whose name matches operand, never the
// process killall itself runs in.
func killallMatchAll(procs []proc.Process, operand, dir string, self int) []proc.Process {
	var matched []proc.Process
	for _, p := range procs {
		if p.PID == self {
			continue
		}
		if killallMatch(operand, dir, p) {
			matched = append(matched, p)
		}
	}
	return matched
}

// killallMatch applies the pidof -x name rules in names_to_pid order: comm for
// a bare name that fits it, the executable's inode for a path operand, and the
// basenames of argv[0] and (for #! scripts) argv[1] as the fallback. A path
// operand that exists but is a different file matches nothing.
func killallMatch(operand, dir string, p proc.Process) bool {
	base := filepath.Base(operand)
	if !strings.ContainsRune(operand, '/') {
		if len(operand) <= 14 && p.Comm == operand {
			return true
		}
	} else if st, err := os.Stat(killallPath(dir, operand)); err == nil {
		return killallSameFile(st, p.Exe)
	}
	if len(p.Argv) > 0 && filepath.Base(p.Argv[0]) == base {
		return true
	}
	return len(p.Argv) > 1 && filepath.Base(p.Argv[1]) == base
}

func killallPath(dir, operand string) string {
	if filepath.IsAbs(operand) || dir == "" {
		return operand
	}
	return filepath.Join(dir, operand)
}

func killallSameFile(operand os.FileInfo, exe string) bool {
	if exe == "" {
		return false
	}
	exeInfo, err := os.Stat(exe)
	if err != nil {
		return false
	}
	return os.SameFile(operand, exeInfo)
}

func killallConfirm(ctx *Context, prompt *bufio.Reader, name string, pid int) bool {
	fmt.Fprintf(ctx.Stderr, "Signal %s(%d)? (y/N) ", name, pid)
	line, err := prompt.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	return strings.HasPrefix(line, "y") || strings.HasPrefix(line, "Y")
}

// killallWaitGone blocks until every pid answers ESRCH to signal 0.
func killallWaitGone(ctx context.Context, pids []int, interval time.Duration) error {
	for {
		alive := 0
		for _, pid := range pids {
			if err := syscall.Kill(pid, 0); err == nil || !errors.Is(err, syscall.ESRCH) {
				alive++
			}
		}
		if alive == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

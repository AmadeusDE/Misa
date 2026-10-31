// pidof — print the PIDs of all processes with the given names.
//
// The option set, the -s/-x/-o behaviour and the name matching follow toybox
// toys/lsb/pidof.c and its names_to_pid() helper in lib/lib.c (0BSD, commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb): the comm fast path for names that
// fit the 15-byte comm field, the /proc/PID/exe inode comparison for operands
// that contain a '/', argv[0]'s basename, and argv[1]'s basename for scripts
// when -x is set. The %PPID entry of -o follows uutils/procps pidof (MIT,
// commit e697667b3f758803e38d8ab4c8f912171beca602); the a-b ranges of -o are
// the same comma list read as inclusive pid ranges. Deliberately unsupported
// there and here: -S/--separator, -d, -q, -t, -w, -c/--check-root and -n. This
// is a native Go implementation; see docs/CREDITS.md.

package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/proc"
)

const pidofSpec = "s(single)o(omit):x(scripts)"

var pidofSpecParsed *Spec

func init() {
	var err error
	pidofSpecParsed, err = ParseSpec(pidofSpec)
	if err != nil {
		panic("builtins: bad pidof spec: " + err.Error())
	}
	Register(Command{Name: "pidof", Run: runPidof})
}

// Seams for tests: the procfs root and the pid %PPID stands for.
var (
	pidofProcRoot = ""
	pidofGetppid  = os.Getppid
)

func runPidof(ctx *Context) int {
	flags, err := pidofSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "pidof: %v\nTry 'pidof --help' for more information.\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, pidofUsage())
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "pidof (Misa) 0.1.0\n")
		return 0
	}
	names := flags.Operands
	if len(names) == 0 {
		return 1
	}

	var omit [][2]int
	if list, ok := flags.String("o"); ok {
		omit, err = pidofParseOmit(list, pidofGetppid())
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "pidof: %v\n", err)
			return 1
		}
	}

	ops := make([]pidofOperand, len(names))
	needsExe := false
	for i, name := range names {
		ops[i] = pidofOperandFor(name, ctx.Dir)
		needsExe = needsExe || ops[i].path != ""
	}

	snapshot, errs := proc.New(pidofProcRoot).Snapshot(ctx.CancellationContext(),
		proc.Options{Argv: true, Exe: needsExe})
	if len(errs) > 0 {
		if err := ctx.CancellationContext().Err(); err != nil {
			return ExitStatus(err)
		}
		fmt.Fprintf(ctx.Stderr, "pidof: %v\n", errs[0])
		return 1
	}

	scripts, single := flags.Bool("x"), flags.Bool("s")
	var pids []string
	for _, p := range snapshot {
		if pidofOmitted(omit, p.PID) || !pidofAnyMatch(p, ops, scripts) {
			continue
		}
		pids = append(pids, strconv.Itoa(p.PID))
		if single {
			break
		}
	}
	if len(pids) == 0 {
		return 1
	}
	fmt.Fprintf(ctx.Stdout, "%s\n", strings.Join(pids, " "))
	return 0
}

func pidofUsage() string {
	return "Usage: pidof [-s] [-o omitpid[,omitpid...]] [-x] [NAME...]\n\n" +
		"Print the PIDs of all processes with the given names.\n\n" +
		"  -s, --single     single shot, only return one pid\n" +
		"  -o, --omit LIST  omit PIDs: a pid, an inclusive A-B range, or %PPID\n" +
		"                   for the parent of the calling shell\n" +
		"  -x, --scripts    match shell scripts too (/bin/sh SCRIPT)\n" +
		"  -h, --help       display this help and exit\n" +
		"      --version    output version information and exit\n\n" +
		"A NAME without '/' matches argv[0]'s basename; with -x it also matches\n" +
		"the process comm (up to 15 bytes) and argv[1]'s basename, so a script\n" +
		"run as '/bin/sh SCRIPT' is found. A NAME with '/' matches only the\n" +
		"same file as the process's executable.\n\n" +
		"Unsupported (rejected, never faked): -S/--separator, -d, -q, -t, -w,\n" +
		"-c/--check-root, -n. Exit status: 0 at least one pid printed,\n" +
		"1 nothing matched or bad usage.\n"
}

// pidofOperand is one NAME resolved once: its basename, and for a name that
// contains a '/' its path (against the invocation directory) plus that file's
// identity when it exists.
type pidofOperand struct {
	name string
	base string
	path string
	info os.FileInfo
	comm bool
}

func pidofOperandFor(name, dir string) pidofOperand {
	op := pidofOperand{name: name, base: filepath.Base(name)}
	if !strings.Contains(name, "/") {
		// The comm field holds at most 15 bytes, and with a full 15 we cannot
		// tell whether the name fit or was truncated, so only 14 or less.
		op.comm = len(name) <= 14
		return op
	}
	op.path = name
	if !filepath.IsAbs(op.path) {
		op.path = filepath.Join(dir, op.path)
	}
	op.path = filepath.Clean(op.path)
	if info, err := os.Stat(op.path); err == nil {
		op.info = info
	}
	return op
}

// pidofMatches reports whether p is a process for op. It is pure: everything
// about the operand, including the stat of its path, is precomputed.
func pidofMatches(p proc.Process, op pidofOperand, scripts bool) bool {
	if scripts && op.comm && p.Comm == op.name {
		return true
	}
	if op.path != "" && p.Exe != "" && pidofSameFile(op, p.Exe) {
		return true
	}
	if len(p.Argv) > 0 && op.base == filepath.Base(p.Argv[0]) {
		return true
	}
	return scripts && len(p.Argv) > 1 && op.base == filepath.Base(p.Argv[1])
}

func pidofAnyMatch(p proc.Process, ops []pidofOperand, scripts bool) bool {
	for _, op := range ops {
		if pidofMatches(p, op, scripts) {
			return true
		}
	}
	return false
}

// pidofSameFile compares the operand's path with a process's executable by
// identity, falling back to the cleaned paths when either cannot be stat'ed.
func pidofSameFile(op pidofOperand, exe string) bool {
	if info, err := os.Stat(exe); err == nil && op.info != nil {
		return os.SameFile(op.info, info)
	}
	return op.path == filepath.Clean(exe)
}

// pidofOmitError names the -o entry that could not be understood.
type pidofOmitError struct{ entry string }

func (e *pidofOmitError) Error() string { return fmt.Sprintf("invalid -o value %q", e.entry) }

// pidofParseOmit reads the -o list into inclusive pid ranges. %PPID stands for
// ppid, the parent of the calling shell.
func pidofParseOmit(list string, ppid int) ([][2]int, error) {
	if list == "" {
		return nil, nil
	}
	var ranges [][2]int
	for _, entry := range strings.Split(list, ",") {
		lo, hi, err := pidofOmitRange(strings.TrimSpace(entry), ppid)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, [2]int{lo, hi})
	}
	return ranges, nil
}

func pidofOmitRange(entry string, ppid int) (int, int, error) {
	bad := &pidofOmitError{entry: entry}
	if entry == "%PPID" {
		return ppid, ppid, nil
	}
	loStr, hiStr, isRange := strings.Cut(entry, "-")
	lo, err := strconv.Atoi(loStr)
	if err != nil || lo < 0 {
		return 0, 0, bad
	}
	if !isRange {
		return lo, lo, nil
	}
	hi, err := strconv.Atoi(hiStr)
	if err != nil || hi < lo {
		return 0, 0, bad
	}
	return lo, hi, nil
}

func pidofOmitted(ranges [][2]int, pid int) bool {
	for _, r := range ranges {
		if pid >= r[0] && pid <= r[1] {
			return true
		}
	}
	return false
}

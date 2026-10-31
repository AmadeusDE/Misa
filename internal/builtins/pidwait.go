// pidwait — wait for processes matching a pattern to finish.
//
// The option set, the "no process matched" exit status and the pgrep-style
// matching rules follow uutils/procps pidwait (MIT, commit
// e697667b3f758803e38d8ab4c8f912171beca602), src/uu/pidwait/src/pidwait.rs and
// the matcher in src/uu/pgrep/src/process_matcher.rs. This is a native Go
// implementation built on internal/proc; uutils' -e/--echo, -c/--count and the
// remaining pgrep selectors are deliberately not implemented.

package builtins

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

const pidwaitSpec = "f(full)x(exact)u(user):"

var pidwaitSpecParsed *Spec

// Seams for tests: the procfs root, the poll delay and its sleeper, and the
// account lookup behind -u.
var (
	pidwaitProcRoot     = ""
	pidwaitPollInterval = 100 * time.Millisecond
	pidwaitSleep        = pidwaitDefaultSleep
)

type pidwaitUserLookup func(string) (*user.User, error)

var pidwaitLookupUser pidwaitUserLookup = user.Lookup

func init() {
	var err error
	pidwaitSpecParsed, err = ParseSpec(pidwaitSpec)
	if err != nil {
		panic("builtins: bad pidwait spec: " + err.Error())
	}
	Register(Command{Name: "pidwait", Run: runPidwait})
}

func pidwaitDefaultSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func runPidwait(ctx *Context) int {
	flags, err := pidwaitSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "pidwait: %v\nTry 'pidwait --help' for more information.\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, pidwaitUsage())
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "pidwait (Misa) 0.1.0\n")
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintf(ctx.Stderr, "pidwait: missing pattern\nTry 'pidwait --help' for more information.\n")
		return 1
	}

	opts := pidwaitMatchOptions{
		pattern: flags.Operands[0],
		exact:   flags.Bool("x"),
		full:    flags.Bool("f"),
	}
	if name, ok := flags.String("u"); ok {
		opts.user = name
	}

	ctx2 := ctx.CancellationContext()
	reader := proc.New(pidwaitProcRoot)
	procs, errs := reader.Snapshot(ctx2, proc.Options{Argv: opts.full})
	if err := ctx2.Err(); err != nil {
		return ExitStatus(err)
	}
	if len(errs) > 0 {
		fmt.Fprintf(ctx.Stderr, "pidwait: %v\n", errs[0])
		return 1
	}

	matched, err := pidwaitMatch(procs, opts, os.Getpid())
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "pidwait: %v\n", err)
		return 1
	}
	if len(matched) == 0 {
		return 1
	}

	tracked := make(map[int]uint64, len(matched))
	for _, p := range matched {
		tracked[p.PID] = p.StartTicks
	}
	return pidwaitWait(ctx2, ctx.Stderr, reader, tracked)
}

// pidwaitWait polls the matched processes until none of them is left. A pid
// that vanished, or that now carries a different start time (the number was
// reused), counts as finished.
func pidwaitWait(ctx context.Context, stderr io.Writer, reader *proc.Reader, tracked map[int]uint64) int {
	for len(tracked) > 0 {
		if err := ctx.Err(); err != nil {
			return ExitStatus(err)
		}
		if err := pidwaitSleep(ctx, pidwaitPollInterval); err != nil {
			return ExitStatus(err)
		}
		if err := ctx.Err(); err != nil {
			return ExitStatus(err)
		}
		snapshot, errs := reader.Snapshot(ctx, proc.Options{PIDs: pidwaitPIDs(tracked)})
		if err := ctx.Err(); err != nil {
			return ExitStatus(err)
		}
		if len(errs) > 0 {
			fmt.Fprintf(stderr, "pidwait: %v\n", errs[0])
			return 1
		}
		tracked = pidwaitStillRunning(tracked, snapshot)
	}
	return 0
}

// pidwaitStillRunning returns the tracked pids that still belong to the process
// pidwait was asked to wait for.
func pidwaitStillRunning(tracked map[int]uint64, snapshot []proc.Process) map[int]uint64 {
	alive := make(map[int]uint64, len(tracked))
	for _, p := range snapshot {
		if start, ok := tracked[p.PID]; ok && start == p.StartTicks {
			alive[p.PID] = start
		}
	}
	return alive
}

func pidwaitPIDs(tracked map[int]uint64) []int {
	pids := make([]int, 0, len(tracked))
	for pid := range tracked {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids
}

// pidwaitMatchOptions is the parsed selection: the pattern, how to apply it and
// the user filter ("", a numeric uid or a name).
type pidwaitMatchOptions struct {
	pattern string
	exact   bool
	full    bool
	user    string
}

func pidwaitMatch(procs []proc.Process, opts pidwaitMatchOptions, self int) ([]proc.Process, error) {
	var re *regexp.Regexp
	if !opts.exact {
		var err error
		re, err = regexp.Compile(opts.pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %v", opts.pattern, err)
		}
	}
	uid, hasUID := uint32(0), false
	if opts.user != "" {
		var err error
		if uid, err = pidwaitResolveUser(opts.user); err != nil {
			return nil, err
		}
		hasUID = true
	}

	var matched []proc.Process
	for _, p := range procs {
		if p.PID == self {
			continue
		}
		if hasUID && p.UID != uid {
			continue
		}
		if pidwaitMatches(p, opts, re) {
			matched = append(matched, p)
		}
	}
	return matched, nil
}

// pidwaitMatches applies the pattern to the process name, or with -f to its
// whole command line. An empty name never matches: the empty pattern would
// otherwise select every kernel thread.
func pidwaitMatches(p proc.Process, opts pidwaitMatchOptions, re *regexp.Regexp) bool {
	subject := p.Comm
	if opts.full {
		if len(p.Argv) == 0 {
			return false
		}
		subject = strings.Join(p.Argv, " ")
	}
	if subject == "" {
		return false
	}
	if opts.exact {
		return subject == opts.pattern
	}
	return re.MatchString(subject)
}

func pidwaitResolveUser(name string) (uint32, error) {
	if id, err := strconv.ParseUint(name, 10, 32); err == nil {
		return uint32(id), nil
	}
	u, err := pidwaitLookupUser(name)
	if err != nil {
		return 0, fmt.Errorf("no such user: %s", name)
	}
	id, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("no such user: %s", name)
	}
	return uint32(id), nil
}

func pidwaitUsage() string {
	return "Usage: pidwait [OPTIONS] PATTERN\n\n" +
		"Wait for the processes matching PATTERN to finish. PATTERN is a Go (RE2)\n" +
		"regular expression matched against the process name.\n\n" +
		"  -f, --full       match PATTERN against the full command line\n" +
		"  -x, --exact      PATTERN must equal the whole name (or command line)\n" +
		"  -u, --user USER  only processes of USER (a name or a numeric uid)\n" +
		"  -h, --help       display this help and exit\n" +
		"      --version    output version information and exit\n\n" +
		"Nothing is printed while waiting. Exit status: 0 every matched process\n" +
		"finished, 1 no process matched or bad usage, 130 interrupted.\n"
}

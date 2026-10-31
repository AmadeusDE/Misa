// Package proc reads process snapshots for the ps family of utilities.
//
// The Linux/Android backend parses a procfs tree: the real /proc, or a
// recorded fixture root handed to New. Other platforms report ErrUnsupported
// instead of inventing data. Data that a platform or the process's permissions
// cannot supply stays at its zero value — an unreadable /proc/PID/cmdline
// leaves Argv nil, an unreadable exe link leaves Exe empty — and a process
// that disappears while the snapshot runs is skipped without an error.
package proc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// ClockTicks is USER_HZ. /proc reports CPU and start times in 1/100 s ticks
// whatever HZ the kernel was built with, so this constant is not a guess.
const ClockTicks = 100

// ErrUnsupported reports a platform without a process-listing backend.
var ErrUnsupported = errors.New("process listing is not supported on this platform")

// Process is one process as seen by a single snapshot.
type Process struct {
	PID, PPID, PGID, SID int

	// Comm is /proc/PID/stat's comm: the thread name, truncated to 15 bytes
	// by the kernel and never wrapped in brackets.
	Comm string

	// State is the one-letter process state from stat (R, S, D, Z, T, t, ...).
	State byte

	// TTY is the controlling terminal as the raw device number stat reports,
	// or 0 when the process has no controlling terminal.
	TTY int
	// TTYName is the resolved terminal name ("pts/3", "tty1") or "?" for none.
	// It is best effort: unreadable fd links fall back to a device-number name.
	TTYName string
	// Tpgid is the foreground process group of that terminal (or -1).
	Tpgid int

	// UID and GID are effective ids (the owner of /proc/PID), RUID and RGID
	// are the real ids from /proc/PID/status. IDsExact is false when status
	// could not be read, in which case the real ids mirror the effective ones.
	UID, GID   uint32
	RUID, RGID uint32
	IDsExact   bool

	// Nice is the static niceness and Priority the raw kernel priority from
	// stat (lower values mean higher priority); realtime processes are
	// negative.
	Nice     int32
	Priority int32

	// Threads is the thread count from stat.
	Threads int

	// RSS is the resident set size in pages and VSZ the virtual size in bytes,
	// both exactly as stat reports them.
	RSS uint64
	VSZ uint64

	// StartTicks is the process start time in clock ticks after boot;
	// UTicks and STicks are its consumed user and system CPU ticks.
	StartTicks uint64
	UTicks     uint64
	STicks     uint64

	// Argv is the argument vector from /proc/PID/cmdline, nil when the file is
	// unreadable or empty (kernel threads have no command line). It is only
	// filled when Options.Argv asks for it.
	Argv []string
	// Exe is the resolved /proc/PID/exe link, empty when the link is
	// unreadable (permissions, or a kernel thread) or Options.Exe is unset.
	Exe string
}

// CPUTicks returns the total user plus system CPU time in clock ticks.
func (p Process) CPUTicks() uint64 { return p.UTicks + p.STicks }

// Options selects what a snapshot reads. The core stat data is always read;
// Argv and Exe cost an extra file per process and are read on request.
type Options struct {
	// PIDs restricts the snapshot to these processes. A nil slice means every
	// process in the tree; PIDs that are gone are skipped silently.
	PIDs []int
	// Argv reads /proc/PID/cmdline into Process.Argv.
	Argv bool
	// Exe resolves the /proc/PID/exe link into Process.Exe.
	Exe bool
}

// Reader reads snapshots from one procfs tree.
type Reader struct{ root string }

// New returns a Reader for root; an empty root means /proc.
func New(root string) *Reader {
	if root == "" {
		root = "/proc"
	}
	return &Reader{root: root}
}

// Root returns the procfs root the reader was built for.
func (r *Reader) Root() string { return r.root }

// Snapshot reads the real /proc.
func Snapshot(ctx context.Context, opts Options) ([]Process, []error) {
	return New("").Snapshot(ctx, opts)
}

// Snapshot reads every selected process, sorted by PID.
//
// The returned errors describe failures that affected the snapshot as a whole
// (an unsupported platform, an unreadable procfs root, cancellation); a single
// process that cannot be read or exits mid-scan is skipped silently.
func (r *Reader) Snapshot(ctx context.Context, opts Options) ([]Process, []error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := r.checkSupported(); err != nil {
		return nil, []error{err}
	}
	if err := ctx.Err(); err != nil {
		return nil, []error{err}
	}
	pids, err := r.list(ctx, opts.PIDs)
	if err != nil {
		return nil, []error{err}
	}
	procs := make([]Process, 0, len(pids))
	for _, pid := range pids {
		if err := ctx.Err(); err != nil {
			return procs, []error{err}
		}
		if p, ok := r.readProcess(pid, opts); ok {
			procs = append(procs, p)
		}
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].PID < procs[j].PID })
	return procs, nil
}

// list returns the PIDs to read: the requested ones, or every numeric entry of
// the procfs root.
func (r *Reader) list(ctx context.Context, want []int) ([]int, error) {
	if want != nil {
		pids := make([]int, 0, len(want))
		for _, pid := range want {
			if pid > 0 {
				pids = append(pids, pid)
			}
		}
		return pids, nil
	}
	entries, err := os.ReadDir(r.root)
	if err != nil {
		return nil, err
	}
	pids := make([]int, 0, len(entries))
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return pids, err
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// Dir returns the directory holding one process.
func (r *Reader) Dir(pid int) string {
	return filepath.Join(r.root, strconv.Itoa(pid))
}

// Uptime reads the time since boot from <root>/uptime.
func (r *Reader) Uptime() (time.Duration, error) {
	data, err := os.ReadFile(filepath.Join(r.root, "uptime"))
	if err != nil {
		return 0, err
	}
	return ParseUptime(data)
}

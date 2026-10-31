//go:build linux

package proc

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const fixtureRoot = "testdata/proc"

func snapshotFixture(t *testing.T, opts Options) []Process {
	t.Helper()
	procs, errs := New(fixtureRoot).Snapshot(context.Background(), opts)
	if len(errs) != 0 {
		t.Fatalf("Snapshot errors = %v", errs)
	}
	return procs
}

func findProcess(t *testing.T, procs []Process, pid int) Process {
	t.Helper()
	for _, p := range procs {
		if p.PID == pid {
			return p
		}
	}
	t.Fatalf("pid %d not in snapshot (got %d processes)", pid, len(procs))
	return Process{}
}

func TestSnapshotFixture(t *testing.T) {
	procs := snapshotFixture(t, Options{Argv: true, Exe: true})

	var pids []int
	for _, p := range procs {
		pids = append(pids, p.PID)
	}
	want := []int{1, 2, 7, 8, 42, 300}
	if len(pids) != len(want) {
		t.Fatalf("pids = %v, want %v (the truncated stat and the non-process entries are skipped)", pids, want)
	}
	for i := range want {
		if pids[i] != want[i] {
			t.Fatalf("pids = %v, want %v", pids, want)
		}
	}

	shell := findProcess(t, procs, 7)
	checks := []struct {
		name      string
		got, want any
	}{
		{"PPID", shell.PPID, 1},
		{"PGID", shell.PGID, 7},
		{"SID", shell.SID, 7},
		{"Comm", shell.Comm, "sh) odd"},
		{"State", string(shell.State), "S"},
		{"TTY", shell.TTY, (136 << 8) | 3},
		{"TTYName", shell.TTYName, "pts/3"},
		{"Tpgid", shell.Tpgid, 7},
		{"UID", shell.UID, uint32(1001)},
		{"RUID", shell.RUID, uint32(1000)},
		{"GID", shell.GID, uint32(200)},
		{"RGID", shell.RGID, uint32(100)},
		{"IDsExact", shell.IDsExact, true},
		{"Nice", shell.Nice, int32(0)},
		{"Priority", shell.Priority, int32(20)},
		{"Threads", shell.Threads, 2},
		{"RSS", shell.RSS, uint64(1000)},
		{"VSZ", shell.VSZ, uint64(9876543)},
		{"StartTicks", shell.StartTicks, uint64(200)},
		{"UTicks", shell.UTicks, uint64(12)},
		{"STicks", shell.STicks, uint64(3)},
		{"CPUTicks", shell.CPUTicks(), uint64(15)},
		{"Exe", shell.Exe, "/usr/bin/sh"},
		{"Argv", strings.Join(shell.Argv, "|"), "/usr/bin/sh|-c|echo hi"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %#v, want %#v", c.name, c.got, c.want)
		}
	}
}

func TestSnapshotThreadsAndZombies(t *testing.T) {
	procs := snapshotFixture(t, Options{Argv: true, Exe: true})

	init := findProcess(t, procs, 1)
	if init.TTY != 0 || init.TTYName != "?" {
		t.Errorf("no terminal: TTY = %d, TTYName = %q, want 0 and \"?\"", init.TTY, init.TTYName)
	}
	if got := strings.Join(init.Argv, " "); got != "/sbin/init --system" {
		t.Errorf("Argv = %q", got)
	}
	if init.Exe != "sbin/init" {
		t.Errorf("Exe = %q, want the raw link target", init.Exe)
	}

	kernel := findProcess(t, procs, 2)
	if kernel.Argv != nil {
		t.Errorf("kernel thread Argv = %q, want nil", kernel.Argv)
	}
	if kernel.Exe != "" {
		t.Errorf("kernel thread Exe = %q, want empty", kernel.Exe)
	}
	if kernel.Nice != -5 {
		t.Errorf("Nice = %d, want -5", kernel.Nice)
	}

	zombie := findProcess(t, procs, 42)
	if zombie.State != 'Z' {
		t.Errorf("state = %q, want Z", string(zombie.State))
	}
	if zombie.Threads != 0 {
		t.Errorf("Threads = %d, want 0", zombie.Threads)
	}
}

func TestSnapshotWithoutOptionalFiles(t *testing.T) {
	for _, p := range snapshotFixture(t, Options{}) {
		if p.Argv != nil || p.Exe != "" {
			t.Errorf("pid %d: Argv = %q, Exe = %q without Options.Argv/Exe", p.PID, p.Argv, p.Exe)
		}
	}
}

func TestSnapshotPIDSelection(t *testing.T) {
	procs, errs := New(fixtureRoot).Snapshot(context.Background(), Options{PIDs: []int{7, 999999}})
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	if len(procs) != 1 || procs[0].PID != 7 {
		t.Fatalf("procs = %+v, want only pid 7 (a vanished pid is skipped)", procs)
	}

	procs, errs = New(fixtureRoot).Snapshot(context.Background(), Options{PIDs: []int{400}})
	if len(errs) != 0 || len(procs) != 0 {
		t.Fatalf("unparsable pid: procs = %+v, errors = %v", procs, errs)
	}
}

// writeTree creates a procfs-like tree for the race and permission tests.
func writeTree(t *testing.T, files map[string]string, dirs ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func statLine(pid int, comm string) string {
	id := strconv.Itoa(pid)
	return id + " (" + comm + ") S 1 " + id + " " + id +
		" 0 -1 4194560 0 0 0 0 0 0 0 0 20 0 1 0 10 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n"
}

func TestSnapshotSkipsVanishedProcess(t *testing.T) {
	// pid 9 is listed but its stat file disappears before it is read.
	root := writeTree(t, map[string]string{
		"8/stat": statLine(8, "eight"),
	}, "9")
	procs, errs := New(root).Snapshot(context.Background(), Options{})
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	if len(procs) != 1 || procs[0].PID != 8 {
		t.Fatalf("procs = %+v, want only pid 8", procs)
	}
}

func TestSnapshotUnreadableOptionalFields(t *testing.T) {
	// A directory where a proc file belongs makes the read fail the same way a
	// permission error does, without needing to drop privileges.
	root := writeTree(t, map[string]string{
		"5/stat": statLine(5, "five"),
	}, "5/status", "5/cmdline", "5/exe")
	procs, errs := New(root).Snapshot(context.Background(), Options{Argv: true, Exe: true})
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	if len(procs) != 1 {
		t.Fatalf("procs = %+v", procs)
	}
	p := procs[0]
	if p.Argv != nil || p.Exe != "" {
		t.Errorf("Argv = %q, Exe = %q, want unavailable", p.Argv, p.Exe)
	}
	if p.IDsExact {
		t.Error("IDsExact = true without a readable status file")
	}
	if want := uint32(os.Geteuid()); p.UID != want || p.RUID != want {
		t.Errorf("uid = %d/%d, want the directory owner %d", p.RUID, p.UID, want)
	}
}

func TestSnapshotPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "5"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"stat": statLine(5, "five"), "cmdline": "five\x00"} {
		if err := os.WriteFile(filepath.Join(root, "5", name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(root, "5", "cmdline"), 0); err != nil {
		t.Fatal(err)
	}
	procs, errs := New(root).Snapshot(context.Background(), Options{Argv: true})
	if len(errs) != 0 || len(procs) != 1 {
		t.Fatalf("procs = %+v, errors = %v", procs, errs)
	}
	if procs[0].Argv != nil {
		t.Errorf("Argv = %q, want nil for an unreadable cmdline", procs[0].Argv)
	}
}

func TestSnapshotMissingRoot(t *testing.T) {
	procs, errs := New(filepath.Join(t.TempDir(), "absent")).Snapshot(context.Background(), Options{})
	if len(procs) != 0 || len(errs) != 1 {
		t.Fatalf("procs = %+v, errors = %v, want one error and no processes", procs, errs)
	}
}

func TestSnapshotCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	procs, errs := New(fixtureRoot).Snapshot(ctx, Options{})
	if len(procs) != 0 {
		t.Fatalf("procs = %+v, want none", procs)
	}
	if len(errs) != 1 || errs[0] != context.Canceled {
		t.Fatalf("errors = %v, want context.Canceled", errs)
	}
}

func TestSnapshotLiveProcess(t *testing.T) {
	procs, errs := Snapshot(context.Background(), Options{Argv: true})
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	self := findProcess(t, procs, os.Getpid())
	if self.Comm == "" {
		t.Error("Comm is empty")
	}
	if self.PPID != os.Getppid() {
		t.Errorf("PPID = %d, want %d", self.PPID, os.Getppid())
	}
	if len(self.Argv) == 0 {
		t.Error("Argv is empty for our own process")
	}
	if !self.IDsExact || self.UID != uint32(os.Geteuid()) {
		t.Errorf("ids = %d/%d exact=%v, want %d", self.RUID, self.UID, self.IDsExact, os.Geteuid())
	}
}

func TestUptime(t *testing.T) {
	got, err := New(fixtureRoot).Uptime()
	if err != nil {
		t.Fatal(err)
	}
	if want := 12345*time.Second + 670*time.Millisecond; got != want {
		t.Errorf("Uptime = %v, want %v", got, want)
	}
	if _, err := New(t.TempDir()).Uptime(); err == nil {
		t.Error("missing uptime file did not fail")
	}
}

func TestParseUptime(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"12345.67 98765.43\n", 12345*time.Second + 670*time.Millisecond, true},
		{"0.00 0.00\n", 0, true},
		{"", 0, false},
		{"garbage\n", 0, false},
		{"-1.5 0\n", 0, false},
	}
	for _, c := range cases {
		got, err := ParseUptime([]byte(c.in))
		if (err == nil) != c.ok {
			t.Errorf("ParseUptime(%q) error = %v, want ok=%v", c.in, err, c.ok)
			continue
		}
		if c.ok && got != c.want {
			t.Errorf("ParseUptime(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestTTYName(t *testing.T) {
	cases := []struct {
		dev  int
		want string
	}{
		{0, "?"},
		{-1, "?"},
		{(136 << 8) | 3, "pts/3"},
		{(143 << 8) | 255, "pts/2047"},
		{(4 << 8) | 0, "tty0"},
		{(4 << 8) | 63, "tty63"},
		{(4 << 8) | 64, "ttyS0"},
		{(5 << 8) | 1, "console"},
		{(10 << 8) | 200, "10:200"},
	}
	for _, c := range cases {
		if got := TTYName(c.dev); got != c.want {
			t.Errorf("TTYName(%d) = %q, want %q", c.dev, got, c.want)
		}
	}
}

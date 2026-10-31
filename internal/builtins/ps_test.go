package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

// installPSFixture points ps at the recorded proc tree in internal/proc and
// pins the clock and the account lookups so output is deterministic.
func installPSFixture(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("../proc/testdata/proc"); err != nil {
		// Device test runs (android-device.sh) stage only this package.
		t.Skipf("proc fixture tree unavailable: %v", err)
	}
	oldRoot, oldNow, oldTTY := psProcRoot, psNow, psOwnTTY
	oldUser, oldPage := psLookupUser, psPageSize
	psProcRoot = "../proc/testdata/proc"
	psNow = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	psOwnTTY = func() int { return (136 << 8) | 3 } // pts/3
	psLookupUser = func(uid uint32) string {
		switch uid {
		case 0:
			return "root"
		case 1000:
			return "bob"
		case 1001:
			return "alice"
		}
		return ""
	}
	psPageSize = 4096
	t.Cleanup(func() {
		psProcRoot, psNow, psOwnTTY = oldRoot, oldNow, oldTTY
		psLookupUser, psPageSize = oldUser, oldPage
	})
}

// psRun executes ps with an explicit stdout TTY state and width.
func psRun(t *testing.T, stdoutTTY bool, width int, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("ps")
	if !ok {
		t.Fatal("ps not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:       t.TempDir(),
		Args:      args,
		Width:     width,
		Height:    24,
		Stdout:    &stdout,
		Stderr:    &stderr,
		Stdin:     strings.NewReader(""),
		StdoutTTY: stdoutTTY,
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestPsRegistered(t *testing.T) {
	if _, ok := Lookup("ps"); !ok {
		t.Fatal("ps not registered")
	}
}

func TestPsDefaultSelection(t *testing.T) {
	installPSFixture(t)
	r := psRun(t, false, 80)
	assertExitCode(t, r, 0)
	want := "" +
		"   PID TTY          TIME CMD\n" +
		"     7 pts/3    00:00:00 sh) odd\n" +
		"     8 pts/3    00:00:00 sleep\n"
	if r.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", r.stdout, want)
	}
}

func TestPsAllProcesses(t *testing.T) {
	installPSFixture(t)
	r := psRun(t, false, 80, "-A", "-o", "pid")
	assertExitCode(t, r, 0)
	if want := "   PID\n     1\n     2\n     7\n     8\n    42\n   300\n"; r.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", r.stdout, want)
	}
}

func TestPsExplicitSelectionOverridesDefault(t *testing.T) {
	installPSFixture(t)
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"-p", "1"}, []string{"1"}},
		{[]string{"1"}, []string{"1"}},
		{[]string{"-p", "1", "-p", "300"}, []string{"1", "300"}},
		{[]string{"--pid", "300,1"}, []string{"1", "300"}},
	}
	for _, c := range cases {
		r := psRun(t, false, 80, append(c.args, "-o", "pid=")...)
		assertExitCode(t, r, 0)
		if got := psColumnValues(r); strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("args %v: rows = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestPsSessionLeaderFilters(t *testing.T) {
	installPSFixture(t)
	all := psRun(t, false, 80, "-A", "-o", "pid=")
	assertExitCode(t, all, 0)
	if got, want := psColumnValues(all), []string{"1", "2", "7", "8", "42", "300"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("-A rows = %v, want %v", got, want)
	}

	noLeaders := psRun(t, false, 80, "-d", "-o", "pid=")
	assertExitCode(t, noLeaders, 0)
	if got, want := psColumnValues(noLeaders), []string{"2", "8"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("-d rows = %v, want %v", got, want)
	}

	withTTY := psRun(t, false, 80, "-a", "-o", "pid=")
	assertExitCode(t, withTTY, 0)
	if got, want := psColumnValues(withTTY), []string{"8"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("-a rows = %v, want %v", got, want)
	}
}

func TestPsFieldValues(t *testing.T) {
	installPSFixture(t)
	fields := "pid,ppid,pgid,sid,uid,user,gid,tty,stat,s,ni,pri,nlwp,rss,vsz,time,etime,comm,args,cmd"
	r := psRun(t, false, 0, "-A", "-p", "7", "-o", fields)
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %q", lines)
	}
	got := strings.Fields(lines[1])
	want := []string{
		"7", "1", "7", "7", "1001", "alice", "200", "pts/3", "Ssl+", "S",
		"0", "19", "2", "4000", "9645", "00:00:00", "03:25:43", "sh",
		"/usr/bin/sh", "-c", "echo", "hi", "sh)", "odd",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("row = %q\nwant %q", got, want)
	}
}

func TestPsUnavailableCommandLine(t *testing.T) {
	installPSFixture(t)
	r := psRun(t, false, 80, "-p", "2", "-o", "comm,args,cmd")
	assertExitCode(t, r, 0)
	if want := "COMM            ARGS                        CMD\n[kthreadd]      [kthreadd]                  kthreadd\n"; r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestPsUserAndGroupSelection(t *testing.T) {
	installPSFixture(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-u", "root"}, "1,2,42"},
		{[]string{"-u", "0"}, "1,2,42"},
		{[]string{"-u", "1001"}, "7"},
		{[]string{"-U", "1000"}, "7,8,300"},
		{[]string{"-g", "200"}, "7"},
		{[]string{"-G", "100"}, "7,8"},
		{[]string{"-t", "pts/3"}, "7,8"},
		{[]string{"-t", "/dev/pts/3"}, "7,8"},
		{[]string{"-t", "tty1"}, "300"},
		{[]string{"-p", "2", "-u", "1000"}, "2,8,300"},
	}
	for _, c := range cases {
		r := psRun(t, false, 80, append(c.args, "-o", "pid=")...)
		assertExitCode(t, r, 0)
		if got := strings.Join(psColumnValues(r), ","); got != c.want {
			t.Errorf("%v = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestPsNoMatch(t *testing.T) {
	installPSFixture(t)
	r := psRun(t, false, 80, "-p", "999999")
	assertExitCode(t, r, 1)
	if r.stdout != "" || r.stderr != "" {
		t.Errorf("stdout = %q, stderr = %q, want both empty", r.stdout, r.stderr)
	}
}

func TestPsFullAndLongLayouts(t *testing.T) {
	installPSFixture(t)
	full := psRun(t, false, 0, "-A", "-p", "7", "-f")
	assertExitCode(t, full, 0)
	header := strings.Fields(strings.SplitN(full.stdout, "\n", 2)[0])
	if want := "USER PID PPID STIME TTY TIME CMD"; strings.Join(header, " ") != want {
		t.Errorf("-f header = %q, want %q", strings.Join(header, " "), want)
	}

	long := psRun(t, false, 0, "-A", "-p", "7", "-l")
	assertExitCode(t, long, 0)
	header = strings.Fields(strings.SplitN(long.stdout, "\n", 2)[0])
	if want := "S UID PID PPID PRI NI NLWP VSZ TTY TIME CMD"; strings.Join(header, " ") != want {
		t.Errorf("-l header = %q, want %q", strings.Join(header, " "), want)
	}
}

func TestPsColumnWidthAndTitle(t *testing.T) {
	installPSFixture(t)
	r := psRun(t, false, 0, "-p", "7", "-o", "pid:8=IDENT,tty:10=TERMINAL,cmd=t")
	assertExitCode(t, r, 0)
	want := "   IDENT TERMINAL   t\n" +
		"       7 pts/3      sh) odd\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestPsHeaderSuppression(t *testing.T) {
	installPSFixture(t)
	r := psRun(t, false, 0, "-p", "7", "-o", "pid=")
	assertExitCode(t, r, 0)
	if want := "     7\n"; r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestPsSort(t *testing.T) {
	installPSFixture(t)
	r := psRun(t, false, 0, "-A", "-o", "pid=", "--sort", "-pid")
	assertExitCode(t, r, 0)
	if got, want := strings.Join(psColumnValues(r), ","), "300,42,8,7,2,1"; got != want {
		t.Errorf("--sort -pid = %q, want %q", got, want)
	}

	r = psRun(t, false, 0, "-A", "-o", "pid=", "-k", "rss,-pid")
	assertExitCode(t, r, 0)
	if got, want := strings.Join(psColumnValues(r), ","), "42,2,300,8,1,7"; got != want {
		t.Errorf("-k rss,-pid = %q, want %q", got, want)
	}
}

func TestPsTruncation(t *testing.T) {
	installPSFixture(t)
	r := psRun(t, true, 20, "-p", "7", "-o", "pid,args")
	assertExitCode(t, r, 0)
	want := "   PID ARGS\n     7 /usr/bin/sh +\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}

	wide := psRun(t, true, 20, "-p", "7", "-w", "-o", "pid,args")
	assertExitCode(t, wide, 0)
	want = "   PID ARGS\n     7 /usr/bin/sh -c echo hi\n"
	if wide.stdout != want {
		t.Errorf("-w stdout = %q, want %q", wide.stdout, want)
	}
}

func TestPsUnsupportedOptions(t *testing.T) {
	installPSFixture(t)
	for _, args := range [][]string{
		{"-M"}, {"-n"}, {"-O", "pid"}, {"-P", "1"}, {"-s", "1"}, {"-T"}, {"-Z"},
	} {
		r := psRun(t, false, 80, args...)
		if r.code != 1 {
			t.Errorf("%v: exit = %d, want 1", args, r.code)
		}
		if !strings.Contains(r.stderr, "unknown option") {
			t.Errorf("%v: stderr = %q, want an unknown option error", args, r.stderr)
		}
		if r.stdout != "" {
			t.Errorf("%v: stdout = %q, want none", args, r.stdout)
		}
	}
}

func TestPsUsageErrors(t *testing.T) {
	installPSFixture(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-o", "bogus"}, "unknown field 'bogus'"},
		{[]string{"-o", "pid,,tty"}, "empty field"},
		{[]string{"-o", "pid:x"}, "bad field width"},
		{[]string{"-p", "abc"}, "invalid process id 'abc'"},
		{[]string{"-p", "7,"}, "missing process id"},
		{[]string{"-u", "nosuchuser_xyz"}, "invalid user name 'nosuchuser_xyz'"},
		{[]string{"-g", "nosuchgroup_xyz"}, "invalid group name 'nosuchgroup_xyz'"},
	}
	for _, c := range cases {
		r := psRun(t, false, 80, c.args...)
		if r.code != 1 {
			t.Errorf("%v: exit = %d, want 1 (stderr %q)", c.args, r.code, r.stderr)
		}
		if !strings.Contains(r.stderr, c.want) {
			t.Errorf("%v: stderr = %q, want %q", c.args, r.stderr, c.want)
		}
		if !strings.Contains(r.stderr, "ps --help") {
			t.Errorf("%v: stderr = %q, want a --help hint", c.args, r.stderr)
		}
	}
}

func TestPsHelp(t *testing.T) {
	installPSFixture(t)
	r := psRun(t, false, 80, "--help")
	assertExitCode(t, r, 0)
	for _, want := range []string{"Usage: ps", "Unsupported (rejected", "-k, --sort", "Exit status"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("--help output misses %q", want)
		}
	}

	fields := psRun(t, false, 80, "-o", "help")
	assertExitCode(t, fields, 0)
	for _, want := range []string{"pid", "etime", "args", "Aliases"} {
		if !strings.Contains(fields.stdout, want) {
			t.Errorf("-o help output misses %q", want)
		}
	}

	version := psRun(t, false, 80, "--version")
	assertExitCode(t, version, 0)
	if !strings.Contains(version.stdout, "ps (Misa)") {
		t.Errorf("--version = %q", version.stdout)
	}
}

func TestPsLiveProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("no procfs backend on this platform")
	}
	r := psRun(t, false, 0, "-A", "-o", "pid,args")
	assertExitCode(t, r, 0)
	self := strconv.Itoa(os.Getpid())
	for _, line := range strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")[1:] {
		if strings.HasPrefix(strings.TrimSpace(line), self+" ") {
			// Our own row must carry a command line, since it is ours to read.
			if !strings.HasSuffix(strings.TrimSpace(line), self) {
				return
			}
		}
	}
	t.Fatalf("our pid %s is missing from ps -A output:\n%s", self, r.stdout)
}

func TestPsNoBootTime(t *testing.T) {
	// Without an uptime file the start and elapsed columns report "?" instead
	// of inventing a boot time.
	p := psProcessFixture()
	env := psEnv{Now: time.Unix(1700000000, 0)}
	for _, name := range []string{"stime", "etime"} {
		field, ok := lookupPSField(name)
		if !ok {
			t.Fatalf("field %s missing", name)
		}
		if text, _ := field.value(p, env); text != "?" {
			t.Errorf("%s = %q, want ?", name, text)
		}
	}
}

func TestPsMissingProcRoot(t *testing.T) {
	installPSFixture(t)
	psProcRoot = filepath.Join(t.TempDir(), "absent")
	r := psRun(t, false, 80, "-A")
	if r.code != 1 {
		t.Errorf("exit = %d, want 1", r.code)
	}
	if !strings.Contains(r.stderr, "ps:") {
		t.Errorf("stderr = %q, want a ps error", r.stderr)
	}
}

// psColumnValues returns the first column of every row, trimmed, with the
// header line already suppressed by an empty title.
func psColumnValues(r runResult) []string {
	var values []string
	for _, line := range strings.Split(strings.TrimRight(r.stdout, "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			values = append(values, strings.SplitN(line, " ", 2)[0])
		}
	}
	return values
}

// psProcessFixture is the pid 7 process of the recorded tree, for field tests
// that do not need a snapshot.
func psProcessFixture() proc.Process {
	return proc.Process{
		PID: 7, PPID: 1, PGID: 7, SID: 7, Comm: "sh) odd", State: 'S',
		TTY: (136 << 8) | 3, TTYName: "pts/3", Tpgid: 7,
		UID: 1001, RUID: 1000, GID: 200, RGID: 100, IDsExact: true,
		Nice: 0, Priority: 20, Threads: 2, RSS: 1000, VSZ: 9876543,
		StartTicks: 200, UTicks: 12, STicks: 3,
		Argv: []string{"/usr/bin/sh", "-c", "echo hi"}, Exe: "/usr/bin/sh",
	}
}

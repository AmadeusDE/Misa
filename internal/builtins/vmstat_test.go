package builtins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// installVmstatFixture points vmstat at the recorded proc tree in internal/proc
// and pins the page size.
func installVmstatFixture(t *testing.T) {
	t.Helper()
	oldRoot, oldPage := vmstatProcRoot, vmstatPageSize
	vmstatProcRoot = "../proc/testdata/proc"
	vmstatPageSize = func() int { return 4096 }
	t.Cleanup(func() {
		vmstatProcRoot, vmstatPageSize = oldRoot, oldPage
	})
}

// vmstatTree is a writable procfs tree: the delta tests rewrite the files
// between samples.
type vmstatTree struct{ dir string }

func newVmstatTree(t *testing.T, files map[string]string) *vmstatTree {
	t.Helper()
	v := &vmstatTree{dir: t.TempDir()}
	for name, content := range files {
		v.write(t, name, content)
	}
	old := vmstatProcRoot
	vmstatProcRoot = v.dir
	t.Cleanup(func() { vmstatProcRoot = old })
	return v
}

func (v *vmstatTree) write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(v.dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func vmstatRun(t *testing.T, args ...string) runResult {
	t.Helper()
	return vmstatRunCtx(t, context.Background(), args...)
}

func vmstatRunCtx(t *testing.T, ctx context.Context, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("vmstat")
	if !ok {
		t.Fatal("vmstat not registered")
	}
	var stdout, stderr strings.Builder
	code := cmd.Run(&Context{
		Dir:     t.TempDir(),
		Args:    args,
		CmdName: "vmstat",
		Ctx:     ctx,
		Width:   80,
		Height:  25,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   strings.NewReader(""),
	})
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestVmstatRegistered(t *testing.T) {
	if _, ok := Lookup("vmstat"); !ok {
		t.Fatal("vmstat not registered")
	}
}

func TestVmstatSinceBoot(t *testing.T) {
	installVmstatFixture(t)
	r := vmstatRun(t)
	assertExitCode(t, r, 0)
	want := "" +
		"procs ------------memory------------ ---swap-- -----io---- --system- ----cpu----\n" +
		" r  b    swpd    free   buff   cache   si   so    bi    bo   in   cs us sy id wa\n" +
		" 2  1  524288 1048576 262144 1835008    1    1   361   721 1001  801 10  5 80  1\n"
	assertStdout(t, r, want)
}

func TestVmstatActive(t *testing.T) {
	installVmstatFixture(t)
	r := vmstatRun(t, "-a")
	assertExitCode(t, r, 0)
	want := "" +
		"procs -----------memory---------- ---swap-- -----io---- --system- ----cpu----\n" +
		" r  b    swpd    free  inact  active   si   so    bi    bo   in   cs us sy id wa\n" +
		" 2  1  524288 1048576 1310720 3145728   1    1   361   721 1001  801 10  5 80  1\n"
	assertStdout(t, r, want)
}

func TestVmstatUnit(t *testing.T) {
	installVmstatFixture(t)
	r := vmstatRun(t, "-S", "m")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, " 2  1     512    1024    256    1792    1    1   361   721 1001  801 10  5 80  1\n")

	for _, unit := range []string{"k", "K", "M"} {
		if r := vmstatRun(t, "-S", unit); r.code != 0 {
			t.Errorf("-S %s: exit %d, stderr %q", unit, r.code, r.stderr)
		}
	}
	r = vmstatRun(t, "-S", "x")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "-S requires k, K, m or M")
}

// vmstatSecond rewrites a tree to the values that follow the boot snapshot.
func vmstatSecond(t *testing.T, v *vmstatTree) {
	t.Helper()
	v.write(t, "stat", "cpu 1100 0 600 8800 500 0 0 0 0 0\nintr 2000\nctxt 3000\nprocs_running 3\nprocs_blocked 0\n")
	v.write(t, "vmstat", "pgpgin 300\npgpgout 600\npswpin 30\npswpout 60\n")
}

func vmstatDeltaTree(t *testing.T) *vmstatTree {
	t.Helper()
	return newVmstatTree(t, map[string]string{
		"uptime": "1000.00 0.00\n",
		"stat":   "cpu 1000 0 500 8000 500 0 0 0 0 0\nintr 1000\nctxt 2000\nprocs_running 3\nprocs_blocked 0\n",
		"meminfo": "MemTotal: 8000 kB\nMemFree: 1000 kB\nBuffers: 100 kB\nCached: 200 kB\n" +
			"SReclaimable: 0 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\nActive: 10 kB\nInactive: 20 kB\n",
		"vmstat": "pgpgin 100\npgpgout 200\npswpin 10\npswpout 20\n",
	})
}

// The second line is the delta since the first, divided by the delay; the first
// line is since boot, divided by the uptime.
func TestVmstatDeltas(t *testing.T) {
	v := vmstatDeltaTree(t)
	stubSampleSleep(t, func(_ context.Context, _ time.Duration) error {
		vmstatSecond(t, v)
		return nil
	})

	r := vmstatRun(t, "1", "2")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4 (header twice plus two samples):\n%s", len(lines), r.stdout)
	}
	wantFirst := " 2  0       0    1000    100     200    1    1     1     1    1    2 10  5 80  5"
	if lines[2] != wantFirst {
		t.Errorf("first sample = %q, want %q", lines[2], wantFirst)
	}
	wantSecond := " 2  0       0    1000    100     200   80  160   800  1600 1000 1000 10 10 80  0"
	if lines[3] != wantSecond {
		t.Errorf("delta sample = %q, want %q", lines[3], wantSecond)
	}
}

// -n prints the header once, and three samples print one header either way when
// stdout is not a terminal.
func TestVmstatHeaderOnce(t *testing.T) {
	v := vmstatDeltaTree(t)
	stubSampleSleep(t, func(_ context.Context, _ time.Duration) error {
		vmstatSecond(t, v)
		return nil
	})
	for _, args := range [][]string{{"-n", "1", "3"}, {"1", "3"}} {
		r := vmstatRun(t, args...)
		assertExitCode(t, r, 0)
		if got := strings.Count(r.stdout, "procs "); got != 1 {
			t.Errorf("%v: headers = %d, want 1\n%s", args, got, r.stdout)
		}
		if got := strings.Count(r.stdout, "\n"); got != 5 {
			t.Errorf("%v: lines = %d, want 5 (two header lines plus three samples)", args, got)
		}
	}
}

// A cancelled context stops the interval loop with the shell's usual status.
func TestVmstatCancelled(t *testing.T) {
	vmstatDeltaTree(t)
	ctx, cancel := context.WithCancel(context.Background())
	stubSampleSleep(t, func(_ context.Context, _ time.Duration) error {
		cancel()
		return context.Canceled
	})

	r := vmstatRunCtx(t, ctx, "1")
	assertExitCode(t, r, ExitCanceled)
	if got := strings.Count(r.stdout, "\n"); got != 3 {
		t.Errorf("lines = %d, want 3 (header plus one sample)", got)
	}
}

// A counter the kernel does not report is printed as "-", never as a zero.
func TestVmstatMissingCounters(t *testing.T) {
	newVmstatTree(t, map[string]string{
		"uptime":  "1000.00 0.00\n",
		"stat":    "cpu 1000 0 500 8000 500 0 0 0 0 0\nintr 1000\nctxt 2000\nprocs_running 1\nprocs_blocked 0\n",
		"meminfo": "MemTotal: 8000 kB\nMemFree: 1000 kB\nBuffers: 100 kB\nCached: 200 kB\n",
		"vmstat":  "pgpgin 100\npgpgout 200\n",
	})
	r := vmstatRun(t)
	assertExitCode(t, r, 0)
	fields := strings.Fields(strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")[2])
	if len(fields) != 16 {
		t.Fatalf("fields = %v, want 16", fields)
	}
	if fields[6] != "-" || fields[7] != "-" {
		t.Errorf("si/so = %q/%q, want -/-: %v", fields[6], fields[7], fields)
	}
	if fields[8] == "-" || fields[9] == "-" {
		t.Errorf("bi/bo = %q/%q, want numbers: %v", fields[8], fields[9], fields)
	}
}

func TestVmstatUsageErrors(t *testing.T) {
	installVmstatFixture(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"1", "2", "3"}, "extra operand '3'"},
		{[]string{"-1"}, "unknown option"},
		{[]string{"abc"}, "invalid delay 'abc'"},
		{[]string{"1", "0"}, "invalid count '0'"},
		{[]string{"-w"}, "unknown option"},
		{[]string{"-t"}, "unknown option"},
		{[]string{"-d"}, "unknown option"},
		{[]string{"-y"}, "unknown option"},
		{[]string{"--slabs"}, "unknown option"},
	}
	for _, c := range cases {
		r := vmstatRun(t, c.args...)
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, c.want)
		if r.stdout != "" {
			t.Errorf("%v: stdout = %q, want none", c.args, r.stdout)
		}
	}
}

func TestVmstatMissingFiles(t *testing.T) {
	cases := map[string]string{
		"the system uptime":         "uptime",
		"cpu statistics":            "stat",
		"memory information":        "meminfo",
		"virtual memory statistics": "vmstat",
	}
	for what, missing := range cases {
		files := map[string]string{
			"uptime":  "1000.00 0.00\n",
			"stat":    "cpu 1000 0 500 8000 500 0 0 0 0 0\n",
			"meminfo": "MemTotal: 8000 kB\n",
			"vmstat":  "pgpgin 1\npgpgout 2\npswpin 3\npswpout 4\n",
		}
		delete(files, missing)
		newVmstatTree(t, files)
		r := vmstatRun(t)
		assertExitCode(t, r, 1)
		if !strings.Contains(r.stderr, "failed to read "+what) || r.stdout != "" {
			t.Errorf("missing %s: stderr = %q, stdout = %q", missing, r.stderr, r.stdout)
		}
	}
}

func TestVmstatHelpVersion(t *testing.T) {
	r := vmstatRun(t, "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage: vmstat")
	assertStdoutContains(t, r, "--one-header")

	r = vmstatRun(t, "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "vmstat (Misa)")
}

func TestVmstatCeilDiv(t *testing.T) {
	cases := []struct{ a, b, want uint64 }{
		{0, 5, 0}, {1, 5, 1}, {5, 5, 1}, {6, 5, 2}, {10, 0, 0},
	}
	for _, c := range cases {
		if got := vmstatCeilDiv(c.a, c.b); got != c.want {
			t.Errorf("vmstatCeilDiv(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

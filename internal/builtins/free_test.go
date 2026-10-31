package builtins

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// installFreeFixture points free at the recorded proc tree in internal/proc.
func installFreeFixture(t *testing.T) {
	t.Helper()
	old := freeProcRoot
	freeProcRoot = "../proc/testdata/proc"
	t.Cleanup(func() { freeProcRoot = old })
}

// stubSampleSleep replaces the interval sleep so loops run instantly. fn gets
// every requested interval.
func stubSampleSleep(t *testing.T, fn func(ctx context.Context, d time.Duration) error) *[]time.Duration {
	t.Helper()
	var calls []time.Duration
	old := sampleSleep
	sampleSleep = func(ctx context.Context, d time.Duration) error {
		calls = append(calls, d)
		if fn != nil {
			return fn(ctx, d)
		}
		return ctx.Err()
	}
	t.Cleanup(func() { sampleSleep = old })
	return &calls
}

// freeRun runs free with a fresh output buffer pair.
func freeRun(t *testing.T, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("free")
	if !ok {
		t.Fatal("free not registered")
	}
	var stdout, stderr strings.Builder
	code := cmd.Run(&Context{
		Dir:     t.TempDir(),
		Args:    args,
		CmdName: "free",
		Width:   80,
		Height:  24,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   strings.NewReader(""),
	})
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestFreeRegistered(t *testing.T) {
	if _, ok := Lookup("free"); !ok {
		t.Fatal("free not registered")
	}
}

func TestFreeDefault(t *testing.T) {
	installFreeFixture(t)
	r := freeRun(t)
	assertExitCode(t, r, 0)
	want := "" +
		"               total        used        free      shared  buff/cache   available\n" +
		"Mem:         8388608     4194304     1048576       65536     2097152     4194304\n" +
		"Swap:        2097152      524288     1572864\n"
	assertStdout(t, r, want)
}

func TestFreeWide(t *testing.T) {
	installFreeFixture(t)
	r := freeRun(t, "-w")
	assertExitCode(t, r, 0)
	want := "" +
		"               total        used        free      shared     buffers       cache   available\n" +
		"Mem:         8388608     4194304     1048576       65536      262144     1835008     4194304\n" +
		"Swap:        2097152      524288     1572864\n"
	assertStdout(t, r, want)
}

func TestFreeTotal(t *testing.T) {
	installFreeFixture(t)
	r := freeRun(t, "-t")
	assertExitCode(t, r, 0)
	if !strings.HasSuffix(r.stdout,
		"Swap:        2097152      524288     1572864\n"+
			"Total:      10485760     4718592     2621440\n") {
		t.Errorf("stdout = %q, want a Total: row of 10485760/4718592/2621440", r.stdout)
	}
	if got := strings.Count(r.stdout, "\n"); got != 4 {
		t.Errorf("lines = %d, want 4", got)
	}
}

func TestFreeHuman(t *testing.T) {
	installFreeFixture(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-h"}, "" +
			"               total        used        free      shared  buff/cache   available\n" +
			"Mem:           8.0Gi       4.0Gi       1.0Gi        64Mi       2.0Gi       4.0Gi\n" +
			"Swap:          2.0Gi       512Mi       1.5Gi\n"},
		{[]string{"-h", "--si"}, "" +
			"               total        used        free      shared  buff/cache   available\n" +
			"Mem:            8.6G        4.3G        1.1G         67M        2.1G        4.3G\n" +
			"Swap:           2.1G        536M        1.6G\n"},
	}
	for _, c := range cases {
		r := freeRun(t, c.args...)
		assertExitCode(t, r, 0)
		if r.stdout != c.want {
			t.Errorf("free %v stdout = %q, want %q", c.args, r.stdout, c.want)
		}
	}
}

func TestFreeUnits(t *testing.T) {
	installFreeFixture(t)
	// MemTotal is 8388608 KiB = 8589934592 B.
	cases := []struct {
		args     []string
		wantSize string
	}{
		{[]string{"-b"}, "8589934592"},
		{[]string{"-k"}, "8388608"},
		{nil, "8388608"},
		{[]string{"--si"}, "8589934"},
		{[]string{"-m"}, "8192"},
		{[]string{"-g"}, "8"},
	}
	for _, c := range cases {
		r := freeRun(t, c.args...)
		assertExitCode(t, r, 0)
		fields := strings.Fields(strings.Split(r.stdout, "\n")[1])
		if len(fields) < 2 || fields[1] != c.wantSize {
			t.Errorf("free %v total = %v, want %s", c.args, fields, c.wantSize)
		}
	}
}

func TestFreeUnitConflict(t *testing.T) {
	installFreeFixture(t)
	r := freeRun(t, "-k", "-m")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "mutually exclusive")
	if r.stdout != "" {
		t.Errorf("stdout = %q, want none", r.stdout)
	}
}

// Kernels before 3.14 have no MemAvailable: free falls back to the page-cache
// estimate instead of reporting an empty machine.
func TestFreeWithoutMemAvailable(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile("../proc/testdata/meminfo-old.txt")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "meminfo", string(data))
	old := freeProcRoot
	freeProcRoot = dir
	t.Cleanup(func() { freeProcRoot = old })

	r := freeRun(t)
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Mem:         8388608     5308416     1048576       65536     2097152     3080192\n")
}

func TestFreeMissingMeminfo(t *testing.T) {
	old := freeProcRoot
	freeProcRoot = t.TempDir()
	t.Cleanup(func() { freeProcRoot = old })

	r := freeRun(t)
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "free: failed to read memory information:")
	if r.stdout != "" {
		t.Errorf("stdout = %q, want none", r.stdout)
	}
}

// -c samples the memory -c times, sleeping -s between samples only, and
// separates the samples with a blank line.
func TestFreeCountAndInterval(t *testing.T) {
	installFreeFixture(t)
	calls := stubSampleSleep(t, nil)

	r := freeRun(t, "-c", "3", "-s", "2")
	assertExitCode(t, r, 0)
	if got := strings.Count(r.stdout, "Mem:"); got != 3 {
		t.Errorf("samples = %d, want 3 (output %q)", got, r.stdout)
	}
	if got := strings.Count(r.stdout, "\n\n"); got != 2 {
		t.Errorf("blank separators = %d, want 2 (output %q)", got, r.stdout)
	}
	if len(*calls) != 2 {
		t.Fatalf("sleeps = %d, want 2", len(*calls))
	}
	for _, d := range *calls {
		if d != 2*time.Second {
			t.Errorf("slept %v, want 2s", d)
		}
	}
}

// -c alone repeats once a second, as in the reference.
func TestFreeCountDefaultsToOneSecond(t *testing.T) {
	installFreeFixture(t)
	calls := stubSampleSleep(t, nil)
	r := freeRun(t, "-c", "2")
	assertExitCode(t, r, 0)
	if len(*calls) != 1 || (*calls)[0] != time.Second {
		t.Fatalf("sleeps = %v, want [1s]", *calls)
	}
}

// -s without -c repeats until the context is cancelled, and the cancellation
// status is the shell's usual 130.
func TestFreeIntervalUntilCancelled(t *testing.T) {
	installFreeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	stubSampleSleep(t, func(_ context.Context, _ time.Duration) error {
		cancel()
		return context.Canceled
	})

	cmd, ok := Lookup("free")
	if !ok {
		t.Fatal("free not registered")
	}
	var stdout, stderr strings.Builder
	code := cmd.Run(&Context{
		Dir:    t.TempDir(),
		Args:   []string{"-s", "1"},
		Ctx:    ctx,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	})
	if code != ExitCanceled {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitCanceled, stderr.String())
	}
	if got := strings.Count(stdout.String(), "Mem:"); got != 1 {
		t.Errorf("samples = %d, want 1", got)
	}
}

func TestFreeUsageErrors(t *testing.T) {
	installFreeFixture(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-c", "0"}, "count argument must be greater than 0"},
		{[]string{"-c", "-1"}, "count argument must be greater than 0"},
		{[]string{"-s", "0"}, "seconds argument must be greater than 0"},
		{[]string{"-s", "soon"}, "invalid seconds value"},
		{[]string{"--kilo"}, "unknown option"},
		{[]string{"-L"}, "unknown option"},
		{[]string{"-l"}, "unknown option"},
		{[]string{"-v"}, "unknown option"},
		{[]string{"--definitely-invalid"}, "unknown option"},
		{[]string{"extra"}, "extra operand 'extra'"},
	}
	for _, c := range cases {
		r := freeRun(t, c.args...)
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, c.want)
		if r.stdout != "" {
			t.Errorf("%v: stdout = %q, want none", c.args, r.stdout)
		}
	}
}

func TestFreeHelpVersion(t *testing.T) {
	r := freeRun(t, "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage: free")
	assertStdoutContains(t, r, "-h, --human")
	assertStdoutContains(t, r, "--si")

	r = freeRun(t, "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "free (Misa)")
}

// The human-readable cases are the ones the reference's own test suite pins.
func TestFreeHumanize(t *testing.T) {
	cases := []struct {
		kib  uint64
		si   bool
		want string
	}{
		{0, false, "0B"},
		{0, true, "0B"},
		{99, false, "99Ki"},
		{100, false, "100Ki"},
		{999, false, "999Ki"},
		{1000, false, "1.0Mi"},
		{1023, false, "1.0Mi"},
		{1024, true, "1.0M"},
		{1024, false, "1.0Mi"},
		{1536, true, "1.6M"},
		{1536, false, "1.5Mi"},
		{8500, true, "8.7M"},
		{8500, false, "8.3Mi"},
		{10138, false, "9.9Mi"},
		{10230, false, "9Mi"},
		{976562, true, "999M"},
		{976563, true, "1.0G"},
		{1023999, false, "999Mi"},
		{1024000, false, "1.0Gi"},
	}
	for _, c := range cases {
		if got := freeHumanize(c.kib, c.si); got != c.want {
			t.Errorf("freeHumanize(%d, si=%v) = %q, want %q", c.kib, c.si, got, c.want)
		}
	}
}

func TestFreeSubSat(t *testing.T) {
	cases := []struct{ a, b, want uint64 }{
		{5, 3, 2}, {3, 5, 0}, {0, 0, 0}, {7, 7, 0},
	}
	for _, c := range cases {
		if got := subSat(c.a, c.b); got != c.want {
			t.Errorf("subSat(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

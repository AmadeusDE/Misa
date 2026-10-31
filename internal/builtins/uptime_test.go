package builtins

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// installUptimeFixture points uptime at the recorded proc tree in internal/proc
// and pins the clock and the time zone so output is deterministic.
func installUptimeFixture(t *testing.T) {
	t.Helper()
	oldRoot, oldNow, oldLocal := uptimeProcRoot, uptimeNow, time.Local
	uptimeProcRoot = "../proc/testdata/proc"
	uptimeNow = func() time.Time { return time.Unix(1700000000, 0) }
	time.Local = time.UTC
	t.Cleanup(func() {
		uptimeProcRoot, uptimeNow, time.Local = oldRoot, oldNow, oldLocal
	})
}

// uptimeTree writes a minimal procfs tree and points uptime at it. Up is the
// /proc/uptime content; files holds any extra procfs files.
func uptimeTree(t *testing.T, up string, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "uptime", up)
	for name, content := range files {
		writeFile(t, dir, name, content)
	}
	old := uptimeProcRoot
	uptimeProcRoot = dir
	t.Cleanup(func() { uptimeProcRoot = old })
}

func uptimeRun(t *testing.T, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("uptime")
	if !ok {
		t.Fatal("uptime not registered")
	}
	var stdout, stderr strings.Builder
	code := cmd.Run(&Context{
		Dir:     t.TempDir(),
		Args:    args,
		CmdName: "uptime",
		Width:   80,
		Height:  24,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   strings.NewReader(""),
	})
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestUptimeRegistered(t *testing.T) {
	if _, ok := Lookup("uptime"); !ok {
		t.Fatal("uptime not registered")
	}
}

func TestUptimeDefault(t *testing.T) {
	installUptimeFixture(t)
	r := uptimeRun(t)
	assertExitCode(t, r, 0)
	assertStdout(t, r, " 22:13:20 up  3:25,  load average: 0.10, 0.20, 0.30\n")
}

func TestUptimeDuration(t *testing.T) {
	cases := []struct {
		seconds float64
		want    string
	}{
		{900000, "10 days, 10:00, "}, // 10 days and 10 hours
		{7265, " 2:01, "},            // 2 hours 1 minute
		{120, "2 min, "},
		{30, "0 min, "},
		{86400, "1 day, 0 min, "}, // exactly one day
	}
	for _, c := range cases {
		installUptimeFixture(t)
		uptimeTree(t, strconvFloat(c.seconds), map[string]string{"loadavg": "0.10 0.20 0.30 1/200 999\n"})
		r := uptimeRun(t)
		assertExitCode(t, r, 0)
		want := " 22:13:20 up " + c.want + " load average: 0.10, 0.20, 0.30\n"
		if r.stdout != want {
			t.Errorf("%.0fs: stdout = %q, want %q", c.seconds, r.stdout, want)
		}
	}
}

func TestUptimePretty(t *testing.T) {
	cases := []struct {
		seconds float64
		want    string
	}{
		{900000, "up 10 days, 10 hours\n"},
		{7265, "up 2 hours, 1 minute\n"},
		{120, "up 2 minutes\n"},
		{30, "up 0 minutes\n"},
		{604800*2 + 3600, "up 14 days, 1 hour\n"},
		{86400*7 + 3600 + 61, "up 7 days, 1 hour, 1 minute\n"},
	}
	for _, c := range cases {
		uptimeTree(t, strconvFloat(c.seconds), nil)
		r := uptimeRun(t, "-p")
		assertExitCode(t, r, 0)
		if r.stdout != c.want {
			t.Errorf("%.0fs: stdout = %q, want %q", c.seconds, r.stdout, c.want)
		}
	}
}

// -s prefers the kernel's btime, which is the authoritative boot time.
func TestUptimeSinceBootTime(t *testing.T) {
	installUptimeFixture(t)
	r := uptimeRun(t, "-s")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "2023-11-14 19:26:40\n")
}

// Without a readable /proc/stat, -s falls back to now minus the uptime, which is
// what the references do.
func TestUptimeSinceFallback(t *testing.T) {
	installUptimeFixture(t)
	uptimeTree(t, "12345.67 0.00\n", nil)
	r := uptimeRun(t, "--since")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "2023-11-14 18:47:34\n")
}

// No readable /proc/loadavg means no load-average field: the line ends after
// the duration rather than showing invented numbers.
func TestUptimeWithoutLoadavg(t *testing.T) {
	installUptimeFixture(t)
	uptimeTree(t, "12345.67 0.00\n", nil)
	r := uptimeRun(t)
	assertExitCode(t, r, 0)
	assertStdout(t, r, " 22:13:20 up  3:25\n")
}

func TestUptimeLoadavgFormatting(t *testing.T) {
	uptimeTree(t, "120.00 0.00\n", map[string]string{"loadavg": "12.5 0.75 0.005 3/100 42\n"})
	r := uptimeRun(t)
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, " load average: 12.50, 0.75, 0.01\n")
}

func TestUptimeMissingUptime(t *testing.T) {
	dir := t.TempDir()
	old := uptimeProcRoot
	uptimeProcRoot = dir
	t.Cleanup(func() { uptimeProcRoot = old })

	r := uptimeRun(t)
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "uptime: failed to read the system uptime:")
	if r.stdout != "" {
		t.Errorf("stdout = %q, want none", r.stdout)
	}
}

func TestUptimeUsageErrors(t *testing.T) {
	installUptimeFixture(t)
	for _, args := range [][]string{{"extra"}, {"-x"}, {"--pretty=1"}} {
		r := uptimeRun(t, args...)
		assertExitCode(t, r, 1)
		if r.stdout != "" {
			t.Errorf("%v: stdout = %q, want none", args, r.stdout)
		}
	}
	r := uptimeRun(t, "extra")
	assertStderrContains(t, r, "extra operand 'extra'")
}

func TestUptimeHelpVersion(t *testing.T) {
	r := uptimeRun(t, "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage: uptime")
	assertStdoutContains(t, r, "-p, --pretty")

	r = uptimeRun(t, "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "uptime (Misa)")
}

// strconvFloat renders a /proc/uptime line for the given uptime in seconds.
func strconvFloat(secs float64) string {
	return strconv.FormatFloat(secs, 'f', 2, 64) + " 0.00\n"
}

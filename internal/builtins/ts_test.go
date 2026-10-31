// Behaviour adapted from toybox ts.c (0BSD), toys/other/ts.c at commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb. toybox has no ts.test.
package builtins

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// fakeTsClock returns the given times in order and repeats the last one.
func fakeTsClock(t *testing.T, times ...time.Time) {
	t.Helper()
	i := 0
	orig := tsNow
	tsNow = func() time.Time {
		if i >= len(times) {
			return times[len(times)-1]
		}
		v := times[i]
		i++
		return v
	}
	t.Cleanup(func() { tsNow = orig })
}

func runTsStdin(t *testing.T, stdin string, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("ts")
	if !ok {
		t.Fatal("ts not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:     t.TempDir(),
		Args:    args,
		CmdName: "ts",
		Width:   80,
		Height:  24,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   strings.NewReader(stdin),
		Env:     []string{"TZ=UTC"},
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

var tsTestTime = time.Date(2024, time.March, 5, 14, 7, 9, 123e6, time.UTC)

func TestTsDefaultFormat(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "hello\n")
	assertExitCode(t, r, 0)
	if got, want := r.stdout, "Mar 05 14:07:09 hello\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsMultipleLines(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "a\nb\n")
	assertExitCode(t, r, 0)
	want := "Mar 05 14:07:09 a\nMar 05 14:07:09 b\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestTsCustomFormat(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "x\n", "%Y-%m-%d %H:%M:%S")
	assertExitCode(t, r, 0)
	if got, want := r.stdout, "2024-03-05 14:07:09 x\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsEmptyFormat(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "x\n", "")
	assertExitCode(t, r, 0)
	if got, want := r.stdout, " x\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsEmptyInput(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "")
	assertExitCode(t, r, 0)
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestTsNoTrailingNewline(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "no newline")
	assertExitCode(t, r, 0)
	if got, want := r.stdout, "Mar 05 14:07:09 no newline"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsNoTrailingNewlineKeepsBlankLine(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "one\n\n")
	assertExitCode(t, r, 0)
	want := "Mar 05 14:07:09 one\nMar 05 14:07:09 \n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestTsSinceStart(t *testing.T) {
	start := time.Date(2024, time.March, 5, 14, 7, 9, 0, time.UTC)
	fakeTsClock(t, start, start.Add(1500*time.Millisecond), start.Add(2*time.Second))
	r := runTsStdin(t, "a\nb\n", "-s")
	assertExitCode(t, r, 0)
	want := "00:00:01 a\n00:00:02 b\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestTsIncremental(t *testing.T) {
	start := time.Date(2024, time.March, 5, 14, 7, 9, 0, time.UTC)
	fakeTsClock(t, start, start.Add(1500*time.Millisecond), start.Add(4*time.Second))
	r := runTsStdin(t, "a\nb\n", "-i")
	assertExitCode(t, r, 0)
	want := "00:00:01 a\n00:00:02 b\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestTsMilliseconds(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "a\n", "-m")
	assertExitCode(t, r, 0)
	if got, want := r.stdout, "14:07:09.123 a\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsMillisecondsPad(t *testing.T) {
	fakeTsClock(t, time.Date(2024, time.March, 5, 14, 7, 9, 5e6, time.UTC))
	r := runTsStdin(t, "a\n", "-m")
	assertExitCode(t, r, 0)
	if got, want := r.stdout, "14:07:09.005 a\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsMillisecondsIncremental(t *testing.T) {
	start := time.Date(2024, time.March, 5, 14, 7, 9, 0, time.UTC)
	fakeTsClock(t, start, start.Add(1250*time.Millisecond), start.Add(1250*time.Millisecond))
	r := runTsStdin(t, "a\nb\n", "-i", "-m")
	assertExitCode(t, r, 0)
	want := "00:00:01.250 a\n00:00:00.000 b\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestTsMillisecondsWithFormat(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "a\n", "-m", "%H:%M")
	assertExitCode(t, r, 0)
	if got, want := r.stdout, "14:07.123 a\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsSinceStartUsesUTC(t *testing.T) {
	// Elapsed times ignore the local zone: TZ=UTC-5 (UTC+5) must not shift them.
	start := time.Date(2024, time.March, 5, 14, 7, 9, 0, time.UTC)
	fakeTsClock(t, start, start.Add(3*time.Second))
	cmd, _ := Lookup("ts")
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:    t.TempDir(),
		Args:   []string{"-s"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader("a\n"),
		Env:    []string{"TZ=UTC-5"},
	}
	if code := cmd.Run(ctx); code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got, want := stdout.String(), "00:00:03 a\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsTimezoneFromEnv(t *testing.T) {
	// POSIX TZ=UTC-5 is five hours east of UTC, so 14:07:09 UTC is 19:07:09.
	fakeTsClock(t, tsTestTime)
	cmd, _ := Lookup("ts")
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:    t.TempDir(),
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader("a\n"),
		Env:    []string{"TZ=UTC-5"},
	}
	if code := cmd.Run(ctx); code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got, want := stdout.String(), "Mar 05 19:07:09 a\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsLongLine(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	long := strings.Repeat("x", 70000)
	r := runTsStdin(t, long+"\n"+long)
	assertExitCode(t, r, 0)
	want := "Mar 05 14:07:09 " + long + "\n" + "Mar 05 14:07:09 " + long
	if r.stdout != want {
		t.Errorf("stdout length = %d, want %d (prefix %q)", len(r.stdout), len(want), r.stdout[:24])
	}
}

func TestTsKeepsCarriageReturns(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "a\r\n")
	assertExitCode(t, r, 0)
	if got, want := r.stdout, "Mar 05 14:07:09 a\r\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestTsUnknownOption(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "a\n", "-x")
	assertExitCode(t, r, 1)
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if !strings.Contains(r.stderr, "unknown option") {
		t.Errorf("stderr = %q, want an unknown option message", r.stderr)
	}
}

func TestTsExtraOperand(t *testing.T) {
	fakeTsClock(t, tsTestTime)
	r := runTsStdin(t, "a\n", "%H", "%M")
	assertExitCode(t, r, 1)
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if !strings.Contains(r.stderr, "extra operand") {
		t.Errorf("stderr = %q, want an extra operand message", r.stderr)
	}
}

func TestTsHelp(t *testing.T) {
	cmd, _ := Lookup("ts")
	for _, arg := range []string{"--help", "--version"} {
		var stdout, stderr bytes.Buffer
		ctx := &Context{
			Dir:     t.TempDir(),
			CmdName: "ts",
			Args:    []string{arg},
			Stdout:  &stdout,
			Stderr:  &stderr,
			Stdin:   strings.NewReader("a\n"),
			Env:     []string{"TZ=UTC"},
		}
		if code := cmd.Run(ctx); code != 0 {
			t.Fatalf("%s: exit code = %d, stderr = %s", arg, code, stderr.String())
		}
		if arg == "--help" && !strings.Contains(stdout.String(), "Usage: ts") {
			t.Errorf("--help stdout = %q, want a usage line", stdout.String())
		}
		if arg == "--version" && !strings.Contains(stdout.String(), "Misa") {
			t.Errorf("--version stdout = %q, want a Misa version line", stdout.String())
		}
		if stdout.Len() > 0 && strings.Contains(stdout.String(), " a\n") {
			t.Errorf("%s wrote timestamps: %q", arg, stdout.String())
		}
	}
}

func TestTsMillis(t *testing.T) {
	if got, want := tsMillis(tsTestTime), tsTestTime.UnixMilli(); got != want {
		t.Errorf("tsMillis = %d, want %d", got, want)
	}
}

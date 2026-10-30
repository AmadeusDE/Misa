// Behaviour adapted from toybox toys/other/count.c (0BSD, commit 6a0c0ef).
// The expected progress strings below were produced with a toybox binary built
// from that commit (count -l), then replayed through the same clock script this
// file's scenario runner uses.
package builtins

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"
)

// countOpKind is one entry of a count scenario: a display point, a chunk of
// copied data, or the closing update.
type countOpKind int

const (
	countStep countOpKind = iota
	countData
	countFinal
)

type countOp struct {
	kind countOpKind
	n    uint64 // milliseconds for steps, bytes for data
}

func step(ms uint64) countOp    { return countOp{countStep, ms} }
func data(bytes uint64) countOp { return countOp{countData, bytes} }
func final(ms uint64) countOp   { return countOp{countFinal, ms} }

// runCountScenario replays a clock script through countProgress exactly the way
// countStream drives it: display when a period is due, then retire the period.
func runCountScenario(long bool, ops []countOp) string {
	p := newCountProgress(long, ops[0].n)
	var b strings.Builder
	for _, op := range ops {
		switch op.kind {
		case countStep:
			if op.n >= p.then {
				b.WriteString(p.line(op.n))
				p.advance(op.n)
			}
		case countData:
			p.add(op.n)
		case countFinal:
			b.WriteString(p.line(op.n) + "\n")
		}
	}
	return b.String()
}

func TestCountProgressLines(t *testing.T) {
	tests := []struct {
		name string
		long bool
		ops  []countOp
		want string
	}{
		{
			name: "fast stream short",
			ops:  []countOp{step(0), data(5), final(0)},
			want: "0 bytes   \r5 bytes   \r\n",
		},
		{
			name: "fast stream long",
			long: true,
			ops:  []countOp{step(0), data(5), final(0)},
			want: "0 bytes, 0b, 0b/s, 0m00s   \r5 bytes, 5b, 20b/s, 0m00s   \r\n",
		},
		{
			name: "empty input short",
			ops:  []countOp{step(0), final(0)},
			want: "0 bytes   \r0 bytes   \r\n",
		},
		{
			name: "empty input long",
			long: true,
			ops:  []countOp{step(0), final(0)},
			want: "0 bytes, 0b, 0b/s, 0m00s   \r0 bytes, 0b, 0b/s, 0m00s   \r\n",
		},
		{
			name: "idle periods then data",
			long: true,
			ops:  []countOp{step(0), step(250), step(500), data(1000), step(500), final(500)},
			want: "0 bytes, 0b, 0b/s, 0m00s   \r" +
				"0 bytes, 0b, 0b/s, 0m00s   \r" +
				"0 bytes, 0b, 0b/s, 0m00s   \r" +
				"1000 bytes, 0.9Kb, 1.9Kb/s, 0m00s   \r\n",
		},
		{
			name: "streaming long",
			long: true,
			ops:  []countOp{step(0), data(100000), step(260), data(50000), step(520), final(520)},
			want: "0 bytes, 0b, 0b/s, 0m00s   \r" +
				"100000 bytes, 98Kb, 391Kb/s, 0m00s   \r" +
				"150000 bytes, 146Kb, 293Kb/s, 0m00s   \r" +
				"150000 bytes, 146Kb, 282Kb/s, 0m00s   \r\n",
		},
		{
			name: "streaming short",
			ops:  []countOp{step(0), data(100000), step(260), data(50000), step(520), final(520)},
			want: "0 bytes   \r100000 bytes   \r150000 bytes   \r150000 bytes   \r\n",
		},
		{
			name: "elapsed minutes and seconds",
			long: true,
			ops:  []countOp{step(0), data(700000), step(4000), data(300000), step(12300), final(12300)},
			want: "0 bytes, 0b, 0b/s, 0m00s   \r" +
				"700000 bytes, 684Kb, 2.6Mb/s, 0m04s   \r" +
				"1000000 bytes, 977Kb, 230Kb/s, 0m12s   \r" +
				"1000000 bytes, 977Kb, 79Kb/s, 0m12s   \r\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := runCountScenario(tc.long, tc.ops); got != tc.want {
				t.Errorf("progress output =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// The 64-slice ring wraps after ~16 seconds; these lines are spot checks from
// the toybox run of 70 periods of 1000 bytes each.
func TestCountProgressRingWrap(t *testing.T) {
	ops := []countOp{step(0)}
	for i := 1; i <= 70; i++ {
		ops = append(ops, data(1000), step(uint64(i)*250))
	}
	ops = append(ops, final(70*250))
	got := strings.Split(runCountScenario(true, ops), "\r")
	if len(got) != 73 {
		t.Fatalf("got %d updates, want 73", len(got))
	}
	want := map[int]string{
		0:  "0 bytes, 0b, 0b/s, 0m00s   ",
		1:  "1000 bytes, 0.9Kb, 3.9Kb/s, 0m00s   ",
		2:  "2000 bytes, 1.9Kb, 3.9Kb/s, 0m00s   ",
		63: "63000 bytes, 62Kb, 3.9Kb/s, 0m15s   ",
		64: "64000 bytes, 63Kb, 3.9Kb/s, 0m16s   ",
		65: "65000 bytes, 63Kb, 3.9Kb/s, 0m16s   ",
		71: "70000 bytes, 68Kb, 3.9Kb/s, 0m17s   ",
	}
	for i, expected := range want {
		if got[i] != expected {
			t.Errorf("update %d = %q, want %q", i, got[i], expected)
		}
	}
}

// Values from toybox's human_readable(buf, num, 0) via count -l.
func TestToyboxHumanReadable(t *testing.T) {
	tests := []struct {
		n    uint64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{5, "5"},
		{999, "999"},
		{1023, "0.9K"},
		{1024, "1.0K"},
		{1500, "1.4K"},
		{10240, "10K"},
		{200000, "195K"},
		{1048575, "1.0M"},
		{1048576, "1.0M"},
		{1536000, "1.4M"},
		{1000000000, "954M"},
		{1073741824, "1.0G"},
		{1<<64 - 1, "18014398509481983.9K"}, // uint64 wrap, as in toybox
	}
	for _, tc := range tests {
		if got := toyboxHumanReadable(tc.n); got != tc.want {
			t.Errorf("toyboxHumanReadable(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// countTestClock is a settable clock; the builtin reads it from another
// goroutine, hence the mutex.
type countTestClock struct {
	mu  sync.Mutex
	now uint64
}

func (c *countTestClock) get() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *countTestClock) set(ms uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = ms
}

// frozenCountClock pins count's clock and disables the periodic ticker. The
// returned clock lets a test move time forward before it sends a tick itself.
func frozenCountClock(t *testing.T, start uint64) (*countTestClock, chan time.Time) {
	t.Helper()
	clock := &countTestClock{now: start}
	ticks := make(chan time.Time)
	oldClock, oldTicker := countClock, countTicker
	countClock = clock.get
	countTicker = func() (<-chan time.Time, func()) { return ticks, func() {} }
	t.Cleanup(func() { countClock, countTicker = oldClock, oldTicker })
	return clock, ticks
}

func TestCountCopiesStdinToStdout(t *testing.T) {
	frozenCountClock(t, 1000)
	// Uncompressible-ish payload with every byte value in it.
	payload := make([]byte, 200000)
	for i := range payload {
		payload[i] = byte(i*7 + i/251)
	}
	cmd, _ := Lookup("count")
	var stdout, stderr bytes.Buffer
	code := cmd.Run(&Context{
		Dir:    t.TempDir(),
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  bytes.NewReader(payload),
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr %q)", code, stderr.String())
	}
	if !bytes.Equal(stdout.Bytes(), payload) {
		t.Errorf("stdout differs from stdin (%d vs %d bytes)", stdout.Len(), len(payload))
	}
	if want := "0 bytes   \r200000 bytes   \r\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestCountLongOutput(t *testing.T) {
	frozenCountClock(t, 0)
	r := runBuiltinStdin(t, "count", "hello", "-l")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello")
	if want := "0 bytes, 0b, 0b/s, 0m00s   \r5 bytes, 5b, 20b/s, 0m00s   \r\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

// A ticker update while stdin is idle goes to stderr only; stdout keeps just
// the copied bytes.
func TestCountTicksWhileIdle(t *testing.T) {
	clock, ticks := frozenCountClock(t, 1000)

	stdinR, stdinW := io.Pipe()
	cmd, _ := Lookup("count")
	stderr := &countLineWriter{lines: make(chan string, 8)}
	var stdout bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- cmd.Run(&Context{
			Dir:    t.TempDir(),
			Stdout: &stdout,
			Stderr: stderr,
			Stdin:  stdinR,
		})
	}()

	// The first update is written before the loop starts, so the next thing the
	// command waits for is either data or a tick.
	if got := <-stderr.lines; got != "0 bytes   \r" {
		t.Fatalf("first update = %q", got)
	}
	clock.set(1250)
	ticks <- time.Unix(0, 0)
	if got := <-stderr.lines; got != "0 bytes   \r" {
		t.Fatalf("tick update = %q", got)
	}
	if _, err := io.WriteString(stdinW, "hello"); err != nil {
		t.Fatal(err)
	}
	stdinW.Close()

	if got := <-stderr.lines; got != "5 bytes   \r\n" {
		t.Fatalf("final update = %q", got)
	}
	if code := <-done; code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stdout.String() != "hello" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "hello")
	}
}

// countLineWriter hands each stderr write to the test goroutine.
type countLineWriter struct {
	lines chan string
}

func (w *countLineWriter) Write(p []byte) (int, error) {
	w.lines <- string(p)
	return len(p), nil
}

func TestCountRejectsOperands(t *testing.T) {
	frozenCountClock(t, 0)
	r := runBuiltinStdin(t, "count", "x", "file")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "count: unexpected argument \"file\"")
}

func TestCountRejectsUnknownOption(t *testing.T) {
	frozenCountClock(t, 0)
	r := runBuiltinStdin(t, "count", "x", "-z")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "unknown option -z")
}

func TestCountHelp(t *testing.T) {
	r := runBuiltin(t, "count", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "-l, --long")
}

func TestCountWriteError(t *testing.T) {
	frozenCountClock(t, 0)
	cmd, _ := Lookup("count")
	var stderr bytes.Buffer
	code := cmd.Run(&Context{
		Dir:    t.TempDir(),
		Stdout: brokenWriter{},
		Stderr: &stderr,
		Stdin:  strings.NewReader("hello"),
	})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	assertContains(t, stderr.String(), "write error")
}

// A stdin that fails mid-way still reports the bytes it got, then fails.
func TestCountReadError(t *testing.T) {
	frozenCountClock(t, 0)
	cmd, _ := Lookup("count")
	var stdout, stderr bytes.Buffer
	code := cmd.Run(&Context{
		Dir:    t.TempDir(),
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  iotest.ErrReader(errors.New("read failed")),
	})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	want := "0 bytes   \r0 bytes   \r\ncount: read failed\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestCountNoStdin(t *testing.T) {
	cmd, _ := Lookup("count")
	var stdout, stderr bytes.Buffer
	code := cmd.Run(&Context{Dir: t.TempDir(), Stdout: &stdout, Stderr: &stderr})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	assertContains(t, stderr.String(), "stdin not available")
}

type brokenWriter struct{}

func (brokenWriter) Write(p []byte) (int, error) { return 0, io.ErrClosedPipe }

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("%q does not contain %q", got, want)
	}
}

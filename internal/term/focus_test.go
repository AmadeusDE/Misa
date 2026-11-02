package term

import (
	"bufio"
	"os"
	"sync"
	"testing"
	"time"
)

const (
	focusIn  = "\x1b[I"
	focusOut = "\x1b[O"
)

// drainReplies reads the answers already written to the PTY side, so a test
// can inspect a burst of replies as one stream.
func drainReplies(t *testing.T, r *os.File, br *bufio.Reader) string {
	t.Helper()
	if err := r.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var out []byte
	buf := make([]byte, 4096)
	for {
		n, err := br.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			break
		}
	}
	if err := r.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func expectQuiet(t *testing.T, r *os.File, br *bufio.Reader) {
	t.Helper()
	if got := drainReplies(t, r, br); got != "" {
		t.Fatalf("reply = %q, want none", got)
	}
}

// waitRepliesWritten blocks until the reply queue has reached the PTY side, so
// the test's own reads see the whole stream.
func waitRepliesWritten(t *testing.T, tm *Terminal) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		tm.input.mu.Lock()
		idle := len(tm.input.pending) == 0 && tm.input.pendingReplies == 0
		tm.input.mu.Unlock()
		if idle {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("replies never reached the PTY")
		}
		time.Sleep(time.Millisecond)
	}
}

// wantWholeReports checks that a reply stream holds whole focus reports and
// nothing else. A program that re-asks for reports may hear the same state
// twice, so repeats are allowed.
func wantWholeReports(t *testing.T, stream string) {
	t.Helper()
	if len(stream)%len(focusIn) != 0 {
		t.Fatalf("reports = %q, want whole focus reports", stream)
	}
	for i := 0; i < len(stream); i += len(focusIn) {
		if got := stream[i : i+len(focusIn)]; got != focusIn && got != focusOut {
			t.Fatalf("reports = %q, want only focus reports", stream)
		}
	}
}

// wantAlternating checks that no state repeats in a stream, which holds while
// the mode stays on and only transitions are reported.
func wantAlternating(t *testing.T, stream string) {
	t.Helper()
	for i := len(focusIn); i < len(stream); i += len(focusIn) {
		if stream[i-len(focusIn):i] == stream[i:i+len(focusIn)] {
			t.Fatalf("reports = %q, want no repeated state", stream)
		}
	}
}

func TestFocusReportsFollowTheProgramsMode(t *testing.T) {
	tm := New(3, 20)
	r, br := newSilentPipe(t, tm)

	// Before the program asks, the host's focus stays private to the terminal.
	tm.SetFocused(true)
	expectQuiet(t, r, br)

	tm.ProcessOutput([]byte("\x1b[?1004h"))
	readReply(t, br, focusIn)
	tm.SetFocused(false)
	readReply(t, br, focusOut)
	tm.SetFocused(true)
	readReply(t, br, focusIn)
}

// A host that never reports its focus leaves the program without reports: the
// terminal does not invent an initial focused state.
func TestNoFocusReportWithoutHostState(t *testing.T) {
	tm := New(3, 20)
	r, br := newSilentPipe(t, tm)

	tm.ProcessOutput([]byte("\x1b[?1004h"))
	expectQuiet(t, r, br)

	tm.SetFocused(true)
	readReply(t, br, focusIn)
}

// A program that enables focus reporting after the host already reported a
// state gets that state once, so a child starting in a hidden pane pauses.
func TestLateFocusEnableReportsCachedState(t *testing.T) {
	for _, focused := range []bool{true, false} {
		tm := New(3, 20)
		tm.SetFocused(focused)
		br := pipeReplies(t, tm)

		tm.ProcessOutput([]byte("\x1b[?1004h"))
		want := focusIn
		if !focused {
			want = focusOut
		}
		readReply(t, br, want)
	}
}

func TestFocusReportsAreDeduplicated(t *testing.T) {
	tm := New(3, 20)
	r, br := newSilentPipe(t, tm)

	tm.ProcessOutput([]byte("\x1b[?1004h"))
	expectQuiet(t, r, br)

	tm.SetFocused(true)
	readReply(t, br, focusIn)
	tm.SetFocused(true)
	expectQuiet(t, r, br)

	tm.SetFocused(false)
	readReply(t, br, focusOut)
	tm.SetFocused(false)
	expectQuiet(t, r, br)
}

// Enabling an already enabled mode is not a transition: the program was told
// the state when it asked the first time.
func TestDuplicateFocusEnableIsQuiet(t *testing.T) {
	tm := New(3, 20)
	r, br := newSilentPipe(t, tm)

	tm.SetFocused(true)
	tm.ProcessOutput([]byte("\x1b[?1004h"))
	readReply(t, br, focusIn)
	tm.ProcessOutput([]byte("\x1b[?1004h"))
	expectQuiet(t, r, br)
}

// Disabling the mode silences later transitions; enabling it again replays the
// cached host state.
func TestFocusDisableSuppressesAndReenableReplays(t *testing.T) {
	tm := New(3, 20)
	r, br := newSilentPipe(t, tm)

	tm.ProcessOutput([]byte("\x1b[?1004h"))
	tm.SetFocused(true)
	readReply(t, br, focusIn)

	tm.ProcessOutput([]byte("\x1b[?1004l"))
	tm.SetFocused(false)
	expectQuiet(t, r, br)

	tm.ProcessOutput([]byte("\x1b[?1004h"))
	readReply(t, br, focusOut)
}

// RIS clears the mode and what the program was told, but the host's focus
// state survives, so a program that enables the mode again is told it once.
func TestFocusResetKeepsHostState(t *testing.T) {
	tm := New(3, 20)
	r, br := newSilentPipe(t, tm)

	tm.ProcessOutput([]byte("\x1b[?1004h"))
	tm.SetFocused(false)
	readReply(t, br, focusOut)

	tm.ProcessOutput([]byte("\x1bc"))
	if tm.FocusReporting() {
		t.Fatal("RIS kept focus reporting enabled")
	}
	tm.SetFocused(true)
	expectQuiet(t, r, br)

	tm.ProcessOutput([]byte("\x1b[?1004h"))
	readReply(t, br, focusIn)
}

func TestFocusModeReportedByDECRQM(t *testing.T) {
	tm := New(3, 20)
	br := pipeReplies(t, tm)

	tm.ProcessOutput([]byte("\x1b[?1004$p"))
	readReply(t, br, "\x1b[?1004;2$y")
	if tm.FocusReporting() {
		t.Fatal("focus reporting on before the program asked")
	}

	tm.ProcessOutput([]byte("\x1b[?1004h\x1b[?1004$p"))
	readReply(t, br, "\x1b[?1004;1$y")
	if !tm.FocusReporting() {
		t.Fatal("focus reporting off after DECSET 1004")
	}
}

// SetFocused without a program is harmless: nothing is queued and a later
// program still gets the cached state.
func TestFocusBeforeProgramStarts(t *testing.T) {
	tm := New(3, 20)
	tm.SetFocused(false)
	if tm.FocusReporting() {
		t.Fatal("focus reporting on without a program")
	}
	br := pipeReplies(t, tm)
	tm.ProcessOutput([]byte("\x1b[?1004h"))
	readReply(t, br, focusOut)
}

// Focus events arrive from the UI goroutine while the parser applies output.
// The state machine stays consistent and every report is whole.
func TestFocusReportingConcurrent(t *testing.T) {
	t.Run("events", func(t *testing.T) {
		tm := New(3, 20)
		r, br := newSilentPipe(t, tm)
		tm.ProcessOutput([]byte("\x1b[?1004h"))
		expectQuiet(t, r, br)

		var wg sync.WaitGroup
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range 50 {
					tm.SetFocused((i+j)%2 == 0)
				}
			}()
		}
		wg.Wait()
		waitRepliesWritten(t, tm)
		stream := drainReplies(t, r, br)
		if stream == "" {
			t.Fatal("no focus report from concurrent transitions")
		}
		wantWholeReports(t, stream)
		wantAlternating(t, stream)
	})

	t.Run("mode", func(t *testing.T) {
		tm := New(3, 20)
		r, br := newSilentPipe(t, tm)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				tm.SetFocused(true)
				tm.SetFocused(false)
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				tm.ProcessOutput([]byte("\x1b[?1004h"))
				tm.ProcessOutput([]byte("\x1b[?1004l"))
			}
		}()
		wg.Wait()
		waitRepliesWritten(t, tm)
		wantWholeReports(t, drainReplies(t, r, br))

		// Whatever the interleaving, the cached host state and the mode still
		// agree on the next report.
		tm.ProcessOutput([]byte("\x1b[?1004l"))
		tm.SetFocused(true)
		tm.ProcessOutput([]byte("\x1b[?1004h"))
		readReply(t, br, focusIn)
		if !tm.FocusReporting() {
			t.Fatal("focus reporting off after DECSET 1004")
		}
	})
}

// Focus reports use the same bounded reply queue as the other answers: a
// program that stopped reading cannot block the host's focus events.
func TestFocusReportsNeverBlock(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	tm := New(4, 10)
	tm.ptm = w // nobody reads r: the pipe fills and the writer blocks

	start := time.Now()
	for i := range 20000 {
		tm.SetFocused(i%2 == 0)
		tm.ProcessOutput([]byte("\x1b[?1004h\x1b[?1004l"))
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("focus reports blocked for %v", d)
	}
	tm.input.mu.Lock()
	pending := tm.input.pendingReplies
	tm.input.mu.Unlock()
	if pending > maxPendingReplies {
		t.Errorf("%d reply bytes queued, want at most %d", pending, maxPendingReplies)
	}
}

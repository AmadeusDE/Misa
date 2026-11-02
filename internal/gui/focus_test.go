package gui

import (
	"runtime"
	"testing"
)

func TestCoreFocusReportingReachesChild(t *testing.T) {
	if runtime.GOOS == "android" {
		t.Skip("PTY input integration runs on Linux; Android exercises pure-Go focus state tests")
	}
	be := newFakeBackend()
	a := newTestApp(t, be)
	tm, got := captureTerm(t, 2, 8)
	a.term = tm
	a.Handle(Event{Kind: EventFocus, Focused: false})
	tm.ProcessOutput([]byte("\x1b[?1004h"))
	a.Handle(Event{Kind: EventFocus, Focused: false})
	a.Handle(Event{Kind: EventFocus, Focused: true})
	a.Handle(Event{Kind: EventFocus, Focused: true})
	a.Handle(Event{Kind: EventFocus, Focused: false})
	tm.ProcessOutput([]byte("\x1b[?1004l"))
	a.Handle(Event{Kind: EventFocus, Focused: true})
	tm.SendBytes([]byte("done"))
	want := "\x1b[O\x1b[I\x1b[Odone"
	waitFor(t, "done", got)
	if s := got(); s != want {
		t.Fatalf("child input %q, want %q", s, want)
	}
}

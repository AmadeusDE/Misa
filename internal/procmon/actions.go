package procmon

import (
	"errors"
	"syscall"
)

// ErrUnsupported is returned by actions the platform does not offer.
var ErrUnsupported = errors.New("not supported on this platform")

type signalEntry struct {
	num  int
	name string
	sig  syscall.Signal
}

// Actions is what the process viewers may do to a process on this platform.
// Signals is the menu the kill dialog offers (entry 0 is always Cancel);
// Levels is the set of priorities the viewers can step through — nil where the
// platform has a continuous priority scale (nice) and the views step by one.
type Actions struct {
	Signals []signalEntry
	Levels  []PriorityLevel
	send    func(pid int, sig syscall.Signal) error
	renice  func(pid, nice int) error
}

// windowsSignals is the whole kill menu on Windows: there are no signals to
// send, only termination.
var windowsSignals = []signalEntry{
	{0, "Cancel", 0},
	{9, "Terminate", syscall.SIGKILL},
}

// CanRenice reports whether priority changes are offered.
func (a Actions) CanRenice() bool { return a.renice != nil }

// Send delivers the menu entry to pid.
func (a Actions) Send(pid int, sig signalEntry) error {
	if a.send == nil {
		return ErrUnsupported
	}
	return a.send(pid, sig.sig)
}

// Renice sets pid's niceness, or the platform's nearest priority class.
func (a Actions) Renice(pid, nice int) error {
	if a.renice == nil {
		return ErrUnsupported
	}
	return a.renice(pid, nice)
}

// PriorityStep is the priority one step up (delta < 0) or down (delta > 0)
// from current: a clamped nice value, or the neighbouring class where the
// platform reports coarse priority levels.
func (a Actions) PriorityStep(current, delta int) int {
	return PriorityStep(a.Levels, current, delta)
}

// PriorityLevel names the level a nice-equivalent priority belongs to.
func (a Actions) PriorityLevel(nice int) (PriorityLevel, bool) {
	return PriorityLevelOf(a.Levels, nice)
}

// defaultSignal is the menu index the kill dialog starts on: SIGTERM, or the
// first real entry where there is no SIGTERM.
func (a Actions) defaultSignal() int {
	for i, s := range a.Signals {
		if s.name == "SIGTERM" {
			return i
		}
	}
	if len(a.Signals) > 1 {
		return 1
	}
	return 0
}

// Terminate asks pid to end, as the default menu entry would.
func (a Actions) Terminate(pid int) error {
	i := a.defaultSignal()
	if i >= len(a.Signals) {
		return ErrUnsupported
	}
	return a.Send(pid, a.Signals[i])
}

package procmon

import (
	"errors"
	"runtime"
	"testing"
)

func TestWindowsSignalMenuOffersOnlyTermination(t *testing.T) {
	if len(windowsSignals) != 2 || windowsSignals[0].sig != 0 || windowsSignals[1].name != "Terminate" {
		t.Fatalf("windows menu = %+v, want Cancel and Terminate", windowsSignals)
	}
	a := Actions{Signals: windowsSignals}
	if a.CanRenice() {
		t.Error("windows must not offer priority changes")
	}
	if got := a.defaultSignal(); got != 1 {
		t.Errorf("default entry = %d, want Terminate at 1", got)
	}
	if err := a.Renice(1, 0); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Renice error = %v, want ErrUnsupported", err)
	}
	if err := a.Send(1, a.Signals[1]); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Send without a sender = %v, want ErrUnsupported", err)
	}
}

func TestPlatformActionsMenuStartsWithCancel(t *testing.T) {
	s := platformActions.Signals
	if len(s) == 0 || s[0].name != "Cancel" || s[0].sig != 0 {
		t.Fatalf("menu must start with Cancel: %+v", s)
	}
	if i := platformActions.defaultSignal(); i <= 0 || i >= len(s) {
		t.Errorf("default entry %d out of range", i)
	}
	switch runtime.GOOS {
	case "windows":
		if !platformActions.CanRenice() || len(s) != 2 || len(platformActions.Levels) != len(windowsPriorityLevels) {
			t.Errorf("windows table = %+v renice=%v levels=%+v", s, platformActions.CanRenice(), platformActions.Levels)
		}
	default:
		if !platformActions.CanRenice() || s[platformActions.defaultSignal()].name != "SIGTERM" || len(s) < 30 {
			t.Errorf("unix table = %d entries, default %q", len(s), s[platformActions.defaultSignal()].name)
		}
	}
}

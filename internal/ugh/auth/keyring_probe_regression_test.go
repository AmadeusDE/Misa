package auth

import (
	"sync/atomic"
	"testing"
	"time"
)

// A Secret Service that D-Bus activates but never answers must not hang
// every credential lookup: the probe gives up, and only once per process.
func TestKeyringProbeGivesUpOnceOnAHungSecretService(t *testing.T) {
	var calls atomic.Int32
	hang := make(chan struct{})
	defer close(hang)
	probe := onceWithin(func() error { calls.Add(1); <-hang; return nil }, 50*time.Millisecond)
	start := time.Now()
	if probe() == nil {
		t.Error("a probe that never answered reported the keyring reachable")
	}
	if probe() == nil {
		t.Error("second probe reported the keyring reachable")
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("two probes took %v", d)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("probed %d times, want once", n)
	}
}

// Tests never read the user's keyring: a test that borrowed real keys passed
// locally and failed everywhere else.
func TestTestsNeverUseTheRealKeyring(t *testing.T) {
	if probeKeyring() == nil {
		t.Error("the keyring probe reached a real Secret Service from a test binary")
	}
}

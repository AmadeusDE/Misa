//go:build linux

package proc

import (
	"errors"
	"os"
	"testing"
)

func TestReaderMeminfoFixture(t *testing.T) {
	m, err := New(fixtureRoot).Meminfo()
	if err != nil {
		t.Fatalf("Meminfo: %v", err)
	}
	if m.MemTotal != 8388608 || m.MemAvailable != 4194304 || m.SwapFree != 1572864 {
		t.Errorf("Meminfo = %+v", m)
	}
	if !m.AvailableKnown || !m.SwapKnown {
		t.Errorf("known flags = %v/%v, want true/true", m.AvailableKnown, m.SwapKnown)
	}
}

func TestReaderLoadavgFixture(t *testing.T) {
	l, err := New(fixtureRoot).Loadavg()
	if err != nil {
		t.Fatalf("Loadavg: %v", err)
	}
	if l.One != 0.10 || l.Five != 0.20 || l.Fifteen != 0.30 || l.Running != 1 || l.Total != 200 {
		t.Errorf("Loadavg = %+v", l)
	}
}

func TestReaderStatFixture(t *testing.T) {
	st, err := New(fixtureRoot).Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if st.CPU.User != 1048576 || st.CPU.Total() != 10518528 || st.Ctxt != 9876543 {
		t.Errorf("Stat = %+v", st)
	}
	if st.BootTime != 1699990000 {
		t.Errorf("BootTime = %d", st.BootTime)
	}
}

func TestReaderVmstatFixture(t *testing.T) {
	v, err := New(fixtureRoot).Vmstat()
	if err != nil {
		t.Fatalf("Vmstat: %v", err)
	}
	if v.PGPGIn != 1111111 || v.PSWPOut != 44 || len(v.Missing) != 0 {
		t.Errorf("Vmstat = %+v", v)
	}
}

func TestReaderUptimeFixture(t *testing.T) {
	up, err := New(fixtureRoot).Uptime()
	if err != nil {
		t.Fatalf("Uptime: %v", err)
	}
	if want := 12345e9 + 670e6; up.Nanoseconds() != int64(want) {
		t.Errorf("Uptime = %v, want %d ns", up, int64(want))
	}
}

// A tree without the files reports the missing file, never a zero snapshot.
func TestReaderMissingFiles(t *testing.T) {
	dir := t.TempDir()
	r := New(dir)
	if _, err := r.Meminfo(); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Meminfo error = %v, want not-exist", err)
	}
	if _, err := r.Loadavg(); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Loadavg error = %v, want not-exist", err)
	}
	if _, err := r.Stat(); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat error = %v, want not-exist", err)
	}
	if _, err := r.Vmstat(); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Vmstat error = %v, want not-exist", err)
	}
}

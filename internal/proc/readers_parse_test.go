package proc

import (
	"strings"
	"testing"
)

func TestParseMeminfoModern(t *testing.T) {
	m, err := ParseMeminfo([]byte(`MemTotal:        8388608 kB
MemFree:         1048576 kB
MemAvailable:    4194304 kB
Buffers:          262144 kB
Cached:          1572864 kB
SwapTotal:       2097152 kB
SwapFree:        1572864 kB
SReclaimable:     262144 kB
Shmem:             65536 kB
Active:          3145728 kB
Inactive:        1310720 kB
CommitLimit:     6291456 kB
Committed_AS:    2621440 kB
HugePages_Total:       0
`))
	if err != nil {
		t.Fatalf("ParseMeminfo: %v", err)
	}
	cases := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"MemTotal", m.MemTotal, 8388608},
		{"MemFree", m.MemFree, 1048576},
		{"MemAvailable", m.MemAvailable, 4194304},
		{"Buffers", m.Buffers, 262144},
		{"Cached", m.Cached, 1572864},
		{"SwapTotal", m.SwapTotal, 2097152},
		{"SwapFree", m.SwapFree, 1572864},
		{"SReclaimable", m.SReclaimable, 262144},
		{"Shmem", m.Shmem, 65536},
		{"Active", m.Active, 3145728},
		{"Inactive", m.Inactive, 1310720},
		{"CommitLimit", m.CommitLimit, 6291456},
		{"CommittedAS", m.CommittedAS, 2621440},
		{"Available", m.Available(), 4194304},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if !m.AvailableKnown || !m.SwapKnown {
		t.Errorf("AvailableKnown=%v SwapKnown=%v, want both true", m.AvailableKnown, m.SwapKnown)
	}
}

// Kernels before 3.14 have no MemAvailable line: free must fall back to the
// page-cache estimate instead of reporting "0 available".
func TestParseMeminfoWithoutMemAvailable(t *testing.T) {
	m, err := ParseMeminfo([]byte(`MemTotal:        8388608 kB
MemFree:         1048576 kB
Buffers:          262144 kB
Cached:          1572864 kB
SReclaimable:     262144 kB
Shmem:             65536 kB
`))
	if err != nil {
		t.Fatalf("ParseMeminfo: %v", err)
	}
	if m.AvailableKnown {
		t.Error("AvailableKnown = true without a MemAvailable line")
	}
	if got, want := m.Available(), uint64(3080192); got != want {
		t.Errorf("Available() = %d, want %d (free+buffers+cached+reclaimable-shmem)", got, want)
	}
}

func TestParseMeminfoFallbackSaturates(t *testing.T) {
	m, err := ParseMeminfo([]byte("MemTotal: 1024 kB\nMemFree: 0 kB\nShmem: 4096 kB\n"))
	if err != nil {
		t.Fatalf("ParseMeminfo: %v", err)
	}
	if got := m.Available(); got != 0 {
		t.Errorf("Available() = %d, want 0", got)
	}
}

func TestParseMeminfoEmpty(t *testing.T) {
	for _, in := range []string{"", "nothing here\n", "MemTotal:\n", "MemTotal: abc kB\n"} {
		if _, err := ParseMeminfo([]byte(in)); err == nil {
			t.Errorf("ParseMeminfo(%q) = nil error, want one", in)
		}
	}
}

func TestParseLoadavg(t *testing.T) {
	cases := []struct {
		in   string
		want Loadavg
	}{
		{"0.10 0.20 0.30 1/200 999\n", Loadavg{One: 0.10, Five: 0.20, Fifteen: 0.30, Running: 1, Total: 200}},
		{"12.50 8.25 3.75\n", Loadavg{One: 12.5, Five: 8.25, Fifteen: 3.75, Running: -1, Total: -1}},
		{"0.00 0.00 0.00 ?/200 1\n", Loadavg{One: 0, Five: 0, Fifteen: 0, Running: -1, Total: -1}},
	}
	for _, c := range cases {
		got, err := ParseLoadavg([]byte(c.in))
		if err != nil {
			t.Errorf("ParseLoadavg(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseLoadavg(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
	for _, in := range []string{"", "0.10 0.20\n", "x 0.20 0.30\n", "-1 0 0\n"} {
		if _, err := ParseLoadavg([]byte(in)); err == nil {
			t.Errorf("ParseLoadavg(%q) = nil error, want one", in)
		}
	}
}

func TestParseStat(t *testing.T) {
	in := `cpu  1048576 262144 524288 8388608 65536 131072 65536 32768 0 0
cpu0 524288 131072 262144 4194304 32768 65536 32768 16384 0 0
intr 12345678 1 2 3
ctxt 9876543
btime 1699990000
processes 4321
procs_running 3
procs_blocked 1
softirq 0 0 0
`
	st, err := ParseStat([]byte(in))
	if err != nil {
		t.Fatalf("ParseStat: %v", err)
	}
	cases := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"CPU.User", st.CPU.User, 1048576},
		{"CPU.Nice", st.CPU.Nice, 262144},
		{"CPU.System", st.CPU.System, 524288},
		{"CPU.Idle", st.CPU.Idle, 8388608},
		{"CPU.Iowait", st.CPU.Iowait, 65536},
		{"CPU.Irq", st.CPU.Irq, 131072},
		{"CPU.Softirq", st.CPU.Softirq, 65536},
		{"CPU.Steal", st.CPU.Steal, 32768},
		{"CPU.Total", st.CPU.Total(), 10518528},
		{"Intr", st.Intr, 12345678},
		{"Ctxt", st.Ctxt, 9876543},
		{"ProcsRunning", st.ProcsRunning, 3},
		{"ProcsBlocked", st.ProcsBlocked, 1},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if st.BootTime != 1699990000 {
		t.Errorf("BootTime = %d, want 1699990000", st.BootTime)
	}

	// A short cpu line (old kernels have no steal/guest columns) still parses.
	old, err := ParseStat([]byte("cpu  1 2 3 4\nintr 5\n"))
	if err != nil {
		t.Fatalf("ParseStat(short): %v", err)
	}
	if old.CPU.Idle != 4 || old.CPU.Iowait != 0 || old.CPU.Total() != 10 || old.BootTime != 0 {
		t.Errorf("short cpu line = %+v", old)
	}

	for _, in := range []string{"", "intr 1\n", "cpu 1 2\n", "cpu a b c d\n"} {
		if _, err := ParseStat([]byte(in)); err == nil {
			t.Errorf("ParseStat(%q) = nil error, want one", in)
		}
	}
}

func TestCPUStatSub(t *testing.T) {
	a := CPUStat{User: 10, Nice: 20, System: 30, Idle: 40, Iowait: 50, Irq: 60, Softirq: 70, Steal: 80}
	b := CPUStat{User: 1, Nice: 2, System: 3, Idle: 4, Iowait: 5, Irq: 6, Softirq: 7, Steal: 8}
	got := a.Sub(b)
	want := CPUStat{User: 9, Nice: 18, System: 27, Idle: 36, Iowait: 45, Irq: 54, Softirq: 63, Steal: 72}
	if got != want {
		t.Errorf("Sub = %+v, want %+v", got, want)
	}
}

func TestParseVmstat(t *testing.T) {
	v, err := ParseVmstat([]byte(`nr_free_pages 262144
pgpgin 1111111
pgpgout 2222222
pswpin 33
pswpout 44
`))
	if err != nil {
		t.Fatalf("ParseVmstat: %v", err)
	}
	if v.PGPGIn != 1111111 || v.PGPGOut != 2222222 || v.PSWPIn != 33 || v.PSWPOut != 44 {
		t.Errorf("vmstat = %+v", v)
	}
	if len(v.Missing) != 0 {
		t.Errorf("Missing = %v, want none", v.Missing)
	}

	// A counter the kernel does not report is named, not silently zeroed.
	partial, err := ParseVmstat([]byte("pgpgin 5\nnr_free_pages 1\n"))
	if err != nil {
		t.Fatalf("ParseVmstat(partial): %v", err)
	}
	if got, want := strings.Join(partial.Missing, ","), "pgpgout,pswpin,pswpout"; got != want {
		t.Errorf("Missing = %q, want %q", got, want)
	}
	if partial.PGPGIn != 5 {
		t.Errorf("PGPGIn = %d, want 5", partial.PGPGIn)
	}

	for _, in := range []string{"", "nr_free_pages 1\n", "pgpgin abc\n"} {
		if _, err := ParseVmstat([]byte(in)); err == nil {
			t.Errorf("ParseVmstat(%q) = nil error, want one", in)
		}
	}
}

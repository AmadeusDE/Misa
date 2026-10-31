//go:build !windows

package procmon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadUptime(t *testing.T) {
	up := ReadUptime()
	if up <= 0 {
		t.Errorf("uptime should be positive, got %f", up)
	}
}

func TestReadLoadAvg(t *testing.T) {
	la := ReadLoadAvg()
	if la[0] < 0 || la[1] < 0 || la[2] < 0 {
		t.Errorf("load averages should be non-negative: %v", la)
	}
}

func TestReadCPUTimes(t *testing.T) {
	all, cores, ok := ReadCPUTimes()
	if !ok {
		t.Fatal("host /proc/stat should be readable here")
	}
	if all.Total() == 0 {
		t.Error("total CPU time should be non-zero")
	}
	if len(cores) == 0 {
		t.Error("should have at least one CPU core")
	}
}

func TestReadMemInfo(t *testing.T) {
	mem := ReadMemInfo()
	if mem.Total == 0 {
		t.Error("total memory should be non-zero")
	}
	if mem.Used() > mem.Total {
		t.Errorf("used memory %d should not exceed total %d", mem.Used(), mem.Total)
	}
}

func TestReadAllProcs(t *testing.T) {
	procs := ReadAllProcs()
	if len(procs) == 0 {
		t.Error("should have at least one process")
	}
	foundSelf := false
	for _, p := range procs {
		if p.Comm != "" && p.PID > 0 {
			foundSelf = true
			break
		}
	}
	if !foundSelf {
		t.Error("should find at least one valid process")
	}
}

func TestCountStates(t *testing.T) {
	procs := ReadAllProcs()
	total, running, sleeping, stopped, zombie := CountStates(procs)
	if total != len(procs) {
		t.Errorf("total %d should equal len(procs) %d", total, len(procs))
	}
	sum := running + sleeping + stopped + zombie
	if sum > total {
		t.Errorf("state counts (%d) should not exceed total (%d)", sum, total)
	}
}

func TestReadNetDev(t *testing.T) {
	devs := ReadNetDev()
	// May be empty in containers, but shouldn't panic
	_ = devs
}

func TestReadDiskStats(t *testing.T) {
	disks := ReadDiskStats()
	_ = disks
}

func TestTakeFullSnapshot(t *testing.T) {
	snap := TakeFullSnapshot()
	if snap.Uptime <= 0 {
		t.Error("snapshot uptime should be positive")
	}
	if snap.Mem.Total == 0 {
		t.Error("snapshot memory total should be non-zero")
	}
	if len(snap.Procs) == 0 {
		t.Error("snapshot should have processes")
	}
}

func TestLookupUser(t *testing.T) {
	name := LookupUser(0)
	if name != "root" && name != "0" {
		t.Errorf("UID 0 should be root, got %q", name)
	}
	name2 := LookupUser(0)
	if name2 != name {
		t.Error("cached lookup should return same value")
	}
}

func fakeStat(pid int, comm string) string {
	return strconv.Itoa(pid) + " (" + comm + ") S 1 1 1 0 -1 0 0 0 0 0 0 0 0 20 0 1 0 100 200 3 0\n"
}

func TestFakeProcFSUnavailableAndMissingPIDs(t *testing.T) {
	dir := t.TempDir()
	old := procRoot
	procRoot = dir
	t.Cleanup(func() { procRoot = old })

	if _, _, ok := ReadCPUTimes(); ok {
		t.Fatal("missing stat was readable")
	}
	snap := TakeSnapshot()
	if snap.CPUAvailable || len(snap.Procs) != 0 {
		t.Fatalf("empty procfs: available=%v procs=%d", snap.CPUAvailable, len(snap.Procs))
	}

	stat := filepath.Join(dir, "stat")
	body := "cpu  10 20 30 40 1 2 3 4\ncpu0 5 10 15 20 0 0 0 0\n"
	if err := os.WriteFile(stat, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	all, cores, ok := ReadCPUTimes()
	if !ok || all.User != 10 || len(cores) != 1 || cores[0].User != 5 {
		t.Fatalf("parsed stat ok=%v all=%+v cores=%+v", ok, all, cores)
	}

	if os.Geteuid() != 0 {
		if err := os.Chmod(stat, 0); err != nil {
			t.Fatal(err)
		}
		if _, _, ok := ReadCPUTimes(); ok {
			t.Fatal("EACCES on stat was treated as available")
		}
		if err := os.Chmod(stat, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Mkdir(filepath.Join(dir, "20"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20", "stat"), []byte(fakeStat(20, "alive")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "21"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "not-a-pid"), 0o755); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 {
		if err := os.Mkdir(filepath.Join(dir, "22"), 0o755); err != nil {
			t.Fatal(err)
		}
		denied := filepath.Join(dir, "22", "stat")
		if err := os.WriteFile(denied, []byte(fakeStat(22, "hidden")), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(denied, 0); err != nil {
			t.Fatal(err)
		}
	}

	procs := ReadAllProcs()
	if len(procs) != 1 || procs[0].PID != 20 || procs[0].Comm != "alive" {
		t.Fatalf("procs = %+v, want only pid 20", procs)
	}
	if _, ok := readProcess(99); ok {
		t.Fatal("missing pid was readable")
	}
}

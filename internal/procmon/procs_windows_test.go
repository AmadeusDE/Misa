//go:build windows

package procmon

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Native Windows tests. They only read other processes and only change or
// terminate a child this test started itself.

func procByPID(t *testing.T, pid int) (Process, bool) {
	t.Helper()
	for _, p := range ReadAllProcs() {
		if p.PID == pid {
			return p, true
		}
	}
	return Process{}, false
}

// waitForProc waits for a just-started process to show up in the table.
func waitForProc(t *testing.T, pid int) Process {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if p, ok := procByPID(t, pid); ok {
			return p
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pid %d never appeared in the process table", pid)
	return Process{}
}

// testChild starts the test binary again as a long-lived helper process: the
// only process these tests change or terminate.
func testChild(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestProcmonHelperProcess")
	cmd.Env = append(os.Environ(), "PROCMON_HELPER_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the helper process: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	return cmd
}

// TestProcmonHelperProcess is the child process: it waits to be killed.
func TestProcmonHelperProcess(t *testing.T) {
	if os.Getenv("PROCMON_HELPER_PROCESS") == "" {
		t.Skip("helper process only")
	}
	for i := 0; i < 600; i++ {
		time.Sleep(100 * time.Millisecond)
	}
}

func TestReadAllProcsFindsSelf(t *testing.T) {
	procs := ReadAllProcs()
	if len(procs) == 0 {
		t.Fatal("Toolhelp enumeration returned no processes")
	}

	self, ok := procByPID(t, os.Getpid())
	if !ok {
		t.Fatalf("pid %d is missing from its own process table", os.Getpid())
	}
	if self.Comm == "" {
		t.Error("self has no executable name")
	}
	if self.Threads < 1 {
		t.Errorf("self thread count = %d", self.Threads)
	}
	if !self.HasCPUTime() {
		t.Error("self CPU time is unavailable")
	}
	if self.CPUTime() == 0 {
		t.Error("self reported no CPU time at all")
	}
	if !self.HasMemory() || self.RSSBytes() == 0 {
		t.Errorf("self resident size = %d bytes (available %v)", self.RSSBytes(), self.HasMemory())
	}
	if !self.HasVirt() || self.VSize == 0 {
		t.Errorf("self committed size = %d bytes (available %v)", self.VSize, self.HasVirt())
	}
	if !self.HasPriority() {
		t.Error("self priority is unavailable")
	}
	if self.Cmdline == "" {
		t.Error("self has no command line")
	}
	// Windows has no state, user or shared-memory counters in this backend.
	if self.HasState() || self.HasUser() || self.HasShared() || self.HasMemPct() {
		t.Errorf("self claims unavailable fields: missing = %b", self.Missing)
	}
}

// TestIdleProcessRowIsKept covers the process that cannot be opened at all:
// PID 0 has no handle, but it is in the table and must not vanish.
func TestIdleProcessRowIsKept(t *testing.T) {
	idle, ok := procByPID(t, 0)
	if !ok {
		t.Fatal("the system idle process (pid 0) is missing from the table")
	}
	if idle.Comm == "" {
		t.Error("pid 0 has no name")
	}
	if idle.HasCPUTime() || idle.HasMemory() || idle.HasVirt() {
		t.Errorf("pid 0 claims detail it cannot have: missing = %b", idle.Missing)
	}
}

func TestSnapshotReportsSystemCountersUnavailable(t *testing.T) {
	snap := TakeSnapshot()
	if snap.SystemAvailable || snap.CPUAvailable {
		t.Error("Windows reports system counters it does not read")
	}
	if len(snap.CPUs) != 0 || snap.Mem.Total != 0 || len(snap.Net) != 0 || len(snap.Disks) != 0 {
		t.Errorf("system sections are not empty: %+v", snap)
	}
	if len(snap.Procs) == 0 {
		t.Fatal("snapshot has no processes")
	}
	if snap.Time.IsZero() {
		t.Error("snapshot is not timestamped")
	}
}

// TestProcessRowsNeverFabricateDetail covers processes of other users and
// protected processes (PID 0, the System process): whatever could not be read
// must be flagged, not filled with a plausible value.
func TestProcessRowsNeverFabricateDetail(t *testing.T) {
	for _, p := range ReadAllProcs() {
		if p.PID < 0 || p.Comm == "" {
			t.Errorf("row without a pid or name: %+v", p)
		}
		if !p.HasCPUTime() && p.CPUTime() != 0 {
			t.Errorf("pid %d has CPU time without the field being available", p.PID)
		}
		if !p.HasMemory() && p.RSS != 0 {
			t.Errorf("pid %d has a resident size it could not read", p.PID)
		}
		if !p.HasVirt() && p.VSize != 0 {
			t.Errorf("pid %d has a virtual size it could not read", p.PID)
		}
		if !p.HasState() && p.State != 0 {
			t.Errorf("pid %d invented a run state", p.PID)
		}
		if !p.HasShared() && p.SHR != 0 {
			t.Errorf("pid %d invented a shared size", p.PID)
		}
	}
}

// TestPriorityOfOwnedChild changes the priority class of a test-owned child
// only, and reads the result back from a fresh snapshot.
func TestPriorityOfOwnedChild(t *testing.T) {
	cmd := testChild(t)
	pid := cmd.Process.Pid
	waitForProc(t, pid)

	for _, want := range []struct {
		nice  int
		class PriorityClass
	}{
		{19, PriorityIdle},
		{-5, PriorityAboveNormal},
		{-10, PriorityHigh},
		{0, PriorityNormal},
	} {
		if err := platformActions.Renice(pid, want.nice); err != nil {
			t.Fatalf("set priority %d on owned child: %v", want.nice, err)
		}
		p := waitForProc(t, pid)
		if !p.HasPriority() {
			t.Fatalf("priority unavailable after setting it to %d", want.nice)
		}
		if p.Nice != int64(want.nice) {
			t.Errorf("nice = %d, want %d (%v)", p.Nice, want.nice, want.class)
		}
		if p.Priority != int64(want.class.Base()) {
			t.Errorf("base priority = %d, want %d for %v", p.Priority, want.class.Base(), want.class)
		}
	}
}

func TestPriorityRefusedForInvalidPID(t *testing.T) {
	if err := platformActions.Renice(0, 0); err == nil {
		t.Error("setting the priority of pid 0 must fail")
	}
	if err := platformActions.Renice(1<<30, 0); err == nil {
		t.Error("setting the priority of a non-existent process must fail")
	}
}

func TestTerminateOwnedChild(t *testing.T) {
	cmd := testChild(t)
	pid := cmd.Process.Pid
	waitForProc(t, pid)

	if err := platformActions.Terminate(pid); err != nil {
		t.Fatalf("terminating the owned child: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		t.Error("child exited cleanly, want it terminated")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := procByPID(t, pid); !ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("terminated child is still in the process table")
}

func TestTerminateRefusedForInvalidPID(t *testing.T) {
	if err := platformActions.Terminate(0); err == nil {
		t.Error("terminating pid 0 must fail")
	}
	if err := platformActions.Terminate(1 << 30); err == nil {
		t.Error("terminating a non-existent process must fail")
	}
}

func TestActionMenuOnWindowsIsTerminateOnly(t *testing.T) {
	var names []string
	for _, sig := range platformActions.Signals {
		names = append(names, sig.name)
	}
	if got, want := strings.Join(names, ","), "Cancel,Terminate"; got != want {
		t.Errorf("signal menu = %q, want %q", got, want)
	}
	if !platformActions.CanRenice() {
		t.Error("priority changes should be offered on Windows")
	}
}

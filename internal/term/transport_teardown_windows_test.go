package term

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procHandleCount = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")

func handleCount(t testing.TB) int {
	t.Helper()
	var n uint32
	h, _ := windows.GetCurrentProcess()
	if r, _, err := procHandleCount.Call(uintptr(h), uintptr(unsafe.Pointer(&n))); r == 0 {
		t.Fatalf("GetProcessHandleCount: %v", err)
	}
	return int(n)
}

// within runs f and fails the test if it does not return in time.
func within(t testing.TB, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { f(); close(done) }()
	select {
	case <-done:
	case <-time.After(nativeTimeout):
		t.Fatalf("%s did not finish within %v", what, nativeTimeout)
	}
}

// processGone reports whether the process has ended (or can no longer be opened).
func processGone(pid int, wait time.Duration) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true
	}
	defer windows.CloseHandle(h)
	ev, _ := windows.WaitForSingleObject(h, uint32(wait.Milliseconds()))
	return ev == windows.WAIT_OBJECT_0
}

func grandchildPID(t testing.TB, m nativePayload) int {
	t.Helper()
	pid, err := strconv.Atoi(m.Input)
	if err != nil || pid <= 0 {
		t.Fatalf("bad grandchild pid payload %+v", m)
	}
	return pid
}

func spawnTree(t *testing.T, mode string) (conn, proc, *nativeCapture, int) {
	c, p, cap := nativeStart(t, mode, "")
	return c, p, cap, grandchildPID(t, nativeWaitMessages(t, cap, 1)[0])
}

// nativeStartBare starts a helper with no reader and no cleanup hook.
func nativeStartBare(t testing.TB, mode string) (conn, proc) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c, p, err := startProcess(startSpec{
		Argv: []string{exe, "-test.run=^TestNativeTransportHelper$"},
		Env:  append(os.Environ(), nativeHelperEnv+"="+mode, "MISA_UNICODE=x"),
		Rows: 24, Cols: 80,
	})
	if err != nil {
		t.Fatalf("startProcess: %v", err)
	}
	return c, p
}

func TestNativeTeardownKillsDescendants(t *testing.T) {
	c, p, _, gc := spawnTree(t, "spawn")
	if processGone(gc, 0) {
		t.Fatal("grandchild exited before teardown")
	}
	within(t, "Close", func() { p.Kill(); c.Close() })
	nativeStatus(t, p, 1)
	if !processGone(gc, nativeTimeout) {
		t.Fatal("grandchild survived Close")
	}
}

func TestNativeTeardownCloseWithoutKill(t *testing.T) {
	c, _, _, gc := spawnTree(t, "spawn")
	within(t, "Close", func() { c.Close() })
	if !processGone(gc, nativeTimeout) {
		t.Fatal("grandchild survived Close")
	}
}

func TestNativeTeardownLeaderExitLeavesDescendantsUntilClose(t *testing.T) {
	c, p, cap, gc := spawnTree(t, "orphan")
	nativeStatus(t, p, 0)
	select {
	case <-cap.done:
		if err := cap.readError(); !errors.Is(err, io.EOF) {
			t.Errorf("read after leader exit: %v, want EOF", err)
		}
	case <-time.After(nativeTimeout):
		t.Fatal("output did not end after leader exit")
	}
	within(t, "Close", func() { c.Close() })
	if !processGone(gc, nativeTimeout) {
		t.Fatal("orphaned descendant survived Close")
	}
}

func TestNativeTeardownCloseBeforeReader(t *testing.T) {
	for _, mode := range []string{"large", "inspect"} {
		c, p := nativeStartBare(t, mode)
		time.Sleep(500 * time.Millisecond) // let output fill the pipes unread
		within(t, mode+" Close", func() { p.Kill(); c.Close() })
		select {
		case <-p.Status():
		case <-time.After(nativeTimeout):
			t.Fatalf("%s: no status after Close", mode)
		}
	}
}

func TestNativeTeardownFullInputBuffer(t *testing.T) {
	c, p, _ := nativeStart(t, "sleep", "")
	wrote := make(chan error, 1)
	go func() {
		chunk := make([]byte, 64*1024)
		for {
			if _, err := c.Write(chunk); err != nil {
				wrote <- err
				return
			}
		}
	}()
	select {
	case err := <-wrote:
		t.Fatalf("writer ended before Close: %v", err)
	case <-time.After(time.Second):
	}
	within(t, "Close", func() { p.Kill(); c.Close() })
	select {
	case <-wrote:
	case <-time.After(nativeTimeout):
		t.Fatal("blocked writer not released by Close")
	}
	if _, err := c.Write([]byte("x")); err == nil {
		t.Error("Write after Close succeeded")
	}
}

func TestNativeTeardownConcurrentClose(t *testing.T) {
	c, p, _ := nativeStart(t, "sleep", "")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); c.Close() }()
	}
	within(t, "concurrent Close", wg.Wait)
	nativeStatus(t, p, 1)
	if err := c.Resize(30, 100); err == nil {
		t.Error("Resize after Close succeeded")
	}
	within(t, "Close again", func() { c.Close() })
}

func TestNativeTeardownNaturalExitThenClose(t *testing.T) {
	c, p := nativeStartBare(t, "inspect")
	nativeStatus(t, p, 23)
	within(t, "Close", func() { c.Close() })
}

func TestNativeStartFailureCleansUp(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "notexe.exe")
	if err := os.WriteFile(bad, []byte("not a PE image"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := startProcess(startSpec{Argv: []string{bad}, Rows: 24, Cols: 80}); err == nil {
		t.Fatal("starting a non-executable succeeded")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var pid uint32
	assignProcess = func(job, process windows.Handle) error {
		pid, _ = windows.GetProcessId(process)
		return errors.New("forced assignment failure")
	}
	t.Cleanup(func() { assignProcess = windows.AssignProcessToJobObject })
	_, _, err = startProcess(startSpec{
		Argv: []string{exe, "-test.run=^TestNativeTransportHelper$"},
		Env:  append(os.Environ(), nativeHelperEnv+"=sleep"),
		Rows: 24, Cols: 80,
	})
	if err == nil || !strings.Contains(err.Error(), "assign process to job") {
		t.Fatalf("assignment failure: got %v", err)
	}
	if pid == 0 || !processGone(int(pid), nativeTimeout) {
		t.Fatalf("child %d was left running after assignment failure", pid)
	}
}

func TestNativeTeardownNoHandleLeaks(t *testing.T) {
	cycle := func() {
		c, p := nativeStartBare(t, "inspect")
		<-p.Status()
		within(t, "Close", func() { c.Close() })

		c, p = nativeStartBare(t, "spawn")
		time.Sleep(200 * time.Millisecond)
		within(t, "Close", func() { p.Kill(); c.Close() })
		<-p.Status()

		if c, _, err := startProcess(startSpec{Argv: []string{`C:\no\such.exe`}, Rows: 24, Cols: 80}); err == nil {
			c.Close()
			t.Error("start of a missing program succeeded")
		}
	}
	cycle() // warm up lazily loaded DLLs and runtime handles
	time.Sleep(200 * time.Millisecond)
	before := handleCount(t)
	for range 15 {
		cycle()
	}
	after := handleCount(t)
	for deadline := time.Now().Add(nativeTimeout); after > before+2 && time.Now().Before(deadline); {
		time.Sleep(100 * time.Millisecond)
		after = handleCount(t)
	}
	if after > before+2 {
		t.Errorf("handle count grew from %d to %d over 15 session cycles", before, after)
	}
}

// The host runs inside an outer job, as under CI runners, IDEs and Windows
// Terminal; the pane's own job nests inside it and must still kill the tree.
func TestNativeTeardownNestedJobHost(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestNativeTransportHelper$", "-test.v")
	cmd.Env = append(os.Environ(), nativeHelperEnv+"=nestedhost")
	var out []byte
	done := make(chan struct{})
	go func() { out, err = cmd.CombinedOutput(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * nativeTimeout):
		cmd.Process.Kill()
		t.Fatal("nested job host timed out")
	}
	if err != nil {
		t.Fatalf("nested job host failed: %v\n%s", err, out)
	}
}

// nativeNestedHost runs inside the re-exec'd host process.
func nativeNestedHost(t *testing.T) {
	outer, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	self, _ := windows.GetCurrentProcess()
	if err := windows.AssignProcessToJobObject(outer, self); err != nil {
		t.Fatalf("assign host to outer job: %v", err)
	}
	t.Run("descendants", func(t *testing.T) {
		c, p, _, gc := spawnTree(t, "spawn")
		within(t, "Close", func() { p.Kill(); c.Close() })
		nativeStatus(t, p, 1)
		if !processGone(gc, nativeTimeout) {
			t.Error("grandchild survived Close in a nested job")
		}
	})
	if t.Failed() {
		os.Exit(1)
	}
}

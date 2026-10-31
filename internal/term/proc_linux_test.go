package term

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProcSessionHandlesSpacesAndParensInComm(t *testing.T) {
	cases := []struct {
		stat string
		want int
		ok   bool
	}{
		{"123 (a ) b) S 1 2 456 0", 456, true},
		{"123 (a b) S 1 2 nope 0", 0, false},
		{"bad stat", 0, false},
	}
	for _, c := range cases {
		got, ok := procSession(c.stat)
		if got != c.want || ok != c.ok {
			t.Errorf("procSession(%q) = %d,%v want %d,%v", c.stat, got, ok, c.want, c.ok)
		}
	}
}

func processAlive(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	s := string(b)
	i := strings.LastIndexByte(s, ')')
	// Z is a zombie, X is dead and not yet reaped. Both are gone.
	state := strings.TrimSpace(s[i+1:])
	return i >= 0 && !strings.HasPrefix(state, "Z") && !strings.HasPrefix(state, "X")
}

func TestCloseKillsBackgroundJobsInPTYSession(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv("", testBin(t, "sh"), "-c", "sleep 300 & echo $!; wait"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	var pid int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && pid == 0 {
		for _, l := range tm.PlainLines() {
			if n, err := strconv.Atoi(strings.TrimSpace(l)); err == nil {
				pid = n
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 || !processAlive(pid) {
		t.Fatalf("background job not found (pid=%d)", pid)
	}
	tm.Close()
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && processAlive(pid) {
		time.Sleep(10 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Fatalf("background job %d survived Close", pid)
	}
}

func TestCloseKillsBackgroundJobsAfterTheLeaderExited(t *testing.T) {
	// A leader that has already exited makes TIOCGSID fail. Hold the sample
	// until then, so Close has to learn the session from the leader's stat.
	prev := ptySession
	ptySession = func(pid int, _ *os.File) (int, bool) {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && processAlive(pid) {
			time.Sleep(time.Millisecond)
		}
		return 0, false
	}
	t.Cleanup(func() { ptySession = prev })

	tm := New(5, 40)
	if err := tm.StartArgv("", testBin(t, "sh"), "-c", "trap '' HUP; sleep 300 & echo $!; exit 0"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	var pid int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && (pid == 0 || processAlive(pid)) {
		for _, l := range tm.PlainLines() {
			if n, err := strconv.Atoi(strings.TrimSpace(l)); err == nil {
				pid = n
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 || !processAlive(pid) {
		t.Fatalf("background job not found (pid=%d)", pid)
	}
	defer killPid(pid)
	tm.Close()
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && processAlive(pid) {
		time.Sleep(10 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Fatalf("background job %d survived Close after its leader exited", pid)
	}
}

func killPid(pid int) {
	if p, err := os.FindProcess(pid); err == nil {
		p.Kill()
	}
}

func TestSessionMembersSkipsUnreadableAndMissing(t *testing.T) {
	root := t.TempDir()
	write := func(pid, sid, comm string) {
		t.Helper()
		dir := filepath.Join(root, pid)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := pid + " (" + comm + ") S 1 1 " + sid + " 0\n"
		if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("10", "42", "ok")
	write("11", "99", "other")
	if err := os.Mkdir(filepath.Join(root, "12"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "not-a-pid"), 0o755); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 {
		write("13", "42", "denied")
		if err := os.Chmod(filepath.Join(root, "13", "stat"), 0); err != nil {
			t.Fatal(err)
		}
	}
	got, err := sessionMembers(root, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 10 {
		t.Fatalf("members = %v, want [10]", got)
	}
	if _, err := sessionMembers(filepath.Join(root, "missing"), 1); err == nil {
		t.Fatal("missing proc root was readable")
	}
}

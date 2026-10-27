package term

import (
	"os"
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
	return i >= 0 && !strings.HasPrefix(strings.TrimSpace(s[i+1:]), "Z")
}

func TestCloseKillsBackgroundJobsInPTYSession(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv("", "/bin/sh", "-c", "sleep 300 & echo $!; wait"); err != nil {
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
	tm := New(5, 40)
	if err := tm.StartArgv("", "/bin/sh", "-c", "trap '' HUP; sleep 300 & echo $!; exit 0"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	var pid int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && (pid == 0 || processAlive(tm.cmd.Process.Pid)) {
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

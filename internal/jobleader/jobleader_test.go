//go:build unix

package jobleader

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/term"
)

func TestMain(m *testing.M) {
	switch os.Getenv("JOBLEADER_HELPER") {
	case "1":
		os.Exit(Run(os.Args[1:]))
	case "builtin":
		SuspendOnCtrlZ()
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		fmt.Print("got " + line)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func startLed(t *testing.T, argv ...string) *term.Terminal {
	t.Helper()
	t.Setenv("JOBLEADER_HELPER", "1")
	tm := term.New(5, 40)
	if err := tm.StartArgv(t.TempDir(), append([]string{os.Args[0]}, argv...)...); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	t.Cleanup(tm.Close)
	time.Sleep(200 * time.Millisecond)
	return tm
}

func waitExit(t *testing.T, tm *term.Terminal) int {
	t.Helper()
	select {
	case <-tm.Exited():
		return tm.ExitCode()
	case <-time.After(5 * time.Second):
		t.Fatal("program never finished")
		return -1
	}
}

func TestCtrlZSuspendsAndContinueResumes(t *testing.T) {
	tm := startLed(t, "sh", "-c", "read x; echo got $x")
	tm.SendBytes([]byte("\x1a"))
	waitStopped(t, tm)
	if err := tm.Continue(); err != nil {
		t.Fatal(err)
	}
	tm.SendBytes([]byte("hi\r"))
	if code := waitExit(t, tm); code != 0 {
		t.Errorf("exit = %d", code)
	}
	if !strings.Contains(strings.Join(tm.PlainLines(), "\n"), "got hi") {
		t.Errorf("screen %q", tm.PlainLines())
	}
}

func TestExitStatusPassesThrough(t *testing.T) {
	if code := waitExit(t, startLed(t, "sh", "-c", "exit 7")); code != 7 {
		t.Errorf("exit = %d, want 7", code)
	}
}

func TestCtrlCReachesTheJob(t *testing.T) {
	tm := startLed(t, "sleep", "30")
	tm.SendBytes([]byte("\x03"))
	if code := waitExit(t, tm); code != 130 {
		t.Errorf("exit = %d, want 130", code)
	}
}

func TestMissingCommand(t *testing.T) {
	if code := waitExit(t, startLed(t, "no-such-command-misa")); code != 127 {
		t.Errorf("exit = %d, want 127", code)
	}
}

func waitStopped(t *testing.T, tm *term.Terminal) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !tm.Stopped() {
		if time.Now().After(deadline) {
			t.Fatal("^Z did not suspend the program")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBuiltinAloneInSessionSuspends(t *testing.T) {
	t.Setenv("JOBLEADER_HELPER", "builtin")
	tm := term.New(5, 40)
	if err := tm.StartArgv(t.TempDir(), os.Args[0]); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	t.Cleanup(tm.Close)
	time.Sleep(200 * time.Millisecond)
	tm.SendBytes([]byte("\x1a"))
	waitStopped(t, tm)
	tm.Continue()
	tm.SendBytes([]byte("hi\r"))
	waitExit(t, tm)
	if !strings.Contains(strings.Join(tm.PlainLines(), "\n"), "got hi") {
		t.Errorf("screen %q", tm.PlainLines())
	}
}

func TestLeaderWithoutTerminalStillRuns(t *testing.T) {
	cmd := exec.Command(os.Args[0], "sh", "-c", "exit 4")
	cmd.Env = append(os.Environ(), "JOBLEADER_HELPER=1")
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 4 {
		t.Errorf("err = %v, want exit 4", err)
	}
}

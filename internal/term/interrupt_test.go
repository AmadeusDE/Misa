package term

import (
	"testing"
	"time"
)

// ^C written to the terminal must reach a running program as an interrupt: ISIG on a Unix
// tty, a console control event through the ConPTY input on Windows.
func TestCtrlCInterruptsRunningProgram(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv(t.TempDir(), helperArgv(t, "sleep")...); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	defer tm.Close()
	time.Sleep(500 * time.Millisecond)
	tm.SendBytes([]byte{3})
	select {
	case <-tm.Exited():
	case <-time.After(2 * time.Second):
		t.Fatal("^C did not end the program")
	}
	t.Logf("exit code %d", tm.WaitExit())
	if tm.WaitExit() == 0 {
		t.Error("program exited normally after ^C")
	}
}

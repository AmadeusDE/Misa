//go:build unix

package builtins

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestInvocationProcessExitStatus(t *testing.T) {
	if os.Getenv("MISA_F02_CHILD") == "1" {
		os.Exit(42)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestInvocationProcessExitStatus$")
	cmd.Env = append(os.Environ(), "MISA_F02_CHILD=1")
	if got := ExitStatus(cmd.Run()); got != 42 {
		t.Fatalf("exit: %d", got)
	}
	cmd = exec.Command(os.Args[0], "-test.run=^TestInvocationSignalChild$")
	cmd.Env = append(os.Environ(), "MISA_F02_SIGNAL_CHILD=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if got := ExitStatus(cmd.Wait()); got != ExitSignalBase+9 {
		t.Fatalf("signal: %d", got)
	}
}

func TestInvocationSignalChild(t *testing.T) {
	if os.Getenv("MISA_F02_SIGNAL_CHILD") == "1" {
		time.Sleep(time.Hour)
	}
}

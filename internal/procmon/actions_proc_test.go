//go:build unix

package procmon

import (
	"os/exec"
	"testing"
)

func TestTerminateEndsOwnedProcess(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("sleep unavailable: %v", err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	if err := platformActions.Terminate(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Error("process exited cleanly, want it terminated")
	}
}

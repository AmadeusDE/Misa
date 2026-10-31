package vi

import (
	"os/exec"
	"strings"
)

// Shell writes share the existing synchronous shell-command lifecycle. T03
// owns asynchronous output presentation and cancellation.
func (e *Editor) writeShell(command string, from, to int) bool {
	if command == "" {
		e.statusMsg = "Shell error: command required"
		return false
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = e.dir
	cmd.Stdin = strings.NewReader(e.rangeContent(from, to))
	out, err := cmd.CombinedOutput()
	e.statusMsg = strings.TrimRight(string(out), "\n")
	if err != nil {
		e.statusMsg = "Shell error: " + err.Error() + " " + e.statusMsg
		return false
	}
	return true
}

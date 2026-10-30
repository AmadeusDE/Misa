//go:build unix

package proceffect

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/commandexec"
)

func execTarget(argv []string) int {
	if len(argv) == 0 {
		return 127
	}
	return commandexec.Exec(argv[0], commandexec.Invocation{Args: argv[1:], Env: os.Environ(), Stderr: os.Stderr})
}
func umaskChild() (int, error) {
	exe, e := registeredHelper()
	if e != nil {
		return 0, e
	}
	c := exec.Command(exe, Flag, "umask")
	b, e := c.Output()
	if e != nil {
		return 0, e
	}
	n, e := strconv.Atoi(strings.TrimSpace(string(b)))
	return n, e
}

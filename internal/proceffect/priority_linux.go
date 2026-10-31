//go:build linux

package proceffect

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

func niceness() (int, error) { n, e := syscall.Getpriority(syscall.PRIO_PROCESS, 0); return 20 - n, e }
func umask() (int, error) {
	b, e := os.ReadFile("/proc/self/status")
	if e == nil {
		if n, ok := parseProcUmask(string(b)); ok {
			return n, nil
		}
	}
	return umaskChild()
}
func parseProcUmask(status string) (int, bool) {
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "Umask:") {
			n, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "Umask:")), 8, 32)
			if err == nil && n <= 0777 {
				return int(n), true
			}
			return 0, false
		}
	}
	return 0, false
}

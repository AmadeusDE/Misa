//go:build unix

package builtins

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// currentUmask reads the umask without changing it. Linux reports it in
// /proc/self/status; elsewhere the only way is to set and restore it, which
// other goroutines can briefly observe.
func currentUmask() int {
	if data, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "Umask:"); ok {
				if n, err := strconv.ParseInt(strings.TrimSpace(v), 8, 32); err == nil {
					return int(n)
				}
			}
		}
	}
	u := syscall.Umask(0)
	syscall.Umask(u)
	return u
}

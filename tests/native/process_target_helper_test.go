//go:build unix

package native

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// processTargetHelper is the child side of the renice tests: it stays alive
// until it is killed and reports the identity a wrapper gave it.
func processTargetHelper(args []string) int {
	command := "ids"
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case "ids":
		pgid, _ := unix.Getpgid(0)
		sid, _ := unix.Getsid(0)
		fmt.Printf("pid=%d pgid=%d sid=%d\n", os.Getpid(), pgid, sid)
	case "sleep":
		// Block until killed; syscall.Pause does not exist on OpenBSD, so sleep
		// in a loop instead.
		for {
			time.Sleep(time.Hour)
		}
	}
	return 0
}

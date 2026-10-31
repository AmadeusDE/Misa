//go:build unix

package builtins

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// processWrapperTestHelper is the child side of the nohup/setsid/setpgid tests
// and a renice fixture: it reports the identity, priority and signal
// disposition the launcher actually gave it.
func processWrapperTestHelper(args []string) int {
	command := "ids"
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case "ids":
		pgid, _ := unix.Getpgid(0)
		sid, _ := unix.Getsid(0)
		fmt.Printf("pid=%d pgid=%d sid=%d nice=%d\n", os.Getpid(), pgid, sid, niceOf(os.Getpid()))
	case "hup":
		// With SIGHUP ignored the signal is discarded; otherwise the default
		// action kills this process before it can print.
		if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("alive")
	case "sleep":
		// Block until killed; syscall.Pause does not exist on OpenBSD, so sleep
		// in a loop instead.
		for {
			time.Sleep(time.Hour)
		}
	case "ctty":
		// A session only reaches /dev/tty when the terminal is its controlling
		// terminal; a detached session gets ENXIO.
		if f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0); err == nil {
			_ = f.Close()
			fmt.Println("ctty")
		} else {
			fmt.Println("no-ctty")
		}
	case "exit":
		code := 0
		if len(args) > 1 {
			code, _ = strconv.Atoi(args[1])
		}
		return code
	}
	return 0
}

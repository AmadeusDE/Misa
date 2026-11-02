//go:build unix

package imgview

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func standaloneSignals(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGWINCH)
}
func standaloneResizeSignal(s os.Signal) bool { return s == syscall.SIGWINCH }
func standaloneSuspend() {
	// SIGSTOP is process-directed: on a multithreaded Go process Kill can
	// return before the stop takes effect. Do not re-enter raw mode until the
	// process has actually been continued.
	continued := make(chan os.Signal, 1)
	signal.Notify(continued, syscall.SIGCONT)
	defer signal.Stop(continued)
	if syscall.Kill(os.Getpid(), syscall.SIGSTOP) == nil {
		<-continued
	}
}
func standalonePixels(f *os.File) (int, int) {
	ws, e := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if e != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
		return 0, 0
	}
	return (int(ws.Xpixel) + int(ws.Col)/2) / int(ws.Col), (int(ws.Ypixel) + int(ws.Row)/2) / int(ws.Row)
}

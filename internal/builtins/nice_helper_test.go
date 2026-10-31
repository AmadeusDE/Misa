package builtins

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/AmadeusDE/Misa/internal/proceffect"
)

func niceTestHelper(args []string) int {
	switch args[0] {
	case "inspect":
		dir, _ := os.Getwd()
		n, _ := proceffect.Niceness()
		fmt.Printf("%s|%s|%d|", dir, os.Getenv("VALUE"), n)
		_, _ = io.Copy(os.Stdout, os.Stdin)
		fmt.Fprint(os.Stderr, "error")
	case "concurrent":
		mask, _ := proceffect.Umask()
		n, _ := proceffect.Niceness()
		fmt.Printf("%04o\n%s\n%d\n", mask, os.Getenv("VALUE"), n)
		f, err := os.OpenFile(args[1], os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return 1
		}
		f.Close()
	case "exit":
		return 37
	case "signal":
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
		time.Sleep(time.Second)
		return 1
	case "sleep-parent":
		exe, _ := os.Executable()
		child := exec.Command(exe, "--nice-test-helper", "sleep")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if child.Start() != nil {
			return 1
		}
		fmt.Fprintln(os.Stdout, "ready")
		child.Wait()
	case "sleep":
		time.Sleep(30 * time.Second)
	}
	return 0
}

//go:build !unix

package jobleader

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Supported is false: there is no SIGTSTP, so a job can never be stopped and resumed.
const Supported = false

// Run starts argv and waits for it without any suspension semantics.
func Run(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "misa: job leader: no command")
		return 2
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "misa: %s: %v\n", argv[0], err)
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return 127
	}
	return 126
}

// SuspendOnCtrlZ does nothing: ^Z reaches the program as input.
func SuspendOnCtrlZ() {}

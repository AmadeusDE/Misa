//go:build !unix

package term

import "os"

func startProcess(startSpec) (conn, proc, error) { return nil, nil, ErrUnsupported }

func openFuncTTY(rows, cols int) (conn, *os.File, error) { return nil, nil, ErrUnsupported }

// ExitStatus is a process's status the way a shell reports it.
func ExitStatus(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(interface{ ExitCode() int }); ok {
		return ee.ExitCode()
	}
	return 1
}

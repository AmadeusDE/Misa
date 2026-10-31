package ui

import (
	"os"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

// TestMain makes the test binary a multicall binary on Windows, as main does for the real one:
// builtins re-exec BuiltinExecutable because StartFunc (an in-process builtin on a tty) is unsupported there.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		if cmd, ok := builtins.Lookup(os.Args[1]); ok {
			dir, _ := os.Getwd()
			os.Exit(cmd.Run(&builtins.Context{
				Dir: dir, Args: os.Args[2:], CmdName: os.Args[1],
				Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin,
			}))
		}
	}
	if exe, err := os.Executable(); err == nil {
		BuiltinExecutable = exe
	}
	os.Exit(m.Run())
}

//go:build windows

package builtins

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var timeoutDefaultPath = filepath.Join(os.Getenv("SystemRoot"), "System32")

func hasPathSeparator(name string) bool { return strings.ContainsAny(name, `/\`) }

// executableNames expands a bare name with the PATHEXT extensions, as the
// Windows shell does, unless it already ends in one of them.
func executableNames(ctx *Context, name string) []string {
	exts, _ := lookupEnv(ctx, "PATHEXT")
	return pathextNames(name, exts)
}

// isExecutableMode is true for any file: Windows keeps no execute bit, and the
// extension decided what executableNames offered.
func isExecutableMode(fs.FileInfo) bool { return true }

// timeoutProcAttr is empty: without job objects (plan P03) the command is not
// grouped with its children.
func timeoutProcAttr() *syscall.SysProcAttr { return nil }

func timeoutShell() string { return "" }

// timeoutDeliver ends the command with TerminateProcess. TERM is not graceful
// here, and the command's children are not reached; signal 0 only times out.
func timeoutDeliver(cmd *exec.Cmd, sig syscall.Signal, _ bool) {
	if sig != 0 {
		_ = cmd.Process.Kill()
	}
}

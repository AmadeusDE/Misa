//go:build unix

package commandexec

import (
	"fmt"
	"os"
	"syscall"
)

// Exec replaces the calling process and is intended only for an isolated child.
// File streams cannot be redirected here; arrange descriptors before calling it.
func Exec(name string, inv Invocation) int {
	path, err := Resolve(name, inv)
	if err == nil && inv.Dir != "" {
		err = os.Chdir(inv.Dir)
	}
	if err == nil {
		arg0 := inv.Argv0
		if arg0 == "" {
			arg0 = name
		}
		env := inv.Env
		if env == nil {
			env = os.Environ()
		}
		err = syscall.Exec(path, append([]string{arg0}, inv.Args...), env)
	}
	if inv.Stderr != nil {
		fmt.Fprintf(inv.Stderr, "%s: %v\n", name, err)
	}
	return ErrorStatus(err)
}

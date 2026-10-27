package builtins

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func init() {
	Register(Command{Name: "arch", Run: runArch})
}

func runArch(ctx *Context) int {
	for _, a := range ctx.Args {
		if a == "--help" {
			fmt.Fprintf(ctx.Stdout, "Usage: arch [OPTION]...\nPrint machine architecture name.\n")
			return 0
		}
		if a == "--version" {
			fmt.Fprintf(ctx.Stdout, "arch (Misa) 0.1.0\n")
			return 0
		}
		if a == "--" {
			break
		}
		if len(a) > 0 && a[0] == '-' {
			fmt.Fprintf(ctx.Stderr, "arch: unknown option %s\n", a)
			return 1
		}
	}
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		fmt.Fprintln(ctx.Stderr, "arch:", err)
		return 1
	}
	machine := make([]byte, 0, len(u.Machine))
	for _, b := range u.Machine {
		if b == 0 {
			break
		}
		machine = append(machine, byte(b))
	}
	fmt.Fprintln(ctx.Stdout, string(machine))
	return 0
}

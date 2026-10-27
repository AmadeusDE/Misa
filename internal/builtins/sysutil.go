package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	whichSpec = "a(all)"
	unameSpec = "a(all)s(kernel-name)n(nodename)r(kernel-release)v(kernel-version)m(machine)p(processor)i(hardware-platform)o(operating-system)A(all-labeled)"
)

var (
	whichSpecParsed *Spec
	unameSpecParsed *Spec
)

func init() {
	var err error
	whichSpecParsed, err = ParseSpec(whichSpec)
	if err != nil {
		panic("builtins: bad which spec: " + err.Error())
	}
	unameSpecParsed, err = ParseSpec(unameSpec)
	if err != nil {
		panic("builtins: bad uname spec: " + err.Error())
	}
	Register(Command{Name: "which", Run: runWhich})
	Register(Command{Name: "uname", Run: runUname})
}

func runWhich(ctx *Context) int {
	flags, err := whichSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "which:", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprintf(ctx.Stdout, "Usage: which [OPTION]... COMMAND...\nWrite the full path of COMMANDs to stdout.\n")
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "which (Misa) 0.1.0\n")
		return 0
	}

	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "Usage: which [OPTION]... COMMAND...")
		return 1
	}
	showAll := flags.Bool("a")
	exitCode := 0

	for _, cmdName := range flags.Operands {
		found := false
		if strings.Contains(cmdName, "/") {
			if isExecutable(cmdName) {
				fmt.Fprintln(ctx.Stdout, cmdName)
				found = true
			}
		} else {
			pathEnv := os.Getenv("PATH")
			for _, dir := range filepath.SplitList(pathEnv) {
				if dir == "" {
					dir = "."
				}
				full := filepath.Join(dir, cmdName)
				if isExecutable(full) {
					fmt.Fprintln(ctx.Stdout, full)
					found = true
					if !showAll {
						break
					}
				}
			}
		}
		if !found {
			exitCode = 1
		}
	}
	return exitCode
}

func isExecutable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

func utsFieldPtr(ptr unsafe.Pointer, maxLen int) string {
	b := unsafe.Slice((*byte)(ptr), maxLen)
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return string(b[:n])
}

func runUname(ctx *Context) int {
	flags, err := unameSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "uname:", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprintf(ctx.Stdout, "Usage: uname [OPTION]...\nPrint certain system information.\n\n")
		fmt.Fprint(ctx.Stdout, unameSpecParsed.GenerateUsage("uname")[len("Usage: uname [OPTION]... [OPERAND]...\n\n"):])
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "uname (Misa) 0.1.0\n")
		return 0
	}

	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		fmt.Fprintf(ctx.Stderr, "uname: %v\n", err)
		return 1
	}

	sysname := utsFieldPtr(unsafe.Pointer(&uts.Sysname[0]), len(uts.Sysname))
	nodename := utsFieldPtr(unsafe.Pointer(&uts.Nodename[0]), len(uts.Nodename))
	release := utsFieldPtr(unsafe.Pointer(&uts.Release[0]), len(uts.Release))
	version := utsFieldPtr(unsafe.Pointer(&uts.Version[0]), len(uts.Version))
	machine := utsFieldPtr(unsafe.Pointer(&uts.Machine[0]), len(uts.Machine))

	if flags.Bool("all-labeled") || flags.Bool("A") {
		fmt.Fprintf(ctx.Stdout, "Kernel name: %s\n", sysname)
		fmt.Fprintf(ctx.Stdout, "Node name: %s\n", nodename)
		fmt.Fprintf(ctx.Stdout, "Kernel release: %s\n", release)
		fmt.Fprintf(ctx.Stdout, "Kernel version: %s\n", version)
		fmt.Fprintf(ctx.Stdout, "Machine: %s\n", machine)
		fmt.Fprintf(ctx.Stdout, "Operating system: %s\n", "GNU/Linux")
		return 0
	}

	all := flags.Bool("a")
	any := flags.Bool("s") || flags.Bool("n") || flags.Bool("r") || flags.Bool("v") || flags.Bool("m") || flags.Bool("p") || flags.Bool("i") || flags.Bool("o")

	if all || !any {
		if all {
			fmt.Fprintln(ctx.Stdout, strings.Join([]string{sysname, nodename, release, version, machine, "GNU/Linux"}, " "))
		} else {
			fmt.Fprintln(ctx.Stdout, sysname)
		}
		return 0
	}

	var parts []string
	if flags.Bool("s") {
		parts = append(parts, sysname)
	}
	if flags.Bool("n") {
		parts = append(parts, nodename)
	}
	if flags.Bool("r") {
		parts = append(parts, release)
	}
	if flags.Bool("v") {
		parts = append(parts, version)
	}
	if flags.Bool("m") {
		parts = append(parts, machine)
	}
	if flags.Bool("p") {
		parts = append(parts, machine)
	}
	if flags.Bool("i") {
		parts = append(parts, machine)
	}
	if flags.Bool("o") {
		parts = append(parts, "GNU/Linux")
	}
	fmt.Fprintln(ctx.Stdout, strings.Join(parts, " "))
	return 0
}

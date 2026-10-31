package builtins

import "fmt"

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
	machine, err := machineName()
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "arch:", err)
		return 1
	}
	fmt.Fprintln(ctx.Stdout, machine)
	return 0
}

// machineFromGOARCH names a Go architecture the way uname -m does.
func machineFromGOARCH(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "386":
		return "i686"
	case "arm64":
		return "aarch64"
	case "arm":
		return "armv7l"
	}
	return goarch
}

// machineFromProcessorArchitecture names a Windows PROCESSOR_ARCHITECTURE_*
// value the way uname -m does; an architecture it does not know is "unknown"
// rather than a guess.
func machineFromProcessorArchitecture(arch uint16) string {
	switch arch {
	case 9:
		return "x86_64"
	case 12:
		return "aarch64"
	case 0:
		return "i686"
	case 5:
		return "armv7l"
	}
	return "unknown"
}

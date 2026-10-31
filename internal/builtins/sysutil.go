package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		if hasPathSeparator(cmdName) {
			if file, ok := whichFind("", cmdName, ctx); ok {
				fmt.Fprintln(ctx.Stdout, file)
				found = true
			}
		} else {
			pathEnv, _ := lookupEnv(ctx, "PATH")
			for _, dir := range filepath.SplitList(pathEnv) {
				if dir == "" {
					dir = "."
				}
				if file, ok := whichFind(dir, cmdName, ctx); ok {
					fmt.Fprintln(ctx.Stdout, file)
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

// whichFind returns the executable that name means in dir (or as given when
// dir is empty), trying the platform's executable spellings.
func whichFind(dir, name string, ctx *Context) (string, bool) {
	for _, file := range executableNames(ctx, name) {
		if dir != "" {
			file = filepath.Join(dir, file)
		}
		info, err := os.Stat(file)
		if err == nil && info.Mode().IsRegular() && isExecutableMode(info) {
			return file, true
		}
	}
	return "", false
}

type unameInfo struct {
	sysname, nodename, release, version, machine, osName string
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

	u, err := readUname()
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "uname: %v\n", err)
		return 1
	}
	sysname, nodename, release, version, machine, osName := u.sysname, u.nodename, u.release, u.version, u.machine, u.osName

	if flags.Bool("all-labeled") || flags.Bool("A") {
		fmt.Fprintf(ctx.Stdout, "Kernel name: %s\n", sysname)
		fmt.Fprintf(ctx.Stdout, "Node name: %s\n", nodename)
		fmt.Fprintf(ctx.Stdout, "Kernel release: %s\n", release)
		fmt.Fprintf(ctx.Stdout, "Kernel version: %s\n", version)
		fmt.Fprintf(ctx.Stdout, "Machine: %s\n", machine)
		fmt.Fprintf(ctx.Stdout, "Operating system: %s\n", osName)
		return 0
	}

	all := flags.Bool("a")
	any := flags.Bool("s") || flags.Bool("n") || flags.Bool("r") || flags.Bool("v") || flags.Bool("m") || flags.Bool("p") || flags.Bool("i") || flags.Bool("o")

	if all || !any {
		if all {
			fmt.Fprintln(ctx.Stdout, strings.Join([]string{sysname, nodename, release, version, machine, osName}, " "))
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
		parts = append(parts, osName)
	}
	fmt.Fprintln(ctx.Stdout, strings.Join(parts, " "))
	return 0
}

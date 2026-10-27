package builtins

import (
	"fmt"
	"os"
	"strings"
)

const printenvSpec = "0(null)"

var printenvSpecParsed *Spec

func init() {
	var err error
	printenvSpecParsed, err = ParseSpec(printenvSpec)
	if err != nil {
		panic("builtins: bad printenv spec: " + err.Error())
	}
	Register(Command{Name: "true", Run: runTrue})
	Register(Command{Name: "false", Run: runFalse})
	Register(Command{Name: "echo", Run: runEcho})
	Register(Command{Name: "yes", Run: runYes})
	Register(Command{Name: "printenv", Run: runPrintenv})
}

func runTrue(ctx *Context) int {
	if len(ctx.Args) == 1 {
		if ctx.Args[0] == "--help" {
			fmt.Fprintf(ctx.Stdout, "Usage: true [ignored command line arguments]\n")
			return 0
		}
		if ctx.Args[0] == "--version" {
			fmt.Fprintf(ctx.Stdout, "true (Misa) 0.1.0\n")
			return 0
		}
	}
	return 0
}

func runFalse(ctx *Context) int {
	if len(ctx.Args) == 1 {
		if ctx.Args[0] == "--help" {
			fmt.Fprintf(ctx.Stdout, "Usage: false [ignored command line arguments]\nExit with a status code indicating false.\n")
		}
		if ctx.Args[0] == "--version" {
			fmt.Fprintf(ctx.Stdout, "false (Misa) 0.1.0\n")
		}
	}
	return 1
}

func runEcho(ctx *Context) int {
	posix := os.Getenv("POSIXLY_CORRECT") != ""

	noNewline := false
	escape := posix // POSIX mode: escapes on by default

	i := 0
	if posix {
		// POSIX mode: only "-n" as the first (separate) arg enables option processing.
		// If first arg is "-n", consume it and keep parsing -n/-e/-E options.
		// Combined flags like "-nE" are not recognized — only single-letter forms.
		if len(ctx.Args) > 0 && ctx.Args[0] == "-n" {
			noNewline = true
			i = 1
			for i < len(ctx.Args) {
				a := ctx.Args[i]
				if a == "-n" {
					i++
				} else if a == "-e" || a == "-E" {
					// -E is a no-op in POSIX mode (escapes always on)
					i++
				} else {
					break
				}
			}
		}
	} else {
		for i < len(ctx.Args) {
			a := ctx.Args[i]
			if len(a) < 2 || a[0] != '-' || a[1] == '-' {
				break
			}
			valid := true
			for _, c := range a[1:] {
				if c != 'n' && c != 'e' && c != 'E' {
					valid = false
					break
				}
			}
			if !valid {
				break
			}
			for _, c := range a[1:] {
				switch c {
				case 'n':
					noNewline = true
				case 'e':
					escape = true
				case 'E':
					escape = false
				}
			}
			i++
		}
	}

	text := strings.Join(ctx.Args[i:], " ")
	if escape {
		var stop bool
		text, stop = expandEscapes(text)
		if stop {
			fmt.Fprint(ctx.Stdout, text)
			return 0
		}
	}
	if noNewline {
		fmt.Fprint(ctx.Stdout, text)
	} else {
		fmt.Fprintln(ctx.Stdout, text)
	}
	return 0
}

func runYes(ctx *Context) int {
	for i, a := range ctx.Args {
		if a == "--help" {
			fmt.Fprintf(ctx.Stdout, "Usage: yes [STRING]...\nRepeatedly output a line with STRING, or 'y'.\n")
			return 0
		}
		if a == "--version" {
			fmt.Fprintf(ctx.Stdout, "yes (Misa) 0.1.0\n")
			return 0
		}
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "--") {
			fmt.Fprintf(ctx.Stderr, "yes: unrecognized option '%s'\n", a)
			return 1
		}
		_ = i
	}
	word := "y"
	if len(ctx.Args) > 0 {
		word = strings.Join(ctx.Args, " ")
	}
	limit := 10000
	if !isTUIMode(ctx) {
		limit = -1
	}
	for i := 0; limit < 0 || i < limit; i++ {
		if _, err := fmt.Fprintln(ctx.Stdout, word); err != nil {
			fmt.Fprintf(ctx.Stderr, "yes: standard output: %s\n", fmtOsErr(err))
			return 1
		}
	}
	return 0
}

func runPrintenv(ctx *Context) int {
	flags, err := printenvSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "printenv:", err)
		return 2
	}
	if HandleHelpVersion(ctx, flags, printenvSpecParsed) {
		return 0
	}
	terminator := "\n"
	if flags.Bool("0") || flags.Bool("null") {
		terminator = "\x00"
	}

	envList := ctx.Env
	if len(envList) == 0 {
		envList = os.Environ()
	}

	envMap := make(map[string]string)
	var orderedList []string
	for _, e := range envList {
		eqIdx := strings.IndexByte(e, '=')
		if eqIdx >= 0 {
			key := e[:eqIdx]
			val := e[eqIdx+1:]
			envMap[key] = val
			orderedList = append(orderedList, e)
		}
	}

	if len(flags.Operands) == 0 {
		for _, e := range orderedList {
			fmt.Fprint(ctx.Stdout, e)
			fmt.Fprint(ctx.Stdout, terminator)
		}
		return 0
	}

	exitCode := 0
	for _, name := range flags.Operands {
		if strings.Contains(name, "=") {
			exitCode = 1
			continue
		}
		val, ok := envMap[name]
		if ok {
			fmt.Fprint(ctx.Stdout, val)
			fmt.Fprint(ctx.Stdout, terminator)
		} else {
			exitCode = 1
		}
	}
	return exitCode
}

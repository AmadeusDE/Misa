package builtins

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

const (
	killSpec  = "^s(signal):l(list)t(table)"
	pgrepSpec = "^x(exact)l(list-full)c(count)n(newest)o(oldest)v(invert)"
	pkillSpec = "^x(exact)n(newest)o(oldest)v(invert)(signal):"
)

var (
	killSpecParsed  *Spec
	pgrepSpecParsed *Spec
	pkillSpecParsed *Spec
)

var sigNumberMap = map[int]string{
	0:  "EXIT",
	1:  "HUP",
	2:  "INT",
	3:  "QUIT",
	4:  "ILL",
	5:  "TRAP",
	6:  "ABRT",
	7:  "BUS",
	8:  "FPE",
	9:  "KILL",
	10: "USR1",
	11: "SEGV",
	12: "USR2",
	13: "PIPE",
	14: "ALRM",
	15: "TERM",
	16: "STKFLT",
	17: "CHLD",
	18: "CONT",
	19: "STOP",
	20: "TSTP",
	21: "TTIN",
	22: "TTOU",
	23: "URG",
	24: "XCPU",
	25: "XFSZ",
	26: "VTALRM",
	27: "PROF",
	28: "WINCH",
	29: "IO",
	30: "PWR",
	31: "SYS",
}

func init() {
	var err error
	killSpecParsed, err = ParseSpec(killSpec)
	if err != nil {
		panic("builtins: bad kill spec: " + err.Error())
	}
	pgrepSpecParsed, err = ParseSpec(pgrepSpec)
	if err != nil {
		panic("builtins: bad pgrep spec: " + err.Error())
	}
	pkillSpecParsed, err = ParseSpec(pkillSpec)
	if err != nil {
		panic("builtins: bad pkill spec: " + err.Error())
	}
	Register(Command{Name: "kill", Run: runKill})
	Register(Command{Name: "pgrep", Run: runPgrep})
	Register(Command{Name: "pkill", Run: runPkill})
}

func parseSignalArg(arg string) (syscall.Signal, bool) {
	upper := strings.ToUpper(arg)
	if upper == "0" || upper == "EXIT" {
		return 0, true
	}
	upper = strings.TrimPrefix(upper, "SIG")
	if sig, ok := signalNames[upper]; ok {
		return sig, true
	}
	n, err := strconv.Atoi(arg)
	if err == nil && n >= 0 && n <= 64 {
		return syscall.Signal(n), true
	}
	return 0, false
}

func runKill(ctx *Context) int {
	for _, arg := range ctx.Args {
		if arg == "--help" {
			fmt.Fprintln(ctx.Stdout, "Usage: kill [-s SIGNAL | -SIGNAL] PID...\n  kill -l [SIGNAL...]\n  kill -t\nNote: a Job object is usually not yours")
			return 0
		}
	}

	sigFromArg := ""
	var filteredArgs []string
	for i := 0; i < len(ctx.Args); i++ {
		a := ctx.Args[i]
		if sigFromArg == "" && strings.HasPrefix(a, "-") && len(a) > 1 && a != "--" && a != "-l" && a != "-t" && a != "-s" && a != "--list" && a != "--table" && a != "--signal" {
			cand := a[1:]
			if _, ok := parseSignalArg(cand); ok {
				sigFromArg = cand
				continue
			}
		}
		filteredArgs = append(filteredArgs, a)
	}

	var finalArgs []string
	sawOptionEnd := false
	for _, a := range filteredArgs {
		if !sawOptionEnd && strings.HasPrefix(a, "-") && len(a) > 1 && a != "--" && a != "-l" && a != "-t" && a != "-s" && a != "--list" && a != "--table" && a != "--signal" {
			if _, err := strconv.Atoi(a); err == nil {
				finalArgs = append(finalArgs, "--")
				sawOptionEnd = true
			}
		}
		finalArgs = append(finalArgs, a)
	}

	flags, err := killSpecParsed.ParseArgs(finalArgs)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "kill:", err)
		return 1
	}

	if flags.Bool("t") {
		for i := 0; i <= 31; i++ {
			if name, ok := sigNumberMap[i]; ok {
				fmt.Fprintf(ctx.Stdout, "%2d %s\n", i, name)
			}
		}
		for i := 34; i <= 64; i++ {
			if i == 34 {
				fmt.Fprintf(ctx.Stdout, "%2d RTMIN\n", i)
			} else if i == 64 {
				fmt.Fprintf(ctx.Stdout, "%2d RTMAX\n", i)
			} else {
				fmt.Fprintf(ctx.Stdout, "%2d RTMIN+%d\n", i, i-34)
			}
		}
		return 0
	}

	if flags.Bool("l") {
		if len(flags.Operands) == 0 {
			all := []string{
				"EXIT", "HUP", "INT", "QUIT", "ILL", "TRAP", "ABRT", "BUS",
				"FPE", "KILL", "USR1", "SEGV", "USR2", "PIPE", "ALRM", "TERM",
				"STKFLT", "CHLD", "CONT", "STOP", "TSTP", "TTIN", "TTOU", "URG",
				"XCPU", "XFSZ", "VTALRM", "PROF", "WINCH", "IO", "PWR", "SYS",
				"RTMIN", "RTMAX",
			}
			fmt.Fprintln(ctx.Stdout, strings.Join(all, " "))
			return 0
		}
		exitCode := 0
		for _, op := range flags.Operands {
			if n, err := strconv.Atoi(op); err == nil {
				modN := n
				if n >= 128 {
					if n%128 == 0 {
						if (n/128)%2 == 1 && n != 128 {
							modN = -1
						} else {
							modN = 0
						}
					} else {
						modN = n % 128
					}
				}
				if name, ok := sigNumberMap[modN]; ok {
					fmt.Fprintln(ctx.Stdout, name)
				} else if modN >= 32 && modN <= 64 {
					fmt.Fprintln(ctx.Stdout, modN)
				} else {
					fmt.Fprintf(ctx.Stderr, "kill: '%s': invalid signal\n", op)
					exitCode = 1
				}
			} else {
				if sig, ok := parseSignalArg(op); ok {
					fmt.Fprintln(ctx.Stdout, int(sig))
				} else {
					fmt.Fprintf(ctx.Stderr, "kill: '%s': invalid signal\n", op)
					exitCode = 1
				}
			}
		}
		return exitCode
	}

	sigStr := "TERM"
	if sigFromArg != "" {
		sigStr = sigFromArg
	} else if s, ok := flags.String("s"); ok {
		sigStr = s
	}

	sig, ok := parseSignalArg(sigStr)
	if !ok {
		fmt.Fprintf(ctx.Stderr, "kill: '%s': invalid signal\n", sigStr)
		return 1
	}

	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "kill: no process ID specified")
		return 1
	}

	exitCode := 0
	for _, pidStr := range flags.Operands {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "kill: invalid process ID '%s'\n", pidStr)
			exitCode = 1
			continue
		}
		if pid < 0 {
			fmt.Fprintf(ctx.Stderr, "kill: sending signal to a negative PID is not supported\n")
			exitCode = 1
			continue
		}
		if err := syscall.Kill(pid, sig); err != nil {
			fmt.Fprintf(ctx.Stderr, "kill: (%s) - No such process\n", pidStr)
			exitCode = 1
		}
	}
	return exitCode
}

func matchProcs(pattern string, exact, invert bool) ([]procInfo, error) {
	procs, err := scanProcs()
	if err != nil {
		return nil, err
	}
	myPid := os.Getpid()
	var matched []procInfo

	var re *regexp.Regexp
	if !exact {
		re, err = regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %v", pattern, err)
		}
	}

	for _, p := range procs {
		if p.pid == myPid {
			continue
		}
		var hit bool
		if exact {
			hit = p.comm == pattern
		} else {
			hit = re.MatchString(p.comm)
		}
		if invert {
			hit = !hit
		}
		if hit {
			matched = append(matched, p)
		}
	}
	return matched, nil
}

func runPgrep(ctx *Context) int {
	flags, err := pgrepSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "pgrep:", err)
		return 1
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "pgrep: missing pattern")
		return 1
	}

	matched, err := matchProcs(flags.Operands[0], flags.Bool("x"), flags.Bool("v"))
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "pgrep:", err)
		return 1
	}

	if len(matched) == 0 {
		return 1
	}

	if flags.Bool("n") {
		matched = matched[len(matched)-1:]
	} else if flags.Bool("o") {
		matched = matched[:1]
	}

	if flags.Bool("c") {
		fmt.Fprintln(ctx.Stdout, len(matched))
		return 0
	}

	listFull := flags.Bool("l")
	for _, p := range matched {
		if listFull {
			fmt.Fprintf(ctx.Stdout, "%d %s\n", p.pid, p.cmdline)
		} else {
			fmt.Fprintln(ctx.Stdout, p.pid)
		}
	}
	return 0
}

func runPkill(ctx *Context) int {
	flags, err := pkillSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "pkill:", err)
		return 1
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "pkill: missing pattern")
		return 1
	}

	sig := syscall.SIGTERM
	if sigName, ok := flags.String("signal"); ok {
		s, found := parseSignalArg(sigName)
		if !found {
			fmt.Fprintf(ctx.Stderr, "pkill: unknown signal %q\n", sigName)
			return 1
		}
		sig = s
	}

	matched, err := matchProcs(flags.Operands[0], flags.Bool("x"), flags.Bool("v"))
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "pkill:", err)
		return 1
	}

	if len(matched) == 0 {
		return 1
	}

	if flags.Bool("n") {
		matched = matched[len(matched)-1:]
	} else if flags.Bool("o") {
		matched = matched[:1]
	}

	exitCode := 0
	for _, p := range matched {
		if err := syscall.Kill(p.pid, sig); err != nil {
			fmt.Fprintf(ctx.Stderr, "pkill: (%d) %v\n", p.pid, err)
			exitCode = 1
		}
	}
	return exitCode
}

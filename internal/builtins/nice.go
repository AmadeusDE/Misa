package builtins

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func init() {
	Register(Command{Name: "nice", Run: runNice})
}

func runNice(ctx *Context) int {
	args := ctx.Args
	if len(args) == 0 {
		prio, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "nice: %v\n", err)
			return 1
		}
		fmt.Fprintln(ctx.Stdout, prio)
		return 0
	}

	adj := 10
	adjSpecified := false
	cmdIdx := -1

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			cmdIdx = i + 1
			break
		}
		if a == "-n" || a == "--adjustment" || a == "--adj" {
			if i+1 >= len(args) {
				fmt.Fprintln(ctx.Stderr, "nice: option requires an argument -- 'n'")
				return 125
			}
			valStr := args[i+1]
			i++
			parsedAdj, err := parseAdj(valStr)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "nice: invalid adjustment '%s': %v\n", valStr, err)
				return 125
			}
			adj = parsedAdj
			adjSpecified = true
			continue
		}
		if strings.HasPrefix(a, "-n") && len(a) > 2 {
			valStr := a[2:]
			parsedAdj, err := parseAdj(valStr)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "nice: invalid adjustment '%s': %v\n", valStr, err)
				return 125
			}
			adj = parsedAdj
			adjSpecified = true
			continue
		}
		if strings.HasPrefix(a, "--adjustment=") || strings.HasPrefix(a, "--adj=") {
			eqIdx := strings.IndexByte(a, '=')
			valStr := a[eqIdx+1:]
			parsedAdj, err := parseAdj(valStr)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "nice: invalid adjustment '%s': %v\n", valStr, err)
				return 125
			}
			adj = parsedAdj
			adjSpecified = true
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 && isDigits(a[1:]) {
			parsedAdj, err := parseAdj(a[1:])
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "nice: invalid adjustment '%s': %v\n", a[1:], err)
				return 125
			}
			adj = parsedAdj
			adjSpecified = true
			continue
		}
		if strings.HasPrefix(a, "+") && len(a) > 1 && isDigits(a[1:]) {
			parsedAdj, err := parseAdj(a[1:])
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "nice: invalid adjustment '%s': %v\n", a[1:], err)
				return 125
			}
			adj = parsedAdj
			adjSpecified = true
			continue
		}
		if a == "--help" {
			fmt.Fprintln(ctx.Stdout, "Usage: nice [OPTION] [COMMAND [ARG]...]")
			return 0
		}
		if a == "--version" {
			fmt.Fprintln(ctx.Stdout, "nice (Misa) 0.1.0")
			return 0
		}
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(ctx.Stderr, "nice: unrecognized option '%s'\n", a)
			return 125
		}

		cmdIdx = i
		break
	}

	if cmdIdx == -1 || cmdIdx >= len(args) {
		if adjSpecified {
			fmt.Fprintln(ctx.Stderr, "nice: a command must be given with an adjustment")
			return 125
		}
		prio, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "nice: %v\n", err)
			return 1
		}
		fmt.Fprintln(ctx.Stdout, prio)
		return 0
	}

	// TODO(F04): priority is a process/thread effect and needs child isolation.
	curPrio, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err == nil {
		newPrio := curPrio + adj
		if err := syscall.Setpriority(syscall.PRIO_PROCESS, 0, newPrio); err != nil || (adj < 0 && os.Geteuid() != 0) {
			fmt.Fprintln(ctx.Stderr, "nice: warning: setpriority: Permission denied")
		}
	}

	subCmdName := args[cmdIdx]
	subArgs := args[cmdIdx+1:]

	return runSubCommand(ctx, subCmdName, subArgs)
}

func parseAdj(s string) (int, error) {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
			if strings.HasPrefix(s, "-") {
				return math.MinInt32, nil
			}
			return math.MaxInt32, nil
		}
		return 0, err
	}
	if val > math.MaxInt32 {
		val = math.MaxInt32
	} else if val < math.MinInt32 {
		val = math.MinInt32
	}
	return int(val), nil
}

func isDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func runSubCommand(ctx *Context, subCmdName string, subArgs []string) int {
	if ctx.Exec != nil {
		subCtx := *ctx
		subCtx.Args, subCtx.CmdName = subArgs, subCmdName
		return ctx.Exec(ctx.CancellationContext(), subCmdName, &subCtx)
	}
	localPath := filepath.Join(ctx.Dir, subCmdName)
	if info, err := os.Stat(localPath); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
		cmd := exec.Command(localPath, subArgs...)
		cmd.Dir = ctx.Dir
		cmd.Stdout = ctx.Stdout
		cmd.Stderr = ctx.Stderr
		cmd.Stdin = ctx.Stdin
		cmd.Env = ctx.Env
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode()
			}
			return 1
		}
		return 0
	}

	cmd, ok := Lookup(subCmdName)
	if !ok {
		cmd, ok = Lookup(filepath.Base(subCmdName))
	}
	if ok {
		subCtx := *ctx
		subCtx.Args, subCtx.CmdName = subArgs, subCmdName
		return cmd.Run(&subCtx)
	}

	execCmd := exec.Command(subCmdName, subArgs...)
	execCmd.Dir = ctx.Dir
	execCmd.Stdout = ctx.Stdout
	execCmd.Stderr = ctx.Stderr
	execCmd.Stdin = ctx.Stdin
	execCmd.Env = ctx.Env
	if err := execCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(ctx.Stderr, "nice: '%s': %v\n", subCmdName, err)
		return 127
	}
	return 0
}

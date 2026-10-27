package builtins

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
)

const nprocSpec = "(all)(ignore):"

var nprocSpecParsed *Spec

func init() {
	var err error
	nprocSpecParsed, err = ParseSpec(nprocSpec)
	if err != nil {
		panic("builtins: bad nproc spec: " + err.Error())
	}
	Register(Command{Name: "nproc", Run: runNproc})
}

func nprocParseUlong(s string) (uint64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
			return math.MaxUint64, true
		}
		return 0, false
	}
	return v, true
}

func nprocGetEnv(ctx *Context, key string) (string, bool) {
	prefix := key + "="
	for _, e := range ctx.Env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix), true
		}
	}
	return "", false
}

func runNproc(ctx *Context) int {
	flags, err := nprocSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "nproc:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, nprocSpecParsed) {
		return 0
	}
	if len(flags.Operands) > 0 {
		fmt.Fprintf(ctx.Stderr, "nproc: extra operand '%s'\n", flags.Operands[0])
		return 1
	}

	all := flags.Bool("all")
	avail := uint64(runtime.NumCPU())
	n := avail

	if !all {
		if omp, ok := nprocGetEnv(ctx, "OMP_NUM_THREADS"); ok {
			first := omp
			if idx := strings.IndexByte(omp, ','); idx >= 0 {
				first = omp[:idx]
			}
			if v, ok := nprocParseUlong(first); ok && v > 0 {
				n = v
			}
		}
		if limitStr, ok := nprocGetEnv(ctx, "OMP_THREAD_LIMIT"); ok {
			if limit, ok := nprocParseUlong(limitStr); ok && limit > 0 && limit < n {
				n = limit
			}
		}
	}

	if ignoreStr, ok := flags.String("ignore"); ok {
		s := strings.TrimSpace(ignoreStr)
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
				n = 1
			} else {
				fmt.Fprintf(ctx.Stderr, "nproc: invalid value '%s' for option --ignore\n", ignoreStr)
				return 1
			}
		} else if v < 0 {
			fmt.Fprintf(ctx.Stderr, "nproc: invalid value '%s' for option --ignore\n", ignoreStr)
			return 1
		} else if uint64(v) >= n {
			n = 1
		} else {
			n -= uint64(v)
		}
	}

	fmt.Fprintln(ctx.Stdout, n)
	return 0
}

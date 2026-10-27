package builtins

import (
	"bufio"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func init() {
	Register(Command{Name: "factor", Run: runFactor})
}

func parseFactor(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "\x00")
	s = strings.TrimPrefix(s, "+")
	return strconv.ParseUint(s, 10, 64)
}

func runFactor(ctx *Context) int {
	exponents := false
	var operands []string
	for _, arg := range ctx.Args {
		if arg == "--" {
			break
		}
		if arg == "--exponents" {
			exponents = true
			continue
		}
		if len(arg) >= 2 && arg[0] == '-' && arg[1] != '-' {
			allH := true
			for _, c := range arg[1:] {
				if c != 'h' {
					allH = false
					break
				}
			}
			if allH {
				exponents = true
				continue
			}
		}
		operands = append(operands, arg)
	}

	if len(operands) > 0 {
		exitCode := 0
		for _, arg := range operands {
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(ctx.Stderr, "factor: %q is not a valid positive integer\n", arg)
				fmt.Fprintf(ctx.Stderr, "Usage: factor [OPTION]... [NUMBER]...\n")
				exitCode = 1
				continue
			}
			n, err := parseFactor(arg)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "factor: %q is not a valid positive integer\n", arg)
				exitCode = 1
				continue
			}
			printFactors(ctx, n, exponents)
		}
		return exitCode
	}

	if ctx.Stdin == nil {
		fmt.Fprintln(ctx.Stderr, "factor: missing operand")
		return 1
	}

	sc := bufio.NewScanner(ctx.Stdin)
	exitCode := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		for _, word := range strings.Fields(line) {
			n, err := parseFactor(word)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "factor: %q is not a valid positive integer\n", word)
				exitCode = 1
				continue
			}
			printFactors(ctx, n, exponents)
		}
	}
	return exitCode
}

func printFactors(ctx *Context, n uint64, exponents bool) {
	fmt.Fprintf(ctx.Stdout, "%d:", n)
	if n > 1 {
		if exponents {
			printFactorsExponent(ctx, n)
		} else {
			printFactorsFlat(ctx, n)
		}
	}
	fmt.Fprintln(ctx.Stdout)
}

func printFactorsFlat(ctx *Context, n uint64) {
	rem := n
	for rem%2 == 0 {
		fmt.Fprint(ctx.Stdout, " 2")
		rem /= 2
	}
	limit := uint64(math.Sqrt(float64(rem)))
	for d := uint64(3); d <= limit; d += 2 {
		for rem%d == 0 {
			fmt.Fprintf(ctx.Stdout, " %d", d)
			rem /= d
			limit = uint64(math.Sqrt(float64(rem)))
		}
	}
	if rem > 1 {
		fmt.Fprintf(ctx.Stdout, " %d", rem)
	}
}

func printFactorsExponent(ctx *Context, n uint64) {
	rem := n
	if rem%2 == 0 {
		count := 0
		for rem%2 == 0 {
			count++
			rem /= 2
		}
		if count > 1 {
			fmt.Fprintf(ctx.Stdout, " 2^%d", count)
		} else {
			fmt.Fprint(ctx.Stdout, " 2")
		}
	}
	limit := uint64(math.Sqrt(float64(rem)))
	for d := uint64(3); d <= limit; d += 2 {
		if rem%d == 0 {
			count := 0
			for rem%d == 0 {
				count++
				rem /= d
			}
			limit = uint64(math.Sqrt(float64(rem)))
			if count > 1 {
				fmt.Fprintf(ctx.Stdout, " %d^%d", d, count)
			} else {
				fmt.Fprintf(ctx.Stdout, " %d", d)
			}
		}
	}
	if rem > 1 {
		fmt.Fprintf(ctx.Stdout, " %d", rem)
	}
}

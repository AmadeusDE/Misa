package builtins

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func init() {
	Register(Command{Name: "comm", Run: runComm})
}

func runComm(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	suppress := [3]bool{}
	var files []string
	optionsEnded := false

	for _, a := range ctx.Args {
		if optionsEnded {
			files = append(files, a)
		} else if a == "-" {
			files = append(files, a)
		} else if a == "--" {
			optionsEnded = true
		} else if strings.HasPrefix(a, "-") {
			for _, c := range a[1:] {
				switch c {
				case '1':
					suppress[0] = true
				case '2':
					suppress[1] = true
				case '3':
					suppress[2] = true
				default:
					fmt.Fprintf(ctx.Stderr, "comm: unknown option -%c\n", c)
					return 1
				}
			}
		} else {
			files = append(files, a)
		}
	}

	if len(files) != 2 {
		fmt.Fprintln(ctx.Stderr, "comm: requires exactly two files")
		return 1
	}

	if files[0] == "-" && files[1] == "-" {
		return commSameStdin(ctx, suppress)
	}

	lines1, err := readAllLines(ctx, files[0])
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "comm: %s: %s\n", files[0], fmtOsErr(err))
		return 1
	}
	lines2, err := readAllLines(ctx, files[1])
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "comm: %s: %s\n", files[1], fmtOsErr(err))
		return 1
	}

	i, j := 0, 0
	for i < len(lines1) || j < len(lines2) {
		if i >= len(lines1) {
			if !commPrint(ctx, suppress, 1, lines2[j]) {
				return 1
			}
			j++
		} else if j >= len(lines2) {
			if !commPrint(ctx, suppress, 0, lines1[i]) {
				return 1
			}
			i++
		} else if lines1[i] < lines2[j] {
			if !commPrint(ctx, suppress, 0, lines1[i]) {
				return 1
			}
			i++
		} else if lines1[i] > lines2[j] {
			if !commPrint(ctx, suppress, 1, lines2[j]) {
				return 1
			}
			j++
		} else {
			if !commPrint(ctx, suppress, 2, lines1[i]) {
				return 1
			}
			i++
			j++
		}
	}
	return 0
}

func commSameStdin(ctx *Context, suppress [3]bool) int {
	if ctx.Stdin == nil {
		fmt.Fprintln(ctx.Stderr, "comm: stdin not available")
		return 1
	}
	scanner := bufio.NewScanner(ctx.Stdin)
	var line1, line2 string
	have1, have2 := false, false
	for {
		if !have1 {
			if scanner.Scan() {
				line1 = scanner.Text()
				have1 = true
			}
		}
		if !have2 {
			if scanner.Scan() {
				line2 = scanner.Text()
				have2 = true
			}
		}
		if !have1 && !have2 {
			break
		}
		if !have1 {
			if !commPrint(ctx, suppress, 1, line2) {
				return 1
			}
			have2 = false
		} else if !have2 {
			if !commPrint(ctx, suppress, 0, line1) {
				return 1
			}
			have1 = false
		} else if line1 < line2 {
			if !commPrint(ctx, suppress, 0, line1) {
				return 1
			}
			have1 = false
		} else if line1 > line2 {
			if !commPrint(ctx, suppress, 1, line2) {
				return 1
			}
			have2 = false
		} else {
			if !commPrint(ctx, suppress, 2, line1) {
				return 1
			}
			have1 = false
			have2 = false
		}
	}
	return 0
}

func commPrint(ctx *Context, suppress [3]bool, col int, line string) bool {
	if suppress[col] {
		return true
	}
	tabs := 0
	for c := 0; c < col; c++ {
		if !suppress[c] {
			tabs++
		}
	}
	_, err := fmt.Fprintf(ctx.Stdout, "%s%s\n", strings.Repeat("\t", tabs), line)
	return err == nil
}

func readAllLines(ctx *Context, name string) ([]string, error) {
	var scanner *bufio.Scanner
	if name == "-" {
		if ctx.Stdin == nil {
			return nil, fmt.Errorf("stdin not available")
		}
		scanner = bufio.NewScanner(ctx.Stdin)
	} else {
		path := resolvePath(ctx, name)
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		scanner = bufio.NewScanner(ContextReader(ctx.CancellationContext(), f))
	}

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

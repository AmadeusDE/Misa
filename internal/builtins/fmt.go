package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const fmtSpec = "w(width):g(goal):s(split-only)q(quick)u(uniform-spacing)p(prefix):m(max-line-length):t(tagged-paragraph)"

var fmtSpecParsed *Spec

func init() {
	var err error
	fmtSpecParsed, err = ParseSpec(fmtSpec)
	if err != nil {
		panic("builtins: bad fmt spec: " + err.Error())
	}
	Register(Command{Name: "fmt", Run: runFmt})
}

func parseFmtNum(val string, optName string) (int64, error) {
	v, err := strconv.ParseInt(val, 10, 64)
	if err != nil || v > 10000000 {
		return 0, fmt.Errorf("fmt: invalid %s: '%s'", optName, val)
	}
	if v < 0 || v > 2500 {
		return 0, fmt.Errorf("fmt: invalid %s: '%s': Numerical result out of range", optName, val)
	}
	return v, nil
}

func runFmt(ctx *Context) int {
	var widthVal, goalVal string
	var hasWidth, hasGoal bool
	var argsForSpec []string

	for i, arg := range ctx.Args {
		if i == 0 && len(arg) > 1 && arg[0] == '-' && arg[1] >= '0' && arg[1] <= '9' {
			widthVal = arg[1:]
			hasWidth = true
			continue
		}
		if i > 0 && len(arg) > 1 && arg[0] == '-' && arg[1] >= '0' && arg[1] <= '9' {
			fmt.Fprintf(ctx.Stderr, "fmt: invalid option -- %c; -WIDTH is recognized only when it is the first\noption; use -w N instead\n", arg[1])
			return 1
		}
		argsForSpec = append(argsForSpec, arg)
	}

	flags, err := fmtSpecParsed.ParseArgs(argsForSpec)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "fmt:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, fmtSpecParsed) {
		return 0
	}

	if w, ok := flags.String("w"); ok {
		widthVal = w
		hasWidth = true
	}
	if g, ok := flags.String("g"); ok {
		goalVal = g
		hasGoal = true
	}

	width := int64(75)
	if hasWidth {
		v, err := parseFmtNum(widthVal, "width")
		if err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return 1
		}
		width = v
	}

	goal := int64(-1)
	if hasGoal {
		v, err := parseFmtNum(goalVal, "goal")
		if err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return 1
		}
		goal = v
	}

	if goal == -1 {
		goal = (width * 93) / 100
	}
	if goal > width {
		fmt.Fprintln(ctx.Stderr, "fmt: GOAL cannot be greater than WIDTH.")
		return 1
	}

	splitOnly := flags.Bool("s")
	files := flags.Operands

	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "fmt: missing operand")
			return 1
		}
		files = []string{"-"}
	}

	exitCode := 0
	for _, name := range files {
		var r io.Reader
		if name == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "fmt: stdin not available")
				exitCode = 1
				continue
			}
			r = ctx.Stdin
		} else {
			path := resolvePath(ctx, name)
			fi, serr := os.Stat(path)
			if serr == nil && fi.IsDir() {
				fmt.Fprintf(ctx.Stderr, "fmt: %s: Is a directory\n", name)
				exitCode = 1
				continue
			}
			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "fmt: cannot open '%s' for reading: %s\n", name, fmtOsErr(err))
				exitCode = 1
				continue
			}
			defer fh.Close()
			r = fh
		}

		fmtProcess(ctx, r, int(width), int(goal), splitOnly)
	}
	return exitCode
}

func fmtProcess(ctx *Context, r io.Reader, width, goal int, splitOnly bool) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var para []string

	flush := func() {
		if len(para) == 0 {
			return
		}
		if splitOnly {
			for _, line := range para {
				fmtSplitLine(ctx, line, width)
			}
		} else {
			words := make([]string, 0)
			for _, line := range para {
				for _, w := range strings.Fields(line) {
					words = append(words, w)
				}
			}
			fmtWrap(ctx, words, width, goal)
		}
		para = para[:0]
	}

	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			flush()
			fmt.Fprintln(ctx.Stdout)
		} else {
			para = append(para, line)
		}
	}
	flush()
}

func fmtWrap(ctx *Context, words []string, width, goal int) {
	if len(words) == 0 {
		return
	}
	totalLen := 0
	for i, w := range words {
		if i > 0 {
			totalLen++
		}
		totalLen += len(w)
	}
	if totalLen <= width {
		fmt.Fprintln(ctx.Stdout, strings.Join(words, " "))
		return
	}

	n := len(words)
	dp := make([]int64, n+1)
	breakAt := make([]int, n+1)
	const inf = int64(1 << 60)

	for i := 0; i < n; i++ {
		dp[i] = inf
	}
	dp[n] = 0

	for i := n - 1; i >= 0; i-- {
		lineLen := 0
		for j := i + 1; j <= n; j++ {
			wlen := len(words[j-1])
			if j == i+1 {
				lineLen = wlen
			} else {
				lineLen += 1 + wlen
			}

			if lineLen > width && j > i+1 {
				break
			}

			diff := int64(lineLen - goal)
			cost := diff * diff
			totalCost := cost + dp[j]
			if totalCost <= dp[i] {
				dp[i] = totalCost
				breakAt[i] = j
			}
		}
	}

	var lines [][]string
	for curr := 0; curr < n; {
		next := breakAt[curr]
		if next <= curr {
			next = curr + 1
		}
		lines = append(lines, words[curr:next])
		curr = next
	}
	for i := len(lines) - 1; i > 0; i-- {
		lineLen := len(strings.Join(lines[i], " "))
		if lineLen >= goal/2 || len(lines[i-1]) < 2 {
			continue
		}
		word := lines[i-1][len(lines[i-1])-1]
		if lineLen+len(word)+1 <= width {
			lines[i-1] = lines[i-1][:len(lines[i-1])-1]
			lines[i] = append([]string{word}, lines[i]...)
		}
	}
	for _, line := range lines {
		fmt.Fprintln(ctx.Stdout, strings.Join(line, " "))
	}
}

func fmtSplitLine(ctx *Context, line string, width int) {
	for len(line) > width {
		split := width
		for split > 0 && line[split] != ' ' {
			split--
		}
		if split == 0 {
			split = width
		}
		fmt.Fprintln(ctx.Stdout, line[:split])
		line = strings.TrimLeft(line[split:], " ")
	}
	fmt.Fprintln(ctx.Stdout, line)
}

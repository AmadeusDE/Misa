package builtins

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
)

const uniqSpec = "c(count)d(repeated)D(all-repeated):?u(unique)i(ignore-case)f(skip-fields)#s(skip-chars)#w(check-chars)#z(zero-terminated)(group):?"

var uniqSpecParsed *Spec

func init() {
	var err error
	uniqSpecParsed, err = ParseSpec(uniqSpec)
	if err != nil {
		panic("builtins: bad uniq spec: " + err.Error())
	}
	Register(Command{Name: "uniq", Run: runUniq})
}

func uniqPreprocess(args []string) []string {
	var out []string
	for _, a := range args {
		if len(a) >= 2 && a[0] == '-' && a[1] >= '0' && a[1] <= '9' {
			j := 1
			for j < len(a) && a[j] >= '0' && a[j] <= '9' {
				j++
			}
			out = append(out, "-f", a[1:j])
			if j < len(a) {
				out = append(out, "-"+a[j:])
			}
			continue
		}
		if len(a) >= 2 && a[0] == '+' && a[1] >= '0' && a[1] <= '9' {
			j := 1
			for j < len(a) && a[j] >= '0' && a[j] <= '9' {
				j++
			}
			out = append(out, "-s", a[1:j])
			if j < len(a) {
				out = append(out, "-"+a[j:])
			}
			continue
		}
		out = append(out, a)
	}
	return out
}

func runUniq(ctx *Context) int {
	args := uniqPreprocess(ctx.Args)
	flags, err := uniqSpecParsed.ParseArgs(args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "uniq:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, uniqSpecParsed) {
		return 0
	}

	showCount := flags.Bool("c")
	onlyDup := flags.Bool("d")
	onlyUniq := flags.Bool("u")
	ignoreCase := flags.Bool("i")
	zeroTerm := flags.Bool("z")

	allRepeated := flags.Bool("D")
	allRepeatedMethod := "none"
	for i := len(args) - 1; i >= 0; i-- {
		a := args[i]
		if a == "-D" || a == "--all-repeated" {
			allRepeatedMethod = "none"
			break
		}
		if strings.HasPrefix(a, "--all-repeated=") {
			allRepeatedMethod = uniqMatchMethod(strings.TrimPrefix(a, "--all-repeated="), []string{"none", "prepend", "separate"})
			break
		}
		if strings.HasPrefix(a, "-D") && len(a) > 2 {
			allRepeatedMethod = uniqMatchMethod(a[2:], []string{"none", "prepend", "separate"})
			break
		}
	}

	groupMode := false
	groupMethod := "separate"
	if s, ok := flags.String("group"); ok {
		groupMode = true
		if s != "" {
			groupMethod = uniqMatchMethod(s, []string{"prepend", "append", "separate", "both"})
		}
	}

	skipFields, _ := flags.Num("f")
	skipChars, _ := flags.Num("s")
	checkChars, hasCheckChars := flags.Num("w")
	byteLocale := getEnv(ctx, "LC_ALL") == "C" || (getEnv(ctx, "LC_ALL") == "" && getEnv(ctx, "LC_CTYPE") == "C")

	delim := byte('\n')
	if zeroTerm {
		delim = 0
	}

	var lines []string
	files := flags.Operands
	if len(files) == 0 || files[0] == "-" {
		lines = splitLines(ctx.Stdin, delim)
	} else {
		path := resolvePath(ctx, files[0])
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "uniq: %s: %s\n", files[0], fmtOsErr(err))
			return 1
		}
		defer f.Close()
		lines = splitLines(f, delim)
	}

	compareKey := func(line string) string {
		key := line
		if skipFields > 0 {
			i := 0
			for count := int64(0); count < skipFields && i < len(key); count++ {
				for i < len(key) && (key[i] == ' ' || key[i] == '\t') {
					i++
				}
				for i < len(key) && key[i] != ' ' && key[i] != '\t' {
					i++
				}
			}
			key = key[i:]
		}
		if skipChars > 0 {
			runes := []rune(key)
			if int(skipChars) < len(runes) {
				key = string(runes[skipChars:])
			} else {
				key = ""
			}
		}
		if hasCheckChars && checkChars >= 0 {
			if byteLocale {
				if int(checkChars) < len(key) {
					key = key[:checkChars]
				}
			} else {
				runes := []rune(key)
				if int(checkChars) < len(runes) {
					key = string(runes[:checkChars])
				}
			}
		}
		if ignoreCase {
			key = strings.ToLower(key)
		}
		return key
	}

	var outBuf bytes.Buffer
	out := &outBuf

	var groups []uniqGroup
	for _, line := range lines {
		key := compareKey(line)
		if len(groups) > 0 && compareKey(groups[len(groups)-1].lines[0]) == key {
			groups[len(groups)-1].count++
			groups[len(groups)-1].lines = append(groups[len(groups)-1].lines, line)
		} else {
			groups = append(groups, uniqGroup{lines: []string{line}, count: 1})
		}
	}

	terminator := string(delim)

	if groupMode {
		for i, g := range groups {
			needSepBefore := false
			if groupMethod == "prepend" && i == 0 {
				needSepBefore = true
			} else if groupMethod == "both" && i == 0 {
				needSepBefore = true
			} else if i > 0 {
				needSepBefore = true
			}
			if needSepBefore {
				fmt.Fprint(out, terminator)
			}
			for _, line := range g.lines {
				fmt.Fprint(out, line)
				fmt.Fprint(out, terminator)
			}
			if (groupMethod == "append" || groupMethod == "both") && i == len(groups)-1 {
				fmt.Fprint(out, terminator)
			}
		}
	} else if allRepeated {
		for i, g := range groups {
			if g.count < 2 {
				continue
			}
			if allRepeatedMethod == "prepend" || (allRepeatedMethod == "separate" && i > 0 && uniqHasPriorDup(groups, i)) {
				fmt.Fprint(out, terminator)
			}
			for _, line := range g.lines {
				fmt.Fprint(out, line)
				fmt.Fprint(out, terminator)
			}
		}
	} else {
		for _, g := range groups {
			if onlyDup && g.count < 2 {
				continue
			}
			if onlyUniq && g.count > 1 {
				continue
			}
			if showCount {
				fmt.Fprintf(out, "%7d %s%s", g.count, g.lines[0], terminator)
			} else {
				fmt.Fprint(out, g.lines[0])
				fmt.Fprint(out, terminator)
			}
		}
	}

	result := outBuf.Bytes()
	if len(files) > 1 && files[1] != "-" {
		path := resolvePath(ctx, files[1])
		f, err := os.Create(path)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "uniq: %s: %s\n", files[1], fmtOsErr(err))
			return 1
		}
		defer f.Close()
		f.Write(result)
	} else {
		ctx.Stdout.Write(result)
	}

	return 0
}

type uniqGroup struct {
	lines []string
	count int
}

func uniqHasPriorDup(groups []uniqGroup, idx int) bool {
	for i := 0; i < idx; i++ {
		if groups[i].count >= 2 {
			return true
		}
	}
	return false
}

func uniqMatchMethod(prefix string, methods []string) string {
	for _, m := range methods {
		if strings.HasPrefix(m, prefix) {
			return m
		}
	}
	return prefix
}

func splitLines(r interface{ Read([]byte) (int, error) }, delim byte) []string {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if delim == 0 {
		sc.Split(scanZeroTerminated)
	}
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

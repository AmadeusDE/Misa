//go:build unix

// Behaviour adapted from toybox mkfifo.c (0BSD) and uutils coreutils mkfifo (MIT).
package builtins

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func init() {
	Register(Command{Name: "mkfifo", Run: runMkfifo})
}

func runMkfifo(ctx *Context) int {
	spec, _ := ParseSpec("m(mode):")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "mkfifo:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "mkfifo: missing operand")
		return 1
	}
	mode := os.FileMode(0666)
	modeStr, hasMode := flags.String("m")
	if hasMode {
		mask := os.FileMode(0)
		for _, clause := range strings.Split(modeStr, ",") {
			if clause != "" && strings.ContainsAny(clause[:1], "+-=") {
				mask, err = mkfifoUmask()
				break
			}
		}
		if err == nil {
			mode, err = parseMkfifoMode(modeStr, mask)
		}
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "mkfifo:", err)
			return 1
		}
	}
	code := 0
	for _, name := range flags.Operands {
		path := name
		if name != "" {
			path = resolvePath(ctx, name)
		}
		err := syscall.Mkfifo(path, 0666)
		if err == nil && hasMode {
			err = os.Chmod(path, mode)
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "mkfifo: cannot create fifo '%s': %s\n", name, fmtOsErr(&os.PathError{Op: "mkfifo", Path: path, Err: err}))
			code = 1
		}
	}
	return code
}

func parseMkfifoMode(text string, mask os.FileMode) (os.FileMode, error) {
	invalid := fmt.Errorf("invalid mode '%s'", text)
	if text == "" {
		return 0, invalid
	}
	if text[0] >= '0' && text[0] <= '9' {
		n, err := strconv.ParseUint(text, 8, 32)
		if err != nil {
			return 0, invalid
		}
		if n&^0777 != 0 {
			return 0, fmt.Errorf("mode must specify only file permission bits")
		}
		return os.FileMode(n), nil
	}
	mode := os.FileMode(0666)
	for _, clause := range strings.Split(text, ",") {
		i := strings.IndexAny(clause, "+-=")
		if i < 0 {
			return 0, invalid
		}
		who := clause[:i]
		omitted := who == ""
		if omitted {
			who = "a"
		}
		for i < len(clause) {
			end := i + 1
			for end < len(clause) && !strings.ContainsAny(clause[end:end+1], "+-=") {
				end++
			}
			next, err := parseSymbolicMode(who+clause[i:end], mode)
			if err != nil {
				return 0, invalid
			}
			if omitted {
				next &^= mask
				if clause[i] != '=' {
					next |= mode & mask
				}
			}
			mode = next
			i = end
		}
	}
	if mode&^os.FileMode(0777) != 0 {
		return 0, fmt.Errorf("mode must specify only file permission bits")
	}
	return mode, nil
}

// Reading procfs avoids the process-global side effects of syscall.Umask.
func mkfifoUmask() (os.FileMode, error) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, fmt.Errorf("cannot read umask: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "Umask:"); ok {
			mask, err := strconv.ParseUint(strings.TrimSpace(value), 8, 32)
			return os.FileMode(mask), err
		}
	}
	return 0, fmt.Errorf("cannot read umask from /proc/self/status")
}

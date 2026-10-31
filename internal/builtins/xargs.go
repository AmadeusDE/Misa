package builtins

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func init() { Register(Command{Name: "xargs", Run: runXargs}) }

type xargsOptions struct {
	delim                           *byte
	eof, replace, input             string
	info                            string
	replacing, noEmpty, trace, exit bool
	n, lines, size, procs           int
}

func parseXargs(args []string) (xargsOptions, []string, error) {
	o := xargsOptions{procs: 1}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return o, args[i+1:], nil
		}
		if len(a) < 2 || a[0] != '-' || a == "-" {
			return o, args[i:], nil
		}
		if strings.HasPrefix(a, "--") {
			name, val, has := strings.Cut(a[2:], "=")
			aliases := map[string]byte{"null": '0', "delimiter": 'd', "eof": 'E', "max-args": 'n', "max-lines": 'L', "max-chars": 's', "exit": 'x', "verbose": 't', "no-run-if-empty": 'r', "replace": 'I', "arg-file": 'a', "max-procs": 'P', "interactive": 'p', "open-tty": 'o'}
			if name == "help" || name == "version" {
				o.info = name
				return o, nil, nil
			}
			c, ok := aliases[name]
			if !ok {
				return o, nil, fmt.Errorf("unrecognized option %s", a)
			}
			if has {
				a = "-" + string(c) + val
			} else {
				a = "-" + string(c)
			}
		}
		for j := 1; j < len(a); j++ {
			c := a[j]
			switch c {
			case '0':
				b := byte(0)
				o.delim = &b
			case 'r':
				o.noEmpty = true
			case 't':
				o.trace = true
			case 'x':
				o.exit = true
			case 'p', 'o':
				return o, nil, fmt.Errorf("option -%c is unsupported", c)
			case 'd', 'E', 'n', 'L', 's', 'I', 'i', 'a', 'P':
				var v string
				if j+1 < len(a) {
					v = a[j+1:]
				} else if c == 'i' {
					v = "{}"
				} else {
					i++
					if i >= len(args) {
						return o, nil, fmt.Errorf("option -%c requires an argument", c)
					}
					v = args[i]
				}
				j = len(a)
				switch c {
				case 'd':
					b, err := xargsDelimiter(v)
					if err != nil {
						return o, nil, err
					}
					o.delim = &b
				case 'E':
					o.eof = v
				case 'I', 'i':
					if v == "" {
						return o, nil, fmt.Errorf("empty replacement string")
					}
					o.replace = v
					o.replacing = true
					o.lines = 1
					o.n = 0
				case 'a':
					o.input = v
				case 'n', 'L', 's', 'P':
					n, err := strconv.Atoi(v)
					if err != nil || n < 0 || (n == 0 && c != 'P') {
						return o, nil, fmt.Errorf("invalid value for -%c: %s", c, v)
					}
					switch c {
					case 'n':
						o.n = n
						o.lines = 0
						o.replacing = false
					case 'L':
						o.lines = n
						o.n = 0
						o.replacing = false
					case 's':
						o.size = n
					case 'P':
						o.procs = n
						if n == 0 {
							o.procs = 256
						}
					}
				}
			default:
				return o, nil, fmt.Errorf("invalid option -%c", c)
			}
		}
	}
	return o, nil, nil
}

func xargsDelimiter(s string) (byte, error) {
	if len(s) == 1 {
		return s[0], nil
	}
	if strings.HasPrefix(s, `\`) {
		switch s {
		case `\n`:
			return '\n', nil
		case `\t`:
			return '\t', nil
		case `\r`:
			return '\r', nil
		case `\0`:
			return 0, nil
		case `\\`:
			return '\\', nil
		case `\a`:
			return '\a', nil
		case `\b`:
			return '\b', nil
		case `\f`:
			return '\f', nil
		case `\v`:
			return '\v', nil
		}
		base := 8
		digits := s[1:]
		if strings.HasPrefix(digits, "x") {
			base = 16
			digits = digits[1:]
		}
		n, e := strconv.ParseUint(digits, base, 8)
		if e == nil && digits != "" {
			return byte(n), nil
		}
	}
	return 0, fmt.Errorf("invalid delimiter %q", s)
}

func runXargs(ctx *Context) int {
	o, argv, err := parseXargs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "xargs:", err)
		return 1
	}
	if o.info == "help" {
		fmt.Fprintln(ctx.Stdout, "Usage: xargs [OPTION]... [COMMAND [INITIAL-ARGS]...]")
		fmt.Fprintln(ctx.Stdout, "  -P N  Run at most N commands concurrently (default 1; 0 uses a cap of 256)")
		return 0
	}
	if o.info == "version" {
		fmt.Fprintln(ctx.Stdout, "xargs (Misa) 0.1.0")
		return 0
	}
	if len(argv) == 0 {
		argv = []string{"echo"}
	}
	budget := 128 * 1024
	for _, e := range ctx.Environ() {
		budget -= len(e) + 1
	}
	if o.size > 0 && o.size < budget {
		budget = o.size
	}
	base := 0
	for _, a := range argv {
		base += len(a) + 1
	}
	if base > budget {
		fmt.Fprintln(ctx.Stderr, "xargs: argument list too long")
		return 1
	}
	r := ctx.Stdin
	if o.input != "" {
		f, e := os.Open(resolvePath(ctx, o.input))
		if e != nil {
			fmt.Fprintln(ctx.Stderr, "xargs:", e)
			return 1
		}
		defer f.Close()
		r = f
	}
	scheduler := newXargsScheduler(ctx, argv[0], o.procs)
	defer scheduler.close()
	tokenizer := newXargsTokenizer(scheduler.inputCtx, ContextReader(scheduler.inputCtx, r), o.delim, o.eof, o.replacing)
	var pending *xargsToken
	done, ran := false, false
	for {
		if !scheduler.ready() {
			return scheduler.finish(0)
		}
		childArgs := append([]string(nil), argv[1:]...)
		size, count, lineCount, lastLine := base, 0, 0, -1
		for !done {
			var tok xargsToken
			if pending != nil {
				tok = *pending
				pending = nil
			} else {
				var err error
				tok, err = tokenizer.Next()
				if err == io.EOF {
					done = true
					break
				}
				if err != nil {
					if ctx.CancellationContext().Err() != nil {
						return scheduler.finish(0)
					}
					if scheduler.inputCtx.Err() != nil {
						return scheduler.finish(0)
					}
					fmt.Fprintln(scheduler.stderr, "xargs:", err)
					return scheduler.finish(1)
				}
			}
			if o.lines > 0 && count > 0 && tok.line != lastLine && lineCount >= o.lines {
				pending = &tok
				break
			}
			if o.replacing {
				for i := range childArgs {
					childArgs[i] = strings.ReplaceAll(childArgs[i], o.replace, tok.text)
				}
				size = len(argv[0]) + 1
				for _, a := range childArgs {
					size += len(a) + 1
				}
				if size > budget {
					fmt.Fprintln(scheduler.stderr, "xargs: command line too long")
					return scheduler.finish(1)
				}
				count++
				break
			}
			next := len(tok.text) + 1
			if size+next > budget {
				if count == 0 || o.exit {
					fmt.Fprintln(scheduler.stderr, "xargs: argument list too long")
					return scheduler.finish(1)
				}
				pending = &tok
				break
			}
			size += next
			if count == 0 || tok.line != lastLine {
				lineCount++
				lastLine = tok.line
			}
			childArgs = append(childArgs, tok.text)
			count++
			if o.n > 0 && count >= o.n {
				break
			}
		}
		if count == 0 && (ran || o.noEmpty) {
			break
		}
		sub := *ctx
		sub.Args, sub.CmdName = childArgs, argv[0]
		if o.input == "" {
			// Children must not consume xargs's own input stream; with -a
			// the input comes from the file, so they inherit stdin.
			sub.Stdin, sub.StdinTTY = strings.NewReader(""), false
		}
		scheduler.launch(&sub, o.trace)
		ran = true
		if done {
			break
		}
	}
	return scheduler.finish(0)
}

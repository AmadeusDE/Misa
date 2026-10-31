package builtins

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

func init() { Register(Command{Name: "xargs", Run: runXargs}) }

type xargsOptions struct {
	delim                           *byte
	eof, replace, input             string
	info                            string
	replacing, noEmpty, trace, exit bool
	n, lines, size                  int
}

func parseXargs(args []string) (xargsOptions, []string, error) {
	var o xargsOptions
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
					if c == 'P' && (err != nil || n != 1) {
						return o, nil, fmt.Errorf("-P %s is unsupported (parallel execution is deferred to U09)", v)
					}
					if err != nil || n <= 0 {
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
						if n != 1 {
							return o, nil, fmt.Errorf("parallel execution is unsupported")
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
		fmt.Fprintln(ctx.Stderr, "xargs: command line too long")
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
	tokenizer := newXargsTokenizer(ctx.CancellationContext(), ContextReader(ctx.CancellationContext(), r), o.delim, o.eof, o.replacing)
	var pending *xargsToken
	done, ran := false, false
	result := 0
	for {
		if ctx.CancellationContext().Err() != nil {
			return ExitStatus(ctx.CancellationContext().Err())
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
						return ExitStatus(ctx.CancellationContext().Err())
					}
					fmt.Fprintln(ctx.Stderr, "xargs:", err)
					return 1
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
					fmt.Fprintln(ctx.Stderr, "xargs: command line too long")
					return 1
				}
				count++
				break
			}
			next := len(tok.text) + 1
			if size+next > budget {
				if count == 0 || o.exit {
					fmt.Fprintln(ctx.Stderr, "xargs: argument list too long")
					return 1
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
		if o.trace {
			// Like Toybox, -t prints each word followed by a space.
			var line strings.Builder
			for _, w := range append([]string{argv[0]}, childArgs...) {
				line.WriteString(w)
				line.WriteByte(' ')
			}
			fmt.Fprintln(ctx.Stderr, line.String())
		}
		sub := *ctx
		sub.Args, sub.CmdName = childArgs, argv[0]
		if o.input == "" {
			// Children must not consume xargs's own input stream; with -a
			// the input comes from the file, so they inherit stdin.
			sub.Stdin, sub.StdinTTY = strings.NewReader(""), false
		}
		status := xargsExec(ctx, argv[0], &sub)
		ran = true
		if ctx.CancellationContext().Err() != nil {
			return ExitStatus(ctx.CancellationContext().Err())
		}
		switch {
		case status == 0:
		case status == 255:
			fmt.Fprintf(ctx.Stderr, "xargs: %s: exited with status 255; aborting\n", argv[0])
			return 124
		case status == 126 || status == 127:
			return status
		case status >= 128:
			return 125
		default:
			result = 123
		}
		if done {
			break
		}
	}
	return result
}

func xargsExec(ctx *Context, name string, sub *Context) int {
	if ctx.Exec != nil {
		return ctx.Exec(ctx.CancellationContext(), name, sub)
	}
	if !strings.ContainsAny(name, `/\`) {
		if _, ok := Lookup(name); ok {
			return proceffect.Run(ctx.CancellationContext(), proceffect.Request{DispatchOnly: true, Argv: append([]string{name}, sub.Args...)}, proceffect.Invocation{Dir: sub.Dir, Env: sub.Env, Stdin: sub.Stdin, Stdout: sub.Stdout, Stderr: sub.Stderr})
		}
	}
	p, err := commandexec.Start(ctx.CancellationContext(), name, commandexec.Invocation{Dir: sub.Dir, Env: sub.Env, Stdin: sub.Stdin, Stdout: sub.Stdout, Stderr: sub.Stderr, Args: sub.Args, Argv0: name}, commandexec.Options{})
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "xargs:", err)
		return commandexec.ErrorStatus(err)
	}
	return p.Wait()
}

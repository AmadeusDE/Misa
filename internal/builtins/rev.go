package builtins

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func init() {
	Register(Command{Name: "rev", Run: runRev})
}

func revLine(line []byte) []byte {
	runes := []rune(string(line))
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return []byte(string(runes))
}

func runRev(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	args := ctx.Args
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	files := args
	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "rev: missing operand")
			return 1
		}
		files = []string{"-"}
	}

	exitCode := 0
	for _, name := range files {
		var r io.Reader
		if name == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "rev: stdin not available")
				exitCode = 1
				continue
			}
			r = ctx.Stdin
		} else {
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(ctx.Dir, path)
			}
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "rev: %s: %s\n", name, fmtOsErr(err))
				exitCode = 1
				continue
			}
			defer f.Close()
			r = f
		}

		data, err := io.ReadAll(ContextReader(ctx.CancellationContext(), r))
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "rev: %v\n", err)
			exitCode = 1
			continue
		}
		endsWithNewline := len(data) > 0 && data[len(data)-1] == '\n'
		lines := bytes.Split(data, []byte{'\n'})
		if endsWithNewline && len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
			lines = lines[:len(lines)-1]
		}
		for i, line := range lines {
			if _, err := ctx.Stdout.Write(revLine(line)); err != nil {
				return 1
			}
			if i < len(lines)-1 || endsWithNewline {
				if _, err := ctx.Stdout.Write([]byte{'\n'}); err != nil {
					return 1
				}
			}
		}
	}
	return exitCode
}

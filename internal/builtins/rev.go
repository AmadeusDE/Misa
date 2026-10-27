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

func runRev(ctx *Context) int {
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

		data, err := io.ReadAll(r)
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
			ctx.Stdout.Write(revLine(line))
			if i < len(lines)-1 || endsWithNewline {
				ctx.Stdout.Write([]byte{'\n'})
			}
		}
	}
	return exitCode
}

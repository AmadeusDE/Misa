package builtins

import (
	"fmt"
	"io"
	"os"
)

const teeSpec = "a(append)"

var teeSpecParsed *Spec

func init() {
	var err error
	teeSpecParsed, err = ParseSpec(teeSpec)
	if err != nil {
		panic("builtins: bad tee spec: " + err.Error())
	}
	Register(Command{Name: "tee", Run: runTee})
}

func runTee(ctx *Context) int {
	flags, err := teeSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "tee:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, teeSpecParsed) {
		return 0
	}

	if ctx.Stdin == nil {
		fmt.Fprintln(ctx.Stderr, "tee: stdin not available")
		return 1
	}

	appendMode := flags.Bool("a")

	openFlag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendMode {
		openFlag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	var writers []io.Writer
	writers = append(writers, ctx.Stdout)

	exitCode := 0
	for _, name := range flags.Operands {
		path := resolvePath(ctx, name)
		f, err := os.OpenFile(path, openFlag, 0644)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "tee: %s: %s\n", name, fmtOsErr(err))
			exitCode = 1
			continue
		}
		defer f.Close()
		writers = append(writers, f)
	}

	w := io.MultiWriter(writers...)
	if _, err := io.Copy(w, ctx.Stdin); err != nil {
		fmt.Fprintf(ctx.Stderr, "tee: %v\n", err)
		return 1
	}
	return exitCode
}

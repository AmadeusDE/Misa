package builtins

import (
	"fmt"
	"path/filepath"
	"strings"
)

const basenameSpec = "^a(multiple)s(suffix):z(zero)"

var basenameSpecParsed *Spec

func init() {
	var err error
	basenameSpecParsed, err = ParseSpec(basenameSpec)
	if err != nil {
		panic("builtins: bad basename spec: " + err.Error())
	}
	Register(Command{Name: "basename", Run: runBasename})
	Register(Command{Name: "dirname", Run: runDirname})
}

func stripSuffix(name, suffix string) string {
	if suffix != "" && name != suffix && strings.HasSuffix(name, suffix) {
		return name[:len(name)-len(suffix)]
	}
	return name
}

func runBasename(ctx *Context) int {
	flags, err := basenameSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "basename:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, basenameSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "basename: missing operand")
		return 1
	}

	suffix, hasSuffix := flags.String("s")
	multi := flags.Bool("a") || hasSuffix
	zeroTerm := flags.Bool("z")
	terminator := "\n"
	if zeroTerm {
		terminator = "\x00"
	}

	if !multi && len(flags.Operands) > 2 {
		fmt.Fprintf(ctx.Stderr, "basename: extra operand '%s'\n", flags.Operands[2])
		return 1
	}

	if !multi && len(flags.Operands) == 2 {
		name := filepath.Base(flags.Operands[0])
		fmt.Fprint(ctx.Stdout, stripSuffix(name, flags.Operands[1]))
		fmt.Fprint(ctx.Stdout, terminator)
		return 0
	}

	for _, arg := range flags.Operands {
		name := filepath.Base(arg)
		fmt.Fprint(ctx.Stdout, stripSuffix(name, suffix))
		fmt.Fprint(ctx.Stdout, terminator)
	}
	return 0
}

const dirnameSpec = "z(zero)"

var dirnameSpecParsed *Spec

func init() {
	var err error
	dirnameSpecParsed, err = ParseSpec(dirnameSpec)
	if err != nil {
		panic("builtins: bad dirname spec: " + err.Error())
	}
}

func runDirname(ctx *Context) int {
	flags, err := dirnameSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "dirname:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, dirnameSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "dirname: missing operand")
		return 1
	}
	terminator := "\n"
	if flags.Bool("z") {
		terminator = "\x00"
	}
	for _, arg := range flags.Operands {
		fmt.Fprint(ctx.Stdout, posixDirname(arg))
		fmt.Fprint(ctx.Stdout, terminator)
	}
	return 0
}

func posixDirname(path string) string {
	if path == "" {
		return "."
	}

	i := len(path) - 1
	for i > 0 && path[i] == '/' {
		i--
	}
	path = path[:i+1]

	if path == "" {
		return "/"
	}

	i = len(path) - 1
	for i >= 0 && path[i] != '/' {
		i--
	}

	if i < 0 {
		return "."
	}

	for i > 0 && path[i] == '/' {
		i--
	}

	if path[i] == '/' {
		return path[:i+1]
	}
	return path[:i+1]
}

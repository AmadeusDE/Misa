package builtins

import (
	"fmt"
	"os"
)

const mkdirSpec = "p(parents)v(verbose)m(mode):"

var mkdirSpecParsed *Spec

func init() {
	var err error
	mkdirSpecParsed, err = ParseSpec(mkdirSpec)
	if err != nil {
		panic("builtins: bad mkdir spec: " + err.Error())
	}
	Register(Command{Name: "mkdir", Run: runMkdir})
}

func runMkdir(ctx *Context) int {
	flags, err := mkdirSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "mkdir:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, mkdirSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "mkdir: error: the following required arguments were not provided:\n  <DIRECTORY>...")
		return 1
	}

	perm := os.FileMode(0777)
	hasMode := false
	if modeStr, ok := flags.String("m"); ok {
		p, err := parseMkdirMode(modeStr)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "mkdir:", err)
			return 1
		}
		perm = p
		hasMode = true
	}

	parents := flags.Bool("p")
	verbose := flags.Bool("v")
	exitCode := 0

	for _, dir := range flags.Operands {
		if dir == "" {
			fmt.Fprintf(ctx.Stderr, "mkdir: cannot create directory '': No such file or directory\n")
			exitCode = 1
			continue
		}
		path := resolvePath(ctx, dir)
		if parents {
			if err := mkdirParents(ctx, dir, path, perm, hasMode, verbose); err != nil {
				fmt.Fprintf(ctx.Stderr, "mkdir: cannot create directory '%s': %s\n", dir, fmtOsErr(err))
				exitCode = 1
			}
		} else {
			err := os.Mkdir(path, perm)
			if err == nil && hasMode {
				os.Chmod(path, perm)
			}
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "mkdir: cannot create directory '%s': %s\n", dir, fmtOsErr(err))
				exitCode = 1
				continue
			}
			if verbose {
				fmt.Fprintf(ctx.Stdout, "mkdir: created directory '%s'\n", dir)
			}
		}
	}
	return exitCode
}

func parseMkdirMode(spec string) (os.FileMode, error) {
	isOctal := true
	for _, c := range spec {
		if c < '0' || c > '7' {
			isOctal = false
			break
		}
	}
	if isOctal {
		return parseOctalMode(spec)
	}
	return parseSymbolicMode(spec, os.ModeDir|0777)
}

package builtins

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

const pathchkSpec = "p(portability)P(portability-extra)"

var pathchkSpecParsed *Spec

func init() {
	var err error
	pathchkSpecParsed, err = ParseSpec(pathchkSpec)
	if err != nil {
		panic("builtins: bad pathchk spec: " + err.Error())
	}
	Register(Command{Name: "pathchk", Run: runPathchk})
}

const (
	pathchkPATHMAX = 4096
	pathchkNAMEMAX = 255
	posixPortable  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-"
)

func runPathchk(ctx *Context) int {
	flags, err := pathchkSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "pathchk:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, pathchkSpecParsed) {
		return 0
	}
	portability := flags.Bool("p")
	extra := flags.Bool("P")

	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "pathchk: error: the following required arguments were not provided:\n  <OPERAND>...")
		return 1
	}

	rc := 0
	for _, path := range flags.Operands {
		if !pathchkCheck(ctx, path, portability, extra) {
			rc = 1
		}
	}
	return rc
}

func pathchkCheck(ctx *Context, path string, portability, extra bool) bool {
	if path == "" {
		fmt.Fprintln(ctx.Stderr, "pathchk: '': No such file or directory")
		return false
	}

	ok := true

	if portability || extra {
		if len(path) > pathchkPATHMAX {
			fmt.Fprintf(ctx.Stderr, "pathchk: %q: pathname too long (%d > %d)\n", path, len(path), pathchkPATHMAX)
			ok = false
		}
		components := strings.Split(path, "/")
		for _, comp := range components {
			if comp == "" {
				continue
			}
			if utf8.RuneCountInString(comp) > pathchkNAMEMAX {
				fmt.Fprintf(ctx.Stderr, "pathchk: %q: component too long\n", comp)
				ok = false
			}
			if portability {
				for _, r := range comp {
					if !strings.ContainsRune(posixPortable, r) {
						fmt.Fprintf(ctx.Stderr, "pathchk: %q: nonportable character %q\n", path, string(r))
						ok = false
						break
					}
				}
			}
			if extra && strings.HasPrefix(comp, "-") {
				fmt.Fprintf(ctx.Stderr, "pathchk: %q: leading hyphen in component %q\n", path, comp)
				ok = false
			}
		}
	} else {
		resolved := resolvePath(ctx, path)
		if _, err := os.Stat(resolved); err != nil {
			if pe, isPathErr := err.(*os.PathError); isPathErr && strings.Contains(pe.Err.Error(), "not a directory") {
				fmt.Fprintf(ctx.Stderr, "pathchk: '%s': Not a directory\n", path)
				return false
			}
		}
	}
	return ok
}

// Behaviour adapted from toybox link.c (0BSD) and uutils coreutils link (MIT).
package builtins

import (
	"fmt"
	"os"
)

func init() {
	Register(Command{Name: "link", Run: runLink})
}

func runLink(ctx *Context) int {
	spec, _ := ParseSpec("")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "link:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	if len(flags.Operands) != 2 {
		fmt.Fprintln(ctx.Stderr, "link: 2 values required\nUsage: link FILE1 FILE2")
		return 1
	}
	old, new := flags.Operands[0], flags.Operands[1]
	oldPath, newPath := old, new
	if old != "" {
		oldPath = resolvePath(ctx, old)
	}
	if new != "" {
		newPath = resolvePath(ctx, new)
	}
	if err := os.Link(oldPath, newPath); err != nil {
		fmt.Fprintf(ctx.Stderr, "link: cannot create link '%s' to '%s': %s\n", new, old, fmtOsErr(err))
		return 1
	}
	return 0
}

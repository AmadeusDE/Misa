// Behaviour adapted from uutils coreutils sum (MIT).
package builtins

import (
	"fmt"
	"io"
)

func init() { Register(Command{Name: "sum", Run: runSum}) }

func runSum(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	spec, _ := ParseSpec("rs(sysv)")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "sum:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}
	status := 0
	for _, name := range files {
		var data []byte
		if name == "-" {
			if ctx.Stdin == nil {
				err = fmt.Errorf("stdin not available")
			} else {
				data, err = io.ReadAll(ctx.Stdin)
			}
		} else {
			data, err = ReadFileContext(ctx.CancellationContext(), resolvePath(ctx, name))
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "sum: %s: %s\n", name, fmtOsErr(err))
			status = 1
			continue
		}
		if flags.Bool("s") {
			fmt.Fprintf(ctx.Stdout, "%d %d", sysvCksum(data), (len(data)+511)/512)
		} else {
			fmt.Fprintf(ctx.Stdout, "%05d %5d", bsdCksum(data), (len(data)+1023)/1024)
		}
		if len(files) > 1 || name != "-" {
			fmt.Fprintf(ctx.Stdout, " %s", name)
		}
		fmt.Fprintln(ctx.Stdout)
	}
	return status
}

//go:build !unix

package builtins

import "fmt"

func init() {
	Register(Command{Name: "mkfifo", Run: runMkfifoUnsupported})
}

// runMkfifoUnsupported keeps mkfifo registered so the user gets the reason
// recorded in support_table.go instead of "command not found".
func runMkfifoUnsupported(ctx *Context) int {
	spec, _ := ParseSpec("m(mode):")
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "mkfifo:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	status, _ := refuseUnsupported(ctx, "mkfifo")
	return status
}

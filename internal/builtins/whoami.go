package builtins

import (
	"fmt"
	"os/user"
)

const whoamiSpec = ""

var whoamiSpecParsed *Spec

func init() {
	var err error
	whoamiSpecParsed, err = ParseSpec(whoamiSpec)
	if err != nil {
		panic("builtins: bad whoami spec: " + err.Error())
	}
	Register(Command{Name: "whoami", Run: runWhoami})
}

func runWhoami(ctx *Context) int {
	flags, err := whoamiSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "whoami:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, whoamiSpecParsed) {
		return 0
	}
	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "whoami: %v\n", err)
		return 1
	}
	fmt.Fprintln(ctx.Stdout, u.Username)
	return 0
}

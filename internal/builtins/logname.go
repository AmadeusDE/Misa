package builtins

import (
	"fmt"
	"os"
	"os/user"
)

const lognameSpec = ""

var lognameSpecParsed *Spec

func init() {
	var err error
	lognameSpecParsed, err = ParseSpec(lognameSpec)
	if err != nil {
		panic("builtins: bad logname spec: " + err.Error())
	}
	Register(Command{Name: "logname", Run: runLogname})
}

func runLogname(ctx *Context) int {
	flags, err := lognameSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "logname:", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, "Usage: logname [OPTION]...\nPrint user's login name.\n")
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "logname (Misa) 0.1.0\n")
		return 0
	}
	name := os.Getenv("LOGNAME")
	if name == "" {
		if u, err := user.Current(); err == nil {
			name = u.Username
		}
	}
	if name == "" {
		fmt.Fprintln(ctx.Stderr, "logname: no login name")
		return 1
	}
	fmt.Fprintln(ctx.Stdout, name)
	return 0
}

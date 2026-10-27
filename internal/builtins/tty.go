package builtins

import "fmt"

const ttySpec = "s(silent)q(quiet)"

var ttySpecParsed *Spec

func init() {
	var err error
	ttySpecParsed, err = ParseSpec(ttySpec)
	if err != nil {
		panic("builtins: bad tty spec: " + err.Error())
	}
	Register(Command{Name: "tty", Run: runTty})
}

func runTty(ctx *Context) int {
	flags, err := ttySpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "tty:", err)
		return 2
	}
	if HandleHelpVersion(ctx, flags, ttySpecParsed) {
		return 0
	}
	if len(flags.Operands) > 0 {
		fmt.Fprintf(ctx.Stderr, "tty: extra operand '%s'\nTry 'tty --help' for more information.\n", flags.Operands[0])
		return 2
	}

	silent := flags.Bool("s") || flags.Bool("q")

	if !silent {
		if _, err := fmt.Fprintln(ctx.Stdout, "not a tty"); err != nil {
			return 3
		}
	}
	return 1
}

package builtinexec

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/proceffect"
	"golang.org/x/term"
)

// ProcessEffect handles the private process-effect protocol in a re-executed child.
// args must exclude the private flag.
func ProcessEffect(args []string) int {
	return proceffect.HandleContext(args, func(ctx context.Context, argv []string) int {
		if len(argv) == 0 {
			return proceffect.Exec(argv)
		}
		name := argv[0]
		if _, ok := builtins.Lookup(name); !ok || strings.ContainsAny(name, `/\`) {
			return proceffect.Exec(argv)
		}
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return builtins.ExitInternal
		}
		width, height, _ := term.GetSize(int(os.Stdout.Fd()))
		inv := &builtins.Context{Dir: dir, Args: argv[1:], CmdName: name,
			Width: width, Height: height, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
			Env: os.Environ(), Exec: For(name),
			StdinTTY: term.IsTerminal(int(os.Stdin.Fd())), StdoutTTY: term.IsTerminal(int(os.Stdout.Fd())), StderrTTY: term.IsTerminal(int(os.Stderr.Fd())),
		}
		return builtins.Run(ctx, name, inv)
	})
}

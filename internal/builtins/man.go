package builtins

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/help"
)

const (
	manSpec  = "^"
	tldrSpec = "^u(update)"
)

var (
	manSpecParsed  *Spec
	tldrSpecParsed *Spec
)

func init() {
	var err error
	manSpecParsed, err = ParseSpec(manSpec)
	if err != nil {
		panic("builtins: bad man spec: " + err.Error())
	}
	tldrSpecParsed, err = ParseSpec(tldrSpec)
	if err != nil {
		panic("builtins: bad tldr spec: " + err.Error())
	}
	Register(Command{Name: "man", Run: runMan})
	Register(Command{Name: "tldr", Run: runTldr})
}

func runMan(ctx *Context) int {
	flags, err := manSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "man:", err)
		return 1
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "man: missing page name")
		return 1
	}

	command := flags.Operands[0]
	path, ok := help.FindManPage(command)
	if !ok {
		fmt.Fprintf(ctx.Stderr, "man: no manual entry for %s\n", command)
		return 1
	}

	content, err := help.ParseManPage(path)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "man: %v\n", err)
		return 1
	}

	lipgloss.Fprintln(ctx.Stdout, help.RenderMarkdown(content))
	return 0
}

func runTldr(ctx *Context) int {
	flags, err := tldrSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "tldr:", err)
		return 1
	}

	if flags.Bool("u") {
		fmt.Fprintln(ctx.Stderr, "tldr: updating pages...")
		if err := help.EnsureTldr(); err != nil {
			fmt.Fprintf(ctx.Stderr, "tldr: %v\n", err)
			return 1
		}
		if len(flags.Operands) == 0 {
			fmt.Fprintln(ctx.Stderr, "tldr: pages updated")
			return 0
		}
	}

	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "tldr: missing page name")
		return 1
	}

	command := flags.Operands[0]
	content, err := help.GetTldrPage(command)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "tldr: %v\n", err)
		return 1
	}

	lipgloss.Fprintln(ctx.Stdout, help.RenderMarkdown(content))
	return 0
}

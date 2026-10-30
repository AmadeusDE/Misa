package aihelp

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/help"
)

var spec = mustSpec("^n(no-cache)(raw)m(model):(fix)(clear-cache)")

func mustSpec(s string) *builtins.Spec {
	sp, err := builtins.ParseSpec(s)
	if err != nil {
		panic(err)
	}
	return sp
}

const usage = `usage: aihelp [--model provider/model] [-n|--no-cache] [--raw] <command> [question...]
       aihelp [options] <question...>
       aihelp --fix "<failed command>" [exit status]   (error output may be piped in)
       aihelp --clear-cache

Answers with the model chosen for help (run aiconfig to pick one). It can read man and tldr pages and
the Arch Wiki. --raw prints the Markdown as is.
`

// Run is the aihelp command.
func Run(ctx *builtins.Context) int {
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "aihelp:", err)
		return 2
	}
	if builtins.HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	if flags.Bool("clear-cache") {
		if err := DefaultCache().Clear(); err != nil {
			fmt.Fprintln(ctx.Stderr, "aihelp:", err)
			return 1
		}
		fmt.Fprintln(ctx.Stdout, "Response cache cleared.")
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprint(ctx.Stderr, usage)
		return 2
	}
	model, _ := flags.String("model")
	h, err := New(model)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "aihelp:", err)
		return 1
	}
	h.Notice = ctx.Stderr
	if flags.Bool("fix") {
		return runFix(ctx, h, flags.Operands)
	}
	q := splitQuery(flags.Operands, isCommand)
	q.NoCache = flags.Bool("no-cache")
	return runAsk(ctx, h, q, !flags.Bool("raw"))
}

func runAsk(ctx *builtins.Context, h *Helper, q Query, styled bool) int {
	width := ctx.Width
	if width <= 0 {
		width = Width(ctx.Stdout)
	}
	p := NewPrinter(ctx.Stdout, styled && IsTTY(ctx.Stdout), width)
	_, err := h.Ask(context.Background(), q, p.Write)
	p.Close()
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "aihelp:", err)
		return 1
	}
	if !p.styled() {
		fmt.Fprintln(ctx.Stdout)
	}
	return 0
}

func runFix(ctx *builtins.Context, h *Helper, operands []string) int {
	exit := 1
	if len(operands) > 1 {
		n, err := strconv.Atoi(operands[len(operands)-1])
		if err == nil {
			exit, operands = n, operands[:len(operands)-1]
		}
	}
	cmd := strings.Join(operands, " ")
	fixed, why, err := h.Fix(context.Background(), cmd, pipedInput(ctx.Stdin), exit)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "aihelp:", err)
		return 1
	}
	fmt.Fprintln(ctx.Stdout, fixed)
	if why != "" {
		fmt.Fprintf(ctx.Stderr, "\n%s\n", why)
	}
	return 0
}

// pipedInput is what was piped to stdin, the failed command's error output; a terminal gives nothing.
func pipedInput(in io.Reader) string {
	f, ok := in.(*os.File)
	if !ok || f == nil {
		return ""
	}
	if info, err := f.Stat(); err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return ""
	}
	data, _ := io.ReadAll(io.LimitReader(f, 1<<16))
	return string(data)
}

// splitQuery reads `<command> <question...>` or a bare question: the first word is the command only when it
// names one that exists.
func splitQuery(operands []string, isCmd func(string) bool) Query {
	first := operands[0]
	if !strings.ContainsAny(first, " \t") && isCmd(first) {
		return Query{Command: first, Question: strings.Join(operands[1:], " ")}
	}
	return Query{Question: strings.Join(operands, " ")}
}

func isCommand(name string) bool {
	if _, ok := builtins.Lookup(name); ok {
		return true
	}
	if _, err := exec.LookPath(name); err == nil {
		return true
	}
	_, ok := help.FindManPage(name)
	return ok
}

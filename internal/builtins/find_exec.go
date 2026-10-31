// Execution action semantics informed by uutils findutils exec.rs, commit
// d71f3242b0bc3b21bb311f04b23ae62e5f3fed67 (MIT), and Toybox find.c,
// commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
package builtins

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

const findExecBudget = 32 * 1024

type findExec struct {
	ctx             *Context
	argv            []string
	dir, batch      bool
	pending         []string
	pendingDir      string
	pendingValidate []func() error
	failed          bool
}

func (p *findParser) execAction(name string) (findNode, error) {
	start := p.pos
	for p.pos < len(p.args) {
		if p.args[p.pos] == ";" || (p.args[p.pos] == "+" && p.pos > start && p.args[p.pos-1] == "{}") {
			break
		}
		p.pos++
	}
	if p.pos == len(p.args) {
		return nil, fmt.Errorf("%s: missing terminator", name)
	}
	argv := append([]string(nil), p.args[start:p.pos]...)
	term := p.args[p.pos]
	p.pos++
	if len(argv) == 0 || argv[0] == "" {
		return nil, fmt.Errorf("%s: missing command", name)
	}
	if term == "+" {
		count := 0
		for _, a := range argv {
			count += strings.Count(a, "{}")
		}
		if len(argv) < 2 || argv[len(argv)-1] != "{}" || count != 1 {
			return nil, fmt.Errorf("%s: + requires one trailing {}", name)
		}
		argv = argv[:len(argv)-1]
	}
	x := &findExec{ctx: p.ctx, argv: argv, dir: name == "-execdir", batch: term == "+"}
	p.execs = append(p.execs, x)
	p.action = true
	return x.eval, nil
}

// flushExecDir runs pending execdir batches before a postorder directory action.
// Failures remain recorded for flushExec's final status.
func (p *findParser) flushExecDir(dir string) {
	for _, x := range p.execs {
		if x.dir && x.batch && x.pendingDir == dir {
			x.flush()
		}
	}
}

func (p *findParser) flushExec() bool {
	failed := false
	for _, x := range p.execs {
		x.flush()
		failed = failed || x.failed
	}
	return failed
}

func (x *findExec) size(args []string) int {
	// Include NULs and 64-bit pointer slots, including terminating pointers.
	n := 16
	for _, s := range x.ctx.Environ() {
		n += len(s) + 1 + 8
	}
	for _, s := range args {
		n += len(s) + 1 + 8
	}
	return n
}
func (x *findExec) eval(e *findEntry) bool {
	name, dir := e.path, x.ctx.Dir
	var validate func() error
	if x.dir {
		var err error
		dir, name, validate, err = findExecDir(e, x.ctx)
		if err != nil {
			return e.fail(err)
		}
	}
	if !x.batch {
		if validate != nil {
			if err := validate(); err != nil {
				return e.fail(err)
			}
		}
		args := make([]string, len(x.argv))
		for i, a := range x.argv {
			args[i] = strings.ReplaceAll(a, "{}", name)
		}
		status := x.run(dir, args)
		if err := x.ctx.CancellationContext().Err(); err != nil {
			e.fail(err)
		}
		return status == 0
	}
	if len(x.pending) > 0 && x.dir && x.pendingDir != dir {
		x.flush()
	}
	args := append(append([]string(nil), x.argv...), x.pending...)
	args = append(args, name)
	if x.size(args) > findExecBudget && len(x.pending) > 0 {
		x.flush()
		args = append(append([]string(nil), x.argv...), name)
	}
	if x.size(args) > findExecBudget {
		fmt.Fprintln(x.ctx.Stderr, "find: -exec: argument list exceeds 32 KiB budget")
		x.failed = true
		return true
	}
	if validate != nil {
		x.pendingValidate = append(x.pendingValidate, validate)
	}
	x.pendingDir = dir
	x.pending = append(x.pending, name)
	return true
}
func (x *findExec) flush() {
	if len(x.pending) == 0 {
		return
	}
	args := append(append([]string(nil), x.argv...), x.pending...)
	x.pending = nil
	validate := x.pendingValidate
	x.pendingValidate = nil
	for _, check := range validate {
		if err := check(); err != nil {
			fmt.Fprintf(x.ctx.Stderr, "find: -execdir: %v\n", err)
			x.failed = true
			return
		}
	}
	if x.run(x.pendingDir, args) != 0 {
		x.failed = true
	}
}
func (x *findExec) run(dir string, args []string) int {
	ctx := x.ctx
	name := args[0]
	if err := ctx.CancellationContext().Err(); err != nil {
		return ExitStatus(err)
	}
	bare := !strings.ContainsAny(name, `/\`)
	builtin := false
	if bare {
		_, builtin = Lookup(name)
	}
	if x.dir && bare && !builtin {
		path, ok := ctx.LookupEnv("PATH")
		if ok {
			for _, part := range strings.Split(path, ":") {
				if !filepath.IsAbs(part) {
					fmt.Fprintln(ctx.Stderr, "find: -execdir: insecure PATH component")
					x.failed = true
					return 1
				}
			}
		}
	}
	sub := *ctx
	sub.Dir = dir
	sub.Args = append([]string(nil), args[1:]...)
	sub.CmdName = name
	if ctx.Exec != nil {
		return ctx.Exec(ctx.CancellationContext(), name, &sub)
	}
	if builtin {
		return proceffect.Run(ctx.CancellationContext(), proceffect.Request{
			DispatchOnly: true, Argv: append([]string{name}, sub.Args...),
		}, proceffect.Invocation{Dir: dir, Env: ctx.Environ(), Stdin: ctx.Stdin, Stdout: ctx.Stdout, Stderr: ctx.Stderr})
	}
	child, err := commandexec.Start(ctx.CancellationContext(), name, commandexec.Invocation{Dir: dir, Env: ctx.Environ(), Stdin: ctx.Stdin, Stdout: ctx.Stdout, Stderr: ctx.Stderr, Args: sub.Args, Argv0: name}, commandexec.Options{})
	if err != nil {
		if ctx.CancellationContext().Err() != nil {
			return ExitStatus(ctx.CancellationContext().Err())
		}
		fmt.Fprintf(ctx.Stderr, "find: %s: %v\n", name, err)
		return commandexec.ErrorStatus(err)
	}
	return child.Wait()
}

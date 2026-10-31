package ui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/builtinexec"
	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/jobleader"
	"github.com/AmadeusDE/Misa/internal/term"
	xterm "golang.org/x/term"
)

// BuiltinExecutable is the multicall binary builtins re-exec through
// (`misa <builtin> args`). main sets it; when empty, builtins run in-process.
var BuiltinExecutable string

// builtinModeInProcess runs builtins on a goroutine instead of re-executing
// the binary. Both modes run without /bin/sh.
const builtinModeInProcess = "inprocess"

// commandPlan is how one command line runs: as a process (argv), or as a
// builtin on a goroutine of this process.
type commandPlan struct {
	argv    []string
	builtin builtins.Command
	fields  []string
}

func (p commandPlan) inProcess() bool { return p.argv == nil }

func shellPlan(command string) commandPlan {
	return commandPlan{argv: []string{"sh", "-c", command}}
}

// planCommand runs plain commands directly and leaves only real shell syntax
// to sh, so a system without a shell still runs builtins and programs.
func (m *Model) planCommand(command string) commandPlan {
	fields := strings.Fields(command)
	if len(fields) == 0 || needsShell(command) || strings.Contains(fields[0], "=") {
		return shellPlan(command)
	}
	if b, ok := builtins.Lookup(fields[0]); ok {
		if m.builtinMode != builtinModeInProcess && BuiltinExecutable != "" {
			return commandPlan{argv: append([]string{BuiltinExecutable}, fields...)}
		}
		return commandPlan{builtin: b, fields: fields}
	}
	if !strings.Contains(fields[0], "/") {
		if _, err := exec.LookPath(fields[0]); err == nil {
			return commandPlan{argv: fields}
		}
	}
	return shellPlan(command)
}

func runBuiltin(p commandPlan, dir string, stdin io.Reader, out io.Writer) int {
	return runBuiltinSized(p, dir, stdin, out, 0, 0)
}

func runBuiltinSized(p commandPlan, dir string, stdin io.Reader, out io.Writer, width, height int) int {
	return runBuiltinContext(context.Background(), p, dir, stdin, out, width, height)
}

func runBuiltinContext(ctx context.Context, p commandPlan, dir string, stdin io.Reader, out io.Writer, width, height int) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(out, "%s: internal error: %v\n", p.fields[0], r)
			code = 2
		}
	}()
	return builtins.Run(ctx, p.fields[0], &builtins.Context{
		Exec:      builtinexec.For(p.fields[0]),
		Width:     width,
		Height:    height,
		StdinTTY:  builtinStreamTTY(stdin),
		StdoutTTY: builtinStreamTTY(out),
		StderrTTY: builtinStreamTTY(out),
		Dir:       dir,
		Args:      p.fields[1:],
		CmdName:   p.fields[0],
		Stdout:    out,
		Stderr:    out,
		Stdin:     stdin,
	})
}

func builtinStreamTTY(stream any) bool {
	file, ok := stream.(*os.File)
	return ok && xterm.IsTerminal(int(file.Fd()))
}

// underJobLeader runs a program as a job of the multicall binary, which gives
// it its own process group so ^Z can suspend it. Builtins suspend themselves.
func underJobLeader(argv []string) []string {
	if BuiltinExecutable == "" || argv[0] == BuiltinExecutable {
		return argv
	}
	return append([]string{BuiltinExecutable, jobleader.Flag}, argv...)
}

func (m *Model) startPlan(p commandPlan, rows, cols int) (*term.Terminal, tea.Cmd, error) {
	t := term.New(rows, cols)
	var err error
	switch {
	case p.inProcess():
		err = t.StartFuncContext(context.Background(), func(ctx context.Context, tty *os.File) int {
			return runBuiltinContext(ctx, p, m.dir, tty, tty, cols, rows)
		})
	case m.jobControl:
		err = t.StartArgv(m.dir, underJobLeader(p.argv)...)
	default:
		err = t.StartArgv(m.dir, p.argv...)
	}
	if err != nil {
		return nil, nil, err
	}
	t.StartReader()
	return t, waitTermUpdate(t), nil
}

// runCaptured runs a command without a terminal and reports its output as a
// shellOutput, for autorun.
func runCaptured(dir string, p commandPlan) tea.Cmd {
	return func() tea.Msg {
		var out bytes.Buffer
		if p.inProcess() {
			code := runBuiltin(p, dir, strings.NewReader(""), &out)
			return shellOutput{output: out.String(), exitCode: code}
		}
		cmd := exec.Command(p.argv[0], p.argv[1:]...)
		cmd.Dir = dir
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		return shellOutput{output: out.String(), err: err, exitCode: term.ExitStatus(err)}
	}
}

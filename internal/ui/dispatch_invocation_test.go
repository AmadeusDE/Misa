package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/creack/pty"
)

func TestRunBuiltinInvocation(t *testing.T) {
	const name = "f02-ui-invocation"
	builtins.Register(builtins.Command{Name: name, Run: func(inv *builtins.Context) int {
		if inv.CmdName != name || inv.Width != 91 || inv.Height != 27 {
			t.Errorf("invocation name/size = %q %dx%d", inv.CmdName, inv.Width, inv.Height)
		}
		if inv.StdinTTY || inv.StdoutTTY || inv.StderrTTY {
			t.Error("captured streams marked as terminals")
		}
		if inv.Exec == nil || inv.CancellationContext() == nil {
			t.Error("missing execution defaults")
		}
		fmt.Fprint(inv.Stdout, "output")
		fmt.Fprint(inv.Stderr, "error")
		return 7
	}})
	command, _ := builtins.Lookup(name)
	var out bytes.Buffer
	code := runBuiltinSized(commandPlan{builtin: command, fields: []string{name}}, t.TempDir(), strings.NewReader(""), &out, 91, 27)
	if code != 7 || out.String() != "outputerror" {
		t.Fatalf("result = %d, %q", code, out.String())
	}
}

func TestRunBuiltinTerminalMetadata(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("PTY unavailable: %v", err)
	}
	defer master.Close()
	defer slave.Close()
	const name = "f02-ui-terminal"
	builtins.Register(builtins.Command{Name: name, Run: func(inv *builtins.Context) int {
		if !inv.StdinTTY || !inv.StdoutTTY || !inv.StderrTTY {
			t.Error("PTY streams not marked terminals")
		}
		if inv.Width != 80 || inv.Height != 24 {
			t.Errorf("size = %dx%d", inv.Width, inv.Height)
		}
		return 0
	}})
	command, _ := builtins.Lookup(name)
	if code := runBuiltinSized(commandPlan{builtin: command, fields: []string{name}}, t.TempDir(), slave, slave, 80, 24); code != 0 {
		t.Errorf("status = %d", code)
	}
}

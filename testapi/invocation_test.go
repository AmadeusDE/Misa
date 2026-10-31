package testapi

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func TestLookupUsesInvocationContract(t *testing.T) {
	const name = "f02-testapi-invocation"
	builtins.Register(builtins.Command{Name: name, Run: func(inv *builtins.Context) int {
		if inv.CmdName != "multicall-alias" {
			t.Errorf("name = %q", inv.CmdName)
		}
		if inv.Exec == nil || inv.CancellationContext() == nil {
			t.Error("missing execution defaults")
		}
		fmt.Fprint(inv.Stdout, "out")
		fmt.Fprint(inv.Stderr, "err")
		return 3
	}})
	cmd, ok := Lookup(name)
	if !ok {
		t.Fatal("registered command missing")
	}
	var out, stderr bytes.Buffer
	inv := NewContext(".", nil, "multicall-alias", 80, 24, &out, &stderr, nil)
	if code := cmd.Run(inv); code != 3 {
		t.Errorf("status = %d", code)
	}
	if out.String() != "out" || stderr.String() != "err" {
		t.Errorf("streams = %q, %q", out.String(), stderr.String())
	}
	if inv.Dir != "." || inv.Stdin != nil {
		t.Error("caller invocation mutated")
	}
}

func TestLookupNilInvocation(t *testing.T) {
	cmd, _ := Lookup("true")
	if code := cmd.Run(nil); code != 0 {
		t.Errorf("status = %d", code)
	}
}

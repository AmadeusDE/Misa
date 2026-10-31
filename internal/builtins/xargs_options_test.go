package builtins

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestXargsParallelOptions(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want int
	}{
		{nil, 1},
		{[]string{"-P1"}, 1},
		{[]string{"-P", "2"}, 2},
		{[]string{"--max-procs=3"}, 3},
		{[]string{"--max-procs", "0"}, 256},
	} {
		o, argv, err := parseXargs(append(tc.args, "echo", "fixed"))
		if err != nil || o.procs != tc.want || !reflect.DeepEqual(argv, []string{"echo", "fixed"}) {
			t.Fatalf("args=%q procs=%d argv=%q err=%v", tc.args, o.procs, argv, err)
		}
	}
	for _, args := range [][]string{{"-P-1"}, {"-P"}, {"-Pno"}, {"-P99999999999999999999999999999999"}} {
		if _, _, err := parseXargs(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}

func TestXargsParallelHelp(t *testing.T) {
	var out bytes.Buffer
	if status := Run(context.Background(), "xargs", &Context{Args: []string{"--help"}, Stdout: &out}); status != 0 {
		t.Fatalf("status=%d", status)
	}
	for _, text := range []string{"-P N", "256"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("help does not mention %q: %q", text, out.String())
		}
	}
}

package aihelp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/ai/aitest"
	"github.com/AmadeusDE/Misa/internal/builtins"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Run(&builtins.Context{CmdName: "aihelp", Args: args, Stdout: &out, Stderr: &errb, Stdin: strings.NewReader("")})
	return code, out.String(), errb.String()
}

func TestSplitQuery(t *testing.T) {
	known := func(s string) bool { return s == "tar" || s == "ls" }
	for _, c := range []struct {
		in   []string
		want Query
	}{
		{[]string{"tar"}, Query{Command: "tar"}},
		{[]string{"tar", "extract", "a", "gz"}, Query{Command: "tar", Question: "extract a gz"}},
		{[]string{"how", "do", "I", "tar"}, Query{Question: "how do I tar"}},
		{[]string{"ls how to sort"}, Query{Question: "ls how to sort"}},
	} {
		if got := splitQuery(c.in, known); got != c.want {
			t.Errorf("splitQuery(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestRunWithoutAModelPointsAtAiconfig(t *testing.T) {
	aitest.Isolate(t)
	ai.SettingsPath = filepath.Join(t.TempDir(), "ai.yaml")
	t.Cleanup(func() { ai.SettingsPath = "" })
	for _, args := range [][]string{{"ls"}, {"--fix", "lss", "127"}} {
		code, out, errs := run(t, args...)
		if code != 1 || out != "" || !strings.Contains(errs, "aiconfig") {
			t.Errorf("aihelp %v = %d %q %q", args, code, out, errs)
		}
	}
}

func TestRunUsageAndFlags(t *testing.T) {
	if code, _, errs := run(t); code != 2 || !strings.Contains(errs, "usage: aihelp") {
		t.Errorf("no args: %d %q", code, errs)
	}
	if code, _, errs := run(t, "--bogus"); code != 2 || errs == "" {
		t.Errorf("bad flag: %d %q", code, errs)
	}
	if code, out, _ := run(t, "--help"); code != 0 || !strings.Contains(out, "model") || !strings.Contains(out, "no-cache") || !strings.Contains(out, "raw") {
		t.Errorf("--help: %d %q", code, out)
	}
}

func TestRunParsesFlagsBeforeTheQuestion(t *testing.T) {
	flags, err := spec.ParseArgs([]string{"--model", "a/b", "-n", "--raw", "tar", "-x", "how"})
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := flags.String("model"); m != "a/b" || !flags.Bool("no-cache") || !flags.Bool("raw") || strings.Join(flags.Operands, " ") != "tar -x how" {
		t.Errorf("flags %+v", flags)
	}
	flags, _ = spec.ParseArgs([]string{"--fix", "git psuh", "1"})
	if !flags.Bool("fix") || strings.Join(flags.Operands, "|") != "git psuh|1" {
		t.Errorf("--fix flags %+v", flags)
	}
}

func TestClearCache(t *testing.T) {
	aitest.Isolate(t)
	c := DefaultCache()
	c.Put("m", "q", "a")
	if code, out, _ := run(t, "--clear-cache"); code != 0 || !strings.Contains(out, "cleared") {
		t.Errorf("clear: %d %q", code, out)
	}
	if _, err := os.Stat(c.Dir); !os.IsNotExist(err) {
		t.Errorf("cache dir remains: %v", err)
	}
}

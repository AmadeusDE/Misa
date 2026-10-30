package builtins

import (
	"bytes"
	"strings"
	"testing"
)

func TestCardGameFlags(t *testing.T) {
	for _, name := range []string{"solitaire", "freecell", "spider", "hearts"} {
		for _, args := range [][]string{{"--seed", "0"}, {"--seed", "18446744073709551615", "--theme", "default", "--ascii"}} {
			g, err := CardGame(name, args)
			if err != nil {
				t.Fatal(name, args, err)
			}
			if !strings.Contains(g.View(), "v1:"+name+":") {
				t.Fatal("missing replay ID")
			}
			g.Close()
		}
		for _, args := range [][]string{{"--seed", "-1"}, {"--seed", "18446744073709551616"}, {"--seed", "x"}, {"--theme", ""}, {"--theme", "xp"}, {"extra"}, {"--unknown"}} {
			if _, err := CardGame(name, args); err == nil {
				t.Fatal(name, args, "accepted")
			}
		}
		command, ok := Lookup(name)
		if !ok {
			t.Fatal(name)
		}
		for _, flag := range []string{"--help", "--version"} {
			var out, err bytes.Buffer
			code := command.Run(&Context{CmdName: name, Args: []string{flag}, Stdout: &out, Stderr: &err})
			if code != 0 || out.Len() == 0 || err.Len() != 0 {
				t.Fatal(name, flag, code, out.String(), err.String())
			}
		}
	}
	for name, flags := range map[string][][]string{"solitaire": {{"--draw", "0"}, {"--draw", "2"}}, "spider": {{"--suits", "3"}}, "hearts": {{"--target", "0"}, {"--target", "-1"}, {"--target", "x"}}, "freecell": {{"--draw", "1"}}} {
		for _, args := range flags {
			if _, err := CardGame(name, args); err == nil {
				t.Fatal(name, args)
			}
		}
	}
	for name, flags := range map[string][]string{"solitaire": {"--draw", "3"}, "spider": {"--suits", "4"}, "hearts": {"--target", "1"}} {
		g, err := CardGame(name, append(flags, "--seed", "42"))
		if err != nil {
			t.Fatal(name, err)
		}
		g.Close()
	}
}

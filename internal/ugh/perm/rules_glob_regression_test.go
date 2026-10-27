package perm_test

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/perm"
)

// A rule for a quoted (literal) argument must not allow the same text
// unquoted, where the shell expands it, and the other way round.
func TestRulesTellQuotedFromExpandedWords(t *testing.T) {
	cwd := t.TempDir()
	for _, c := range []struct {
		rule, cmd string
		want      bool
	}{
		{"rm '*'", "rm *", false},
		{"rm *", "rm '*'", false},
		{"rm 'a?'", "rm a?", false},
		{"rm '[ab]'", "rm [ab]", false},
		{"cat '~/x'", "cat ~/x", false},
		{"echo '{a,b}'", "echo {a,b}", false},
		{"rm '*'", "rm \\*", true},
		{"rm '*'", `rm "*"`, true},
		{"rm *.o", "rm *.o", true},
		{"git commit -m 'fix: x'", `git commit -m "fix: x"`, true},
		{"grep 'a b' .", `grep a\ b .`, true},
		{`echo "a\"b"`, `echo 'a"b'`, true},
	} {
		r := newRules(t, perm.Rule{Tool: "bash", Command: c.rule, Exact: true})
		if _, got := r.Match(bash(cwd, c.cmd)); got != c.want {
			t.Errorf("rule %q, command %q: matched=%v, want %v", c.rule, c.cmd, got, c.want)
		}
	}
}

// A proposed rule matches the call it came from and nothing that differs in
// quoting.
func TestProposedRulesKeepQuoting(t *testing.T) {
	cwd := t.TempDir()
	for _, cmd := range []string{"rm *.o", "rm '*.o'", "cat 'a b' \"c d\"", "echo $'x'"} {
		r := newRules(t)
		rules, ok := r.Propose(bash(cwd, cmd))
		if !ok {
			if cmd == "echo $'x'" {
				continue
			}
			t.Fatalf("%q: no proposal", cmd)
		}
		r.Add(rules...)
		if _, ok := r.Match(bash(cwd, cmd)); !ok {
			t.Errorf("%q: saved rule %+v does not match its own call", cmd, rules)
		}
	}
	r := newRules(t)
	rules, _ := r.Propose(bash(cwd, "rm '*.o'"))
	r.Add(rules...)
	if _, ok := r.Match(bash(cwd, "rm *.o")); ok {
		t.Errorf("rule %+v for a literal *.o allows the glob", rules)
	}
}

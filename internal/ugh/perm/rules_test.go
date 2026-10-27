package perm_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ugh/perm/static"
)

func newRules(t *testing.T, rules ...perm.Rule) *perm.Rules {
	t.Helper()
	r := &perm.Rules{Path: filepath.Join(t.TempDir(), "allow.toml"), Split: static.SimpleCommands}
	if len(rules) > 0 {
		if err := r.Add(rules...); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func bash(cwd, cmd string) perm.Call {
	args, _ := json.Marshal(map[string]string{"command": cmd})
	return perm.NewCall(cwd, "bash", args)
}

func pathCall(cwd, tool, p string) perm.Call {
	args, _ := json.Marshal(map[string]string{"path": p})
	return perm.NewCall(cwd, tool, args)
}

func TestRulesMatchBash(t *testing.T) {
	cwd := t.TempDir()
	r := newRules(t,
		perm.Rule{Tool: "bash", Command: "git status"},
		perm.Rule{Tool: "bash", Command: "go test"},
		perm.Rule{Tool: "bash", Command: "ls", Exact: true},
		perm.Rule{Tool: "bash", Command: "make", Cwd: filepath.Join(cwd, "proj") + "/**"},
	)
	for cmd, want := range map[string]bool{
		"git status":                  true,
		"git status -s":               true,
		"git stash":                   false,
		"go test ./... && git status": true,
		"go test ./... && rm -rf ~":   false,
		"git status; $(rm -rf ~)":     false,
		"git status > /etc/passwd":    false,
		"ls":                          true,
		"ls -la /":                    false,
		"make":                        false, // cwd outside proj
	} {
		if _, got := r.Match(bash(cwd, cmd)); got != want {
			t.Errorf("%q matched=%v, want %v", cmd, got, want)
		}
	}
	os.MkdirAll(filepath.Join(cwd, "proj", "sub"), 0o700)
	if _, ok := r.Match(bash(filepath.Join(cwd, "proj", "sub"), "make")); !ok {
		t.Error("cwd-scoped rule should match under its tree")
	}
}

func TestRulesMatchPaths(t *testing.T) {
	proj, outside := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600)
	os.Symlink(outside, filepath.Join(proj, "link"))
	r := newRules(t, perm.Rule{Tool: "read", Path: proj + "/**"}, perm.Rule{Tool: "spawn"})
	cases := []struct {
		c    perm.Call
		want bool
	}{
		{pathCall(proj, "read", "a/b.go"), true},
		{pathCall(proj, "read", "../x"), false},
		{pathCall(proj, "read", "link/secret"), false}, // symlink out of the tree
		{pathCall(proj, "write", "a.go"), false},       // rule is read-only
		{perm.NewCall(proj, "spawn", json.RawMessage(`{"tasks":[]}`)), true},
	}
	for _, c := range cases {
		if _, got := r.Match(c.c); got != c.want {
			t.Errorf("%s %s matched=%v, want %v", c.c.Tool, c.c.Args, got, c.want)
		}
	}
}

func TestRulesPropose(t *testing.T) {
	r := newRules(t)
	got, ok := r.Propose(bash("/x", "go test ./... && rm -rf build"))
	if !ok || len(got) != 2 || got[0].Command != "go test" || got[0].Exact || got[1].Command != "rm -rf build" || !got[1].Exact {
		t.Fatalf("proposal = %+v %v", got, ok)
	}
	if _, ok := r.Propose(bash("/x", "echo $HOME")); ok {
		t.Error("no rule should be proposed for a non-plain line")
	}
	// A saved proposal matches the call it came from.
	c := bash("/x", "git commit -m 'it works'")
	rules, _ := r.Propose(c)
	r.Add(rules...)
	if _, ok := r.Match(c); !ok {
		t.Errorf("saved rule %+v does not match its own call", rules)
	}
}

// Manual mode: a rule skips the prompt, [p] saves one, and classify mode
// never consults the rules.
func TestEngineManualRules(t *testing.T) {
	r := newRules(t)
	pol := perm.DefaultPolicy()
	pol.Mode = "manual"
	e := perm.NewEngine(pol, perm.NewBroker(strings.NewReader("p\n"), io.Discard, true, "deny"), nil)
	e.Rules = r
	c := bash("/x", "git status")
	if oc := e.Check(context.Background(), c); oc.Action != perm.Allow || !strings.Contains(oc.Reason, "rule saved") {
		t.Fatalf("first = %+v", oc)
	}
	// stdin is exhausted: a second prompt would deny.
	if oc := e.Check(context.Background(), bash("/y", "git status -s")); oc.Action != perm.Allow || !strings.Contains(oc.Reason, "allowed by rule") {
		t.Fatalf("second = %+v", oc)
	}
	pol.Mode = "classify"
	pol.Classifiers = map[string]perm.ClassifierConfig{"static_extreme": {State: "on", OnFlag: "prompt", Tools: []string{"*"}}}
	e2 := perm.NewEngine(pol, perm.NewBroker(nil, io.Discard, false, "deny"), nil)
	e2.Rules = r
	e2.Add(flagAll{})
	if oc := e2.Check(context.Background(), bash("/x", "git status")); oc.Action != perm.Deny {
		t.Fatalf("classify used a manual rule: %+v", oc)
	}
}

type flagAll struct{}

func (flagAll) Name() string { return "static_extreme" }
func (flagAll) Classify(context.Context, perm.Call) perm.Verdict {
	return perm.Verdict{Classifier: "static_extreme", Flagged: true, Reason: "test"}
}

package perm

import (
	"context"
	"encoding/json"
	"testing"
)

func hashOf(t *testing.T, tool, args string) string {
	t.Helper()
	h, err := CallHash(Call{Tool: tool, Args: json.RawMessage(args)})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestCallHashIgnoresKeyOrderWhitespaceAndNumberForms(t *testing.T) {
	a := hashOf(t, "bash", `{"command":"ls","timeout":30,"n":{"x":1,"y":[1.5,2]}}`)
	b := hashOf(t, "bash", "{ \"n\" : {\"y\":[1.50, 2.0],\n\"x\":1e0}, \"timeout\":3e1,\n\"command\":\"ls\" }")
	if a != b {
		t.Errorf("equivalent args hash differently: %s vs %s", a, b)
	}
}

func TestCallHashChangesWithArgsAndTool(t *testing.T) {
	base := hashOf(t, "bash", `{"command":"ls"}`)
	for name, h := range map[string]string{
		"args": hashOf(t, "bash", `{"command":"ls -a"}`),
		"tool": hashOf(t, "write", `{"command":"ls"}`),
		"num":  hashOf(t, "bash", `{"command":"ls","n":1}`),
	} {
		if h == base {
			t.Errorf("%s change kept the hash", name)
		}
	}
	if hashOf(t, "t", `{"n":1}`) == hashOf(t, "t", `{"n":2}`) {
		t.Error("numbers 1 and 2 collide")
	}
}

func TestCallHashRejectsBadJSON(t *testing.T) {
	for _, raw := range []string{`{"a":`, `{} {}`} {
		if _, err := CallHash(Call{Tool: "t", Args: json.RawMessage(raw)}); err == nil {
			t.Errorf("%q accepted", raw)
		}
	}
}

func TestVerifyApproved(t *testing.T) {
	c := Call{Tool: "bash", Args: json.RawMessage(`{"command":"ls","a":1}`)}
	h, _ := CallHash(c)
	if !VerifyApproved(h, Call{Tool: "bash", Args: json.RawMessage(`{ "a":1, "command":"ls" }`)}) {
		t.Error("reordered call not verified")
	}
	if VerifyApproved(h, Call{Tool: "bash", Args: json.RawMessage(`{"command":"rm -rf /","a":1}`)}) {
		t.Error("changed call verified")
	}
	if VerifyApproved("", c) || VerifyApproved(h, Call{Tool: "bash", Args: json.RawMessage(`{`)}) {
		t.Error("empty hash or invalid args verified")
	}
}

type deferPrompter struct {
	ans     Answer
	tickets []string
}

func (d *deferPrompter) Ask(Call, []Verdict) Answer { return d.ans }
func (d *deferPrompter) Allowed(Call) bool          { return false }
func (d *deferPrompter) Interactive() bool          { return true }

type deferringPrompter struct{ deferPrompter }

func (d *deferringPrompter) Defer(c Call, _ []Verdict) string {
	d.tickets = append(d.tickets, c.Tool)
	return "7"
}

func TestEngineDefersOnlyThroughDeferrer(t *testing.T) {
	c := Call{Tool: "bash", Args: json.RawMessage(`{"command":"ls"}`)}
	d := &deferringPrompter{deferPrompter{ans: AnswerDefer}}
	e := NewEngine(Policy{Mode: "manual"}, d, nil)
	oc := e.Check(context.Background(), c)
	if oc.Action != Defer || oc.Ticket != "7" || len(d.tickets) != 1 {
		t.Errorf("outcome = %+v tickets=%v, want defer #7", oc, d.tickets)
	}

	plain := &deferPrompter{ans: AnswerDefer}
	e = NewEngine(Policy{Mode: "manual"}, plain, nil)
	if oc := e.Check(context.Background(), c); oc.Action != Deny {
		t.Errorf("non-Deferrer prompter: action = %q, want deny", oc.Action)
	}

	allow := &deferringPrompter{deferPrompter{ans: AnsAllowOnce}}
	e = NewEngine(Policy{Mode: "manual"}, allow, nil)
	if oc := e.Check(context.Background(), c); oc.Action != Allow || len(allow.tickets) != 0 {
		t.Errorf("allow answer = %+v, want plain allow", oc)
	}
}

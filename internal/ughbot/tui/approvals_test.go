package tui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/charmbracelet/x/ansi"
)

type approvalFake struct {
	*fakeBackend
	list    []api.ApprovalInfo
	decided []string
}

func (f *approvalFake) Approvals(_ context.Context, state string) ([]api.ApprovalInfo, error) {
	var out []api.ApprovalInfo
	for _, a := range f.list {
		if state == "" || a.State == state {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *approvalFake) decide(id int64, by, state string) (api.ApprovalInfo, error) {
	for i := range f.list {
		if f.list[i].ID == id {
			f.list[i].State, f.list[i].DecidedBy = state, by
			f.decided = append(f.decided, state+" "+by)
			return f.list[i], nil
		}
	}
	return api.ApprovalInfo{}, api.ErrNoApproval
}

func (f *approvalFake) Approve(_ context.Context, id int64, by string) (api.ApprovalInfo, error) {
	return f.decide(id, by, "approved")
}
func (f *approvalFake) Deny(_ context.Context, id int64, by string) (api.ApprovalInfo, error) {
	return f.decide(id, by, "denied")
}

func newApprovalFake() *approvalFake {
	args, _ := json.Marshal(map[string]any{"name": "helper", "model": "fake/m", "prompt_text": "You help.",
		"capabilities": map[string]any{"shell": true}})
	return &approvalFake{fakeBackend: newFake(), list: []api.ApprovalInfo{
		{ID: 2, Bot: "a", Action: "create_bot", Summary: "create bot helper: model fake/m, shell", Args: args,
			State: "pending", Created: time.Now(), Expires: time.Now().Add(time.Hour)},
		{ID: 1, Bot: "a", Action: "delete_bot", Summary: "delete bot b", Args: json.RawMessage(`{"name":"b"}`),
			State: "denied", Created: time.Now()},
	}}
}

func TestApprovalsScreenApprovesWithConfirmation(t *testing.T) {
	t.Setenv("WEBUI_DEVICE", "")
	be := newApprovalFake()
	m := ready(t, be, "")
	run(m, m.fetchApprovals())
	wantIn(t, m.View().Content, "Approvals 1")
	run(m, m.goTo(screenApprovals))
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "create bot helper") || strings.Contains(view, "delete bot b") {
		t.Fatalf("pending only:\n%s", view)
	}
	press(m, "enter")
	wantIn(t, m.View().Content, `"shell": true`, "You help.", "then it is denied")
	press(m, "a")
	wantIn(t, m.View().Content, "approve this request? y/N")
	press(m, "n")
	if len(be.decided) != 0 {
		t.Fatal("decided without y")
	}
	press(m, "a")
	run(m, press(m, "y"))
	if len(be.decided) != 1 || be.decided[0] != "approved tui" {
		t.Fatalf("%v", be.decided)
	}
	run(m, m.fetchApprovals()) // the refresh decided batches
	wantIn(t, m.View().Content, "#2 approved", "nothing waits on you")
	if strings.Contains(ansi.Strip(m.header()), "Approvals 1") {
		t.Fatal("badge kept counting a decided request")
	}
	run(m, press(m, "A"))
	wantIn(t, m.View().Content, "(all)", "delete bot b", "approved")
}

func TestApprovalsRecordTheWebDevice(t *testing.T) {
	t.Setenv("WEBUI_DEVICE", "phone")
	be := newApprovalFake()
	m := ready(t, be, "")
	run(m, m.goTo(screenApprovals))
	press(m, "d")
	run(m, press(m, "y"))
	if len(be.decided) != 1 || be.decided[0] != "denied web:phone" {
		t.Fatalf("%v", be.decided)
	}
}

func TestNoApprovalsScreenWithoutTheBackend(t *testing.T) {
	m := ready(t, newFake(), "")
	for _, s := range m.screens() {
		if s == screenApprovals {
			t.Fatal("a backend without approvals got the screen")
		}
	}
}

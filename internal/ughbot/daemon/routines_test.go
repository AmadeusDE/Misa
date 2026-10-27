package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/bot"
	"github.com/AmadeusDE/Misa/internal/ughbot/client"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/fakellm"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

// schedDaemon has "sched" (schedule capability, may message everyone) and "plain" (nothing).
// It is not Run, so routines fire only when the test calls fireDue.
func schedDaemon(t *testing.T) (*Daemon, *client.Client) {
	t.Helper()
	h := newHarness(t)
	cfg, err := config.Parse([]byte(`
state_dir = "` + filepath.Join(h.dir, "state") + `"
socket = "` + filepath.Join(h.dir, "u.sock") + `"
[bots.sched]
model = "fake/m"
capabilities = { schedule = true, message = ["*"] }
[bots.plain]
model = "fake/m"
`))
	if err != nil {
		t.Fatal(err)
	}
	d, err := New(Options{Config: cfg, Provider: fakellm.Echo(0)})
	if err != nil {
		t.Fatal(err)
	}
	srv := api.NewServer(d)
	ctx, cancel := context.WithCancel(context.Background())
	go srv.Serve(ctx, d.ln)
	c, err := client.Dial(cfg.Socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); c.Close(); d.closeAll() })
	return d, c
}

func routineMail(t *testing.T, d *Daemon, to string) []store.Mail {
	t.Helper()
	all, err := d.st.MailTo(to, 100)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.Mail
	for _, m := range all {
		if strings.HasPrefix(m.From, "routine #") {
			out = append(out, m)
		}
	}
	return out
}

func tool(t *testing.T, d *Daemon, botName, name, args string) (string, error) {
	t.Helper()
	b, _ := d.Lookup(botName)
	return b.RunTool(context.Background(), name, json.RawMessage(args))
}

func TestUserRoutineFiresOncePerTime(t *testing.T) {
	d, c := schedDaemon(t)
	ctx := context.Background()
	info, err := c.AddRoutine(ctx, api.RoutineRequest{Bot: "plain", Prompt: "daily check", Cron: "0 9 * * *", Owner: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Owner != "cli" || info.State != "active" || info.Next.Hour() != 9 || info.Missed != "once" {
		t.Fatalf("%+v", info)
	}
	at := info.Next.Add(time.Second)
	d.fireDue(at)
	d.fireDue(at)
	mail := routineMail(t, d, "plain")
	if len(mail) != 1 || mail[0].Body != "daily check" || mail[0].From != "routine #1" {
		t.Fatalf("%+v", mail)
	}
	list, _ := c.Routines(ctx, "plain")
	if len(list) != 1 || list[0].Fired != 1 || !list[0].Next.After(at) {
		t.Fatalf("%+v", list)
	}
}

func TestMissedFiresDoNotStack(t *testing.T) {
	d, c := schedDaemon(t)
	ctx := context.Background()
	once, _ := c.AddRoutine(ctx, api.RoutineRequest{Bot: "plain", Prompt: "once", Cron: "* * * * *"})
	skip, _ := c.AddRoutine(ctx, api.RoutineRequest{Bot: "sched", Prompt: "skip", Cron: "* * * * *", Missed: "skip"})
	later := time.Now().Add(3 * time.Hour) // the daemon was "down" for ~180 fires
	d.fireDue(later)
	if n := len(routineMail(t, d, "plain")); n != 1 {
		t.Fatalf("missed=once fired %d times", n)
	}
	if n := len(routineMail(t, d, "sched")); n != 0 {
		t.Fatalf("missed=skip fired %d times", n)
	}
	for _, id := range []int64{once.ID, skip.ID} {
		r, _ := d.st.Routine(id)
		if !r.Next.After(later) {
			t.Fatalf("#%d next %v not after %v", id, r.Next, later)
		}
	}
}

func TestOneShotIsDoneAfterFiring(t *testing.T) {
	d, c := schedDaemon(t)
	info, err := c.AddRoutine(context.Background(), api.RoutineRequest{Bot: "plain", Prompt: "remind", At: "in 10m"})
	if err != nil {
		t.Fatal(err)
	}
	d.fireDue(info.Next.Add(time.Hour))
	r, _ := d.st.Routine(info.ID)
	if r.State != store.RoutineDone || len(routineMail(t, d, "plain")) != 1 {
		t.Fatalf("%+v", r)
	}
	if _, err := c.PauseRoutine(context.Background(), info.ID, false); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("resume a done routine: %v", err)
	}
}

func TestRoutineValidation(t *testing.T) {
	_, c := schedDaemon(t)
	ctx := context.Background()
	for name, c2 := range map[string]struct {
		req  api.RoutineRequest
		code int
	}{
		"unknown bot": {api.RoutineRequest{Bot: "ghost", Prompt: "x", Cron: "@daily"}, http.StatusNotFound},
		"no prompt":   {api.RoutineRequest{Bot: "plain", Cron: "@daily"}, http.StatusBadRequest},
		"no time":     {api.RoutineRequest{Bot: "plain", Prompt: "x"}, http.StatusBadRequest},
		"both":        {api.RoutineRequest{Bot: "plain", Prompt: "x", Cron: "@daily", At: "in 1h"}, http.StatusBadRequest},
		"bad cron":    {api.RoutineRequest{Bot: "plain", Prompt: "x", Cron: "61 * * * *"}, http.StatusBadRequest},
		"past":        {api.RoutineRequest{Bot: "plain", Prompt: "x", At: "2020-01-01 00:00"}, http.StatusBadRequest},
		"bad tz":      {api.RoutineRequest{Bot: "plain", Prompt: "x", Cron: "@daily", TZ: "Mars/Olympus"}, http.StatusBadRequest},
		"bad missed":  {api.RoutineRequest{Bot: "plain", Prompt: "x", Cron: "@daily", Missed: "twice"}, http.StatusBadRequest},
	} {
		if _, err := c.AddRoutine(ctx, c2.req); statusCode(err) != c2.code {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestPauseResumeDeleteAndMissingBot(t *testing.T) {
	d, c := schedDaemon(t)
	ctx := context.Background()
	info, _ := c.AddRoutine(ctx, api.RoutineRequest{Bot: "plain", Prompt: "x", Cron: "* * * * *"})
	if p, err := c.PauseRoutine(ctx, info.ID, true); err != nil || p.State != "paused" {
		t.Fatalf("%+v %v", p, err)
	}
	d.fireDue(time.Now().Add(time.Hour))
	if len(routineMail(t, d, "plain")) != 0 {
		t.Fatal("a paused routine fired")
	}
	if p, err := c.PauseRoutine(ctx, info.ID, false); err != nil || p.State != "active" {
		t.Fatalf("%+v %v", p, err)
	}
	if err := c.DeleteRoutine(ctx, info.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteRoutine(ctx, info.ID); statusCode(err) != http.StatusNotFound {
		t.Fatalf("delete twice: %v", err)
	}
	orphan, _ := d.st.AddRoutine(store.Routine{Owner: "user", Bot: "gone", Prompt: "x", Cron: "* * * * *", Missed: "once", Next: time.Now()})
	d.fireDue(time.Now().Add(time.Minute))
	if r, _ := d.st.Routine(orphan); r.State != store.RoutinePaused {
		t.Fatalf("routine for a missing bot: %+v", r)
	}
}

func TestBotsScheduleDirectlyOnlyWithTheCapability(t *testing.T) {
	d, c := schedDaemon(t)
	ctx := context.Background()
	out, err := tool(t, d, "sched", "schedule_add", `{"prompt":"ping plain","bot":"plain","cron":"@hourly"}`)
	if err != nil || !strings.Contains(out, "routine #1 added") {
		t.Fatalf("%q %v", out, err)
	}
	if list, _ := c.Routines(ctx, "plain"); len(list) != 1 || list[0].Owner != "sched" {
		t.Fatalf("%+v", list)
	}
	if _, err := tool(t, d, "plain", "schedule_add", `{"prompt":"x","bot":"sched","cron":"@hourly"}`); err == nil {
		t.Fatal("plain may not message sched, so it may not schedule mail to it")
	}
	out, err = tool(t, d, "plain", "schedule_add", `{"prompt":"remind me","at":"in 1h"}`)
	if err != nil || !strings.Contains(out, "approval #") {
		t.Fatalf("%q %v", out, err)
	}
	if list, _ := c.Routines(ctx, ""); len(list) != 1 {
		t.Fatalf("added before approval: %+v", list)
	}
	pending, _ := c.Approvals(ctx, "pending")
	if len(pending) != 1 || pending[0].Action != bot.ActionAddRoutine || !strings.Contains(pending[0].Summary, "mail plain at ") {
		t.Fatalf("%+v", pending)
	}
	if _, err := c.Approve(ctx, pending[0].ID, "cli"); err != nil {
		t.Fatal(err)
	}
	list, _ := c.Routines(ctx, "plain")
	if len(list) != 2 || list[1].Owner != "plain" || list[1].Prompt != "remind me" {
		t.Fatalf("%+v", list)
	}
	if out, _ := tool(t, d, "plain", "schedule_cancel", `{"id":1}`); !strings.Contains(out, "approval #") {
		t.Fatalf("plain cancels its own routine through approval: %q", out)
	}
	if _, err := tool(t, d, "plain", "schedule_cancel", `{"id":99}`); err == nil {
		t.Fatal("cancel of a routine that is not plain's")
	}
	if out, _ := tool(t, d, "sched", "schedule_list", `{}`); !strings.Contains(out, "#1 active -> plain") {
		t.Fatalf("%q", out)
	}
}

func TestFlaggedScheduleCallIsDeferredAndRunsAsStored(t *testing.T) {
	d, c := schedDaemon(t)
	ctx := context.Background()
	b, _ := d.Lookup("plain")
	p := b.Prompter()
	args := json.RawMessage(`{"prompt":"x","cron":"@daily"}`)
	call := perm.Call{Tool: "schedule_add", Args: args}
	if p.Ask(call, nil) != perm.AnswerDefer {
		t.Fatal("a flagged schedule call should be deferred")
	}
	if p.Ask(perm.Call{Tool: "bash"}, nil) == perm.AnswerDefer {
		t.Fatal("other flagged calls keep the headless policy")
	}
	ticket := p.(perm.Deferrer).Defer(call, []perm.Verdict{{Classifier: "jev", Flagged: true, Reason: "odd"}})
	pending, _ := c.Approvals(ctx, "pending")
	if len(pending) != 1 || pending[0].Action != bot.ActionToolCall || !strings.Contains(pending[0].Summary, "flagged: jev: odd") ||
		ticket != "1" {
		t.Fatalf("%q %+v", ticket, pending)
	}
	got, err := c.Approve(ctx, pending[0].ID, "cli")
	if err != nil || !strings.Contains(got.Result, "routine #1 added") {
		t.Fatalf("approved call should add directly, not ask again: %+v %v", got, err)
	}
}

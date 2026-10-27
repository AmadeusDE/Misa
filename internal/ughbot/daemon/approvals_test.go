package daemon

import (
	"context"
	"database/sql"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/bot"
	"github.com/AmadeusDE/Misa/internal/ughbot/client"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/fakellm"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

// maker is a bot that may create bots; it calls create_bot when asked to "make <name>".
func makerDaemon(t *testing.T) (*Daemon, *client.Client, func()) {
	t.Helper()
	h := newHarness(t)
	h.p = &fakellm.Provider{Script: func(req llm.Request, _ int) []llm.Event {
		last := fakellm.LastUser(req)
		if name, ok := strings.CutPrefix(last, "make "); ok {
			for _, m := range req.Messages {
				if m.Role == llm.RoleTool {
					return fakellm.Reply("asked", 0)
				}
			}
			return fakellm.ToolCall("c1", bot.ActionCreate,
				`{"name":"`+name+`","model":"fake/m","prompt":"helper","capabilities":{"files":"read","shell":true}}`, 0)
		}
		return fakellm.Reply("ok: "+last, 0)
	}}
	cfg, err := config.Parse([]byte(`
state_dir = "` + filepath.Join(h.dir, "state") + `"
socket = "` + filepath.Join(h.dir, "u.sock") + `"
[bots.maker]
model = "fake/m"
capabilities = { create_bots = true }
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := fakellm.NoModelClassifiers(cfg.ConfigDir); err != nil {
		t.Fatal(err)
	}
	d, err := New(Options{Config: cfg, Provider: h.p})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	c, err := client.Dial(cfg.Socket)
	if err != nil {
		t.Fatal(err)
	}
	return d, c, func() { cancel(); <-done; c.Close() }
}

func pendingSoon(t *testing.T, c *client.Client, n int) []api.ApprovalInfo {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		list, err := c.Approvals(context.Background(), store.ApprovalPending)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) >= n {
			return list
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fewer than %d pending approvals", n)
	return nil
}

func mailFrom(t *testing.T, d *Daemon, to, prefix string) store.Mail {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mail, err := d.st.MailTo(to, 50)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range mail {
			if strings.HasPrefix(m.From, prefix) {
				return m
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no mail from %s* to %s", prefix, to)
	return store.Mail{}
}

func TestBotCreationWaitsForApproval(t *testing.T) {
	d, c, stop := makerDaemon(t)
	defer stop()
	ctx := context.Background()
	if _, err := c.Send(ctx, "maker", "make helper"); err != nil {
		t.Fatal(err)
	}
	list := pendingSoon(t, c, 1)
	a := list[0]
	if a.Bot != "maker" || a.Action != bot.ActionCreate || !strings.Contains(a.Summary, "create bot helper: model fake/m, capabilities: files=read shell") {
		t.Fatalf("%+v", a)
	}
	if bots, _ := c.ListBots(ctx); len(bots) != 1 {
		t.Fatalf("created before approval: %+v", bots)
	}
	got, err := c.Approve(ctx, a.ID, "cli")
	if err != nil || got.State != store.ApprovalApproved || got.DecidedBy != "cli" {
		t.Fatalf("%+v %v", got, err)
	}
	if bots, _ := c.ListBots(ctx); len(bots) != 2 || bots[0].Name != "helper" || !bots[0].Editable {
		t.Fatalf("after approval: %+v", bots)
	}
	if def, _ := c.BotDef(ctx, "helper"); !def.Capabilities.Shell || def.PromptText != "helper" {
		t.Fatalf("created %+v", def)
	}
	if m := mailFrom(t, d, "maker", "approval #"); !strings.Contains(m.Body, "approved by cli") {
		t.Fatalf("outcome mail: %+v", m)
	}
	if _, err := c.Approve(ctx, a.ID, "cli"); statusCode(err) != http.StatusConflict {
		t.Fatalf("second approval: %v", err)
	}
	if _, err := c.Deny(ctx, 999, "cli"); statusCode(err) != http.StatusNotFound {
		t.Fatalf("unknown id: %v", err)
	}
}

func TestDeniedAndExpiredRequestsChangeNothing(t *testing.T) {
	d, c, stop := makerDaemon(t)
	defer stop()
	ctx := context.Background()
	id, err := d.ProposeBot("maker", bot.ActionCreate, config.Bot{Name: "x", Model: "fake/m"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Deny(ctx, id, "web:phone"); err != nil {
		t.Fatal(err)
	}
	if m := mailFrom(t, d, "maker", "approval #"); !strings.Contains(m.Body, "denied by web:phone") {
		t.Fatalf("%+v", m)
	}
	args := `{"name":"y","model":"fake/m"}`
	old, _ := d.st.AddApproval(store.Approval{Bot: "maker", Action: bot.ActionCreate, Args: args,
		Hash: actionHash(bot.ActionCreate, args), Summary: "create bot y", Expires: time.Now().Add(-time.Second)})
	if _, err := c.Approve(ctx, old, "cli"); statusCode(err) != http.StatusConflict || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("an expired request: %v", err)
	}
	if m := mailFrom(t, d, "maker", "approval #"+strconv.FormatInt(old, 10)); !strings.Contains(m.Body, "so it is denied") {
		t.Fatalf("%+v", m)
	}
	if bots, _ := c.ListBots(ctx); len(bots) != 1 {
		t.Fatalf("bots changed: %+v", bots)
	}
}

func TestTamperedRequestIsRefused(t *testing.T) {
	d, c, stop := makerDaemon(t)
	defer stop()
	id, err := d.ProposeBot("maker", bot.ActionCreate, config.Bot{Name: "x", Model: "fake/m"})
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+d.cfg.DBPath()+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE approvals SET args=? WHERE id=?`, `{"name":"x","model":"fake/m","capabilities":{"shell":true}}`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Approve(context.Background(), id, "cli"); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("tampered approval: %v", err)
	}
}

func TestProposalsAreCheckedFirst(t *testing.T) {
	d, _, stop := makerDaemon(t)
	defer stop()
	for name, c := range map[string]struct {
		action string
		def    config.Bot
	}{
		"exists":       {bot.ActionCreate, config.Bot{Name: "maker", Model: "m"}},
		"no model":     {bot.ActionCreate, config.Bot{Name: "x"}},
		"unknown ref":  {bot.ActionCreate, config.Bot{Name: "x", Model: "m", Capabilities: config.Capabilities{Manage: []string{"ghost"}}}},
		"toml bot":     {bot.ActionUpdate, config.Bot{Name: "maker", Model: "m2"}},
		"missing bot":  {bot.ActionDelete, config.Bot{Name: "ghost"}},
		"unknown verb": {"rename_bot", config.Bot{Name: "maker"}},
	} {
		if _, err := d.ProposeBot("maker", c.action, c.def); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

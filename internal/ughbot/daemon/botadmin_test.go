package daemon

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/client"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

func emptyConfig(t *testing.T, h *harness) *config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(`
state_dir = "` + filepath.Join(h.dir, "state") + `"
config_dir = "` + filepath.Join(h.dir, "config") + `"
socket = "` + filepath.Join(h.dir, "u.sock") + `"
`))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func statusCode(err error) int {
	var se *api.StatusError
	if errors.As(err, &se) {
		return se.Code
	}
	return 0
}

func TestBotsMadeInTheUIWithNoConfiguredBots(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	d, err := New(Options{Config: emptyConfig(t, h), Provider: h.p})
	if err != nil {
		t.Fatalf("a daemon with no bots must start: %v", err)
	}
	done := make(chan error, 1)
	runCtx, cancel := context.WithCancel(ctx)
	go func() { done <- d.Run(runCtx) }()
	c, err := client.Dial(d.cfg.Socket)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if bots, _ := c.ListBots(ctx); len(bots) != 0 {
		t.Fatalf("bots = %v", bots)
	}
	def := api.BotDef{Name: "alpha", Model: "fake/m", PromptText: "You are Alpha.", Capabilities: config.Capabilities{Files: config.FilesRead}, Spend: config.Spend{Daily: 1, Monthly: 5}}
	if err := c.CreateBot(ctx, def); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateBot(ctx, def); statusCode(err) != http.StatusConflict {
		t.Fatalf("duplicate create: %v", err)
	}
	if err := c.CreateBot(ctx, api.BotDef{Name: "Bad Name", Model: "fake/m"}); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("invalid name: %v", err)
	}
	if err := c.CreateBot(ctx, api.BotDef{Name: "x", Model: "fake/m", Capabilities: config.Capabilities{Files: "all"}}); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("bad files capability: %v", err)
	}
	if err := c.CreateBot(ctx, api.BotDef{Name: "x", Model: "fake/m", Capabilities: config.Capabilities{Message: []string{"ghost"}}}); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("unknown bot in message: %v", err)
	}
	bots, _ := c.ListBots(ctx)
	if len(bots) != 1 || bots[0].Name != "alpha" || !bots[0].Editable || bots[0].State != "idle" {
		t.Fatalf("bots = %+v", bots)
	}

	events, err := c.Subscribe(ctx, "alpha", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Send(ctx, "alpha", "hello"); err != nil {
		t.Fatal(err)
	}
	awaitType(t, events, "turn_end")

	def.Model, def.Capabilities.Message = "fake/m2", []string{"*"}
	if err := c.UpdateBot(ctx, def); err != nil {
		t.Fatal(err)
	}
	got, err := c.BotDef(ctx, "alpha")
	if err != nil || got.Model != "fake/m2" || got.PromptText != "You are Alpha." {
		t.Fatalf("def = %+v, %v", got, err)
	}

	cancel()
	<-done

	d2, err := New(Options{Config: emptyConfig(t, h), Provider: h.p})
	if err != nil {
		t.Fatal(err)
	}
	if bots, _ := d2.ListBots(ctx); len(bots) != 1 || bots[0].Name != "alpha" {
		t.Fatalf("after restart: %+v", bots)
	}
	if err := d2.DeleteBot(ctx, "alpha"); err != nil {
		t.Fatal(err)
	}
	if bots, _ := d2.ListBots(ctx); len(bots) != 0 {
		t.Fatalf("after delete: %+v", bots)
	}
	d2.closeAll()
	d3, err := New(Options{Config: emptyConfig(t, h), Provider: h.p})
	if err != nil {
		t.Fatal(err)
	}
	if bots, _ := d3.ListBots(ctx); len(bots) != 0 {
		t.Fatalf("deleted bot came back: %+v", bots)
	}
	d3.closeAll()
}

func TestConfigBotsAreReadOnly(t *testing.T) {
	h := newHarness(t)
	d, c, stop := h.run(t, 0)
	defer stop()
	ctx := context.Background()
	def := api.BotDef{Name: "alpha", Model: "fake/m"}
	if err := c.UpdateBot(ctx, def); statusCode(err) != http.StatusForbidden {
		t.Fatalf("update: %v", err)
	}
	if err := c.DeleteBot(ctx, "alpha"); statusCode(err) != http.StatusForbidden {
		t.Fatalf("delete: %v", err)
	}
	if err := c.CreateBot(ctx, def); statusCode(err) != http.StatusConflict {
		t.Fatalf("create over config bot: %v", err)
	}
	_ = d
}

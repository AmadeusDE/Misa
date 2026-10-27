// Package daemon assembles the ughbot service: store, ledger, one bot per configured
// bot and the API they are reached through.
package daemon

import (
	"context"
	"fmt"
	"github.com/AmadeusDE/Misa/internal/ughbot/sandbox"
	"github.com/AmadeusDE/Misa/internal/ughbot/vm"
	"log"
	"maps"
	"net"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/bot"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/ledger"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

const Version = "0.1"

type Options struct {
	Config *config.Config
	// Provider replaces every bot's model client (tests).
	Provider llm.Provider
	// Sandbox replaces the VM manager code tasks run in (tests).
	Sandbox sandbox.Sandbox
}

type Daemon struct {
	cfg      *config.Config
	st       *store.Store
	led      *ledger.Ledger
	provider llm.Provider
	mu       sync.RWMutex // guards bots, cfg.Bots, ui and runCtx
	bots     map[string]*bot.Bot
	view     atomic.Pointer[view] // what the bots themselves read, without mu
	ui       map[string]bool      // bots made in the UI (stored in the database)
	runCtx   context.Context
	ln       net.Listener
	started  time.Time

	web api.WebAdmin

	vm      *vm.Manager // nil when Options.Sandbox replaced it
	sandbox sandbox.Sandbox
	locks   *bot.DirLocks

	routineWake chan struct{}

	stopOnce sync.Once
	stop     chan struct{}
}

var _ api.Backend = (*Daemon)(nil)

// New claims the socket, opens the store and builds every bot, resuming their sessions.
// Nothing is built when the socket is taken by another daemon or a bot's config is wrong.
func New(o Options) (*Daemon, error) {
	cfg := o.Config
	ln, err := api.Listen(cfg.Socket)
	if err != nil {
		return nil, err
	}
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		ln.Close()
		return nil, err
	}
	ui, err := mergeStoredBots(cfg, st)
	if err == nil {
		err = cfg.CheckReferences(tomlBots(cfg, ui)...)
	}
	if err != nil {
		ln.Close()
		st.Close()
		return nil, err
	}
	d := &Daemon{cfg: cfg, st: st, led: ledger.New(st), ln: ln, provider: o.Provider, ui: ui,
		bots: map[string]*bot.Bot{}, stop: make(chan struct{}), routineWake: make(chan struct{}, 1),
		sandbox: o.Sandbox, locks: bot.NewDirLocks()}
	if d.sandbox == nil {
		d.vm = vm.New(cfg, log.Writer())
		d.sandbox = d.vm
	}
	for _, name := range cfg.BotNames() {
		b, err := d.newBot(cfg.Bots[name])
		if err != nil {
			d.closeAll()
			return nil, err
		}
		d.bots[name] = b
	}
	d.publishPeers()
	return d, nil
}

func tomlBots(cfg *config.Config, ui map[string]bool) []string {
	var names []string
	for _, n := range cfg.BotNames() {
		if !ui[n] {
			names = append(names, n)
		}
	}
	return names
}

func (d *Daemon) newBot(def config.Bot) (*bot.Bot, error) {
	return bot.New(bot.Deps{Config: d.cfg, Bot: def, Store: d.st, Ledger: d.led, Provider: d.provider,
		Directory: d, Proposer: d, Scheduler: d, Sandbox: d.sandbox, Locks: d.locks})
}

// view is a copy of the bots and their definitions for the bots' own tools, which must never
// wait on mu: the daemon holds it while it waits for a bot to stop.
type view struct {
	bots map[string]*bot.Bot
	cfg  *config.Config
	ui   map[string]bool
}

// publishPeers refreshes the view; call it with mu held after bots change.
func (d *Daemon) publishPeers() {
	cfg := *d.cfg
	cfg.Bots = maps.Clone(d.cfg.Bots)
	d.view.Store(&view{bots: maps.Clone(d.bots), cfg: &cfg, ui: maps.Clone(d.ui)})
}

// Lookup and All implement bot.Directory.
func (d *Daemon) Lookup(name string) (*bot.Bot, bool) {
	b, ok := d.view.Load().bots[name]
	return b, ok
}

func (d *Daemon) All() []*bot.Bot {
	m := d.view.Load().bots
	out := make([]*bot.Bot, 0, len(m))
	for _, b := range m {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// SetWebAdmin enables the /v1/web/* device endpoints; call it before Run.
func (d *Daemon) SetWebAdmin(w api.WebAdmin) { d.web = w }

func (d *Daemon) closeAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, b := range d.bots {
		b.Stop()
	}
	d.ln.Close()
	_ = os.Remove(d.cfg.Socket)
	d.st.Close()
}

// Run serves until ctx ends or Stop is called, then shuts down in order: the API, the bots
// (mail they had claimed goes back to the queue), and the store.
func (d *Daemon) Run(ctx context.Context) error {
	d.started = time.Now()
	if d.vm != nil {
		d.vm.Cleanup()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	d.mu.Lock()
	d.runCtx = ctx
	for _, name := range d.cfg.BotNames() {
		if err := d.bots[name].Start(ctx); err != nil {
			d.mu.Unlock()
			d.closeAll()
			return fmt.Errorf("start %s: %w", name, err)
		}
	}
	d.mu.Unlock()
	go func() {
		select {
		case <-d.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	go d.expireLoop(ctx)
	go d.routineLoop(ctx)
	srv := api.NewServer(d)
	srv.Web = d.web
	err := srv.Serve(ctx, d.ln)
	d.closeAll()
	return err
}

func (d *Daemon) bot(name string) (*bot.Bot, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	b, ok := d.bots[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", api.ErrUnknownBot, name)
	}
	return b, nil
}

func (d *Daemon) infos() []api.BotInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]api.BotInfo, 0, len(d.bots))
	for _, name := range d.cfg.BotNames() {
		info := d.bots[name].Info()
		info.Editable = d.ui[name]
		info.Capabilities = d.bots[name].Capabilities().String()
		out = append(out, info)
	}
	return out
}

func (d *Daemon) ListBots(context.Context) ([]api.BotInfo, error) { return d.infos(), nil }

func (d *Daemon) Status(context.Context) (api.Status, error) {
	return api.Status{Version: Version, UptimeSeconds: int64(time.Since(d.started).Seconds()), Bots: d.infos(), VM: d.vmLine()}, nil
}

func (d *Daemon) Cost(_ context.Context, q api.CostQuery) ([]api.CostRow, error) {
	if q.Bot != "" {
		if _, err := d.bot(q.Bot); err != nil {
			return nil, err
		}
	}
	days, err := d.st.CostByDay(q.Bot, q.From, q.To)
	if err != nil {
		return nil, err
	}
	rows := make([]api.CostRow, len(days))
	for i, c := range days {
		rows[i] = api.CostRow{Bot: c.Bot, Day: c.Day, USD: c.USD}
	}
	return rows, nil
}

func (d *Daemon) Send(_ context.Context, name, text string) (string, error) {
	b, err := d.bot(name)
	if err != nil {
		return "", err
	}
	return b.Send(text)
}

func (d *Daemon) Subscribe(ctx context.Context, name string, after uint64) (<-chan api.Event, error) {
	b, err := d.bot(name)
	if err != nil {
		return nil, err
	}
	return b.Subscribe(ctx, after), nil
}

func (d *Daemon) Stop(context.Context) error {
	d.stopOnce.Do(func() { close(d.stop) })
	return nil
}

func (d *Daemon) vmLine() string {
	if d.vm == nil {
		return ""
	}
	s := d.vm.Status()
	if !s.Prepared {
		return "no base prepared (ughbot vm prepare)"
	}
	line := fmt.Sprintf("base from %s, %d/%d running", s.Base.At.Format(time.DateOnly), len(s.Running), s.Max)
	for _, r := range s.Running {
		line += "; " + r
	}
	return line
}

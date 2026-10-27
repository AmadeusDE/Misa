// Package bot runs one configured ughbot bot: a resumable main-thread agent that
// reads its mail, spends within its caps and streams what it does to subscribers.
package bot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/agent"
	"github.com/AmadeusDE/Misa/internal/ugh/agent/host"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/ledger"
	"github.com/AmadeusDE/Misa/internal/ughbot/sandbox"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

const (
	StateIdle    = "idle"
	StateRunning = "running"
	StateStopped = "stopped"

	UserSender = "user"
	pollEvery  = 2 * time.Second
)

type Deps struct {
	Config *config.Config
	Bot    config.Bot
	Store  *store.Store
	Ledger *ledger.Ledger
	// Directory finds the other bots for the bot-to-bot tools; nil leaves the bot alone.
	Directory Directory
	// Proposer queues what a bot asks a person to approve: bot changes, routines, flagged calls.
	Proposer Proposer
	// Scheduler adds, lists and deletes routines; nil leaves the schedule tools out.
	Scheduler Scheduler
	// Sandbox starts the VMs code tasks run their tools in; Locks keeps two tasks out of one directory.
	Sandbox sandbox.Sandbox
	Locks   *DirLocks
	// Provider replaces the model client resolved from the bot's model (tests).
	Provider llm.Provider
}

type Bot struct {
	d     Deps
	name  string
	caps  ledger.Caps
	roots roots.Roots
	cwd   string
	feed  *feed
	run   *host.Runner
	wake  chan struct{}
	state atomic.Value
	tasks taskBook
	code  codeBook
	// codeTools is set when the bot was built with the code tools.
	codeTools bool

	mu        sync.Mutex
	cancel    context.CancelFunc
	ctx       context.Context
	runCancel context.CancelFunc // the current turn's run, for Interrupt
	wg        sync.WaitGroup
}

// New builds the bot's agent, resuming its latest session when one exists.
func New(d Deps) (*Bot, error) {
	b := &Bot{d: d, name: d.Bot.Name, feed: newFeed(), wake: make(chan struct{}, 1),
		caps:  ledger.Caps{Daily: d.Bot.Spend.Daily, Monthly: d.Bot.Spend.Monthly},
		roots: roots.Roots{Config: d.Config.ConfigDir, State: d.Config.SessionDir(d.Bot.Name)}}
	b.state.Store(StateStopped)
	b.cwd = d.Bot.Workdir
	if b.cwd == "" {
		b.cwd = filepath.Join(b.roots.State, "work")
	}
	if err := os.MkdirAll(b.cwd, 0o755); err != nil {
		return nil, err
	}
	reg := b.mainTools()
	_, b.codeTools = reg.Get("code_task")
	run, err := b.newRunner(runnerSpec{roots: b.roots, task: "main", resume: true, tools: reg, hook: b.hook("main"), events: true})
	if err != nil {
		return nil, fmt.Errorf("bot %q: %w", b.name, err)
	}
	b.run = run
	return b, nil
}

func (b *Bot) Name() string { return b.name }

func (b *Bot) Info() api.BotInfo {
	return api.BotInfo{Name: b.name, State: b.state.Load().(string), Model: b.d.Bot.Model}
}

func (b *Bot) hook(task string) ledger.Hook { return b.d.Ledger.Hook(b.name, task, b.caps) }

type runnerSpec struct {
	roots  roots.Roots
	task   string
	resume bool
	tools  *tools.Registry
	hook   ledger.Hook
	events bool
}

// newRunner builds one isolated agent. Delegation (spawn, council, worktrees) stays off
// until child agents get path isolation: start_task is the only way to run another agent.
func (b *Bot) newRunner(s runnerSpec) (*host.Runner, error) {
	system, err := b.system()
	if err != nil {
		return nil, err
	}
	o := host.Options{
		Cwd: b.cwd, Model: b.d.Bot.Model, System: system, Roots: s.roots,
		Prompter: &headlessPrompter{bot: b}, Provider: b.d.Provider, Tools: s.tools,
		Configure: func(c *agent.Config) {
			c.BeforeRequest = s.hook
			c.CompactThreshold = b.d.Bot.CompactAt
			c.Spawn, c.StartChild, c.Resume = nil, nil, nil
			c.Context.AskUser = nil
			if s.events {
				c.Events = b.onAgentEvent
			}
		},
	}
	if infos, err := session.ListRootsIn(s.roots, b.cwd, 1); s.resume && err == nil && len(infos) > 0 {
		o.Resume = true
		o.OverrideResumed = true
	}
	return host.New(context.Background(), o)
}

// system is the configured prompt, plus the bot's own name when it deals with other bots.
func (b *Bot) system() (string, error) {
	prompt := b.d.Bot.PromptText
	if prompt == "" && b.d.Bot.Prompt != "" {
		raw, err := os.ReadFile(b.d.Bot.Prompt)
		if err != nil {
			return "", err
		}
		prompt = string(raw)
	}
	if c := b.d.Bot.Capabilities; len(c.Message) > 0 || len(c.Manage) > 0 || c.CreateBots {
		prompt = strings.TrimRight(prompt, "\n") + fmt.Sprintf("\n\nYou are the bot named %q; other bots know you by that name.", b.name)
	}
	return strings.TrimLeft(prompt, "\n"), nil
}

// Resumed is how many history messages the main thread was rebuilt from.
func (b *Bot) Resumed() int { return b.run.Resumed }

// Start requeues mail a previous run left claimed and begins serving the bot's mail.
func (b *Bot) Start(ctx context.Context) error {
	if _, err := b.d.Store.RequeueClaimed(b.name); err != nil {
		return err
	}
	b.mu.Lock()
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.mu.Unlock()
	b.setState(StateIdle)
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.loop(b.ctx)
	}()
	b.Notify()
	return nil
}

// Stop ends the bot's loop and tasks, then returns mail it had claimed to the queue.
func (b *Bot) Stop() {
	b.mu.Lock()
	cancel := b.cancel
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	b.wg.Wait()
	b.setState(StateStopped)
	b.feed.closeAll()
	_, _ = b.d.Store.RequeueClaimed(b.name)
	_ = b.run.Close()
}

func (b *Bot) setState(s string) { b.state.Store(s) }

// Notify tells the bot that mail is waiting.
func (b *Bot) Notify() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Send queues text from the user and returns the mail id as the ticket.
func (b *Bot) Send(text string) (string, error) {
	id, err := b.deliver(store.Mail{From: UserSender, Body: text})
	return strconv.FormatInt(id, 10), err
}

// Deliver queues mail from another part of the daemon (an approval outcome, say).
func (b *Bot) Deliver(from, text string) (string, error) {
	id, err := b.deliver(store.Mail{From: from, Body: text})
	return strconv.FormatInt(id, 10), err
}

func (b *Bot) deliver(m store.Mail) (int64, error) {
	m.To = b.name
	id, err := b.d.Store.Enqueue(m)
	if err != nil {
		return 0, err
	}
	b.Notify()
	return id, nil
}

// Interrupt cancels the turn in progress, if any; the mail it was handling counts as read.
func (b *Bot) Interrupt(by string) bool {
	b.mu.Lock()
	cancel := b.runCancel
	b.mu.Unlock()
	if cancel == nil {
		return false
	}
	b.feed.publish("note", textData{Text: "turn interrupted by " + by})
	cancel()
	return true
}

// Status describes the bot for a managing bot: state, queue, tasks and spend.
func (b *Bot) Status() (string, error) {
	queued, err := b.d.Store.Pending(b.name)
	if err != nil {
		return "", err
	}
	now := time.Now()
	today, err := b.d.Store.SpendSince(b.name, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
	if err != nil {
		return "", err
	}
	month, err := b.d.Store.SpendSince(b.name, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()))
	if err != nil {
		return "", err
	}
	limit := func(v float64) string {
		if v <= 0 {
			return "no cap"
		}
		return fmt.Sprintf("cap $%.2f", v)
	}
	return fmt.Sprintf("%s: %s, %d mail queued, %d/%d tasks and %d code tasks running, spent $%.4f today (%s), $%.4f this month (%s)",
		b.name, b.state.Load(), queued, b.tasks.running.Load(), b.d.Bot.Capabilities.Tasks, len(b.code.list()),
		today, limit(b.caps.Daily), month, limit(b.caps.Monthly)), nil
}

func (b *Bot) Subscribe(ctx context.Context, after uint64) <-chan api.Event {
	return b.feed.subscribe(ctx, after)
}

func (b *Bot) loop(ctx context.Context) {
	tick := time.NewTicker(pollEvery)
	defer tick.Stop()
	for ctx.Err() == nil {
		m, ok, err := b.d.Store.Claim(b.name)
		switch {
		case err != nil:
			b.feed.publish("error", errorData{Message: err.Error()})
		case ok:
			b.turn(ctx, m)
			continue
		}
		select {
		case <-ctx.Done():
		case <-b.wake:
		case <-tick.C:
		}
	}
}

func mailPrompt(m store.Mail) string {
	switch {
	case m.From == UserSender:
		return m.Body
	case m.Kind == store.KindAsk:
		return fmt.Sprintf("[question #%d from %s; answer with reply(question=%d)]\n%s", m.ID, m.From, m.ID, m.Body)
	case m.Kind == store.KindReply:
		return fmt.Sprintf("[reply from %s to your question #%d]\n%s", m.From, m.ReplyTo, m.Body)
	case m.Kind == store.KindSteer:
		return fmt.Sprintf("[steering from %s, who manages you]\n%s", m.From, m.Body)
	}
	return fmt.Sprintf("[mail from %s]\n%s", m.From, m.Body)
}

func (b *Bot) announce(m store.Mail) {
	if m.From == UserSender {
		b.feed.publish("user", textData{Text: m.Body})
		return
	}
	b.feed.publish("mail", mailData{From: m.From, Text: m.Body})
}

// turn runs the agent on first and on any mail that arrives meanwhile: the agent takes it at
// its next turn boundary, and what it never reached starts a further run.
func (b *Bot) turn(ctx context.Context, first store.Mail) {
	b.setState(StateRunning)
	defer b.setState(StateIdle)
	handled := []store.Mail{first}
	b.announce(first)
	prompt := mailPrompt(first)
	for {
		runCtx, cancel := context.WithCancel(ctx)
		b.mu.Lock()
		b.runCancel = cancel
		b.mu.Unlock()
		done := make(chan struct{})
		var final string
		var err error
		go func() { final, err = b.run.Run(runCtx, prompt); close(done) }()
	wait:
		for {
			select {
			case <-done:
				break wait
			case <-b.wake:
				for {
					m, ok, cerr := b.d.Store.Claim(b.name)
					if cerr != nil || !ok {
						break
					}
					handled = append(handled, m)
					b.announce(m)
					b.run.Agent.Steer(mailPrompt(m))
				}
			case <-ctx.Done():
				cancel()
				<-done
				break wait
			}
		}
		b.mu.Lock()
		b.runCancel = nil
		b.mu.Unlock()
		cancel()
		if ctx.Err() != nil {
			return // claimed mail goes back to the queue on Stop
		}
		b.finish(final, err)
		rest := b.run.Agent.TakeSteered()
		if len(rest) == 0 {
			break
		}
		prompt = joinLines(rest)
	}
	for _, m := range handled {
		_ = b.d.Store.Ack(m.ID)
	}
}

func joinLines(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n\n"
		}
		out += p
	}
	return out
}

func (b *Bot) finish(final string, err error) {
	switch {
	case err == nil:
		b.feed.publish("turn_end", turnEndData{Text: final, Usage: b.run.Agent.Usage})
	case errors.Is(err, agent.ErrBudget):
		b.feed.publish("budget", errorData{Message: err.Error()})
	default:
		b.feed.publish("error", errorData{Message: err.Error()})
	}
}

func (b *Bot) onAgentEvent(ev agent.Event) {
	switch ev.Kind {
	case "text":
		b.feed.publish("text", textData{Text: ev.Text})
	case "tool_start":
		b.feed.publish("tool_call", toolCallData{ID: ev.Call.ID, Name: ev.Call.Name, Args: ev.Call.Args})
	case "tool_done":
		b.feed.publish("tool_result", toolResultData{ID: ev.Call.ID, Name: ev.Call.Name, Output: clip(ev.Output), Error: ev.Err != nil})
	case "note":
		b.feed.publish("note", textData{Text: ev.Text})
	}
}

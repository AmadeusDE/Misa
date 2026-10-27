package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/agent"
	"github.com/AmadeusDE/Misa/internal/ugh/agent/host"
	"github.com/AmadeusDE/Misa/internal/ugh/confine"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
	"github.com/AmadeusDE/Misa/internal/ugh/worktree"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

const codeSender = "code:"

const (
	codeBooting = "booting"
	codeRunning = "running"
	codeAsking  = "waiting for your reply"
)

// DirLocks keeps two code tasks, of any bots, out of the same directory.
type DirLocks struct {
	mu   sync.Mutex
	held map[string]string
}

func NewDirLocks() *DirLocks { return &DirLocks{held: map[string]string{}} }

func (l *DirLocks) take(dir, who string) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for d, holder := range l.held {
		if inside(dir, d) || inside(d, dir) {
			return nil, fmt.Errorf("%s is busy: code task %s is working in %s", dir, holder, d)
		}
	}
	l.held[dir] = who
	return func() {
		l.mu.Lock()
		delete(l.held, dir)
		l.mu.Unlock()
	}, nil
}

func inside(path, dir string) bool {
	r, err := filepath.Rel(dir, path)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

type codeBook struct {
	mu    sync.Mutex
	next  int
	tasks map[string]*codeTask
}

func (c *codeBook) add(t *codeTask) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next++
	t.id = fmt.Sprintf("c%d", c.next)
	if c.tasks == nil {
		c.tasks = map[string]*codeTask{}
	}
	c.tasks[t.id] = t
}

func (c *codeBook) get(id string) (*codeTask, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.tasks[id]
	return t, ok
}

func (c *codeBook) remove(id string) {
	c.mu.Lock()
	delete(c.tasks, id)
	c.mu.Unlock()
}

func (c *codeBook) list() []*codeTask {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*codeTask, 0, len(c.tasks))
	for _, t := range c.tasks {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].started.Before(out[j].started) })
	return out
}

// codeTask is one ugh session working in a code directory with its tools in a VM. It runs on
// the bot's whole budget, and its questions and permission prompts go to the bot as mail.
type codeTask struct {
	b       *Bot
	id      string
	root    string // the code directory the VM shares
	cwd     string
	started time.Time
	ctx     context.Context
	cancel  context.CancelFunc
	state   atomic.Value
	machine atomic.Value
	run     atomic.Pointer[host.Runner]

	mu   sync.Mutex
	asks map[int64]chan string
}

func (t *codeTask) sender() string { return codeSender + t.id }

func (t *codeTask) describe() string {
	line := fmt.Sprintf("%s · %s · %s · %s", t.id, t.state.Load(), t.cwd, time.Since(t.started).Round(time.Second))
	if m, _ := t.machine.Load().(string); m != "" {
		line += " · " + m
	}
	t.mu.Lock()
	for id := range t.asks {
		line += fmt.Sprintf(" · question #%d open", id)
	}
	t.mu.Unlock()
	return line
}

func (b *Bot) startCode(dir, prompt string) (string, error) {
	root, err := config.CodeDir(b.codeDirs(), dir)
	if err != nil {
		return "", err
	}
	cwd := filepath.Clean(config.ExpandHome(dir))
	if fi, err := os.Stat(cwd); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("%s is not a directory", cwd)
	}
	if b.d.Sandbox == nil || b.d.Locks == nil {
		return "", errors.New("code tasks are not available: this daemon runs no VMs")
	}
	b.mu.Lock()
	ctx := b.ctx
	b.mu.Unlock()
	if ctx == nil || ctx.Err() != nil {
		return "", errors.New(b.name + " is stopping")
	}
	t := &codeTask{b: b, root: root, cwd: cwd, started: time.Now(), asks: map[int64]chan string{}}
	b.code.add(t)
	release, err := b.d.Locks.take(root, b.name+"/"+t.id)
	if err != nil {
		b.code.remove(t.id)
		return "", err
	}
	t.state.Store(codeBooting)
	t.ctx, t.cancel = context.WithCancel(ctx)
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer release()
		defer b.code.remove(t.id)
		b.runCode(t, prompt)
	}()
	return fmt.Sprintf("code task %s started in %s. It boots a VM first; its questions arrive as [question #N from %s] "+
		"for you to answer with reply, and its final report as mail from %s.", t.id, cwd, t.sender(), t.sender()), nil
}

func (b *Bot) runCode(t *codeTask, prompt string) {
	defer t.cancel()
	b.feed.publish("task", taskData{ID: t.id, State: codeBooting})
	report, err := t.work(prompt)
	b.mu.Lock()
	stopping := b.ctx.Err() != nil
	b.mu.Unlock()
	if stopping {
		return
	}
	state := "done"
	switch {
	case t.ctx.Err() != nil:
		state, report = "stopped", fmt.Sprintf("code task %s was stopped.\n\n%s", t.id, report)
	case err != nil:
		state, report = "failed", fmt.Sprintf("code task %s failed: %v\n\n%s", t.id, err, report)
	}
	b.feed.publish("task", taskData{ID: t.id, State: state})
	_, _ = b.deliver(store.Mail{From: t.sender(), Body: strings.TrimSpace(report)})
}

func (t *codeTask) work(prompt string) (string, error) {
	b := t.b
	base := filepath.Join(b.d.Config.StateDir, "code", b.name, t.id)
	worktrees := filepath.Join(base, "worktrees")
	if err := os.MkdirAll(worktrees, 0o700); err != nil {
		return "", err
	}
	defer os.Remove(worktrees) // only when no worktree was left behind
	shares := []string{t.root, worktrees}
	for _, d := range shares {
		defer confine.Add(d)()
	}
	mc, err := b.d.Sandbox.Start(t.ctx, shares)
	if err != nil {
		return "", err
	}
	defer mc.Close()
	t.machine.Store(mc.Describe())
	t.state.Store(codeRunning)
	b.feed.publish("task", taskData{ID: t.id, State: codeRunning})
	run, err := host.New(t.ctx, host.Options{
		Cwd: t.cwd, Model: b.d.Bot.Model, System: t.system(), Provider: b.d.Provider,
		Roots:    roots.Roots{Config: b.roots.Config, State: filepath.Join(base, "state")},
		Prompter: codePrompter{t},
		Configure: func(c *agent.Config) {
			c.BeforeRequest = b.hook(t.sender())
			c.Context.AskUser = t.ask
		},
	})
	if err != nil {
		return "", err
	}
	defer run.Close()
	run.Runtime.Config.Worktree.Dir = worktrees
	t.run.Store(run)
	ctx := tools.WithExecutor(worktree.WithGit(t.ctx, mc.Git), mc)
	final, err := run.Run(ctx, prompt)
	return final + t.gitSummary(ctx), err
}

func (t *codeTask) system() string {
	return agent.DefaultSystem + fmt.Sprintf("\n\nYou work for the bot %q, which gave you this task. Your tools run in a throwaway "+
		"Arch Linux VM that shares only %s (at the same path); nothing else of the host is there, and the VM is discarded "+
		"when you finish, so anything that must last has to be written inside %s. ask_user reaches that bot, not a person.",
		t.b.name, t.root, t.root)
}

// gitSummary tells the bot where the work landed: the branch and what is left uncommitted.
func (t *codeTask) gitSummary(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	branch, _, err := worktree.Git(ctx, t.cwd, nil, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	status, _, _ := worktree.Git(ctx, t.cwd, nil, "status", "--short")
	s := "\n\n---\nbranch: " + strings.TrimSpace(string(branch))
	lines := strings.Split(strings.TrimRight(string(status), "\n"), "\n")
	switch {
	case len(status) == 0:
		s += "\nworking tree clean"
	case len(lines) > 20:
		s += fmt.Sprintf("\nuncommitted:\n%s\n… %d more", strings.Join(lines[:20], "\n"), len(lines)-20)
	default:
		s += "\nuncommitted:\n" + strings.Join(lines, "\n")
	}
	if list, _, err := worktree.Git(ctx, t.cwd, nil, "worktree", "list"); err == nil {
		if wts := strings.Split(strings.TrimSpace(string(list)), "\n"); len(wts) > 1 {
			s += "\nworktrees still present:\n" + strings.Join(wts[1:], "\n")
		}
	}
	return s
}

// question mails the bot and waits for its reply, the task's end, or the bot's.
func (t *codeTask) question(body string) (string, error) {
	t.mu.Lock()
	id, err := t.b.deliver(store.Mail{From: t.sender(), Kind: store.KindAsk, Body: body})
	if err != nil {
		t.mu.Unlock()
		return "", err
	}
	ch := make(chan string, 1)
	t.asks[id] = ch
	t.mu.Unlock()
	t.state.Store(codeAsking)
	defer t.state.Store(codeRunning)
	defer func() {
		t.mu.Lock()
		delete(t.asks, id)
		t.mu.Unlock()
	}()
	select {
	case text := <-ch:
		return text, nil
	case <-t.ctx.Done():
		return "", t.ctx.Err()
	}
}

func (t *codeTask) answer(question int64, text string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	ch, ok := t.asks[question]
	if !ok {
		return fmt.Errorf("code task %s is no longer waiting on #%d", t.id, question)
	}
	delete(t.asks, question)
	ch <- text
	return nil
}

func (t *codeTask) ask(_ context.Context, req agent.AskRequest) (agent.AskAnswer, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Code task %s (in %s) asks:\n", t.id, t.cwd)
	for i, q := range req.Questions {
		fmt.Fprintf(&b, "\n%d. [%s] %s\n", i+1, q.Header, q.Question)
		for _, o := range q.Options {
			fmt.Fprintf(&b, "   - %s: %s\n", o.Label, o.Description)
		}
	}
	b.WriteString("\nAnswer every question in one reply; pick an option or write your own.")
	text, err := t.question(b.String())
	if err != nil {
		return agent.AskAnswer{Declined: true}, err
	}
	replies := make([]agent.AskReply, len(req.Questions))
	for i := range replies {
		replies[i].Other = "(answered together with question 1)"
	}
	if len(replies) > 0 {
		replies[0].Other = text
	}
	return agent.AskAnswer{Replies: replies}, nil
}

func (b *Bot) answerCode(q store.Mail, text string) (string, error) {
	t, ok := b.code.get(strings.TrimPrefix(q.From, codeSender))
	if !ok {
		return "", fmt.Errorf("%s has finished; #%d needs no reply", q.From, q.ID)
	}
	if err := t.answer(q.ID, text); err != nil {
		return "", err
	}
	return fmt.Sprintf("reply to #%d sent to %s", q.ID, q.From), nil
}

type codeTaskTool struct{ b *Bot }

func (t codeTaskTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "code_task",
		Description: "Start a coding session (ugh, with its subagents, git worktrees and council) in a directory inside " +
			strings.Join(t.b.codeDirs(), ", ") + ". Its tools run in a throwaway Linux VM that shares only that " +
			"directory, so its edits land in the real files. It works in the background on your budget. Its questions and " +
			"permission requests arrive as [question #N from code:<id>] mail for you to answer with reply; its final report " +
			"arrives as mail from code:<id>. Give it everything it needs: it cannot see your conversation.",
		Parameters: objectSchema([]string{"dir", "prompt"}, map[string]any{
			"dir": str("the directory to work in"), "prompt": str("the task, with its acceptance checks")})}
}

func (t codeTaskTool) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var a struct{ Dir, Prompt string }
	if err := decode(raw, &a, "dir", "prompt"); err != nil {
		return "", err
	}
	return t.b.startCode(a.Dir, a.Prompt)
}

type codeTasksTool struct{ b *Bot }

func (codeTasksTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "code_tasks", Description: "List your running code tasks.", Parameters: objectSchema(nil, map[string]any{})}
}

func (t codeTasksTool) Run(context.Context, string, json.RawMessage) (string, error) {
	var lines []string
	for _, c := range t.b.code.list() {
		lines = append(lines, c.describe())
	}
	if len(lines) == 0 {
		return "no code tasks running", nil
	}
	return strings.Join(lines, "\n"), nil
}

type codeSteerTool struct{ b *Bot }

func (codeSteerTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "code_steer", Description: "Send a running code task a message it reads at its next step, or stop it.",
		Parameters: objectSchema([]string{"task"}, map[string]any{
			"task": str("the code task id, e.g. c1"),
			"text": str("the message; leave out with stop"),
			"stop": map[string]any{"type": "boolean", "description": "stop the task instead; its VM is discarded"}})}
}

func (s codeSteerTool) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var a struct {
		Task, Text string
		Stop       bool
	}
	if err := decode(raw, &a, "task"); err != nil {
		return "", err
	}
	t, ok := s.b.code.get(a.Task)
	if !ok {
		return "", fmt.Errorf("no running code task %q", a.Task)
	}
	if a.Stop {
		t.cancel()
		return "stopping code task " + t.id, nil
	}
	run := t.run.Load()
	if strings.TrimSpace(a.Text) == "" || run == nil {
		return "", errors.New("give text to send, and wait until the task has booted")
	}
	run.Agent.Steer("[message from " + s.b.name + ", who started this task]\n" + a.Text)
	return "sent to code task " + t.id, nil
}

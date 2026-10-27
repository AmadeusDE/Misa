package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
	"github.com/AmadeusDE/Misa/internal/ughbot/ledger"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

const (
	taskSender = "task:"
	// defaultSliceShare is the part of a bot's remaining cap a task gets when it names no budget.
	defaultSliceShare = 0.25
)

type taskBook struct {
	next    atomic.Int64
	running atomic.Int64
}

// startTask is the tool a main thread uses to run a separate agent. The task has its own
// session and a slice of the bot's remaining budget; its final result comes back as mail.
type startTask struct{ b *Bot }

func (startTask) Spec() llm.ToolSpec {
	return llm.ToolSpec{
		Name: "start_task",
		Description: "Start a separate agent on a self-contained task and keep working. It has its own session and a " +
			"slice of your remaining budget; when it finishes, its final result arrives as mail from task:<id>.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt":     map[string]any{"type": "string", "description": "everything the task agent needs to know"},
				"budget_usd": map[string]any{"type": "number", "description": "most the task may spend; clipped to your remaining budget"},
			},
			"required": []string{"prompt"},
		},
	}
}

func (s startTask) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var args struct {
		Prompt    string  `json:"prompt"`
		BudgetUSD float64 `json:"budget_usd"`
	}
	if err := json.Unmarshal(raw, &args); err != nil || args.Prompt == "" {
		return "", errors.New("bad arguments: prompt is required")
	}
	return s.b.startTask(args.Prompt, args.BudgetUSD, nil)
}

// startTask runs a task session as b, within b's tasks capability and budget. Its result
// is mailed to owner, a managing bot, or to b itself when owner is nil.
func (b *Bot) startTask(prompt string, want float64, owner *Bot) (string, error) {
	limit := int64(b.d.Bot.Capabilities.Tasks)
	if limit == 0 {
		return "", fmt.Errorf("%s may not run tasks", b.name)
	}
	if b.tasks.running.Add(1) > limit {
		b.tasks.running.Add(-1)
		return "", fmt.Errorf("%s is already running %d tasks; wait for one to finish", b.name, limit)
	}
	started := false
	defer func() {
		if !started {
			b.tasks.running.Add(-1)
		}
	}()
	if owner == nil {
		owner = b
	}
	slice, err := b.slice(want)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	ctx := b.ctx
	b.mu.Unlock()
	if ctx == nil || ctx.Err() != nil {
		return "", errors.New(b.name + " is stopping")
	}
	id := fmt.Sprintf("t%d", b.tasks.next.Add(1))
	sender := taskSender + id
	if owner != b {
		sender = taskSender + b.name + "/" + id
	}
	started = true
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer b.tasks.running.Add(-1)
		b.runTask(ctx, id, prompt, slice, owner, sender)
	}()
	budget := "no budget of its own beyond your caps"
	if slice > 0 {
		budget = fmt.Sprintf("a $%.2f budget", slice)
	}
	return fmt.Sprintf("task %s started with %s; its result will arrive as mail from %s", id, budget, sender), nil
}

// slice is the task's spend limit; 0 means only the bot's own caps apply.
func (b *Bot) slice(want float64) (float64, error) {
	left, limited, err := b.d.Ledger.Remaining(b.name, b.caps)
	if err != nil {
		return 0, err
	}
	if !limited {
		return max(want, 0), nil
	}
	if left <= 0 {
		return 0, errors.New("no budget remaining for a task")
	}
	if want <= 0 {
		want = left * defaultSliceShare
	}
	return min(want, left), nil
}

func (b *Bot) runTask(ctx context.Context, id, prompt string, slice float64, owner *Bot, sender string) {
	b.feed.publish("task", taskData{ID: id, State: "running"})
	hook := ledger.Limit(slice, b.hook(id))
	tasksRoot := roots.Roots{Config: b.roots.Config, State: filepath.Join(b.d.Config.StateDir, "tasks", b.name, id)}
	run, err := b.newRunner(runnerSpec{roots: tasksRoot, task: id, tools: baseTools(b.d.Bot.Capabilities), hook: hook})
	body := ""
	if err == nil {
		defer run.Close()
		var final string
		final, err = run.Run(ctx, prompt)
		body = final
	}
	if ctx.Err() != nil {
		return
	}
	state := "done"
	if err != nil {
		state, body = "failed", fmt.Sprintf("task failed: %v\n\n%s", err, body)
	}
	b.feed.publish("task", taskData{ID: id, State: state})
	_, _ = owner.deliver(store.Mail{From: sender, Body: body})
}

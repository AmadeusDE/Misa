package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

const (
	scheduleAddTool    = "schedule_add"
	scheduleListTool   = "schedule_list"
	scheduleCancelTool = "schedule_cancel"
)

// Scheduler is the daemon's routine store as the schedule tools need it.
type Scheduler interface {
	AddRoutine(ctx context.Context, req api.RoutineRequest) (api.RoutineInfo, error)
	Routines(ctx context.Context, bot string) ([]api.RoutineInfo, error)
	DeleteRoutine(ctx context.Context, id int64) error
}

type approvedKey struct{}

// RunApproved runs one of the bot's tools for a call a person approved, so the tool acts
// directly instead of asking for approval again.
func (b *Bot) RunApproved(ctx context.Context, tool string, args json.RawMessage) (string, error) {
	t, ok := b.mainTools().Get(tool)
	if !ok {
		return "", fmt.Errorf("%s no longer has the %s tool", b.name, tool)
	}
	return t.Run(context.WithValue(ctx, approvedKey{}, true), b.cwd, args)
}

// RunTool runs one of the bot's tools as the model would (tests and tools that act for the bot).
func (b *Bot) RunTool(ctx context.Context, tool string, args json.RawMessage) (string, error) {
	t, ok := b.mainTools().Get(tool)
	if !ok {
		return "", fmt.Errorf("%s has no %s tool", b.name, tool)
	}
	return t.Run(ctx, b.cwd, args)
}

// Prompter is the permission prompter the bot's agents use.
func (b *Bot) Prompter() perm.Prompter { return &headlessPrompter{bot: b} }

// mayScheduleDirectly is true for a bot with the schedule capability or a call already approved.
func (b *Bot) mayScheduleDirectly(ctx context.Context) bool {
	approved, _ := ctx.Value(approvedKey{}).(bool)
	return approved || b.d.Bot.Capabilities.Schedule
}

func (b *Bot) target(name string) (string, error) {
	if name == "" || name == b.name {
		return b.name, nil
	}
	if _, err := b.peer(name, b.d.Bot.Capabilities.MayMessage, "schedule mail to"); err != nil {
		return "", err
	}
	return name, nil
}

func (b *Bot) routineNote() string {
	if b.d.Bot.Capabilities.Schedule {
		return ""
	}
	return " You lack the schedule capability, so each add or cancel waits for a person's approval; the outcome arrives as mail."
}

type scheduleAdd struct{ b *Bot }

func (t scheduleAdd) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: scheduleAddTool,
		Description: "Add a routine: at each fire, prompt is mailed to a bot (yourself by default, or a bot you may " +
			"message) as mail from routine #N. Give cron (5 fields, e.g. \"0 9 * * 1\" or @daily) for a repeating " +
			"routine, or at (\"in 2h\", \"2026-10-20 09:00\", RFC 3339) for a one-shot reminder." + t.b.routineNote(),
		Parameters: objectSchema([]string{"prompt"}, map[string]any{
			"prompt": str("what the bot is told when it fires; make it self-contained"),
			"bot":    str("who gets it; default yourself"),
			"cron":   str("minute hour day-of-month month day-of-week"),
			"at":     str("one-shot time"),
			"tz":     str("IANA timezone such as Europe/Berlin; default the server's"),
			"missed": map[string]any{"type": "string", "enum": []string{"once", "skip"},
				"description": "if the daemon was down at fire time: fire once on return (default) or skip"},
		})}
}

func (t scheduleAdd) Run(ctx context.Context, _ string, raw json.RawMessage) (string, error) {
	var req api.RoutineRequest
	if err := decode(raw, &req, "prompt"); err != nil {
		return "", err
	}
	to, err := t.b.target(req.Bot)
	if err != nil {
		return "", err
	}
	req.Bot, req.Owner = to, t.b.name
	if !t.b.mayScheduleDirectly(ctx) {
		if t.b.d.Proposer == nil {
			return "", fmt.Errorf("routines need approval and approvals are not available")
		}
		id, err := t.b.d.Proposer.ProposeRoutine(t.b.name, req)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("approval #%d requested for the routine; the outcome arrives as mail. Keep working.", id), nil
	}
	info, err := t.b.d.Scheduler.AddRoutine(ctx, req)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("routine #%d added; next fire %s", info.ID, info.Next.Local().Format("2006-01-02 15:04 MST")), nil
}

type scheduleList struct{ b *Bot }

func (scheduleList) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: scheduleListTool, Description: "List routines that mail you or that you made.",
		Parameters: objectSchema(nil, map[string]any{})}
}

func (t scheduleList) Run(ctx context.Context, _ string, _ json.RawMessage) (string, error) {
	all, err := t.b.d.Scheduler.Routines(ctx, "")
	if err != nil {
		return "", err
	}
	var lines []string
	for _, r := range all {
		if r.Bot != t.b.name && r.Owner != t.b.name {
			continue
		}
		when := "cron " + r.Cron
		if r.Cron == "" {
			when = "at " + r.At.Local().Format("2006-01-02 15:04")
		}
		next := "-"
		if !r.Next.IsZero() && r.State == "active" {
			next = r.Next.Local().Format("2006-01-02 15:04 MST")
		}
		lines = append(lines, fmt.Sprintf("#%d %s -> %s, %s, next %s, made by %s: %s", r.ID, r.State, r.Bot, when, next, r.Owner, r.Prompt))
	}
	if len(lines) == 0 {
		return "no routines", nil
	}
	return strings.Join(lines, "\n"), nil
}

type scheduleCancel struct{ b *Bot }

func (t scheduleCancel) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: scheduleCancelTool,
		Description: "Delete a routine that mails you or that you made." + t.b.routineNote(),
		Parameters:  objectSchema([]string{"id"}, map[string]any{"id": map[string]any{"type": "integer"}})}
}

func (t scheduleCancel) Run(ctx context.Context, _ string, raw json.RawMessage) (string, error) {
	var a struct{ ID int64 }
	if err := decode(raw, &a); err != nil || a.ID == 0 {
		return "", fmt.Errorf("bad arguments: id is required")
	}
	all, err := t.b.d.Scheduler.Routines(ctx, "")
	if err != nil {
		return "", err
	}
	mine := false
	for _, r := range all {
		mine = mine || r.ID == a.ID && (r.Bot == t.b.name || r.Owner == t.b.name)
	}
	if !mine {
		return "", fmt.Errorf("#%d is not a routine that mails you or that you made", a.ID)
	}
	if !t.b.mayScheduleDirectly(ctx) {
		id, err := t.b.d.Proposer.ProposeCancel(t.b.name, a.ID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("approval #%d requested to delete routine #%d; the outcome arrives as mail.", id, a.ID), nil
	}
	if err := t.b.d.Scheduler.DeleteRoutine(ctx, a.ID); err != nil {
		return "", err
	}
	return "routine #" + strconv.FormatInt(a.ID, 10) + " deleted", nil
}

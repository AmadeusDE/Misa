package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

// Directory is how a bot finds the others; the daemon implements it. It must not block
// on the daemon's own lock, since tools call it while the daemon may be stopping bots.
type Directory interface {
	Lookup(name string) (*Bot, bool)
	All() []*Bot
}

var errNoDirectory = errors.New("no other bots are reachable")

// peer resolves a target the bot may reach; allowed is the capability check, made on every call.
func (b *Bot) peer(name string, allowed func(string) bool, verb string) (*Bot, error) {
	if name == b.name {
		return nil, fmt.Errorf("you cannot %s yourself", verb)
	}
	if !allowed(name) {
		return nil, fmt.Errorf("you may not %s %q", verb, name)
	}
	if b.d.Directory == nil {
		return nil, errNoDirectory
	}
	p, ok := b.d.Directory.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("there is no bot named %q", name)
	}
	return p, nil
}

func objectSchema(required []string, props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func decode(raw json.RawMessage, v any, required ...string) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("bad arguments: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	for _, r := range required {
		s, isText := m[r].(string)
		if m[r] == nil || isText && strings.TrimSpace(s) == "" {
			return fmt.Errorf("bad arguments: %s is required", r)
		}
	}
	return nil
}

type listBots struct{ b *Bot }

func (listBots) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "list_bots", Description: "List the bots you may message or manage, with their state and model.",
		Parameters: objectSchema(nil, map[string]any{})}
}

func (t listBots) Run(context.Context, string, json.RawMessage) (string, error) {
	if t.b.d.Directory == nil {
		return "", errNoDirectory
	}
	c := t.b.d.Bot.Capabilities
	var lines []string
	for _, p := range t.b.d.Directory.All() {
		if p.name == t.b.name {
			continue
		}
		var may []string
		if c.MayMessage(p.name) {
			may = append(may, "message")
		}
		if c.MayManage(p.name) {
			may = append(may, "manage")
		}
		if len(may) > 0 {
			info := p.Info()
			lines = append(lines, fmt.Sprintf("%s (%s, %s): you may %s", p.name, info.State, info.Model, strings.Join(may, ", ")))
		}
	}
	if len(lines) == 0 {
		return "no bots you may reach exist yet", nil
	}
	return strings.Join(lines, "\n"), nil
}

type messageBot struct{ b *Bot }

func (messageBot) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "message_bot",
		Description: "Send mail to another bot and keep working. It reads it at its next turn; no reply is expected.",
		Parameters:  objectSchema([]string{"to", "text"}, map[string]any{"to": str("bot name"), "text": str("the message")})}
}

func (t messageBot) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var a struct{ To, Text string }
	if err := decode(raw, &a, "to", "text"); err != nil {
		return "", err
	}
	p, err := t.b.peer(a.To, t.b.d.Bot.Capabilities.MayMessage, "message")
	if err != nil {
		return "", err
	}
	id, err := p.deliver(store.Mail{From: t.b.name, Kind: store.KindMail, Body: a.Text})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("mail #%d queued for %s", id, a.To), nil
}

type askBot struct{ b *Bot }

func (askBot) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "ask_bot",
		Description: "Ask another bot a question and keep working. Its answer arrives later as a reply in your mail.",
		Parameters:  objectSchema([]string{"to", "question"}, map[string]any{"to": str("bot name"), "question": str("the question")})}
}

func (t askBot) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var a struct{ To, Question string }
	if err := decode(raw, &a, "to", "question"); err != nil {
		return "", err
	}
	p, err := t.b.peer(a.To, t.b.d.Bot.Capabilities.MayMessage, "message")
	if err != nil {
		return "", err
	}
	id, err := p.deliver(store.Mail{From: t.b.name, Kind: store.KindAsk, Body: a.Question})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("question #%d sent to %s; the reply will arrive as mail", id, a.To), nil
}

type replyTool struct{ b *Bot }

func (replyTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "reply",
		Description: "Answer a question another bot asked you (mail marked [question #N ...]). Each question takes one reply.",
		Parameters: objectSchema([]string{"question", "text"}, map[string]any{
			"question": map[string]any{"type": "integer", "description": "the question's number"},
			"text":     str("your answer")})}
}

func (t replyTool) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var a struct {
		Question int64
		Text     string
	}
	if err := decode(raw, &a, "text"); err != nil {
		return "", err
	}
	q, ok, err := t.b.d.Store.MailByID(a.Question)
	if err != nil {
		return "", err
	}
	if !ok || q.To != t.b.name || q.Kind != store.KindAsk {
		return "", fmt.Errorf("#%d is not a question asked of you", a.Question)
	}
	if strings.HasPrefix(q.From, codeSender) {
		return t.b.answerCode(q, a.Text)
	}
	if done, err := t.b.d.Store.Replied(t.b.name, q.ID); err != nil || done {
		return "", errors.Join(err, fmt.Errorf("you already replied to #%d", q.ID))
	}
	if t.b.d.Directory == nil {
		return "", errNoDirectory
	}
	p, ok := t.b.d.Directory.Lookup(q.From)
	if !ok {
		return "", fmt.Errorf("%s, who asked #%d, no longer exists", q.From, q.ID)
	}
	if _, err := p.deliver(store.Mail{From: t.b.name, Kind: store.KindReply, ReplyTo: q.ID, Body: a.Text}); err != nil {
		return "", err
	}
	return fmt.Sprintf("reply to #%d sent to %s", q.ID, q.From), nil
}

const (
	manageStatus    = "status"
	manageSteer     = "steer"
	manageInterrupt = "interrupt"
	manageTask      = "start_task"
)

type manageBot struct{ b *Bot }

func (manageBot) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "manage_bot",
		Description: "Manage a bot you are responsible for. status: its state, queue and spend. steer: send it " +
			"guidance it reads before other mail. interrupt: stop its current turn. start_task: run a task " +
			"session as that bot (its tools and budget); the result comes back to you as mail.",
		Parameters: objectSchema([]string{"bot", "action"}, map[string]any{
			"bot":        str("bot name"),
			"action":     map[string]any{"type": "string", "enum": []string{manageStatus, manageSteer, manageInterrupt, manageTask}},
			"text":       str("guidance for steer, or the task prompt for start_task"),
			"budget_usd": map[string]any{"type": "number", "description": "start_task only: most the task may spend"},
		})}
}

func (t manageBot) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var a struct {
		Bot, Action, Text string
		BudgetUSD         float64 `json:"budget_usd"`
	}
	if err := decode(raw, &a, "bot", "action"); err != nil {
		return "", err
	}
	p, err := t.b.peer(a.Bot, t.b.d.Bot.Capabilities.MayManage, "manage")
	if err != nil {
		return "", err
	}
	needText := func() error {
		if strings.TrimSpace(a.Text) == "" {
			return fmt.Errorf("bad arguments: %s needs text", a.Action)
		}
		return nil
	}
	switch a.Action {
	case manageStatus:
		return p.Status()
	case manageSteer:
		if err := needText(); err != nil {
			return "", err
		}
		id, err := p.deliver(store.Mail{From: t.b.name, Kind: store.KindSteer, Body: a.Text})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("steering #%d queued for %s", id, a.Bot), nil
	case manageInterrupt:
		if p.Interrupt(t.b.name) {
			return a.Bot + "'s current turn was interrupted", nil
		}
		return a.Bot + " was not running a turn", nil
	case manageTask:
		if err := needText(); err != nil {
			return "", err
		}
		return p.startTask(a.Text, a.BudgetUSD, t.b)
	}
	return "", fmt.Errorf("unknown action %q", a.Action)
}

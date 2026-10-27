package bot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

// Actions a bot with create_bots can ask for; each waits in the approval queue.
const (
	ActionCreate = "create_bot"
	ActionUpdate = "update_bot"
	ActionDelete = "delete_bot"
	// Routine changes by bots without the schedule capability.
	ActionAddRoutine    = "add_routine"
	ActionCancelRoutine = "cancel_routine"
	// A tool call the permission classifiers flagged, run as stored once approved.
	ActionToolCall = "tool_call"
)

// Proposer checks a requested change and queues it for a person to approve; the daemon implements it.
type Proposer interface {
	ProposeBot(from, action string, def config.Bot) (id int64, err error)
	ProposeRoutine(from string, req api.RoutineRequest) (id int64, err error)
	ProposeCancel(from string, routine int64) (id int64, err error)
	ProposeToolCall(from, tool string, args json.RawMessage, why string) (id int64, err error)
	ProposeRepo(from string, req RepoRequest) (id int64, err error)
}

// botSpec is a bot definition as the tools take it: the prompt is the text itself.
type botSpec struct {
	Name         string              `json:"name"`
	Model        string              `json:"model"`
	Prompt       string              `json:"prompt"`
	Capabilities config.Capabilities `json:"capabilities"`
	Spend        config.Spend        `json:"spend"`
	CompactAt    int                 `json:"compact_at,omitempty"`
	Workdir      string              `json:"workdir,omitempty"`
}

func specOf(b config.Bot) botSpec {
	return botSpec{Name: b.Name, Model: b.Model, Prompt: b.PromptText, Capabilities: b.Capabilities,
		Spend: b.Spend, CompactAt: b.CompactAt, Workdir: b.Workdir}
}

func (s botSpec) bot() config.Bot {
	return config.Bot{Name: s.Name, Model: s.Model, PromptText: s.Prompt, Capabilities: s.Capabilities,
		Spend: s.Spend, CompactAt: s.CompactAt, Workdir: s.Workdir}
}

func botSchema(required []string) map[string]any {
	names := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	return objectSchema(required, map[string]any{
		"name":   str("lowercase letters, digits, - and _"),
		"model":  str("an ugh model ref such as openrouter/<provider>/<model>"),
		"prompt": str("the bot's whole system prompt"),
		"capabilities": map[string]any{"type": "object", "properties": map[string]any{
			"files":       map[string]any{"type": "string", "enum": []string{"", config.FilesRead, config.FilesWrite}},
			"shell":       map[string]any{"type": "boolean"},
			"tasks":       map[string]any{"type": "integer", "description": "task sessions at once; 0 = none"},
			"message":     names,
			"manage":      names,
			"create_bots": map[string]any{"type": "boolean"},
			"schedule":    map[string]any{"type": "boolean"},
			"code": map[string]any{"type": "array", "items": map[string]any{"type": "string"},
				"description": "absolute host directories it may start code tasks in"},
		}},
		"spend": map[string]any{"type": "object", "properties": map[string]any{
			"daily": map[string]any{"type": "number"}, "monthly": map[string]any{"type": "number"}}},
		"compact_at": map[string]any{"type": "integer"},
		"workdir":    str("working directory; default is inside the bot's state dir"),
	})
}

const approvalNote = " Nothing changes until a person approves it; the outcome arrives as mail from approval #N. Keep working meanwhile."

func (b *Bot) propose(action string, def config.Bot) (string, error) {
	if b.d.Proposer == nil {
		return "", fmt.Errorf("bot changes are not available")
	}
	id, err := b.d.Proposer.ProposeBot(b.name, action, def)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("approval #%d requested: %s %s.%s", id, action, def.Name, approvalNote), nil
}

type createBot struct{ b *Bot }

func (createBot) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: ActionCreate,
		Description: "Ask to create a new bot. Its capabilities decide its tools: files read|write, shell (bash), " +
			"tasks (start_task), message/manage (lists of bot names or *), create_bots." + approvalNote,
		Parameters: botSchema([]string{"name", "model", "prompt"})}
}

func (t createBot) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var s botSpec
	if err := decode(raw, &s, "name", "model", "prompt"); err != nil {
		return "", err
	}
	return t.b.propose(ActionCreate, s.bot())
}

type updateBot struct{ b *Bot }

func (updateBot) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: ActionUpdate,
		Description: "Ask to change a bot made in the UI or by a bot. Give its name and only the fields to change; " +
			"lists and the prompt are replaced whole. Use show_bot first." + approvalNote,
		Parameters: botSchema([]string{"name"})}
}

func (t updateBot) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var named struct{ Name string }
	if err := decode(raw, &named, "name"); err != nil {
		return "", err
	}
	cur, ok := t.b.lookupAny(named.Name)
	if !ok {
		return "", fmt.Errorf("there is no bot named %q", named.Name)
	}
	s := specOf(cur.d.Bot)
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("bad arguments: %v", err)
	}
	return t.b.propose(ActionUpdate, s.bot())
}

type deleteBot struct{ b *Bot }

func (deleteBot) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: ActionDelete, Description: "Ask to delete a bot made in the UI or by a bot." + approvalNote,
		Parameters: objectSchema([]string{"name"}, map[string]any{"name": str("bot name")})}
}

func (t deleteBot) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var a struct{ Name string }
	if err := decode(raw, &a, "name"); err != nil {
		return "", err
	}
	return t.b.propose(ActionDelete, config.Bot{Name: a.Name})
}

type showBot struct{ b *Bot }

func (showBot) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "show_bot", Description: "Show a bot's full definition: model, prompt, capabilities, spend.",
		Parameters: objectSchema([]string{"name"}, map[string]any{"name": str("bot name")})}
}

func (t showBot) Run(_ context.Context, _ string, raw json.RawMessage) (string, error) {
	var a struct{ Name string }
	if err := decode(raw, &a, "name"); err != nil {
		return "", err
	}
	p, ok := t.b.lookupAny(a.Name)
	if !ok {
		return "", fmt.Errorf("there is no bot named %q", a.Name)
	}
	s := specOf(p.d.Bot)
	if s.Prompt == "" && p.d.Bot.Prompt != "" {
		s.Prompt = "(read from " + p.d.Bot.Prompt + "; defined in ughbot.toml, so it cannot be changed by a bot)"
	}
	out, _ := json.MarshalIndent(s, "", "  ")
	return string(out), nil
}

// lookupAny finds any bot, itself included; bot management is not limited to the message or manage lists.
func (b *Bot) lookupAny(name string) (*Bot, bool) {
	if name == b.name {
		return b, true
	}
	if b.d.Directory == nil {
		return nil, false
	}
	return b.d.Directory.Lookup(name)
}

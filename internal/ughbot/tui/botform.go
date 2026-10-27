package tui

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

// BotBackend is the bot management the Bots screen needs; *client.Client satisfies it.
type BotBackend interface {
	BotDef(ctx context.Context, name string) (api.BotDef, error)
	CreateBot(ctx context.Context, def api.BotDef) error
	UpdateBot(ctx context.Context, def api.BotDef) error
	DeleteBot(ctx context.Context, name string) error
}

type botDefMsg struct {
	def api.BotDef
	err error
}

type botSavedMsg struct {
	name    string
	deleted bool
	err     error
}

const (
	fName = iota
	fModel
	fPrompt
	fFiles
	fShell
	fTasks
	fMessage
	fManage
	fCreate
	fSchedule
	fCode
	fCompact
	fDaily
	fMonthly
	fWorkdir
	fSave
)

var filesChoices = []string{"none", config.FilesRead, config.FilesWrite}

type botForm struct {
	formCore
	editing bool
	preset  string
}

// newPresetForm opens a new-bot form filled from a preset; the user still names it and picks the model.
func newPresetForm(name string, def api.BotDef, w, h int) *botForm {
	f := newBotForm(nil, w, h)
	f.preset = name
	f.load(def)
	f.focusOn(fName)
	return f
}

func newBotForm(def *api.BotDef, w, h int) *botForm {
	area := textarea.New()
	area.ShowLineNumbers = false
	area.Prompt = ""
	area.Placeholder = "system prompt"
	browserSafeTextarea(&area)
	area.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("enter", "alt+enter", "ctrl+j"))
	f := &botForm{editing: def != nil, formCore: formCore{fixed: -1, fields: []formField{
		fName:     newTextField("name", "lowercase letters, digits, - _"),
		fModel:    newTextField("model", "openrouter/<provider>/<model>"),
		fPrompt:   {label: "prompt", kind: fieldPrompt, area: area},
		fFiles:    {label: "files", kind: fieldChoice, choices: filesChoices},
		fShell:    {label: "shell", kind: fieldFlag},
		fTasks:    newTextField("tasks", "task sessions at once, 0 = none"),
		fMessage:  newTextField("may message", "bot names or *"),
		fManage:   newTextField("may manage", "bot names or *"),
		fCreate:   {label: "create bots", kind: fieldFlag},
		fSchedule: {label: "schedule", kind: fieldFlag},
		fCode:     newTextField("code dirs", "directories for code tasks (VM)"),
		fCompact:  newTextField("compact_at", "tokens, 0 = model default"),
		fDaily:    newTextField("daily $", "0 = no cap"),
		fMonthly:  newTextField("monthly $", "0 = no cap"),
		fWorkdir:  newTextField("workdir", "default: inside the bot's state dir"),
		fSave:     {label: "Save", kind: fieldSave},
	}}}
	if f.editing {
		f.fixed = fName
	}
	if def != nil {
		f.load(*def)
	}
	f.resize(w, h)
	f.focusOn(fName)
	if f.editing {
		f.focusOn(fModel)
	}
	return f
}

func joinList(l []string) string { return strings.Join(l, ", ") }

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func numText(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func (f *botForm) load(d api.BotDef) {
	f.fields[fName].in.SetValue(d.Name)
	f.fields[fModel].in.SetValue(d.Model)
	f.fields[fPrompt].area.SetValue(d.PromptText)
	c := d.Capabilities
	f.fields[fFiles].choice = max(slices.Index(filesChoices, c.Files), 0)
	f.fields[fShell].on = c.Shell
	if c.Tasks != 0 {
		f.fields[fTasks].in.SetValue(strconv.Itoa(c.Tasks))
	}
	f.fields[fMessage].in.SetValue(joinList(c.Message))
	f.fields[fManage].in.SetValue(joinList(c.Manage))
	f.fields[fCreate].on = c.CreateBots
	f.fields[fSchedule].on = c.Schedule
	f.fields[fCode].in.SetValue(joinList(c.Code))
	if d.CompactAt != 0 {
		f.fields[fCompact].in.SetValue(strconv.Itoa(d.CompactAt))
	}
	f.fields[fDaily].in.SetValue(numText(d.Spend.Daily))
	f.fields[fMonthly].in.SetValue(numText(d.Spend.Monthly))
	f.fields[fWorkdir].in.SetValue(d.Workdir)
}

// def reads the fields back; the error names the first number that does not parse.
func (f *botForm) def() (api.BotDef, string) {
	text := func(i int) string { return strings.TrimSpace(f.fields[i].in.Value()) }
	var d api.BotDef
	d.Name, d.Model, d.Workdir = text(fName), text(fModel), text(fWorkdir)
	d.PromptText = strings.TrimSpace(f.fields[fPrompt].area.Value())
	switch {
	case d.Name == "":
		return d, "name is required"
	case d.Model == "":
		return d, "model is required (an ugh ref such as openrouter/<provider>/<model>)"
	}
	c := &d.Capabilities
	if files := filesChoices[f.fields[fFiles].choice]; files != "none" {
		c.Files = files
	}
	c.Shell, c.CreateBots, c.Schedule = f.fields[fShell].on, f.fields[fCreate].on, f.fields[fSchedule].on
	c.Message, c.Manage, c.Code = splitList(text(fMessage)), splitList(text(fManage)), splitList(text(fCode))
	for _, n := range []struct {
		i   int
		dst *int
	}{{fTasks, &c.Tasks}, {fCompact, &d.CompactAt}} {
		if s := text(n.i); s != "" {
			v, err := strconv.Atoi(s)
			if err != nil {
				return d, f.fields[n.i].label + " must be a whole number"
			}
			*n.dst = v
		}
	}
	for _, c := range []struct {
		i   int
		dst *float64
	}{{fDaily, &d.Spend.Daily}, {fMonthly, &d.Spend.Monthly}} {
		if s := text(c.i); s != "" {
			v, err := strconv.ParseFloat(strings.TrimPrefix(s, "$"), 64)
			if err != nil {
				return d, f.fields[c.i].label + " must be a number"
			}
			*c.dst = v
		}
	}
	return d, ""
}

func (f *botForm) hint() string {
	return "tab/shift+tab field  ctrl+s save  esc cancel  enter = newline in the prompt"
}

func (f *botForm) view() string {
	title := "New bot"
	if f.preset != "" {
		title = "New bot from the " + f.preset + " preset: pick a name and model, review, ctrl+s saves"
	}
	if f.editing {
		title = "Edit bot " + f.fields[fName].in.Value()
	}
	return f.render(title)
}

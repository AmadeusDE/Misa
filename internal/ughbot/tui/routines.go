package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/sched"
)

// RoutineBackend is the routine store the Routines screen needs; *client.Client satisfies it.
type RoutineBackend interface {
	Routines(ctx context.Context, bot string) ([]api.RoutineInfo, error)
	AddRoutine(ctx context.Context, req api.RoutineRequest) (api.RoutineInfo, error)
	PauseRoutine(ctx context.Context, id int64, paused bool) (api.RoutineInfo, error)
	DeleteRoutine(ctx context.Context, id int64) error
}

type routinesMsg struct {
	list []api.RoutineInfo
	err  error
}

type routineDoneMsg struct {
	note string
	err  error
}

type routinesModel struct {
	list    []api.RoutineInfo
	cursor  int
	open    bool
	confirm bool // delete waits for y
	note    string
	err     error
	loaded  bool
	form    *routineForm
}

func (m *Model) routineBackend() (RoutineBackend, bool) {
	rb, ok := m.be.(RoutineBackend)
	return rb, ok
}

func (m *Model) fetchRoutines() tea.Cmd {
	rb, ok := m.routineBackend()
	if !ok {
		return nil
	}
	ctx := m.ctx
	return func() tea.Msg {
		list, err := rb.Routines(ctx, "")
		return routinesMsg{list, err}
	}
}

func (r *routinesModel) load(msg routinesMsg) {
	r.loaded, r.err = true, msg.err
	if msg.err == nil {
		r.list = msg.list
	}
	r.cursor = min(max(r.cursor, 0), max(len(r.list)-1, 0))
}

func (r *routinesModel) selected() (api.RoutineInfo, bool) {
	if len(r.list) == 0 {
		return api.RoutineInfo{}, false
	}
	return r.list[r.cursor], true
}

// routineAction runs one call against the routine store and reports it on the screen.
func (m *Model) routineAction(do func(context.Context, RoutineBackend) (string, error)) tea.Cmd {
	rb, _ := m.routineBackend()
	ctx := m.ctx
	return func() tea.Msg {
		note, err := do(ctx, rb)
		return routineDoneMsg{note, err}
	}
}

func (m *Model) routinesKey(msg tea.KeyPressMsg) tea.Cmd {
	r := &m.routines
	if r.confirm {
		r.confirm = false
		sel, ok := r.selected()
		if s := msg.String(); !ok || s != "y" && s != "Y" {
			return nil
		}
		return m.routineAction(func(ctx context.Context, rb RoutineBackend) (string, error) {
			return fmt.Sprintf("routine #%d deleted", sel.ID), rb.DeleteRoutine(ctx, sel.ID)
		})
	}
	switch msg.String() {
	case "up", "k":
		r.cursor = max(r.cursor-1, 0)
	case "down", "j":
		r.cursor = min(r.cursor+1, max(len(r.list)-1, 0))
	case "enter":
		r.open = !r.open
	case "esc":
		r.open = false
	case "n":
		bot := ""
		if names := m.botNames(); len(names) > 0 {
			bot = names[min(m.picker.cursor, len(names)-1)]
		}
		r.form, r.note = newRoutineForm(bot, m.w, m.bodyHeight()), ""
	case "x":
		if _, ok := r.selected(); ok {
			r.confirm, r.note = true, ""
		}
	case "space":
		sel, ok := r.selected()
		if !ok || sel.State == "done" {
			return nil
		}
		pause := sel.State == "active"
		return m.routineAction(func(ctx context.Context, rb RoutineBackend) (string, error) {
			info, err := rb.PauseRoutine(ctx, sel.ID, pause)
			return fmt.Sprintf("routine #%d %s", info.ID, info.State), err
		})
	case "r":
		return m.fetchRoutines()
	}
	return nil
}

func (m *Model) routineDone(msg routineDoneMsg) tea.Cmd {
	r := &m.routines
	if msg.err != nil {
		if r.form != nil {
			r.form.saving, r.form.err = false, msg.err.Error()
			return nil
		}
		r.note = errStyle.Render(msg.err.Error())
		return m.fetchRoutines()
	}
	r.form, r.note = nil, goodStyle.Render(msg.note)
	return m.fetchRoutines()
}

func (r *routinesModel) hint() string {
	switch {
	case r.form != nil:
		return "tab/shift+tab field  space/left/right change a choice  ctrl+s save  esc cancel"
	case r.confirm:
		return "delete this routine? y/N"
	}
	return "n new  space pause/resume  x delete  enter details  r refresh"
}

func when(r api.RoutineInfo) string {
	w := r.Cron
	if w == "" {
		w = "at " + r.At.Local().Format("2006-01-02 15:04")
	}
	if r.TZ != "" {
		w += " " + r.TZ
	}
	return w
}

func (r *routinesModel) view(w, h int) string {
	if r.form != nil {
		return r.form.view()
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Routines") + "\n\n")
	if r.err != nil {
		b.WriteString(errStyle.Render("cannot list routines: "+r.err.Error()) + "\n")
	}
	switch {
	case !r.loaded:
		b.WriteString(dimStyle.Render("loading..."))
	case len(r.list) == 0 && r.err == nil:
		b.WriteString(dimStyle.Render("no routines yet: press n to mail a bot on a schedule (cron) or once (at)"))
	}
	var detail []string
	if sel, ok := r.selected(); ok && r.open {
		detail = routineDetail(sel, w)
	}
	rows := max(h-3-len(detail), 1)
	start := min(max(r.cursor-rows+1, 0), max(len(r.list)-rows, 0))
	for i := start; i < len(r.list) && i < start+rows; i++ {
		x := r.list[i]
		next := "-"
		if x.State == "active" && !x.Next.IsZero() {
			next = x.Next.Local().Format("Mon 01-02 15:04")
		}
		head := fmt.Sprintf("#%-4d %-12s ", x.ID, clip(x.Bot, 12))
		state := stateStyle(routineColor(x.State)).Render(fmt.Sprintf("%-7s", x.State))
		rest := fmt.Sprintf(" %-15s %s  %s", next, cut(when(x), 18), x.Prompt)
		rest = cut(rest, max(w-len(head)-11, 10))
		if i == r.cursor {
			b.WriteString(selectedStyle.Render("> "+head) + state + rest + "\n")
			if r.open {
				b.WriteString(strings.Join(detail, "\n") + "\n")
			}
		} else {
			b.WriteString("  " + head + state + dimStyle.Render(rest) + "\n")
		}
	}
	if r.note != "" {
		b.WriteString("\n" + r.note)
	}
	return strings.TrimRight(b.String(), "\n")
}

func routineColor(state string) string {
	switch state {
	case "active":
		return "ok"
	case "done":
		return "idle"
	}
	return state
}

func routineDetail(x api.RoutineInfo, w int) []string {
	var lines []string
	add := func(k, v string) { lines = append(lines, "    "+dimStyle.Render(fmt.Sprintf("%-9s", k))+v) }
	add("when", when(x))
	add("missed", x.Missed+dimStyle.Render("  (if the daemon was down at fire time)"))
	add("made by", x.Owner+dimStyle.Render(", "+x.Created.Local().Format("2006-01-02 15:04")))
	last := "never"
	if !x.Last.IsZero() {
		last = x.Last.Local().Format("2006-01-02 15:04")
	}
	add("fired", fmt.Sprintf("%d times, last %s", x.Fired, last))
	lines = append(lines, "    "+dimStyle.Render("prompt:"))
	for _, l := range strings.Split(wrap(x.Prompt, max(w-8, 10)), "\n") {
		lines = append(lines, "      "+l)
	}
	return lines
}

// wrap breaks text into lines at most w columns wide.
func wrap(text string, w int) string { return lipgloss.NewStyle().Width(w).Render(text) }

const (
	rBot = iota
	rKind
	rWhen
	rTZ
	rMissed
	rPrompt
	rSave
)

var (
	kindChoices   = []string{"cron", "at"}
	missedChoices = []string{"once", "skip"}
)

type routineForm struct{ formCore }

func newRoutineForm(bot string, w, h int) *routineForm {
	area := textarea.New()
	area.ShowLineNumbers = false
	area.Prompt = ""
	area.Placeholder = "what the bot is told each time; make it self-contained"
	browserSafeTextarea(&area)
	area.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("enter", "alt+enter", "ctrl+j"))
	f := &routineForm{formCore{fixed: -1, fields: []formField{
		rBot:    newTextField("bot", "who gets the mail"),
		rKind:   {label: "repeat", kind: fieldChoice, choices: kindChoices},
		rWhen:   newTextField("when", `cron "0 9 * * 1", @daily; or at "in 2h", "2026-10-20 09:00"`),
		rTZ:     newTextField("timezone", "IANA zone, default the daemon's"),
		rMissed: {label: "if missed", kind: fieldChoice, choices: missedChoices},
		rPrompt: {label: "prompt", kind: fieldPrompt, area: area},
		rSave:   {label: "Save", kind: fieldSave},
	}}}
	f.fields[rBot].in.SetValue(bot)
	f.resize(w, h)
	f.focusOn(rWhen)
	if bot == "" {
		f.focusOn(rBot)
	}
	return f
}

func (f *routineForm) request() api.RoutineRequest {
	text := func(i int) string { return strings.TrimSpace(f.fields[i].in.Value()) }
	req := api.RoutineRequest{Bot: text(rBot), TZ: text(rTZ), Prompt: strings.TrimSpace(f.fields[rPrompt].area.Value()),
		Missed: missedChoices[f.fields[rMissed].choice], Owner: decidedBy()}
	if kindChoices[f.fields[rKind].choice] == "cron" {
		req.Cron = text(rWhen)
	} else {
		req.At = text(rWhen)
	}
	return req
}

// preview says when the routine would first fire, or what is wrong with the time.
func (f *routineForm) preview() string {
	req := f.request()
	spec := sched.Spec{Cron: req.Cron, TZ: req.TZ}
	if req.Cron == "" && req.At == "" {
		return dimStyle.Render("enter a time")
	}
	if req.At != "" {
		loc, err := spec.Location()
		if err == nil {
			spec.At, err = sched.ParseAt(req.At, time.Now(), loc)
		}
		if err != nil {
			return errStyle.Render(err.Error())
		}
	}
	if err := spec.Check(); err != nil {
		return errStyle.Render(err.Error())
	}
	return goodStyle.Render("first fires " + spec.After(time.Now()).Local().Format("Mon 2006-01-02 15:04 MST"))
}

func (f *routineForm) view() string {
	return f.render("New routine  " + f.preview())
}

func (m *Model) routineFormKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "ctrl+c" {
		return m, m.quit()
	}
	f := m.routines.form
	submit, cancel := f.update(msg)
	switch {
	case cancel:
		m.routines.form = nil
	case submit:
		req := f.request()
		if req.Bot == "" || req.Prompt == "" {
			f.err = "bot and prompt are required"
			return m, nil
		}
		f.err, f.saving = "", true
		return m, m.routineAction(func(ctx context.Context, rb RoutineBackend) (string, error) {
			info, err := rb.AddRoutine(ctx, req)
			return fmt.Sprintf("routine #%d added; first fires %s", info.ID, info.Next.Local().Format("Mon 2006-01-02 15:04")), err
		})
	}
	return m, nil
}

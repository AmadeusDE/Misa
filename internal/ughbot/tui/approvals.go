package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

// ApprovalBackend is the approval queue the Approvals screen needs; *client.Client satisfies it.
type ApprovalBackend interface {
	Approvals(ctx context.Context, state string) ([]api.ApprovalInfo, error)
	Approve(ctx context.Context, id int64, by string) (api.ApprovalInfo, error)
	Deny(ctx context.Context, id int64, by string) (api.ApprovalInfo, error)
}

const approvalPoll = 5 * time.Second

type approvalsMsg struct {
	all  bool
	list []api.ApprovalInfo
	err  error
}

type decidedMsg struct {
	info api.ApprovalInfo
	err  error
}

type approvalTick struct{}

type approvalsModel struct {
	list    []api.ApprovalInfo
	pending int
	all     bool
	cursor  int
	open    bool
	confirm string // "approve" or "deny" while waiting for y
	note    string
	err     error
	loaded  bool
}

// decidedBy names who decided, for the record: the web device when the TUI runs in the browser.
func decidedBy() string {
	if dev := os.Getenv("WEBUI_DEVICE"); dev != "" {
		return "web:" + dev
	}
	return "tui"
}

func (m *Model) approvalBackend() (ApprovalBackend, bool) {
	ab, ok := m.be.(ApprovalBackend)
	return ab, ok
}

func (m *Model) fetchApprovals() tea.Cmd {
	ab, ok := m.approvalBackend()
	if !ok {
		return nil
	}
	ctx, all := m.ctx, m.approvals.all
	return func() tea.Msg {
		state := "pending"
		if all {
			state = ""
		}
		list, err := ab.Approvals(ctx, state)
		return approvalsMsg{all, list, err}
	}
}

func (m *Model) pollApprovals() tea.Cmd {
	if _, ok := m.approvalBackend(); !ok {
		return nil
	}
	return tea.Tick(approvalPoll, func(time.Time) tea.Msg { return approvalTick{} })
}

func (a *approvalsModel) load(msg approvalsMsg) {
	a.loaded, a.err = true, msg.err
	if msg.err != nil || msg.all != a.all {
		return
	}
	a.list = msg.list
	a.pending = 0
	for _, x := range msg.list {
		if x.State == "pending" {
			a.pending++
		}
	}
	a.cursor = min(max(a.cursor, 0), max(len(a.list)-1, 0))
}

func (a *approvalsModel) selected() (api.ApprovalInfo, bool) {
	if len(a.list) == 0 {
		return api.ApprovalInfo{}, false
	}
	return a.list[a.cursor], true
}

func (m *Model) approvalsKey(msg tea.KeyPressMsg) tea.Cmd {
	a := &m.approvals
	if a.confirm != "" {
		verb := a.confirm
		a.confirm = ""
		sel, ok := a.selected()
		if s := msg.String(); !ok || s != "y" && s != "Y" {
			return nil
		}
		ab, _ := m.approvalBackend()
		ctx, by := m.ctx, decidedBy()
		return func() tea.Msg {
			decide := ab.Deny
			if verb == "approve" {
				decide = ab.Approve
			}
			info, err := decide(ctx, sel.ID, by)
			return decidedMsg{info, err}
		}
	}
	switch msg.String() {
	case "up", "k":
		a.cursor = max(a.cursor-1, 0)
	case "down", "j":
		a.cursor = min(a.cursor+1, max(len(a.list)-1, 0))
	case "enter", "space":
		a.open = !a.open
	case "esc":
		a.open = false
	case "a", "d":
		if sel, ok := a.selected(); ok && sel.State == "pending" {
			a.confirm = map[string]string{"a": "approve", "d": "deny"}[msg.String()]
			a.note = ""
		}
	case "A":
		a.all, a.cursor, a.open = !a.all, 0, false
		return m.fetchApprovals()
	case "r":
		return m.fetchApprovals()
	}
	return nil
}

func (m *Model) decided(msg decidedMsg) tea.Cmd {
	a := &m.approvals
	if msg.err != nil {
		a.note = errStyle.Render(msg.err.Error())
	} else {
		a.note = fmt.Sprintf("#%d %s", msg.info.ID, msg.info.State)
		if msg.info.Result != "" {
			a.note += ": " + msg.info.Result
		}
		a.note = stateStyle(msg.info.State).Render(a.note)
	}
	return tea.Batch(m.fetchApprovals(), m.fetchBots())
}

func (a *approvalsModel) hint() string {
	if a.confirm != "" {
		return a.confirm + " this request? y/N"
	}
	which := "A all"
	if a.all {
		which = "A pending only"
	}
	return "enter details  a approve  d deny  " + which + "  r refresh"
}

func (a *approvalsModel) view(w, h int) string {
	var b strings.Builder
	title := "Approvals"
	if a.all {
		title += dimStyle.Render("  (all)")
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")
	if a.err != nil {
		b.WriteString(errStyle.Render("cannot list approvals: "+a.err.Error()) + "\n")
	}
	switch {
	case !a.loaded:
		b.WriteString(dimStyle.Render("loading..."))
	case len(a.list) == 0 && a.err == nil:
		b.WriteString(dimStyle.Render("nothing waits on you; bots that ask to change bots show up here"))
	}
	var detail []string
	if sel, ok := a.selected(); ok && a.open {
		detail = approvalDetail(sel, w)
	}
	rows := max(h-3-len(detail), 1)
	start := min(max(a.cursor-rows+1, 0), max(len(a.list)-rows, 0))
	for i := start; i < len(a.list) && i < start+rows; i++ {
		x := a.list[i]
		head := fmt.Sprintf("#%-4d %-12s ", x.ID, clip(x.Bot, 12))
		state := stateStyle(approvalColor(x.State)).Render(fmt.Sprintf("%-9s", x.State))
		line := cut(x.Summary, max(w-len(head)-14, 10))
		if i == a.cursor {
			b.WriteString(selectedStyle.Render("> "+head) + state + " " + line + "\n")
			if a.open {
				b.WriteString(strings.Join(detail, "\n") + "\n")
			}
		} else {
			b.WriteString("  " + head + state + " " + dimStyle.Render(line) + "\n")
		}
	}
	if a.note != "" {
		b.WriteString("\n" + a.note)
	}
	return strings.TrimRight(b.String(), "\n")
}

func approvalColor(state string) string {
	switch state {
	case "approved":
		return "ok"
	case "denied", "expired":
		return "failed"
	}
	return state
}

func approvalDetail(x api.ApprovalInfo, w int) []string {
	var lines []string
	add := func(k, v string) { lines = append(lines, "    "+dimStyle.Render(fmt.Sprintf("%-11s", k))+v) }
	add("asked", x.Created.Local().Format("2006-01-02 15:04"))
	if x.State == "pending" {
		add("expires", x.Expires.Local().Format("2006-01-02 15:04")+dimStyle.Render("  (then it is denied)"))
	} else if x.DecidedBy != "" {
		add("decided by", x.DecidedBy)
	}
	if x.Result != "" {
		add("result", x.Result)
	}
	add("summary", x.Summary)
	var fields map[string]any
	if json.Unmarshal(x.Args, &fields) != nil {
		return lines
	}
	prompt, _ := fields["prompt_text"].(string)
	delete(fields, "prompt_text")
	raw, _ := json.Marshal(fields)
	var pretty bytes.Buffer
	_ = json.Indent(&pretty, raw, "", "  ")
	lines = append(lines, "    "+dimStyle.Render(x.Action+":"))
	for _, l := range strings.Split(pretty.String(), "\n") {
		lines = append(lines, "      "+cut(l, max(w-8, 10)))
	}
	if prompt != "" {
		lines = append(lines, "    "+dimStyle.Render("prompt:"))
		for _, l := range strings.Split(wrap(prompt, max(w-8, 10)), "\n") {
			lines = append(lines, "      "+l)
		}
	}
	return lines
}

// cut shortens s to n runes, marking the cut, and keeps its spacing (clip collapses it).
func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:max(n-1, 0)]) + "…"
}

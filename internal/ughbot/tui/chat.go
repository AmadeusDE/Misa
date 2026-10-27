package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

var reconnectDelay = time.Second

const inputRows = 3

type connState int

const (
	connConnecting connState = iota
	connUp
	connDown
)

type (
	streamMsg struct {
		id     int
		events <-chan api.Event
	}
	eventMsg struct {
		id int
		ev api.Event
	}
	closedMsg struct{ id int }
	failedMsg struct {
		id  int
		err error
	}
	retryMsg   struct{ id int }
	sendErrMsg struct {
		id  int
		err error
	}
)

type chatModel struct {
	id       int
	bot      string
	be       Backend
	ctx      context.Context
	cancel   context.CancelFunc
	events   <-chan api.Event
	lastSeq  uint64
	conn     connState
	attempts int
	lastErr  string
	sendErr  string
	body     strings.Builder
	midLine  bool
	vp       viewport.Model
	input    textarea.Model
}

func newChat(ctx context.Context, be Backend, id int, bot string, w, h int) *chatModel {
	ctx, cancel := context.WithCancel(ctx)
	in := textarea.New()
	in.Prompt = "> "
	in.ShowLineNumbers = false
	in.Placeholder = "message " + bot
	browserSafeTextarea(&in)
	in.Focus()
	vp := viewport.New()
	vp.SoftWrap = true
	c := &chatModel{id: id, bot: bot, be: be, ctx: ctx, cancel: cancel, vp: vp, input: in}
	c.resize(w, h)
	return c
}

// browserSafeTextarea drops the editing chords a browser keeps for itself (ctrl+w, ctrl+n, ctrl+t...).
func browserSafeTextarea(in *textarea.Model) {
	in.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"))
	in.KeyMap.LineNext = key.NewBinding(key.WithKeys("down"))
	in.KeyMap.LinePrevious = key.NewBinding(key.WithKeys("up"))
	in.KeyMap.WordBackward = key.NewBinding(key.WithKeys("ctrl+left", "alt+b"))
	in.KeyMap.WordForward = key.NewBinding(key.WithKeys("ctrl+right", "alt+f"))
	in.KeyMap.TransposeCharacterBackward = key.NewBinding()
	in.KeyMap.DeleteWordBackward = key.NewBinding(key.WithKeys("alt+backspace"))
}

func (c *chatModel) resize(w, h int) {
	c.vp.SetWidth(max(w, 1))
	c.vp.SetHeight(max(h-inputRows-1, 1))
	c.input.SetWidth(max(w, 4))
	c.input.SetHeight(inputRows)
	c.refresh()
}

func (c *chatModel) init() tea.Cmd { return tea.Batch(c.subscribe(), textarea.Blink) }

func (c *chatModel) subscribe() tea.Cmd {
	id, after := c.id, c.lastSeq
	return func() tea.Msg {
		ch, err := c.be.Subscribe(c.ctx, c.bot, after)
		if err != nil {
			return failedMsg{id, err}
		}
		return streamMsg{id, ch}
	}
}

func (c *chatModel) wait() tea.Cmd {
	id, ch := c.id, c.events
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return closedMsg{id}
		}
		return eventMsg{id, ev}
	}
}

func (c *chatModel) retryLater() tea.Cmd {
	c.conn = connDown
	c.attempts++
	id := c.id
	return tea.Tick(reconnectDelay, func(time.Time) tea.Msg { return retryMsg{id} })
}

func msgID(msg tea.Msg) (int, bool) {
	switch m := msg.(type) {
	case streamMsg:
		return m.id, true
	case eventMsg:
		return m.id, true
	case closedMsg:
		return m.id, true
	case failedMsg:
		return m.id, true
	case retryMsg:
		return m.id, true
	case sendErrMsg:
		return m.id, true
	}
	return 0, false
}

func (c *chatModel) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case streamMsg:
		c.events, c.conn, c.attempts, c.lastErr = msg.events, connUp, 0, ""
		return c.wait()
	case eventMsg:
		c.lastSeq = msg.ev.Seq
		c.show(msg.ev)
		return c.wait()
	case closedMsg:
		return c.retryLater()
	case failedMsg:
		c.lastErr = msg.err.Error()
		return c.retryLater()
	case retryMsg:
		c.conn = connConnecting
		return c.subscribe()
	case sendErrMsg:
		c.sendErr = msg.err.Error()
	}
	return nil
}

func (c *chatModel) key(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "pgup", "pgdown":
		var cmd tea.Cmd
		c.vp, cmd = c.vp.Update(msg)
		return cmd
	case "enter":
		text := strings.TrimSpace(c.input.Value())
		if text == "" {
			return nil
		}
		c.input.Reset()
		c.sendErr = ""
		id := c.id
		return func() tea.Msg {
			if _, err := c.be.Send(c.ctx, c.bot, text); err != nil {
				return sendErrMsg{id, err}
			}
			return nil
		}
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

func (c *chatModel) passthrough(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

func (c *chatModel) line(s string) {
	if c.midLine {
		c.body.WriteByte('\n')
		c.midLine = false
	}
	c.body.WriteString(s)
	c.body.WriteByte('\n')
}

func field(ev api.Event, key string) string {
	var d map[string]any
	if json.Unmarshal(ev.Data, &d) != nil {
		return ""
	}
	if s, ok := d[key].(string); ok {
		return s
	}
	return ""
}

func (c *chatModel) show(ev api.Event) {
	switch ev.Type {
	case "text":
		c.body.WriteString(field(ev, "text"))
		c.midLine = true
	case "turn_end":
		if c.midLine {
			c.body.WriteByte('\n')
			c.midLine = false
		}
		c.body.WriteByte('\n')
	case "user":
		c.line(userStyle.Render("> " + field(ev, "text")))
	case "mail":
		c.line(dimStyle.Render(fmt.Sprintf("[mail from %s] %s", field(ev, "from"), field(ev, "text"))))
	case "tool_call":
		c.line(dimStyle.Render(fmt.Sprintf("* %s %s", field(ev, "name"), clip(field(ev, "args"), 100))))
	case "tool_result":
		c.line(dimStyle.Render("  -> " + clip(field(ev, "output"), 100)))
	case "budget":
		c.line(warnStyle.Render("! " + field(ev, "message")))
	case "error":
		c.line(errStyle.Render("! error: " + field(ev, "message")))
	case "task":
		c.line(dimStyle.Render(fmt.Sprintf("* task %s %s", field(ev, "id"), field(ev, "state"))))
	case "note":
		c.line(dimStyle.Render("  " + field(ev, "text")))
	}
	c.refresh()
}

func (c *chatModel) refresh() {
	stick := c.vp.AtBottom()
	content := strings.TrimRight(c.body.String(), "\n")
	if content == "" {
		content = dimStyle.Render("No messages yet. Type below and press enter. Mail from other bots, task results " +
			"and approval outcomes show up here too.")
	}
	c.vp.SetContent(content)
	if stick {
		c.vp.GotoBottom()
	}
}

func (c *chatModel) statusText() string {
	switch c.conn {
	case connUp:
		s := goodStyle.Render("● attached to " + c.bot)
		if c.sendErr != "" {
			s += "  " + errStyle.Render("send failed: "+c.sendErr)
		}
		return s
	case connDown:
		s := fmt.Sprintf("✕ disconnected, retrying (attempt %d)", c.attempts)
		if c.lastErr != "" {
			s += ": " + clip(c.lastErr, 60)
		}
		return errStyle.Render(s)
	}
	return warnStyle.Render("◌ connecting to " + c.bot)
}

func (c *chatModel) view() string {
	return c.vp.View() + "\n" + c.statusText() + "\n" + c.input.View()
}

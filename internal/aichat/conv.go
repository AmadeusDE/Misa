package aichat

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

const errStopped = "stopped"

type renderTickMsg struct{ gen int }

func (m *Chat) last() *Message {
	if len(m.conv.Messages) == 0 {
		return nil
	}
	return &m.conv.Messages[len(m.conv.Messages)-1]
}

func (m *Chat) say(text string, isErr bool) { m.notice, m.noticeErr = text, isErr }

// modelRef is the model this conversation uses: its own, else the chat model of ai.yaml or ugh.
func (m *Chat) modelRef() (string, error) {
	if m.conv.Model != "" {
		return m.conv.Model, nil
	}
	return m.opt.ModelFor()
}

func (m *Chat) clientFor(ref string) (*ai.Client, error) {
	if m.client == nil || m.client.Ref != ref {
		c, err := m.opt.Open(ref)
		if err != nil {
			return nil, err
		}
		m.client = c
	}
	return m.client, nil
}

// send adds text as the next user message (replacing the message being edited) and asks the model.
func (m *Chat) send(text string) tea.Cmd {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if m.job != nil {
		m.say("still answering: esc stops it", true)
		return nil
	}
	if m.editing >= 0 {
		m.conv.Messages = m.conv.Messages[:m.editing]
		m.editing = -1
	}
	if m.conv.Title == "" {
		m.conv.Title = TitleFrom(text)
	}
	m.conv.Messages = append(m.conv.Messages, Message{Role: "user", Text: text})
	m.input.Reset()
	m.layout()
	return m.generate()
}

// generate asks the model to answer the conversation as it stands.
func (m *Chat) generate() tea.Cmd {
	m.say("", false)
	m.invalidate()
	m.persist()
	ref, err := m.modelRef()
	if err != nil {
		m.say(err.Error(), true)
		return nil
	}
	c, err := m.clientFor(ref)
	if err != nil {
		m.say(err.Error(), true)
		return nil
	}
	req := llm.Request{System: m.conv.System, Messages: m.history(), Foreground: true}
	m.conv.Messages = append(m.conv.Messages, Message{Role: "assistant", Model: ref})
	m.gen++
	m.job = startJob(m.gen, c, req, m.searchTools())
	m.needBreak, m.scroll, m.streamLines = false, 0, nil
	m.renderStreamNow()
	return m.job.wait()
}

// searchTools are the web tools when the conversation has search on and ai.yaml allows it.
func (m *Chat) searchTools() []ai.Tool {
	if !m.conv.Search {
		return nil
	}
	tools, err := m.opt.Tools()
	switch {
	case err != nil:
		m.say("web search: "+err.Error(), true)
	case len(tools) == 0:
		m.say("web search is off in ai.yaml", true)
	}
	return tools
}

// history is what the model sees: the user turns and the answers that exist.
func (m *Chat) history() []llm.Message {
	var out []llm.Message
	for _, msg := range m.conv.Messages {
		switch {
		case msg.Role == "user":
			out = append(out, llm.Message{Role: llm.RoleUser, Text: msg.Text})
		case msg.Text != "":
			out = append(out, llm.Message{Role: llm.RoleAssistant, Text: msg.Text})
		}
	}
	return out
}

// generated folds a batch of events of the running generation into the answer.
func (m *Chat) generated(msg genMsg) tea.Cmd {
	if m.job == nil || msg.gen != m.gen {
		return nil
	}
	answer := m.last()
	for _, ev := range msg.events {
		switch ev.kind {
		case evText:
			if m.needBreak && answer.Text != "" && ev.text != "" {
				answer.Text += "\n\n"
			}
			m.needBreak = m.needBreak && ev.text == ""
			answer.Text += ev.text
		case evThinking:
			answer.Thinking += ev.text
		case evNote:
			answer.Notes = append(answer.Notes, ev.text)
			m.needBreak = true
		case evDone:
			m.finish(ev)
			return nil
		}
	}
	cmds := []tea.Cmd{m.job.wait()}
	if since := time.Since(m.lastRender); since >= renderEvery {
		m.renderStreamNow()
	} else if !m.renderPending {
		m.renderPending = true
		gen := m.gen
		cmds = append(cmds, tea.Tick(renderEvery-since, func(time.Time) tea.Msg { return renderTickMsg{gen} }))
	}
	return tea.Batch(cmds...)
}

func (m *Chat) finish(done genEvent) {
	answer := m.last()
	m.job = nil
	m.conv.Usage.In += done.usage.In
	m.conv.Usage.Out += done.usage.Out
	m.conv.Usage.Cost += done.usage.Cost
	if done.noTools {
		m.conv.Search = false
	}
	if done.err != nil {
		answer.Error = done.err.Error()
		if errors.Is(done.err, context.Canceled) {
			answer.Error = errStopped
		}
	}
	m.settle()
}

// settle ends a generation: the answer is final, so it is cached and saved.
func (m *Chat) settle() {
	m.streamLines, m.renderPending = nil, false
	m.invalidate()
	m.persist()
}

// stop abandons the running generation, keeping what was written; later events of it are dropped.
func (m *Chat) stop() {
	if m.job == nil {
		return
	}
	m.job.cancel()
	m.job = nil
	m.gen++
	if answer := m.last(); answer.Text == "" && answer.Thinking == "" && len(answer.Notes) == 0 {
		m.conv.Messages = m.conv.Messages[:len(m.conv.Messages)-1]
	} else {
		answer.Error = errStopped
	}
	m.settle()
}

// regenerate asks again for the last answer.
func (m *Chat) regenerate() tea.Cmd {
	if m.job != nil {
		return nil
	}
	if l := m.last(); l != nil && l.Role == "assistant" {
		m.conv.Messages = m.conv.Messages[:len(m.conv.Messages)-1]
	}
	if l := m.last(); l == nil || l.Role != "user" {
		m.say("nothing to regenerate", true)
		return nil
	}
	return m.generate()
}

// editLast puts the last user message back in the input; sending it replaces that turn.
func (m *Chat) editLast() {
	if m.job != nil {
		return
	}
	for i := len(m.conv.Messages) - 1; i >= 0; i-- {
		if m.conv.Messages[i].Role == "user" {
			m.input.SetValue(m.conv.Messages[i].Text)
			m.input.MoveToEnd()
			m.editing = i
			m.layout()
			m.say("editing your last message: enter resends it, esc cancels", false)
			return
		}
	}
}

func (m *Chat) cancelEdit() {
	m.editing = -1
	m.input.Reset()
	m.layout()
	m.say("", false)
}

// persist saves the conversation once it has something to keep.
func (m *Chat) persist() {
	if len(m.conv.Messages) == 0 && !m.saved {
		return
	}
	m.conv.Updated = m.opt.Now()
	if err := m.store.Save(m.conv); err != nil {
		m.say("not saved: "+err.Error(), true)
		return
	}
	m.saved = true
	m.touchMeta()
}

// touchMeta moves the current conversation to the top of the sidebar list.
func (m *Chat) touchMeta() {
	meta := Meta{ID: m.conv.ID, Title: m.conv.Title, Updated: m.conv.Updated}
	m.metas = slices.DeleteFunc(m.metas, func(o Meta) bool { return o.ID == meta.ID })
	m.metas = append([]Meta{meta}, m.metas...)
}

func (m *Chat) invalidate() { m.blocks = nil }

// reset switches to conv with a clean screen.
func (m *Chat) reset(c *Conversation, saved bool) {
	m.stop()
	m.conv, m.saved = c, saved
	m.editing, m.scroll = -1, 0
	m.input.Reset()
	m.mode, m.focus = modeChat, focusInput
	m.say("", false)
	m.layout()
	m.invalidate()
}

// newConversation starts an empty chat that keeps the model and web search choices.
func (m *Chat) newConversation() {
	c := NewConversation(m.opt.Now())
	c.Model, c.Search = m.conv.Model, m.conv.Search
	m.reset(c, false)
}

func (m *Chat) openConversation(id string) {
	if id == m.conv.ID {
		m.focus = focusInput
		return
	}
	c, err := m.store.Load(id)
	if err != nil {
		m.say(err.Error(), true)
		return
	}
	m.reset(c, true)
}

func (m *Chat) deleteConversation(id string) {
	if err := m.store.Delete(id); err != nil {
		m.say(err.Error(), true)
		return
	}
	m.metas = slices.DeleteFunc(m.metas, func(o Meta) bool { return o.ID == id })
	if id == m.conv.ID {
		m.reset(NewConversation(m.opt.Now()), false)
	}
	m.sideCursor = min(m.sideCursor, len(m.metas))
}

// renameByID retitles a saved conversation, the open one or another from the sidebar, keeping its place in the list.
func (m *Chat) renameByID(id, title string) {
	if title = strings.TrimSpace(title); title == "" {
		return
	}
	c := m.conv
	if id != m.conv.ID {
		var err error
		if c, err = m.store.Load(id); err != nil {
			m.say(err.Error(), true)
			return
		}
	}
	c.Title = title
	if err := m.store.Save(c); err != nil {
		m.say("not saved: "+err.Error(), true)
		return
	}
	for i := range m.metas {
		if m.metas[i].ID == id {
			m.metas[i].Title = title
		}
	}
}

func (m *Chat) setModel(ref string) {
	m.conv.Model = ref
	m.say("model: "+ref, false)
	m.persist()
}

func (m *Chat) toggleSearch() {
	if m.conv.Search {
		m.conv.Search = false
		m.say("web search off", false)
		m.persist()
		return
	}
	tools, err := m.opt.Tools()
	switch {
	case err != nil:
		m.say("web search: "+err.Error(), true)
	case len(tools) == 0:
		m.say("web search is off in ai.yaml: set search.engine there or run aiconfig", true)
	default:
		m.conv.Search = true
		m.say("web search on: the model may search the web and read pages", false)
		m.persist()
	}
}

func (m *Chat) copyLast() tea.Cmd {
	for i := len(m.conv.Messages) - 1; i >= 0; i-- {
		if msg := m.conv.Messages[i]; msg.Role == "assistant" && msg.Text != "" {
			m.say("copied the last answer", false)
			return m.opt.Copy(msg.Text)
		}
	}
	m.say("no answer to copy", true)
	return nil
}

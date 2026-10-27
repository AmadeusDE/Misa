package tui

import (
	tea "charm.land/bubbletea/v2"
)

func (m *Model) botBackend() (BotBackend, bool) {
	bb, ok := m.be.(BotBackend)
	if !ok {
		m.note = "this daemon connection cannot manage bots"
	}
	return bb, ok
}

func (m *Model) selectedBot() (name string, editable, ok bool) {
	vis := m.picker.visible()
	if len(vis) == 0 {
		return "", false, false
	}
	b := vis[m.picker.cursor]
	return b.Name, b.Editable, true
}

// botAdminKey handles n (new), e (edit) and x (delete) on the Bots screen.
func (m *Model) botAdminKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if m.picker.filtering {
		return nil, false
	}
	switch msg.String() {
	case "n", "p":
		m.note = ""
		if _, ok := m.botBackend(); !ok {
			return nil, true
		}
		if msg.String() == "p" {
			m.form = newPresetForm("bot manager", botManagerPreset(), m.w, m.bodyHeight())
		} else {
			m.form = newBotForm(nil, m.w, m.bodyHeight())
		}
		return nil, true
	case "e", "x":
		m.note = ""
		name, editable, ok := m.selectedBot()
		if !ok {
			return nil, true
		}
		if !editable {
			m.note = name + " is defined in ughbot.toml; edit that file"
			return nil, true
		}
		bb, ok := m.botBackend()
		if !ok {
			return nil, true
		}
		if msg.String() == "x" {
			m.confirm = name
			return nil, true
		}
		ctx := m.ctx
		return func() tea.Msg {
			def, err := bb.BotDef(ctx, name)
			return botDefMsg{def, err}
		}, true
	}
	return nil, false
}

func (m *Model) confirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := m.confirm
	m.confirm = ""
	if s := msg.String(); s != "y" && s != "Y" {
		return m, nil
	}
	bb, ok := m.botBackend()
	if !ok {
		return m, nil
	}
	ctx := m.ctx
	return m, func() tea.Msg { return botSavedMsg{name: name, deleted: true, err: bb.DeleteBot(ctx, name)} }
}

func (m *Model) formKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, m.quit()
	}
	f := m.form
	submit, cancel := f.update(msg)
	switch {
	case cancel:
		m.form = nil
	case submit:
		def, bad := f.def()
		if bad != "" {
			f.err = bad
			return m, nil
		}
		bb, ok := m.botBackend()
		if !ok {
			f.err = m.note
			return m, nil
		}
		f.err, f.saving = "", true
		ctx, editing := m.ctx, f.editing
		return m, func() tea.Msg {
			var err error
			if editing {
				err = bb.UpdateBot(ctx, def)
			} else {
				err = bb.CreateBot(ctx, def)
			}
			return botSavedMsg{name: def.Name, err: err}
		}
	}
	return m, nil
}

func (m *Model) botSaved(msg botSavedMsg) tea.Cmd {
	if msg.err != nil {
		if m.form != nil && !msg.deleted {
			m.form.saving = false
			m.form.err = msg.err.Error()
		} else {
			m.note = msg.err.Error()
		}
		return nil
	}
	m.form = nil
	verb := "saved"
	if msg.deleted {
		verb = "deleted"
	}
	m.note = "bot " + msg.name + " " + verb
	return m.fetchBots()
}

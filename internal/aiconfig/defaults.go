package aiconfig

import (
	"errors"
	"net/url"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ai"
)

const (
	defaultsHelp = "enter edit/cycle  x clear  esc back"
	editHelp     = "enter save  esc cancel"
	defaultRows  = 5
)

const (
	rowDefault = iota
	rowChat
	rowHelp
	rowEngine
	rowURL
)

var defaultLabels = [defaultRows]string{"Default model", "Chat model", "Help model", "Search engine", "SearXNG URL"}

var defaultHints = [defaultRows]string{
	"ugh's default model (models.default in config.toml), provider/model. Every feature without its own model uses it.",
	"The model AI chat uses, provider/model. Blank: the default model.",
	"The model AI command help uses, provider/model. Blank: the default model.",
	"Web search for chat: duckduckgo (no key), searxng (your own instance) or off.",
	"Base URL of your SearXNG instance, used when the engine is searxng.",
}

var engines = []string{"duckduckgo", "searxng", "off"}

type defaultsView struct {
	list    cursorList
	editing bool
	in      textinput.Model
	// engineAfterURL is the engine to save together with the URL being typed.
	engineAfterURL string
}

func (m *Model) openDefaults() {
	m.defs = defaultsView{}
	m.defs.in = textinput.New()
	m.defs.in.Prompt = ""
	m.defs.list.setN(defaultRows)
	m.mode = modeDefaults
	m.fit()
}

func (m *Model) defaultValue(row int) string {
	s := m.cur.Settings
	switch row {
	case rowDefault:
		return m.cur.Default
	case rowChat:
		return s.Models[ai.FeatureChat]
	case rowHelp:
		return s.Models[ai.FeatureHelp]
	case rowEngine:
		return s.Search.Engine
	}
	return s.Search.SearxngURL
}

func (m *Model) defaultsKey(msg tea.KeyPressMsg) tea.Cmd {
	v := &m.defs
	k := msg.String()
	if v.editing {
		switch k {
		case "enter":
			m.saveDefault()
		case "esc":
			v.editing, v.engineAfterURL = false, ""
			v.in.Blur()
		default:
			return m.forward(msg)
		}
		return nil
	}
	if v.list.key(k) {
		return nil
	}
	switch k {
	case "esc", "q":
		m.mode = modeProviders
	case "enter", "space":
		return m.editDefault()
	case "x", "delete", "backspace":
		m.clearDefault()
	case "?":
		m.helpFrom, m.mode = modeDefaults, modeHelp
	}
	return nil
}

func (m *Model) beginEdit(value string) {
	m.defs.editing = true
	m.defs.in.SetValue(value)
	m.defs.in.Focus()
	m.defs.in.CursorEnd()
}

func (m *Model) editDefault() tea.Cmd {
	row := m.defs.list.cur
	if row == rowEngine {
		m.cycleEngine()
		return nil
	}
	m.beginEdit(m.defaultValue(row))
	return nil
}

func (m *Model) cycleEngine() {
	next := engines[0]
	for i, e := range engines {
		if e == m.cur.Settings.Search.Engine {
			next = engines[(i+1)%len(engines)]
		}
	}
	if next == "searxng" && m.cur.Settings.Search.SearxngURL == "" {
		m.defs.list.cur = rowURL
		m.defs.engineAfterURL = next
		m.beginEdit("")
		m.ok("enter your SearXNG URL to use it for search")
		return
	}
	s := m.cur.Settings
	s.Search.Engine = next
	m.saveSettings(s, "search engine is "+next)
}

func (m *Model) saveSettings(s ai.Settings, what string) {
	if err := ai.SaveSettings(s); err != nil {
		m.fail(err)
		return
	}
	m.reload()
	m.ok("%s", what)
}

func (m *Model) clearDefault() {
	var err error
	switch row := m.defs.list.cur; row {
	case rowDefault:
		err = ai.SetDefaultModel("")
	case rowChat:
		err = setFeature(ai.FeatureChat, "")
	case rowHelp:
		err = setFeature(ai.FeatureHelp, "")
	case rowURL:
		s := m.cur.Settings
		if s.Search.Engine == "searxng" {
			m.failf("the engine is searxng: switch it before clearing the URL")
			return
		}
		s.Search.SearxngURL = ""
		m.saveSettings(s, "SearXNG URL cleared")
		return
	default:
		return
	}
	if err != nil {
		m.fail(err)
		return
	}
	m.reload()
	m.ok("%s cleared", strings.ToLower(defaultLabels[m.defs.list.cur]))
}

func (m *Model) saveDefault() {
	v := &m.defs
	row := v.list.cur
	value := strings.TrimSpace(v.in.Value())
	switch row {
	case rowDefault, rowChat, rowHelp:
		if err := m.checkRef(value); err != nil {
			m.fail(err)
			return
		}
		var err error
		if row == rowDefault {
			err = ai.SetDefaultModel(value)
		} else {
			err = setFeature([]string{"", ai.FeatureChat, ai.FeatureHelp}[row], value)
		}
		if err != nil {
			m.fail(err)
			return
		}
		m.reload()
		m.ok("%s saved", strings.ToLower(defaultLabels[row]))
	case rowURL:
		if err := checkURL(value); err != nil {
			m.fail(err)
			return
		}
		s := m.cur.Settings
		s.Search.SearxngURL = value
		if v.engineAfterURL != "" && value != "" {
			s.Search.Engine = v.engineAfterURL
		}
		m.saveSettings(s, "SearXNG URL saved")
		if m.failed {
			return
		}
	}
	v.editing, v.engineAfterURL = false, ""
	v.in.Blur()
}

func (m *Model) checkRef(ref string) error {
	if ref == "" {
		return nil
	}
	name, model, ok := strings.Cut(ref, "/")
	if !ok || name == "" || model == "" {
		return errors.New("a model is written provider/model, for example ollama/llama3.2")
	}
	if _, known := m.provider(name); !known {
		return errors.New("unknown provider " + name + ": add it first")
	}
	return nil
}

func checkURL(s string) error {
	if s == "" {
		return nil
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("the SearXNG URL must look like http://localhost:8888")
	}
	return nil
}

func (m *Model) defaultsView() ([]string, string) {
	lines := []string{title("Defaults", m.w), ""}
	for i := 0; i < defaultRows; i++ {
		value := m.defaultValue(i)
		if value == "" {
			value = m.unsetText(i)
		}
		label := cell(defaultLabels[i], 15)
		switch {
		case i == m.defs.list.cur && m.defs.editing:
			lines = append(lines, cursorSt.Render("▸ ")+labelSt.Render(label)+m.defs.in.View())
		case i == m.defs.list.cur:
			lines = append(lines, selSt.Render(cell("▸ "+label+value, m.w)))
		default:
			st := textSt
			if m.defaultValue(i) == "" {
				st = dimSt
			}
			lines = append(lines, "  "+labelSt.Render(label)+st.Render(value))
		}
	}
	lines = append(lines, "", dimSt.Render(defaultHints[m.defs.list.cur]))
	if !m.defs.editing {
		lines = append(lines, "", dimSt.Render("ai.yaml holds the chat, help and search settings; config.toml the default model."))
	}
	if m.defs.editing {
		return lines, editHelp
	}
	return lines, defaultsHelp
}

func (m *Model) unsetText(row int) string {
	switch row {
	case rowChat, rowHelp:
		if f := m.cur.fallback(); f != "" {
			return "(default: " + f + ")"
		}
	}
	return "(not set)"
}

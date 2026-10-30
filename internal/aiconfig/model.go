// Package aiconfig is Misa's AI provider configurator: a full-screen procmon.Viewer to add providers from presets,
// set keys, test them, pick models and choose the defaults Misa's AI features use. Changes go through internal/ai,
// so ugh sees every one of them.
package aiconfig

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AmadeusDE/Misa/internal/ai"
)

type mode int

const (
	modeProviders mode = iota
	modePresets
	modeForm
	modeKey
	modeConfirm
	modeModels
	modeDefaults
	modeHelp
)

// Model is the configurator. It satisfies procmon.Viewer.
type Model struct {
	w, h   int
	quit   bool
	mode   mode
	status string
	failed bool

	providers []ai.ProviderInfo
	list      cursorList
	cur       current
	testGen   int

	pick     presetPicker
	form     form
	key      keyPrompt
	models   modelsView
	defs     defaultsView
	confirm  string
	helpFrom mode
}

// New opens the configurator on the provider list.
func New() *Model {
	m := &Model{w: 80, h: 24}
	m.reload()
	m.fit()
	return m
}

func (m *Model) Init() tea.Cmd   { return nil }
func (m *Model) IsQuit() bool    { return m.quit }
func (m *Model) Resize(w, h int) { m.w, m.h = w, h; m.fit() }

func (m *Model) ok(format string, args ...any) {
	m.status, m.failed = fmt.Sprintf(format, args...), false
}

func (m *Model) fail(err error, secrets ...string) {
	m.status, m.failed = safeError(err, secrets...), true
}

func (m *Model) failf(format string, args ...any) {
	m.status, m.failed = fmt.Sprintf(format, args...), true
}

// reload re-reads the providers and defaults, keeping the selection on the same provider.
func (m *Model) reload() {
	name := m.selectedName()
	providers, err := ai.Providers()
	if err != nil {
		m.fail(err)
	}
	m.providers = providers
	m.list.setN(len(providers))
	m.selectProvider(name)
	cur, err := loadCurrent()
	if err != nil {
		m.fail(err)
	}
	m.cur = cur
}

func (m *Model) selectedName() string {
	if m.list.cur < len(m.providers) {
		return m.providers[m.list.cur].Name
	}
	return ""
}

func (m *Model) selectProvider(name string) {
	for i, p := range m.providers {
		if p.Name == name {
			m.list.cur = i
		}
	}
	m.list.fit()
}

func (m *Model) provider(name string) (ai.ProviderInfo, bool) {
	for _, p := range m.providers {
		if p.Name == name {
			return p, true
		}
	}
	return ai.ProviderInfo{}, false
}

func (m *Model) selected() (ai.ProviderInfo, bool) {
	if m.list.cur < len(m.providers) {
		return m.providers[m.list.cur], true
	}
	return ai.ProviderInfo{}, false
}

// fit places every list and input for the current size.
func (m *Model) fit() {
	body := max(1, m.h-2)
	m.list.place(3, body-3)
	m.pick.list.place(2, body-2)
	m.models.list.place(2, body-2)
	m.defs.list.place(2, defaultRows)
	width := max(10, m.w-22)
	for i := range m.form.fields {
		m.form.fields[i].in.SetWidth(width)
	}
	m.key.in.SetWidth(width)
	m.models.filter.SetWidth(max(10, m.w-14))
	m.defs.in.SetWidth(width)
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Resize(msg.Width, msg.Height)
	case testDoneMsg:
		m.testDone(msg)
	case modelsMsg:
		m.modelsLoaded(msg)
	case tea.KeyPressMsg:
		m.status = ""
		if msg.String() == "ctrl+c" {
			m.quit = true
			return nil
		}
		return m.keyPress(msg)
	case tea.PasteMsg:
		return m.forward(msg)
	case tea.MouseClickMsg:
		return m.click(msg.Mouse())
	case tea.MouseWheelMsg:
		m.wheel(msg.Mouse().Button == tea.MouseWheelUp)
	}
	return nil
}

func (m *Model) keyPress(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	switch m.mode {
	case modeProviders:
		return m.providersKey(k)
	case modePresets:
		return m.presetsKey(k)
	case modeForm:
		return m.formKey(msg)
	case modeKey:
		return m.keyPromptKey(msg)
	case modeConfirm:
		m.confirmKey(k)
	case modeModels:
		return m.modelsKey(msg)
	case modeDefaults:
		return m.defaultsKey(msg)
	case modeHelp:
		m.mode = m.helpFrom
	}
	return nil
}

// input is the text input that has the keyboard, if any.
func (m *Model) input() *textinput.Model {
	switch {
	case m.mode == modeForm && m.form.cur < len(m.form.fields):
		return &m.form.fields[m.form.cur].in
	case m.mode == modeKey:
		return &m.key.in
	case m.mode == modeModels && m.models.filtering:
		return &m.models.filter
	case m.mode == modeDefaults && m.defs.editing:
		return &m.defs.in
	}
	return nil
}

// forward hands a message to the focused text input. Blink commands are dropped: the cursor stays solid.
func (m *Model) forward(msg tea.Msg) tea.Cmd {
	if in := m.input(); in != nil {
		*in, _ = in.Update(msg)
		if m.mode == modeModels {
			m.models.refilter()
		}
	}
	return nil
}

func (m *Model) click(mo tea.Mouse) tea.Cmd {
	if mo.Button != tea.MouseLeft {
		return nil
	}
	switch m.mode {
	case modeProviders:
		if hit, again := m.list.click(mo.Y); hit && again {
			return m.edit()
		}
	case modePresets:
		if hit, again := m.pick.list.click(mo.Y); hit && again {
			return m.pickPreset()
		}
	case modeForm:
		m.form.focus(mo.Y - formTop)
	case modeModels:
		if hit, again := m.models.list.click(mo.Y); hit && again {
			m.setModel("")
		}
	case modeDefaults:
		if !m.defs.editing {
			if hit, again := m.defs.list.click(mo.Y); hit && again {
				return m.editDefault()
			}
		}
	case modeConfirm:
		m.mode = modeProviders
	case modeHelp:
		m.mode = m.helpFrom
	}
	return nil
}

func (m *Model) wheel(up bool) {
	d := 1
	if up {
		d = -1
	}
	switch m.mode {
	case modeProviders:
		m.list.wheel(d)
	case modePresets:
		m.pick.list.wheel(d)
	case modeModels:
		m.models.list.wheel(d)
	case modeDefaults:
		if !m.defs.editing {
			m.defs.list.move(d)
		}
	}
}

func (m *Model) View() string {
	if m.w <= 0 || m.h <= 0 {
		return ""
	}
	var body []string
	var help string
	switch m.mode {
	case modeProviders, modeConfirm:
		body, help = m.providersView()
	case modePresets:
		body, help = m.presetsView()
	case modeForm:
		body, help = m.formView()
	case modeKey:
		body, help = m.keyView()
	case modeModels:
		body, help = m.modelsView()
	case modeDefaults:
		body, help = m.defaultsView()
	case modeHelp:
		body, help = helpView(m.helpFrom)
	}
	return m.frame(body, help)
}

// frame fits the body above a status line and a key help line, one terminal line each.
func (m *Model) frame(body []string, help string) string {
	rows := max(1, m.h-2)
	lines := make([]string, 0, m.h)
	for _, l := range body[:min(len(body), rows)] {
		lines = append(lines, clip(l, m.w))
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	status := m.status
	if m.mode == modeConfirm {
		status = fmt.Sprintf("Delete provider %s and its stored key? y/N", m.confirm)
	}
	st := okSt
	if m.failed || m.mode == modeConfirm {
		st = errSt
	}
	lines = append(lines, st.Render(clip(status, m.w)), dimSt.Render(clip(help, m.w)))
	return strings.Join(lines, "\n")
}

func title(text string, w int) string {
	return titleSt.Render(ansi.Truncate(text, max(1, w), "…"))
}

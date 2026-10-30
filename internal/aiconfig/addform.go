package aiconfig

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ai"
	uc "github.com/AmadeusDE/Misa/internal/ugh/config"
)

const (
	presetsHelp = "up/down choose  enter select  esc cancel"
	formHelp    = "tab/shift+tab field  enter next, save on the last  ctrl+s save  esc cancel"
	formTop     = 2
)

const (
	fieldName = iota
	fieldURL
	fieldKeyEnv
	fieldHeaders
	fieldNoThinking
)

var fieldLabels = [...]string{"Name", "Base URL", "Key env var", "Headers", "No-thinking"}

var fieldHints = [...]string{
	"Lowercase letters, digits . _ - ; models are named name/model.",
	"The OpenAI-compatible endpoint, https (or http on localhost).",
	"Environment variable holding the key. Blank: the name in capitals + _API_KEY.",
	"Extra request headers as Name: value; Name2: value2 (values cannot contain ;).",
	"Model id prefixes that never get a thinking parameter, separated by commas.",
}

type presetPicker struct {
	list    cursorList
	presets []ai.Preset
}

type field struct {
	in     textinput.Model
	locked bool
}

// form is the provider form: a new provider from a preset or from scratch, or an existing one being edited.
type form struct {
	fields  []field
	cur     int
	editing bool
	base    uc.Provider // settings the form has no field for, kept on edit
	keyURL  string
}

func newField(value string, locked bool) field {
	in := textinput.New()
	in.Prompt = ""
	in.SetValue(value)
	return field{in: in, locked: locked}
}

func (f *form) focus(i int) {
	if i < 0 || i >= len(f.fields) || f.fields[i].locked {
		return
	}
	if f.cur < len(f.fields) {
		f.fields[f.cur].in.Blur()
	}
	f.cur = i
	f.fields[i].in.Focus()
	f.fields[i].in.CursorEnd()
}

func (f *form) step(d int) {
	for i := f.cur + d; i >= 0 && i < len(f.fields); i += d {
		if !f.fields[i].locked {
			f.focus(i)
			return
		}
	}
}

func (m *Model) openForm(values [5]string, editing bool) {
	m.form = form{editing: editing}
	for i, v := range values {
		m.form.fields = append(m.form.fields, newField(v, editing && i == fieldName))
	}
	m.form.cur = len(values)
	m.form.focus(fieldName)
	if editing {
		m.form.focus(fieldURL)
	}
	m.mode = modeForm
	m.fit()
}

func (m *Model) startAdd() {
	m.pick.presets = ai.Presets()
	m.pick.list.setN(len(m.pick.presets) + 1)
	m.pick.list.cur = 0
	m.mode = modePresets
	m.fit()
}

func (m *Model) presetsKey(k string) tea.Cmd {
	if m.pick.list.key(k) {
		return nil
	}
	switch k {
	case "esc", "q":
		m.mode = modeProviders
	case "enter":
		return m.pickPreset()
	}
	return nil
}

func (m *Model) pickPreset() tea.Cmd {
	i := m.pick.list.cur
	if i >= len(m.pick.presets) {
		m.openForm([5]string{}, false)
		return nil
	}
	p := m.pick.presets[i]
	m.openForm([5]string{p.Name, p.BaseURL, p.KeyEnv}, false)
	m.form.keyURL = p.KeyURL
	return nil
}

func (m *Model) startEdit(p ai.ProviderInfo) {
	c := p.Custom
	m.openForm([5]string{p.Name, c.BaseURL, c.KeyEnv, formatHeaders(c.Headers), strings.Join(c.NoThinking, ", ")}, true)
	m.form.base = c
}

func (m *Model) formKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = modeProviders
		if !m.form.editing {
			m.mode = modePresets
		}
	case "tab", "down":
		m.form.step(1)
	case "shift+tab", "up":
		m.form.step(-1)
	case "ctrl+s":
		return m.submitForm()
	case "enter":
		if m.form.cur == len(m.form.fields)-1 {
			return m.submitForm()
		}
		m.form.step(1)
	default:
		return m.forward(msg)
	}
	return nil
}

func (m *Model) value(i int) string { return strings.TrimSpace(m.form.fields[i].in.Value()) }

func (m *Model) submitForm() tea.Cmd {
	name := strings.ToLower(m.value(fieldName))
	headers, err := parseHeaders(m.value(fieldHeaders))
	if err != nil {
		m.fail(err)
		return nil
	}
	if _, exists := m.provider(name); exists && !m.form.editing {
		m.failf("%s already exists: select it in the list and press enter to edit", name)
		return nil
	}
	p := m.form.base
	p.BaseURL, p.KeyEnv, p.Headers, p.NoThinking = m.value(fieldURL), m.value(fieldKeyEnv), headers, splitList(m.value(fieldNoThinking))
	if err := ai.SaveProvider(name, p); err != nil {
		m.fail(err)
		return nil
	}
	m.reload()
	m.selectProvider(name)
	m.mode = modeProviders
	m.ok("saved %s", name)
	if info, ok := m.provider(name); ok && !m.form.editing && info.KeySource == "" && !info.Local {
		m.startKey(info, true)
	}
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseHeaders(s string) (map[string]string, error) {
	if s == "" {
		return nil, nil
	}
	out := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		name, value, ok := strings.Cut(part, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("header %q must look like Name: value", strings.TrimSpace(part))
		}
		out[name] = strings.TrimSpace(value)
	}
	return out, nil
}

func formatHeaders(h map[string]string) string {
	parts := make([]string, 0, len(h))
	for name, value := range h {
		parts = append(parts, name+": "+value)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func (m *Model) presetsView() ([]string, string) {
	lines := []string{title("Add a provider", m.w), dimSt.Render("Pick a preset or a custom OpenAI-compatible endpoint")}
	lo, hi := m.pick.list.window()
	for i := lo; i < hi; i++ {
		name, label, url, note := "custom", "Custom (OpenAI-compatible)", "", ""
		if i < len(m.pick.presets) {
			p := m.pick.presets[i]
			name, label, url = p.Name, p.Label, p.BaseURL
			if _, ok := m.provider(p.Name); ok {
				note = " (configured)"
			}
		}
		text := cell(name, 10) + columnGap + cell(label+note, 30) + columnGap + url
		if i == m.pick.list.cur {
			lines = append(lines, selSt.Render(cell("▸ "+text, m.w)))
			continue
		}
		lines = append(lines, "  "+text)
	}
	return lines, presetsHelp
}

func (m *Model) formView() ([]string, string) {
	verb := "Add provider"
	if m.form.editing {
		verb = "Edit provider " + m.value(fieldName)
	}
	lines := []string{title(verb, m.w), ""}
	for i, f := range m.form.fields {
		label := labelSt.Render(cell(fieldLabels[i], 12))
		marker := "  "
		if i == m.form.cur {
			marker = cursorSt.Render("▸ ")
		}
		value := f.in.View()
		if f.locked {
			value = dimSt.Render(f.in.Value() + " (fixed)")
		}
		lines = append(lines, marker+label+" "+value)
	}
	lines = append(lines, "", dimSt.Render(fieldHints[m.form.cur]))
	if m.form.keyURL != "" {
		lines = append(lines, dimSt.Render("Get a key at "+m.form.keyURL))
	}
	return lines, formHelp
}

package aiconfig

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func click(m *Model, y int) { run(m, tea.MouseClickMsg{X: 5, Y: y, Button: tea.MouseLeft}) }

func wheel(m *Model, up bool) {
	b := tea.MouseWheelDown
	if up {
		b = tea.MouseWheelUp
	}
	run(m, tea.MouseWheelMsg{X: 5, Y: 5, Button: b})
}

func TestClickSelectsAndSecondClickEdits(t *testing.T) {
	m := newModel(t)
	addLocal(t, m, "ollama", server(t, "", "alpha"))
	row := m.list.top + 3 // the fourth provider
	click(m, row)
	if m.list.cur != 3 || m.mode != modeProviders {
		t.Fatalf("cur %d mode %v", m.list.cur, m.mode)
	}
	click(m, row)
	if m.selectedName() != "ollama" || m.mode != modeForm {
		t.Fatalf("second click edits a custom provider: %q %v", m.selectedName(), m.mode)
	}
}

func TestClickBelowTheListDoesNothing(t *testing.T) {
	m := newModel(t)
	click(m, m.list.top+20)
	if m.list.cur != 0 {
		t.Fatalf("cur %d", m.list.cur)
	}
}

func TestWheelScrollsLongLists(t *testing.T) {
	m := newModel(t)
	addLocal(t, m, "ollama", server(t, "", "alpha")) // five providers
	m.Resize(80, 8)
	if m.list.rows != 3 {
		t.Fatalf("rows %d", m.list.rows)
	}
	wheel(m, false)
	if m.list.off != 2 || m.list.cur < 2 {
		t.Fatalf("off %d cur %d", m.list.off, m.list.cur)
	}
	m.models = modelsView{}
	for i := 0; i < 40; i++ {
		m.models.ids = append(m.models.ids, "model-"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	m.mode = modeModels
	m.fit()
	m.models.refilter()
	wheel(m, false)
	if m.models.list.off != 3 || m.models.list.cur < 3 {
		t.Fatalf("off %d cur %d", m.models.list.off, m.models.list.cur)
	}
	wheel(m, true)
	if m.models.list.off != 0 {
		t.Fatalf("off %d", m.models.list.off)
	}
}

func TestClickInTheFormFocusesAField(t *testing.T) {
	m := newModel(t)
	press(m, "a", "enter")
	click(m, formTop+fieldKeyEnv)
	if m.form.cur != fieldKeyEnv {
		t.Fatalf("cur %d", m.form.cur)
	}
}

func TestClickOnModelAndPresetRows(t *testing.T) {
	m := newModel(t)
	press(m, "a")
	click(m, m.pick.list.top+2)
	if m.pick.list.cur != 2 || m.mode != modePresets {
		t.Fatalf("cur %d mode %v", m.pick.list.cur, m.mode)
	}
	click(m, m.pick.list.top+2)
	if m.mode != modeForm {
		t.Fatalf("mode %v", m.mode)
	}
	if got := m.form.fields[fieldName].in.Value(); got != m.pick.presets[2].Name {
		t.Fatalf("name %q", got)
	}
}

func TestClickClosesHelpAndConfirm(t *testing.T) {
	m := newModel(t)
	press(m, "?")
	click(m, 3)
	if m.mode != modeProviders {
		t.Fatalf("mode %v", m.mode)
	}
}

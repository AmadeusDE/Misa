package aiconfig

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/ai"
)

const providersHelp = "a add  enter edit  k key  K remove key  d delete  t test  m models  D defaults  ? help  q quit"

func (m *Model) providersKey(k string) tea.Cmd {
	if m.list.key(k) {
		return nil
	}
	p, have := m.selected()
	switch k {
	case "q", "esc":
		m.quit = true
	case "a":
		m.startAdd()
	case "D":
		m.openDefaults()
	case "?":
		m.helpFrom, m.mode = modeProviders, modeHelp
	case "r":
		m.reload()
		m.ok("reloaded")
	}
	if !have {
		return nil
	}
	switch k {
	case "enter", "e":
		return m.edit()
	case "k":
		m.startKey(p, false)
	case "K":
		m.removeKey(p)
	case "d":
		m.askDelete(p)
	case "t":
		return m.testProvider(p)
	case "m":
		return m.openModels(p)
	}
	return nil
}

func (m *Model) removeKey(p ai.ProviderInfo) {
	if p.Kind == "codex" || p.Kind == "console" {
		m.failf("%s uses a login, not a key: run `ugh login`", p.Name)
		return
	}
	err := ai.RemoveKey(p.Name)
	if err != nil && p.KeySource == "env" {
		m.failf("no stored key to remove; %s comes from the environment", p.KeyEnv)
		return
	}
	if err != nil {
		m.fail(err)
		return
	}
	m.reload()
	m.ok("removed the stored key for %s", p.Name)
}

func (m *Model) askDelete(p ai.ProviderInfo) {
	if p.Kind != "custom" {
		m.failf("%s is built in and cannot be deleted", p.Name)
		return
	}
	m.confirm, m.mode = p.Name, modeConfirm
}

func (m *Model) confirmKey(k string) {
	m.mode = modeProviders
	if k != "y" && k != "Y" {
		return
	}
	if err := ai.RemoveProvider(m.confirm); err != nil {
		m.fail(err)
		return
	}
	m.reload()
	m.ok("deleted %s", m.confirm)
}

// edit opens the provider form for a config-defined provider and explains the others.
func (m *Model) edit() tea.Cmd {
	p, ok := m.selected()
	switch {
	case !ok:
	case p.Kind == "codex":
		m.failf("codex is a login, not a provider to edit: run `ugh login codex`")
	case p.Kind == "console":
		m.failf("%s comes from the OpenCode Console login: run `ugh login opencode-console`", p.Name)
	case p.Kind == "builtin":
		m.failf("%s is built in: press k to set its key, m to pick a model", p.Name)
	default:
		m.startEdit(p)
	}
	return nil
}

var columnGap = "  "

func keyStatus(p ai.ProviderInfo) (string, lipgloss.Style) {
	switch p.KeySource {
	case "env":
		return "env " + p.KeyMasked, okSt
	case "store":
		return "key " + p.KeyMasked, okSt
	case "login":
		return "logged in", okSt
	}
	switch {
	case p.Kind == "codex" || p.Kind == "console":
		return "no login", warnSt
	case p.Local:
		return "no key needed", dimSt
	}
	return "missing", errSt
}

func (m *Model) uses(p ai.ProviderInfo) string {
	var u []string
	if providerOf(m.cur.Default) == p.Name {
		u = append(u, "default")
	}
	for _, f := range []string{ai.FeatureChat, ai.FeatureHelp} {
		if providerOf(m.cur.effective(f)) == p.Name && m.cur.effective(f) != m.cur.Default {
			u = append(u, f)
		}
	}
	return strings.Join(u, " ")
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (m *Model) providersView() ([]string, string) {
	lines := []string{title("AI providers", m.w)}
	lines = append(lines, dimSt.Render(fmt.Sprintf("default %s   chat %s   help %s   search %s",
		orDash(m.cur.Default), orDash(m.cur.effective(ai.FeatureChat)), orDash(m.cur.effective(ai.FeatureHelp)),
		m.cur.Settings.Search.Engine)))
	nameW, kindW, keyW, useW := 8, 7, 18, 0
	for _, p := range m.providers {
		nameW = max(nameW, len(p.Name))
		useW = max(useW, len(m.uses(p)))
	}
	nameW = min(nameW, 20)
	lines = append(lines, headSt.Render("  "+cell("PROVIDER", nameW)+columnGap+cell("KIND", kindW)+columnGap+
		cell("KEY", keyW)+columnGap+cell("USED FOR", max(useW, 8))+columnGap+"BASE URL"))
	lo, hi := m.list.window()
	for i := lo; i < hi; i++ {
		lines = append(lines, m.providerRow(i, nameW, kindW, keyW, max(useW, 8)))
	}
	if len(m.providers) == 0 {
		lines = append(lines, dimSt.Render("  no providers: press a to add one"))
	}
	return lines, providersHelp
}

func (m *Model) providerRow(i, nameW, kindW, keyW, useW int) string {
	p := m.providers[i]
	key, keySt := keyStatus(p)
	cols := []struct {
		text string
		w    int
		st   lipgloss.Style
	}{
		{p.Name, nameW, textSt}, {p.Kind, kindW, dimSt}, {key, keyW, keySt}, {m.uses(p), useW, tagSt}, {p.BaseURL, 0, dimSt},
	}
	selected := i == m.list.cur
	prefix := "  "
	if selected {
		prefix = "▸ "
	}
	var plain, styled strings.Builder
	plain.WriteString(prefix)
	styled.WriteString(cursorSt.Render(prefix))
	for n, c := range cols {
		text := c.text
		if c.w > 0 {
			text = cell(text, c.w)
		}
		if n > 0 {
			plain.WriteString(columnGap)
			styled.WriteString(columnGap)
		}
		plain.WriteString(text)
		styled.WriteString(c.st.Render(text))
	}
	if selected {
		return selSt.Render(cell(plain.String(), m.w))
	}
	return styled.String()
}

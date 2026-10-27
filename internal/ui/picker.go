package ui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/ascii"
	"github.com/AmadeusDE/Misa/internal/color"
)

type pickerItem struct {
	name     string
	desc     string
	selected bool
}

func (p pickerItem) Title() string {
	mark := "[ ]"
	if p.selected {
		mark = "[x]"
	}
	return mark + " " + p.name
}
func (p pickerItem) Description() string { return p.desc }
func (p pickerItem) FilterValue() string { return p.name }

type pickerKind int

const (
	pickerAnime pickerKind = iota
	pickerDistro
	pickerTheme
)

type pickerState struct {
	kind       pickerKind
	pList      list.Model
	selected   map[string]bool
	allNames   []string
	previewing bool
	previewVP  viewport.Model
	onDone     func(selected []string)
}

func (m *Model) enterAnimePicker(onDone func([]string)) {
	names := ascii.ListAnimeNames()
	sort.Strings(names)

	current := ""
	if m.settings != nil {
		current = m.settings.cfg.Ascii.Anime
	}
	sel := parseCSVSet(current)
	if sel["random"] {
		delete(sel, "random")
	}

	items := buildPickerItems(names, sel, "Anime ASCII art")
	pList := newPickerList(items, "Anime Art — space: toggle  enter: preview  /: search  esc: done")

	m.picker = &pickerState{
		kind:     pickerAnime,
		pList:    pList,
		selected: sel,
		allNames: names,
		onDone:   onDone,
	}
	m.screen = screenPicker
	m.resize()
}

func (m *Model) enterDistroPicker(onDone func([]string)) {
	names := ascii.ListDistroNames()

	current := ""
	if m.settings != nil {
		current = m.settings.cfg.Ascii.Distro
	}
	sel := parseCSVSet(current)

	items := buildPickerItems(names, sel, "Distro ASCII art")
	pList := newPickerList(items, "Distro Art — space: toggle  enter: preview  /: search  esc: done")

	m.picker = &pickerState{
		kind:     pickerDistro,
		pList:    pList,
		selected: sel,
		allNames: names,
		onDone:   onDone,
	}
	m.screen = screenPicker
	m.resize()
}

func (m *Model) enterThemePicker(onDone func([]string)) {
	palettes := color.LoadPalettes("")
	var names []string
	for k := range palettes {
		names = append(names, k)
	}
	sort.Strings(names)
	names = append([]string{"default", "native"}, names...)

	current := ""
	if m.settings != nil {
		current = m.settings.cfg.Ascii.Theme
	}
	sel := parseCSVSet(current)

	items := buildPickerItems(names, sel, "Color theme")
	pList := newPickerList(items, "Themes — space: toggle  enter: preview  /: search  esc: done")

	m.picker = &pickerState{
		kind:     pickerTheme,
		pList:    pList,
		selected: sel,
		allNames: names,
		onDone:   onDone,
	}
	m.screen = screenPicker
	m.resize()
}

func newPickerList(items []list.Item, title string) list.Model {
	l := list.New(items, compactDelegate{}, 0, 0)
	l.Title = title
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)
	l.DisableQuitKeybindings()
	return l
}

func buildPickerItems(names []string, sel map[string]bool, desc string) []list.Item {
	items := make([]list.Item, len(names))
	for i, n := range names {
		items[i] = pickerItem{name: n, desc: desc, selected: sel[n]}
	}
	return items
}

func parseCSVSet(csv string) map[string]bool {
	m := make(map[string]bool)
	for _, s := range strings.Split(csv, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			m[s] = true
		}
	}
	return m
}

func (m *Model) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.picker == nil {
		m.screen = screenSettings
		return m, nil
	}

	if m.picker.previewing {
		return m.updatePickerPreview(msg)
	}

	switch msg := msg.(type) {
	case mouseMsg:
		return m.handlePickerMouse(msg)
	case tea.KeyPressMsg:
		if m.picker.pList.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.picker.pList, cmd = m.picker.pList.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "esc", "backspace", "q":
			m.finishPicker()
			return m, nil

		case "space":
			m.togglePickerItem()
			return m, nil

		case "enter":
			m.showPickerPreview()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.picker.pList, cmd = m.picker.pList.Update(msg)
	return m, cmd
}

func (m *Model) updatePickerPreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "backspace", "q":
			m.picker.previewing = false
			m.resize()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.picker.previewVP, cmd = m.picker.previewVP.Update(msg)
	return m, cmd
}

func (m *Model) togglePickerItem() {
	it, ok := m.picker.pList.SelectedItem().(pickerItem)
	if !ok {
		return
	}
	it.selected = !it.selected
	m.picker.selected[it.name] = it.selected
	if !it.selected {
		delete(m.picker.selected, it.name)
	}

	items := m.picker.pList.Items()
	idx := m.picker.pList.Index()
	items[idx] = it
	m.picker.pList.SetItems(items)
	m.picker.pList.Select(idx)
}

func (m *Model) showPickerPreview() {
	it, ok := m.picker.pList.SelectedItem().(pickerItem)
	if !ok {
		return
	}

	var lines []string
	switch m.picker.kind {
	case pickerAnime:
		lines = ascii.LoadAnimeArt(it.name)
	case pickerDistro:
		lines = ascii.LoadDistroPreview(it.name)
	case pickerTheme:
		lines = renderThemePreview(it.name)
	}

	if len(lines) == 0 {
		lines = []string{"(no preview available)"}
	}

	divStyle := lipgloss.NewStyle().Foreground(m.uiColors.divider)
	header := settingKeyStyle.Render("Preview: "+it.name) + "\n" +
		divStyle.Render(strings.Repeat("─", 40)) + "\n"
	footer := "\n" + settingHelpStyle.Render("  esc: back to list")
	content := header + strings.Join(lines, "\n") + footer

	vp := viewport.New(viewport.WithWidth(m.width-4), viewport.WithHeight(m.height-4))
	vp.SetContent(content)
	m.picker.previewVP = vp
	m.picker.previewing = true
}

func renderThemePreview(name string) []string {
	if name == "default" {
		return []string{"Default distro colors (red, uses config distro color)"}
	}
	if name == "native" {
		return []string{"Native art colors (uses colors embedded in the ASCII art file)"}
	}
	palettes := color.LoadPalettes("")
	pal, ok := palettes[name]
	if !ok {
		return []string{"(palette not found)"}
	}

	var swatchLine strings.Builder
	for _, hex := range pal {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
		swatchLine.WriteString(style.Render("████"))
		swatchLine.WriteString(" ")
	}

	var hexLine strings.Builder
	for _, hex := range pal {
		padded := hex
		for len(padded) < 4 {
			padded += " "
		}
		hexLine.WriteString(padded + " ")
	}

	return []string{
		swatchLine.String(),
		hexLine.String(),
		"",
		strings.Repeat("─", 40),
		"Applied to sample art:",
		"",
		renderThemedSample(name, pal),
	}
}

func renderThemedSample(name string, pal []string) string {
	sample := []string{
		`       /\       `,
		`      /  \      `,
		`     /\   \     `,
		`    /      \    `,
		`   /   ,,   \   `,
		`  /   |  |  -\  `,
		` /_-''    ''-_\ `,
	}

	var ansiPal []string
	for _, hex := range pal {
		if code, err := color.HexToANSI(hex); err == nil {
			ansiPal = append(ansiPal, code)
		}
	}
	if len(ansiPal) == 0 {
		return strings.Join(sample, "\n")
	}

	var result []string
	for i, line := range sample {
		idx := (i * len(ansiPal)) / len(sample)
		if idx >= len(ansiPal) {
			idx = len(ansiPal) - 1
		}
		result = append(result, ansiPal[idx]+line+color.Reset)
	}
	return strings.Join(result, "\n")
}

func (m *Model) finishPicker() {
	if m.picker == nil {
		return
	}
	var selected []string
	for _, name := range m.picker.allNames {
		if m.picker.selected[name] {
			selected = append(selected, name)
		}
	}
	onDone := m.picker.onDone
	m.picker = nil
	if onDone != nil {
		onDone(selected)
	}
	if m.picker == nil && m.screen == screenPicker {
		m.screen = screenSettings
	}
	m.resize()
}

func (m *Model) handlePickerMouse(msg mouseMsg) (tea.Model, tea.Cmd) {
	if m.picker == nil {
		return m, nil
	}
	if isLeftClick(msg) {
		startY := 3
		idx := listClickIndex(m.picker.pList, msg.Y, startY)
		if idx >= 0 && idx < len(m.picker.pList.Items()) {
			if idx == m.picker.pList.Index() {
				m.togglePickerItem()
			} else {
				m.picker.pList.Select(idx)
			}
		}
		return m, nil
	}
	if isWheelUp(msg) || isWheelDown(msg) {
		idx := m.picker.pList.Index()
		if isWheelUp(msg) {
			if idx > 0 {
				m.picker.pList.Select(idx - 3)
			}
		} else {
			total := len(m.picker.pList.Items())
			if idx < total-1 {
				next := idx + 3
				if next >= total {
					next = total - 1
				}
				m.picker.pList.Select(next)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) pickerContent() string {
	if m.picker == nil {
		return ""
	}
	if m.picker.previewing {
		return m.picker.previewVP.View()
	}

	count := 0
	for _, v := range m.picker.selected {
		if v {
			count++
		}
	}
	footer := settingHelpStyle.Render(
		strings.Repeat(" ", 2) + "Selected: " + strings.Repeat("█", count) +
			" (" + lipgloss.NewStyle().Bold(true).Render(strings.TrimSpace(strings.Repeat("1", count))) + ")")
	if count == 0 {
		footer = settingHelpStyle.Render("  No items selected (will use defaults)")
	} else {
		var names []string
		for _, n := range m.picker.allNames {
			if m.picker.selected[n] {
				names = append(names, n)
			}
		}
		footer = settingHelpStyle.Render("  Selected: " + strings.Join(names, ", "))
	}

	return m.picker.pList.View() + "\n" + footer
}

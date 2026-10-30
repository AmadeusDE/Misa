package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/companion"
	"github.com/AmadeusDE/Misa/internal/config"
)

var (
	settingKeyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#d3869b")).Bold(true)
	settingValStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8ec07c"))
	settingDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#665c54"))
	settingHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#665c54")).Italic(true)
)

type settingKind int

const (
	settingSelect settingKind = iota
	settingText
	settingNumber
	settingMultiSelect
	settingPicker
)

type settingDef struct {
	key     string
	label   string
	kind    settingKind
	options []string
	get     func(ss *settingsState) string
	set     func(ss *settingsState, val string)
	onPick  func(m *Model)
}

func settingDefs() []settingDef {
	return []settingDef{
		{
			key:   "theme",
			label: "Color themes",
			kind:  settingPicker,
			get:   func(ss *settingsState) string { return ss.cfg.Ascii.Theme },
			set:   func(ss *settingsState, v string) { ss.cfg.Ascii.Theme = v },
			onPick: func(m *Model) {
				m.enterThemePicker(func(sel []string) {
					if len(sel) == 0 {
						m.settings.cfg.Ascii.Theme = "default"
					} else {
						m.settings.cfg.Ascii.Theme = strings.Join(sel, ",")
					}
					m.refreshSettingsItems()
				})
			},
		},
		{
			key:   "anime",
			label: "Anime art",
			kind:  settingPicker,
			get:   func(ss *settingsState) string { return ss.cfg.Ascii.Anime },
			set:   func(ss *settingsState, v string) { ss.cfg.Ascii.Anime = v },
			onPick: func(m *Model) {
				m.enterAnimePicker(func(sel []string) {
					if len(sel) == 0 {
						m.settings.cfg.Ascii.Anime = "random"
					} else {
						m.settings.cfg.Ascii.Anime = strings.Join(sel, ",")
					}
					m.refreshSettingsItems()
				})
			},
		},
		{
			key:   "distro",
			label: "Distro art",
			kind:  settingPicker,
			get:   func(ss *settingsState) string { return ss.cfg.Ascii.Distro },
			set:   func(ss *settingsState, v string) { ss.cfg.Ascii.Distro = v },
			onPick: func(m *Model) {
				m.enterDistroPicker(func(sel []string) {
					m.settings.cfg.Ascii.Distro = strings.Join(sel, ",")
					m.refreshSettingsItems()
				})
			},
		},
		{
			key:     "color_mode",
			label:   "Color mode",
			kind:    settingSelect,
			options: []string{"line", "char"},
			get:     func(ss *settingsState) string { return ss.cfg.Ascii.ColorMode },
			set:     func(ss *settingsState, v string) { ss.cfg.Ascii.ColorMode = v },
		},
		{
			key:   "max_height",
			label: "Max art height",
			kind:  settingNumber,
			get: func(ss *settingsState) string {
				if ss.cfg.Ascii.MaxHeight == 0 {
					return "16"
				}
				return strconv.Itoa(ss.cfg.Ascii.MaxHeight)
			},
			set: func(ss *settingsState, v string) {
				n, err := strconv.Atoi(v)
				if err == nil {
					ss.cfg.Ascii.MaxHeight = n
				}
			},
		},
		{
			key:   "statusbar_info",
			label: "Status bar info items",
			kind:  settingText,
			get: func(ss *settingsState) string {
				if ss.cfg.Statusbar.Info != "" {
					return ss.cfg.Statusbar.Info
				}
				if ss.cfg.Info != "" {
					return ss.cfg.Info
				}
				return config.DefaultInfo
			},
			set: func(ss *settingsState, v string) {
				ss.cfg.Statusbar.Info = v
			},
		},
		{
			key:     "version_color",
			label:   "Version color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.Version },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.Version = v },
		},
		{
			key:     "time_color",
			label:   "Time color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.Time },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.Time = v },
		},
		{
			key:     "cwd_color",
			label:   "CWD color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.CWD },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.CWD = v },
		},
		{
			key:     "ram_color",
			label:   "RAM color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.RAM },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.RAM = v },
		},
		{
			key:     "uptime_color",
			label:   "Uptime color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.Uptime },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.Uptime = v },
		},
		{
			key:     "kernel_color",
			label:   "Kernel color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.Kernel },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.Kernel = v },
		},
		{
			key:     "terminal_color",
			label:   "Terminal color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.Terminal },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.Terminal = v },
		},
		{
			key:     "swap_color",
			label:   "Swap color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.Swap },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.Swap = v },
		},
		{
			key:     "distro_color",
			label:   "Distro art color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.Distro },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.Distro = v },
		},
		{
			key:     "anime_color",
			label:   "Anime art color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.Anime },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.Anime = v },
		},
		{
			key:     "git_color",
			label:   "Git status color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.Git },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.Git = v },
		},
		{
			key:     "localip_color",
			label:   "Local IP color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.LocalIP },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.LocalIP = v },
		},
		{
			key:     "user_color",
			label:   "User color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.Colors.User },
			set:     func(ss *settingsState, v string) { ss.cfg.Colors.User = v },
		},
		{
			key:     "prompt_color",
			label:   "Prompt color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.UI.Prompt },
			set:     func(ss *settingsState, v string) { ss.cfg.UI.Prompt = v },
		},
		{
			key:     "prompt_arrow_color",
			label:   "Prompt arrow color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.UI.PromptArrow },
			set:     func(ss *settingsState, v string) { ss.cfg.UI.PromptArrow = v },
		},
		{
			key:     "prompt_ok_color",
			label:   "Prompt success color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.UI.PromptOK },
			set:     func(ss *settingsState, v string) { ss.cfg.UI.PromptOK = v },
		},
		{
			key:     "prompt_err_color",
			label:   "Prompt error color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.UI.PromptErr },
			set:     func(ss *settingsState, v string) { ss.cfg.UI.PromptErr = v },
		},
		{
			key:     "divider_color",
			label:   "Divider color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.UI.Divider },
			set:     func(ss *settingsState, v string) { ss.cfg.UI.Divider = v },
		},
		{
			key:     "ghost_color",
			label:   "Ghost text color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.UI.Ghost },
			set:     func(ss *settingsState, v string) { ss.cfg.UI.Ghost = v },
		},
		{
			key:     "output_color",
			label:   "Shell output color",
			kind:    settingSelect,
			options: colorOptions(),
			get:     func(ss *settingsState) string { return ss.cfg.UI.Output },
			set:     func(ss *settingsState, v string) { ss.cfg.UI.Output = v },
		},
		{
			key:   "companion",
			label: "Companion",
			kind:  settingPicker,
			get: func(ss *settingsState) string {
				return companionSummary(companion.LoadCompanionConfig())
			},
			set: func(ss *settingsState, v string) {},
			onPick: func(m *Model) {
				m.enterCompanionSetup()
			},
		},
		{
			key:     "start_in_shell",
			label:   "Start in shell",
			kind:    settingSelect,
			options: []string{"off", "on"},
			get: func(ss *settingsState) string {
				if ss.cfg.StartInShell {
					return "on"
				}
				return "off"
			},
			set: func(ss *settingsState, v string) {
				ss.cfg.StartInShell = v == "on"
			},
		},
		{
			key:     "disable_minishell",
			label:   "Disable minishell",
			kind:    settingSelect,
			options: []string{"off", "on"},
			get: func(ss *settingsState) string {
				if ss.cfg.DisableMinishell {
					return "on"
				}
				return "off"
			},
			set: func(ss *settingsState, v string) {
				ss.cfg.DisableMinishell = v == "on"
			},
		},
		{
			key:     "history_search",
			label:   "History search",
			kind:    settingSelect,
			options: []string{"on", "off"},
			get: func(ss *settingsState) string {
				if config.BoolDefault(ss.cfg.HistorySearch, true) {
					return "on"
				}
				return "off"
			},
			set: func(ss *settingsState, v string) {
				ss.cfg.HistorySearch = config.BoolPtr(v == "on")
			},
		},
		{
			key:     "mux_auto_pane",
			label:   "Mux auto-open pane",
			kind:    settingSelect,
			options: []string{"on", "off"},
			get: func(ss *settingsState) string {
				if config.BoolDefault(ss.muxCfg.AutoPane, true) {
					return "on"
				}
				return "off"
			},
			set: func(ss *settingsState, v string) {
				ss.muxCfg.AutoPane = config.BoolPtr(v == "on")
			},
		},
		{
			key:     "mux_bar_auto_hide",
			label:   "Mux bar auto-hide",
			kind:    settingSelect,
			options: []string{"on", "off"},
			get: func(ss *settingsState) string {
				if config.BoolDefault(ss.muxCfg.BarAutoHide, true) {
					return "on"
				}
				return "off"
			},
			set: func(ss *settingsState, v string) {
				ss.muxCfg.BarAutoHide = config.BoolPtr(v == "on")
			},
		},
		{
			key:     "mux_focus_follows_mouse",
			label:   "Mux focus follows mouse",
			kind:    settingSelect,
			options: []string{"on", "off"},
			get: func(ss *settingsState) string {
				if config.BoolDefault(ss.muxCfg.FocusFollowsMouse, true) {
					return "on"
				}
				return "off"
			},
			set: func(ss *settingsState, v string) {
				ss.muxCfg.FocusFollowsMouse = config.BoolPtr(v == "on")
			},
		},
		{
			key:     "mux_bar_position",
			label:   "Mux bar position",
			kind:    settingSelect,
			options: []string{"bottom", "top"},
			get: func(ss *settingsState) string {
				if ss.muxCfg.BarPosition == "" {
					return "bottom"
				}
				return ss.muxCfg.BarPosition
			},
			set: func(ss *settingsState, v string) {
				ss.muxCfg.BarPosition = v
			},
		},
		{
			key:   "mux_bar_info",
			label: "Mux bar info items",
			kind:  settingText,
			get: func(ss *settingsState) string {
				if ss.muxCfg.BarInfo == "" {
					return "version,user,ram,uptime,time"
				}
				return ss.muxCfg.BarInfo
			},
			set: func(ss *settingsState, v string) {
				ss.muxCfg.BarInfo = v
			},
		},
		{
			key:   "ai_providers",
			label: "AI providers",
			kind:  settingPicker,
			get:   func(ss *settingsState) string { return "" },
			set:   func(ss *settingsState, v string) {},
			onPick: func(m *Model) {
				m.enterAIConfig()
			},
		},
		{
			key:   "setup_wizard",
			label: "Run Setup Wizard",
			kind:  settingPicker,
			get:   func(ss *settingsState) string { return "" },
			set:   func(ss *settingsState, v string) {},
			onPick: func(m *Model) {
				m.enterOnboarding()
			},
		},
	}
}

func colorOptions() []string {
	return []string{
		"",
		"#fb4934", "#fe8019", "#fabd2f", "#d79921",
		"#b8bb26", "#8ec07c", "#83a598", "#458588",
		"#d3869b", "#b16286", "#a89984", "#928374",
		"#ebdbb2", "#665c54", "#504945",
	}
}

type settingItem struct {
	def settingDef
	val string
}

func (s settingItem) Title() string {
	val := s.val
	if val == "" {
		val = "(default)"
	}
	// For color options, render a swatch
	if strings.HasPrefix(val, "#") {
		swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(val)).Render("██")
		return settingKeyStyle.Render(s.def.label) + "  " + swatch + " " + settingValStyle.Render(val)
	}
	return settingKeyStyle.Render(s.def.label) + "  " + settingValStyle.Render(val)
}

func (s settingItem) Description() string {
	switch s.def.kind {
	case settingPicker:
		if s.def.key == "companion" {
			return settingDimStyle.Render("enter: moods, pronouns, names, prefixes, items/parts, kemonos, modifiers")
		}
		return settingDimStyle.Render("enter: open picker with search & preview")
	case settingSelect:
		return settingDimStyle.Render("enter: cycle  e: type custom/csv")
	case settingText:
		return settingDimStyle.Render("enter: edit value")
	case settingNumber:
		return settingDimStyle.Render("enter: edit  +/-: adjust")
	}
	return ""
}

func (s settingItem) FilterValue() string { return s.def.label }

type settingsState struct {
	cfg     config.Config
	muxCfg  config.MuxConfig
	defs    []settingDef
	editing bool
	input   textinput.Model
}

func (m *Model) enterSettings() tea.Cmd {
	cfg := config.Load()
	muxCfg := config.LoadMux()
	defs := settingDefs()

	ss := &settingsState{
		cfg:    cfg,
		muxCfg: muxCfg,
		defs:   defs,
	}

	items := make([]list.Item, len(defs))
	for i, d := range defs {
		items[i] = settingItem{def: d, val: d.get(ss)}
	}

	m.screen = screenSettings
	m.settings = ss

	ti := textinput.New()
	ti.CharLimit = 256
	m.settings.input = ti

	m.settingsList.SetItems(items)
	m.settingsList.Title = "Settings"
	m.resize()
	return nil
}

func (m *Model) refreshSettingsItems() {
	items := make([]list.Item, len(m.settings.defs))
	for i, d := range m.settings.defs {
		items[i] = settingItem{def: d, val: d.get(m.settings)}
	}
	m.settingsList.SetItems(items)
}

func (m *Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.settings.editing {
		return m.updateSettingsEdit(msg)
	}

	switch msg := msg.(type) {
	case mouseMsg:
		return m.handleSettingsMouse(msg)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "backspace", "q":
			m.screen = screenMenu
			m.settings = nil
			m.status = ""
			return m, nil

		case "enter":
			it, ok := m.settingsList.SelectedItem().(settingItem)
			if !ok {
				return m, nil
			}
			switch it.def.kind {
			case settingPicker:
				if it.def.onPick != nil {
					it.def.onPick(m)
				}
				return m, nil
			case settingSelect:
				m.cycleOption(it.def)
				return m, nil
			case settingText, settingNumber:
				m.settings.editing = true
				m.settings.input.SetValue(it.val)
				m.settings.input.Focus()
				m.settings.input.Placeholder = it.def.label
				return m, nil
			}

		case "e":
			it, ok := m.settingsList.SelectedItem().(settingItem)
			if !ok {
				return m, nil
			}
			m.settings.editing = true
			m.settings.input.SetValue(it.val)
			m.settings.input.Focus()
			m.settings.input.Placeholder = it.def.label
			return m, nil

		case "+", "=":
			it, ok := m.settingsList.SelectedItem().(settingItem)
			if ok && it.def.kind == settingNumber {
				n, _ := strconv.Atoi(it.val)
				n++
				it.def.set(m.settings, strconv.Itoa(n))
				m.refreshSettingsItems()
			}
			return m, nil

		case "-":
			it, ok := m.settingsList.SelectedItem().(settingItem)
			if ok && it.def.kind == settingNumber {
				n, _ := strconv.Atoi(it.val)
				if n > 0 {
					n--
				}
				it.def.set(m.settings, strconv.Itoa(n))
				m.refreshSettingsItems()
			}
			return m, nil

		case "s", "ctrl+s":
			if m.settings != nil {
				var errs []string
				if err := config.Save(m.settings.cfg); err != nil {
					errs = append(errs, "config: "+err.Error())
				}
				if err := config.SaveMux(m.settings.muxCfg); err != nil {
					errs = append(errs, "mux: "+err.Error())
				}
				if len(errs) > 0 {
					m.status = "error saving: " + strings.Join(errs, "; ")
				} else {
					m.status = "config saved"
					m.startInShell = m.settings.cfg.StartInShell
					m.disableMinishell = m.settings.cfg.DisableMinishell
					m.applyMuxConfig(m.settings.muxCfg)
					return m, muxConfigSaved(m.settings.muxCfg)
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.settingsList, cmd = m.settingsList.Update(msg)
	return m, cmd
}

func (m *Model) updateSettingsEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			idx := m.settingsList.Index()
			if idx >= 0 && idx < len(m.settings.defs) {
				val := m.settings.input.Value()
				m.settings.defs[idx].set(m.settings, val)
				m.refreshSettingsItems()
			}
			m.settings.editing = false
			m.settings.input.Blur()
			return m, nil
		case "esc":
			m.settings.editing = false
			m.settings.input.Blur()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.settings.input, cmd = m.settings.input.Update(msg)
	return m, cmd
}

func (m *Model) cycleOption(def settingDef) {
	current := def.get(m.settings)
	opts := def.options
	if len(opts) == 0 {
		return
	}

	idx := 0
	for i, o := range opts {
		if o == current {
			idx = i
			break
		}
	}
	next := (idx + 1) % len(opts)
	def.set(m.settings, opts[next])
	m.refreshSettingsItems()
}

func (m *Model) handleSettingsMouse(msg mouseMsg) (tea.Model, tea.Cmd) {
	if isLeftClick(msg) {
		idx := listClickIndex(m.settingsList, msg.Y, 0)
		if idx >= 0 && idx < len(m.settingsList.Items()) {
			if idx == m.settingsList.Index() {
				it, ok := m.settingsList.SelectedItem().(settingItem)
				if !ok {
					return m, nil
				}
				switch it.def.kind {
				case settingPicker:
					if it.def.onPick != nil {
						it.def.onPick(m)
					}
				case settingSelect:
					m.cycleOption(it.def)
				}
				return m, nil
			}
			m.settingsList.Select(idx)
		}
		return m, nil
	}
	if isWheelUp(msg) || isWheelDown(msg) {
		idx := m.settingsList.Index()
		if isWheelUp(msg) {
			if idx > 0 {
				m.settingsList.Select(idx - 3)
			}
		} else {
			total := len(m.settingsList.Items())
			if idx < total-1 {
				next := idx + 3
				if next >= total {
					next = total - 1
				}
				m.settingsList.Select(next)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) settingsView() string {
	var b strings.Builder

	b.WriteString(m.settingsList.View())

	if m.settings != nil && m.settings.editing {
		b.WriteString("\n\n")
		it, ok := m.settingsList.SelectedItem().(settingItem)
		label := "value"
		if ok {
			label = it.def.label
		}
		b.WriteString(fmt.Sprintf("  %s: %s", settingKeyStyle.Render(label), renderInput(m.settings.input)))
	}

	b.WriteString("\n\n")
	b.WriteString(settingHelpStyle.Render("  s: save  esc: back  enter: cycle/edit  e: type value  +/-: adjust"))

	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(m.uiColors.status).Render("  " + m.status))
	}

	return b.String()
}

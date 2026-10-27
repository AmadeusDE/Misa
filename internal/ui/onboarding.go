package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/companion"
	"github.com/AmadeusDE/Misa/internal/config"
)

type onboardingStep int

const (
	obWelcome onboardingStep = iota
	obCompanionMoods
	obCompanionYourPronouns
	obCompanionPronouns
	obCompanionYourTerms
	obCompanionTerms
	obCompanionPrefixes
	obCompanionItems
	obCompanionYourParts
	obCompanionParts
	obCompanionEmotes
	obCompanionYourKemonos
	obCompanionKemonos
	obCompanionModifiers
	obTheme
	obInfo
	obDistro
	obAnime
	obImportHistory
	obHistorySearch
	obStartInShell
	obDisableMinishell
	obMuxBarPosition
	obMuxBarAutoHide
	obDone
)

type onboardingState struct {
	step                  onboardingStep
	cfg                   config.Config
	muxCfg                config.MuxConfig
	companionOnly         bool
	companionOff          bool
	companionMoods        map[string]bool
	companionYourPronouns map[string]bool
	companionPronouns     map[string]bool
	companionYourTerms    map[string]bool
	companionTerms        map[string]bool
	companionPrefixPos    map[string]bool
	companionPrefixNeg    map[string]bool
	companionItems        map[string]bool
	companionYourParts    map[string]bool
	companionParts        map[string]bool
	companionEmotes       map[string]bool
	companionYourKemonos  map[string]bool
	companionKemonos      map[string]bool
	companionModifiers    map[string]bool
	writeTarget           string
	previewing            bool
	previewVP             viewport.Model
	editing               bool
	input                 textinput.Model
	importResult          string
}

var obStepCount = int(obDone)

func obStepNum(s onboardingStep) int {
	return int(s)
}

func (m *Model) enterOnboarding() tea.Cmd {
	ti := textinput.New()
	ti.CharLimit = 256
	m.onboarding = newCompanionOnboarding()
	m.onboarding.step = obWelcome
	m.onboarding.cfg = config.Load()
	m.onboarding.muxCfg = config.LoadMux()
	m.onboarding.input = ti
	m.screen = screenOnboarding
	return nil
}

func (m *Model) updateOnboarding(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.onboarding == nil {
		m.screen = screenMenu
		return m, nil
	}

	if m.onboarding.editing {
		return m.updateOnboardingEdit(msg)
	}
	if m.onboarding.previewing {
		return m.updateCompanionPreview(msg)
	}

	switch msg := msg.(type) {
	case mouseMsg:
		return m.handleOnboardingMouse(msg)
	case tea.KeyPressMsg:
		switch m.onboarding.step {
		case obWelcome:
			switch msg.String() {
			case "enter":
				m.obAdvance()
			case "esc", "q":
				return m, m.finishOnboarding()
			}
			return m, nil

		case obCompanionMoods, obCompanionYourPronouns, obCompanionPronouns, obCompanionYourTerms, obCompanionTerms, obCompanionPrefixes, obCompanionItems, obCompanionYourParts, obCompanionParts, obCompanionEmotes, obCompanionYourKemonos, obCompanionKemonos, obCompanionModifiers:
			switch msg.String() {
			case "p":
				m.showCompanionPreview()
				return m, nil
			case "enter":
				if it, ok := m.mainList.SelectedItem().(menuItem); ok {
					if it.preview != "" {
						m.showCompanionPreview()
						return m, nil
					}
					return m, it.action(m)
				}
			case "space":
				if it, ok := m.mainList.SelectedItem().(menuItem); ok {
					return m, it.action(m)
				}
			case "esc":
				if m.onboarding.step == obCompanionMoods {
					m.abandonCompanionSetup()
					return m, nil
				}
				m.obAdvance()
				return m, nil
			}
			var cmd tea.Cmd
			m.mainList, cmd = m.mainList.Update(msg)
			return m, cmd

		case obTheme, obDistro, obAnime:
			return m, nil

		case obInfo:
			switch msg.String() {
			case "enter":
				m.onboarding.editing = true
				cur := m.onboarding.cfg.Info
				if cur == "" {
					cur = config.DefaultInfo
				}
				m.onboarding.input.SetValue(cur)
				m.onboarding.input.Focus()
				m.onboarding.input.Placeholder = "version,time,user|cwd|ram,swap"
			case "esc", "tab":
				m.obAdvance()
			}
			return m, nil

		case obImportHistory:
			return m.obUpdateImportHistory(msg)

		case obHistorySearch:
			return m.obUpdateToggle(msg, func() bool {
				return config.BoolDefault(m.onboarding.cfg.HistorySearch, true)
			}, func(v bool) {
				m.onboarding.cfg.HistorySearch = config.BoolPtr(v)
			})

		case obStartInShell:
			return m.obUpdateToggle(msg, func() bool {
				return m.onboarding.cfg.StartInShell
			}, func(v bool) {
				m.onboarding.cfg.StartInShell = v
			})

		case obDisableMinishell:
			return m.obUpdateToggle(msg, func() bool {
				return m.onboarding.cfg.DisableMinishell
			}, func(v bool) {
				m.onboarding.cfg.DisableMinishell = v
			})

		case obMuxBarPosition:
			return m.obUpdateSelect(msg, func() string {
				if m.onboarding.muxCfg.BarPosition == "" {
					return "bottom"
				}
				return m.onboarding.muxCfg.BarPosition
			}, func(v string) {
				m.onboarding.muxCfg.BarPosition = v
			}, []string{"bottom", "top"})

		case obMuxBarAutoHide:
			return m.obUpdateToggle(msg, func() bool {
				return config.BoolDefault(m.onboarding.muxCfg.BarAutoHide, true)
			}, func(v bool) {
				m.onboarding.muxCfg.BarAutoHide = config.BoolPtr(v)
			})

		case obDone:
			switch msg.String() {
			case "enter", "esc":
				return m, m.finishOnboarding()
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *Model) updateOnboardingEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(m.onboarding.input.Value())
			m.onboarding.editing = false
			m.onboarding.input.Blur()
			if m.onboarding.writeTarget != "" {
				m.commitCompanionWriteIn(val)
				return m, nil
			}
			m.onboarding.cfg.Info = val
			m.obAdvance()
			return m, nil
		case "esc":
			m.onboarding.editing = false
			m.onboarding.writeTarget = ""
			m.onboarding.input.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.onboarding.input, cmd = m.onboarding.input.Update(msg)
	return m, cmd
}

func (m *Model) obUpdateToggle(msg tea.Msg, get func() bool, set func(bool)) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "left", "right", "h", "l", "space":
		set(!get())
	case "enter", "tab", "esc":
		m.obAdvance()
	}
	return m, nil
}

func (m *Model) obUpdateSelect(msg tea.Msg, get func() string, set func(string), opts []string) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "left", "right", "h", "l", "space":
		cur := get()
		idx := 0
		for i, o := range opts {
			if o == cur {
				idx = i
				break
			}
		}
		next := (idx + 1) % len(opts)
		set(opts[next])
	case "enter", "tab", "esc":
		m.obAdvance()
	}
	return m, nil
}

func (m *Model) handleOnboardingMouse(msg mouseMsg) (tea.Model, tea.Cmd) {
	if m.onboarding == nil {
		return m, nil
	}
	switch m.onboarding.step {
	case obCompanionMoods, obCompanionYourPronouns, obCompanionPronouns, obCompanionYourTerms, obCompanionTerms, obCompanionPrefixes, obCompanionItems, obCompanionYourParts, obCompanionParts, obCompanionEmotes, obCompanionYourKemonos, obCompanionKemonos, obCompanionModifiers:
		if isLeftClick(msg) {
			startY := 6
			idx := listClickIndex(m.mainList, msg.Y, startY)
			if idx >= 0 && idx < len(m.mainList.Items()) {
				if idx == m.mainList.Index() {
					if it, ok := m.mainList.SelectedItem().(menuItem); ok {
						return m, it.action(m)
					}
				}
				m.mainList.Select(idx)
			}
			return m, nil
		}
		if isWheelUp(msg) || isWheelDown(msg) {
			idx := m.mainList.Index()
			if isWheelUp(msg) {
				if idx > 0 {
					m.mainList.Select(idx - 3)
				}
			} else {
				total := len(m.mainList.Items())
				if idx < total-1 {
					next := idx + 3
					if next >= total {
						next = total - 1
					}
					m.mainList.Select(next)
				}
			}
			return m, nil
		}
	case obHistorySearch, obStartInShell, obDisableMinishell, obMuxBarAutoHide:
		if isLeftClick(msg) {
			switch m.onboarding.step {
			case obHistorySearch:
				v := !config.BoolDefault(m.onboarding.cfg.HistorySearch, true)
				m.onboarding.cfg.HistorySearch = config.BoolPtr(v)
			case obStartInShell:
				m.onboarding.cfg.StartInShell = !m.onboarding.cfg.StartInShell
			case obDisableMinishell:
				m.onboarding.cfg.DisableMinishell = !m.onboarding.cfg.DisableMinishell
			case obMuxBarAutoHide:
				v := !config.BoolDefault(m.onboarding.muxCfg.BarAutoHide, true)
				m.onboarding.muxCfg.BarAutoHide = config.BoolPtr(v)
			}
			return m, nil
		}
	case obMuxBarPosition:
		if isLeftClick(msg) {
			cur := m.onboarding.muxCfg.BarPosition
			if cur == "" {
				cur = "bottom"
			}
			if cur == "bottom" {
				m.onboarding.muxCfg.BarPosition = "top"
			} else {
				m.onboarding.muxCfg.BarPosition = "bottom"
			}
			return m, nil
		}
	case obImportHistory:
		if isLeftClick(msg) && m.onboarding.importResult == "" {
			count, err := ImportExternalHistory()
			if err != nil {
				m.onboarding.importResult = "Error: " + err.Error()
			} else if count == 0 {
				m.onboarding.importResult = "No new commands to import."
			} else {
				m.onboarding.importResult = fmt.Sprintf("Imported %d commands.", count)
				m.shell.loadFromDisk()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) obAdvance() {
	next := m.onboarding.step + 1
	m.onboarding.step = next

	m.settings = &settingsState{cfg: m.onboarding.cfg, muxCfg: m.onboarding.muxCfg}

	switch next {
	case obCompanionMoods:
		m.showCompanionStep()
	case obCompanionYourPronouns, obCompanionPronouns, obCompanionYourTerms, obCompanionTerms, obCompanionPrefixes, obCompanionItems, obCompanionEmotes, obCompanionYourKemonos, obCompanionKemonos, obCompanionModifiers:
		if m.onboarding.companionOff || !m.companionSetupStarted() {
			m.skipRemainingCompanion()
			return
		}
		m.showCompanionStep()
	case obCompanionYourParts, obCompanionParts:
		if m.onboarding.companionOff || !m.companionSetupStarted() {
			m.skipRemainingCompanion()
			return
		}
		if !companion.HasThirsty(selectedIDs(m.onboarding.companionMoods)) {
			m.obAdvance()
			return
		}
		m.showCompanionStep()
	case obTheme:
		if m.onboarding.companionOnly {
			m.finishCompanionOnly()
			return
		}
		m.enterThemePicker(func(sel []string) {
			if len(sel) == 0 {
				m.onboarding.cfg.Ascii.Theme = "default"
			} else {
				m.onboarding.cfg.Ascii.Theme = strings.Join(sel, ",")
			}
			m.obAdvance()
		})
	case obDistro:
		m.enterDistroPicker(func(sel []string) {
			m.onboarding.cfg.Ascii.Distro = strings.Join(sel, ",")
			m.obAdvance()
		})
	case obAnime:
		m.enterAnimePicker(func(sel []string) {
			if len(sel) == 0 {
				m.onboarding.cfg.Ascii.Anime = "random"
			} else {
				m.onboarding.cfg.Ascii.Anime = strings.Join(sel, ",")
			}
			m.obAdvance()
		})
	case obDone:
		var errs []string
		if err := config.Save(m.onboarding.cfg); err != nil {
			errs = append(errs, "config: "+err.Error())
		}
		if err := config.SaveMux(m.onboarding.muxCfg); err != nil {
			errs = append(errs, "mux: "+err.Error())
		}
		if len(errs) > 0 {
			m.status = "error saving: " + strings.Join(errs, "; ")
		} else {
			m.status = "config saved"
		}
		m.applyCompanionSelection()
		m.screen = screenOnboarding
	default:
		m.screen = screenOnboarding
	}
}

func (m *Model) finishOnboarding() tea.Cmd {
	ob := m.onboarding
	m.onboarding = nil
	m.settings = nil

	var cmd tea.Cmd
	if ob != nil {
		cfg := config.Load()
		muxCfg := config.LoadMux()
		m.startInShell = cfg.StartInShell
		m.disableMinishell = cfg.DisableMinishell
		m.historySearch = config.BoolDefault(cfg.HistorySearch, true)
		m.applyMuxConfig(muxCfg)
		cmd = muxConfigSaved(muxCfg)
		infoStr := cfg.Statusbar.Info
		if infoStr == "" {
			infoStr = cfg.Info
		}
		m.infoItems = parseInfoItems(infoStr)
		m.menuConf = cfg.Menu
	}

	items := buildRootItems(m.dir, m.menuConf)
	m.mainList.SetItems(items)
	m.mainList.Title = ""
	m.mainList.Select(0)

	if m.startInShell {
		m.enterFullShell()
	} else {
		m.screen = screenMenu
		m.resize()
	}
	return cmd
}

var (
	obTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d3869b")).
			Bold(true)
	obTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a89984"))
	obHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#b8bb26")).
				Bold(true)
	obActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#83a598")).
			Bold(true).
			Padding(0, 2)
	obInactiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#665c54")).
			Padding(0, 2)
	obProgressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#665c54"))
)

func (m *Model) onboardingContent() string {
	if m.onboarding == nil {
		return ""
	}

	switch m.onboarding.step {
	case obWelcome:
		return m.obWelcomeView()
	case obCompanionMoods, obCompanionYourPronouns, obCompanionPronouns, obCompanionYourTerms, obCompanionTerms, obCompanionPrefixes, obCompanionItems, obCompanionYourParts, obCompanionParts, obCompanionEmotes, obCompanionYourKemonos, obCompanionKemonos, obCompanionModifiers:
		return m.obListStepView(companionStepTitle(m.onboarding.step), companionStepDesc(m.onboarding.step))
	case obTheme:
		return m.obProgress() + "\n\n" +
			obTitleStyle.Render("Color Theme") + "\n\n" +
			obTextStyle.Render("Opening theme picker...")
	case obInfo:
		return m.obInfoView()
	case obDistro:
		return m.obProgress() + "\n\n" +
			obTitleStyle.Render("Distro Art") + "\n\n" +
			obTextStyle.Render("Opening distro art picker...")
	case obAnime:
		return m.obProgress() + "\n\n" +
			obTitleStyle.Render("Anime Art") + "\n\n" +
			obTextStyle.Render("Opening anime art picker...")
	case obImportHistory:
		return m.obImportHistoryView()
	case obHistorySearch:
		return m.obToggleView("History Search",
			"When typing in the shell, up-arrow filters history by what you've typed.\nTurn off for bash-style cycling through all history.",
			config.BoolDefault(m.onboarding.cfg.HistorySearch, true))
	case obStartInShell:
		return m.obToggleView("Start in Shell",
			"Skip the menu and go straight to the shell when Misa starts.",
			m.onboarding.cfg.StartInShell)
	case obDisableMinishell:
		return m.obToggleView("Disable Minishell",
			"Hide the mini shell input at the bottom of the menu screen.",
			m.onboarding.cfg.DisableMinishell)
	case obMuxBarPosition:
		pos := m.onboarding.muxCfg.BarPosition
		if pos == "" {
			pos = "bottom"
		}
		return m.obSelectView("Mux Bar Position",
			"Where to show the multiplexer status bar.",
			[]string{"bottom", "top"}, pos)
	case obMuxBarAutoHide:
		return m.obToggleView("Mux Bar Auto-Hide",
			"Only show the mux status bar when more than one pane is open.",
			config.BoolDefault(m.onboarding.muxCfg.BarAutoHide, true))
	case obDone:
		return m.obDoneView()
	}

	return ""
}

func (m *Model) obProgress() string {
	step := obStepNum(m.onboarding.step)
	total := obStepCount
	filled := step
	empty := total - step
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return obProgressStyle.Render(bar + "  " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#a89984")).Bold(true).Render(stepName(m.onboarding.step)))
}

func stepName(s onboardingStep) string {
	switch s {
	case obWelcome:
		return "Welcome"
	case obCompanionMoods:
		return "Moods"
	case obCompanionYourPronouns:
		return "Your Pronouns"
	case obCompanionPronouns:
		return "Companion Pronouns"
	case obCompanionYourTerms:
		return "What They Call You"
	case obCompanionTerms:
		return "Companion Names"
	case obCompanionPrefixes:
		return "Call Prefixes"
	case obCompanionItems:
		return "Items"
	case obCompanionYourParts:
		return "Your Parts"
	case obCompanionParts:
		return "Companion Parts"
	case obCompanionEmotes:
		return "Emotes"
	case obCompanionYourKemonos:
		return "Your Kemonos"
	case obCompanionKemonos:
		return "Companion Kemonos"
	case obCompanionModifiers:
		return "Modifiers"
	case obTheme:
		return "Color Theme"
	case obInfo:
		return "Status Bar Items"
	case obDistro:
		return "Distro Art"
	case obAnime:
		return "Anime Art"
	case obImportHistory:
		return "Import History"
	case obHistorySearch:
		return "History Search"
	case obStartInShell:
		return "Start in Shell"
	case obDisableMinishell:
		return "Minishell"
	case obMuxBarPosition:
		return "Mux Bar Position"
	case obMuxBarAutoHide:
		return "Mux Bar Auto-Hide"
	case obDone:
		return "Done"
	}
	return ""
}

func (m *Model) obWelcomeView() string {
	return obTitleStyle.Render("Welcome to Misa!") + "\n\n" +
		obTextStyle.Render("Let's walk through your setup, one step at a time.") + "\n" +
		obTextStyle.Render("Each setting can be changed later from the Settings menu.") + "\n\n" +
		obHighlightStyle.Render("  Steps:") + "\n" +
		obTextStyle.Render("   Companion  ·  Color themes  ·  Status bar items") + "\n" +
		obTextStyle.Render("   Distro art  ·  Anime art  ·  History import") + "\n" +
		obTextStyle.Render("   Shell behavior  ·  Mux bar settings") + "\n\n" +
		settingHelpStyle.Render("  enter: start  esc: skip setup")
}

func (m *Model) obListStepView(title, desc string) string {
	if m.onboarding != nil && m.onboarding.previewing {
		return m.companionPreviewView()
	}
	var b strings.Builder
	b.WriteString(m.obProgress() + "\n\n")
	b.WriteString(obTitleStyle.Render(title) + "\n\n")
	for _, line := range strings.Split(desc, "\n") {
		b.WriteString(obTextStyle.Render(line) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(m.mainList.View() + "\n\n")
	if m.onboarding.editing && m.onboarding.writeTarget != "" {
		b.WriteString(settingKeyStyle.Render("Write-in: ") + renderInput(m.onboarding.input) + "\n\n")
		b.WriteString(settingHelpStyle.Render("  enter: add  esc: cancel"))
		return b.String()
	}
	help := "  enter: select  esc: skip"
	if isCompanionListStep(m.onboarding.step) {
		help = "  space: toggle  enter/p: preview  esc: skip"
	}
	b.WriteString(settingHelpStyle.Render(help))
	return b.String()
}

func (m *Model) obInfoView() string {
	cur := m.onboarding.cfg.Info
	if cur == "" {
		cur = config.DefaultInfo
	}

	var b strings.Builder
	b.WriteString(m.obProgress() + "\n\n")
	b.WriteString(obTitleStyle.Render("Status Bar Items") + "\n\n")
	b.WriteString(obTextStyle.Render("Configure which info items appear in the status bar.") + "\n")
	b.WriteString(obTextStyle.Render("Comma-separated items, pipe (|) for line breaks.") + "\n\n")
	b.WriteString(obTextStyle.Render("Available: version, time, terminal, user, cwd, ram, swap, uptime, kernel, git, localip") + "\n\n")
	b.WriteString(settingKeyStyle.Render("Current: "))
	if m.onboarding.editing {
		b.WriteString(renderInput(m.onboarding.input))
	} else {
		b.WriteString(settingValStyle.Render(cur))
	}
	b.WriteString("\n\n")
	if m.onboarding.editing {
		b.WriteString(settingHelpStyle.Render("  enter: confirm  esc: cancel"))
	} else {
		b.WriteString(settingHelpStyle.Render("  enter: edit  tab/esc: skip"))
	}
	return b.String()
}

func (m *Model) obToggleView(title, desc string, val bool) string {
	var b strings.Builder
	b.WriteString(m.obProgress() + "\n\n")
	b.WriteString(obTitleStyle.Render(title) + "\n\n")
	for _, line := range strings.Split(desc, "\n") {
		b.WriteString(obTextStyle.Render(line) + "\n")
	}
	b.WriteString("\n")

	onStyle, offStyle := obInactiveStyle, obInactiveStyle
	if val {
		onStyle = obActiveStyle
	} else {
		offStyle = obActiveStyle
	}
	b.WriteString("  " + onStyle.Render("ON") + "  " + offStyle.Render("OFF") + "\n\n")
	b.WriteString(settingHelpStyle.Render("  ←/→: toggle  enter: next  esc: skip"))
	return b.String()
}

func (m *Model) obSelectView(title, desc string, opts []string, cur string) string {
	var b strings.Builder
	b.WriteString(m.obProgress() + "\n\n")
	b.WriteString(obTitleStyle.Render(title) + "\n\n")
	for _, line := range strings.Split(desc, "\n") {
		b.WriteString(obTextStyle.Render(line) + "\n")
	}
	b.WriteString("\n  ")
	for _, o := range opts {
		style := obInactiveStyle
		if o == cur {
			style = obActiveStyle
		}
		b.WriteString(style.Render(strings.ToUpper(o)) + "  ")
	}
	b.WriteString("\n\n")
	b.WriteString(settingHelpStyle.Render("  ←/→: switch  enter: next  esc: skip"))
	return b.String()
}

func (m *Model) obImportHistoryView() string {
	var b strings.Builder
	b.WriteString(m.obProgress() + "\n\n")
	b.WriteString(obTitleStyle.Render("Import Shell History") + "\n\n")
	b.WriteString(obTextStyle.Render("Import your existing bash and zsh history into misa.") + "\n")
	b.WriteString(obTextStyle.Render("This reads from ~/.bash_history and ~/.zsh_history.") + "\n")
	b.WriteString(obTextStyle.Render("Your original history files are never modified.") + "\n\n")

	if m.onboarding.importResult != "" {
		b.WriteString(obHighlightStyle.Render("  "+m.onboarding.importResult) + "\n\n")
		b.WriteString(settingHelpStyle.Render("  enter: continue"))
	} else {
		b.WriteString(settingHelpStyle.Render("  enter: import  esc: skip"))
	}
	return b.String()
}

func (m *Model) obUpdateImportHistory(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "enter":
		if m.onboarding.importResult != "" {
			m.obAdvance()
			return m, nil
		}
		count, err := ImportExternalHistory()
		if err != nil {
			m.onboarding.importResult = "Error: " + err.Error()
		} else if count == 0 {
			m.onboarding.importResult = "No new commands to import."
		} else {
			m.onboarding.importResult = fmt.Sprintf("Imported %d commands.", count)
			m.shell.loadFromDisk()
		}
	case "esc", "tab":
		m.obAdvance()
	}
	return m, nil
}

func (m *Model) obDoneView() string {
	return m.obProgress() + "\n\n" +
		obTitleStyle.Render("Setup Complete!") + "\n\n" +
		obTextStyle.Render("Your configuration has been saved.") + "\n" +
		obTextStyle.Render("You can change any of these settings from the Settings menu.") + "\n\n" +
		settingHelpStyle.Render("  enter: start using misa")
}

func needsOnboarding() bool {
	return config.ActivePath() == ""
}

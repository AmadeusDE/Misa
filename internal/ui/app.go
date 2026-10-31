package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/ascii"
	"github.com/AmadeusDE/Misa/internal/battleship"
	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/color"
	"github.com/AmadeusDE/Misa/internal/companion"
	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/gradle"
	"github.com/AmadeusDE/Misa/internal/jobleader"
	"github.com/AmadeusDE/Misa/internal/justfile"
	"github.com/AmadeusDE/Misa/internal/launcher"
	mkf "github.com/AmadeusDE/Misa/internal/makefile"
	"github.com/AmadeusDE/Misa/internal/mines"
	"github.com/AmadeusDE/Misa/internal/pkgmanager"
	"github.com/AmadeusDE/Misa/internal/procmon"
	"github.com/AmadeusDE/Misa/internal/sysinfo"
	"github.com/AmadeusDE/Misa/internal/term"
	"github.com/AmadeusDE/Misa/internal/thefuck"
	"github.com/AmadeusDE/Misa/internal/vi"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type screen int

const (
	screenMenu screen = iota
	screenFileBrowser
	screenNewFile
	screenConfirmDelete
	screenSettings
	screenAbout
	screenHelp
	screenPicker
	screenOnboarding
	screenShell
	screenEditor
	screenProcMon
)

type menuState struct {
	title      string
	items      []list.Item
	index      int
	filterable bool
}

type Model struct {
	dir      string
	screen   screen
	menuConf []config.Entry

	mainList     list.Model
	fileList     list.Model
	settingsList list.Model
	menuStack    []menuState
	cmdInput     textinput.Model
	fileInput    textinput.Model
	settings     *settingsState
	picker       *pickerState
	onboarding   *onboardingState
	aboutView    viewport.Model
	shellView    viewport.Model
	fullShellVP  viewport.Model
	helpView     viewport.Model

	distroArt    []string
	animeArt     []string
	infoItems    []string
	statusColors statusColors
	uiColors     uiColors

	pendingDelete string
	shellBuffer   []string
	shell         shellState
	status        string
	inputMode     bool
	lastExitCode  int
	thefuckFix    string

	viEditor     *vi.Editor
	editorReturn screen
	viRegister   []string
	viRegLine    bool

	procViewer    procmon.Viewer
	procMonReturn screen

	mux     muxState
	extInfo sysinfo.ExtendedInfo

	terminal    *term.Terminal
	cursorBlink bool
	copyMode    copyState
	sel         textSel
	aliases     map[string]string
	autorun     []string
	builtinMode string

	startInShell         bool
	disableMinishell     bool
	historySearch        bool
	muxAutoPane          bool
	muxBarAutoHide       bool
	muxBarPosition       string
	muxBarInfo           []string
	muxFocusFollowsMouse bool
	muxBarRight          string
	muxBarReady          bool
	frame                muxFrame
	jobs                 []*job
	jobControl           bool
	session              *agentSession
	detached             []*agentSession
	nestedMode           bool
	menuLaunch           bool
	wantsClose           bool

	width, height int
}

func New(dir string) *Model {
	delegate := compactDelegate{}
	cfg := config.Load()

	items := buildRootItems(dir, cfg.Menu)

	mainList := list.New(items, delegate, 0, 0)
	mainList.SetShowTitle(false)
	mainList.SetShowStatusBar(false)
	mainList.SetFilteringEnabled(false)
	mainList.DisableQuitKeybindings()

	fileList := list.New(nil, delegate, 0, 0)
	fileList.AdditionalShortHelpKeys = fileHelpKeys
	fileList.AdditionalFullHelpKeys = fileHelpKeys

	settingsList := list.New(nil, delegate, 0, 0)
	settingsList.SetShowTitle(false)
	settingsList.SetShowStatusBar(false)
	settingsList.SetFilteringEnabled(false)
	settingsList.SetShowHelp(false)
	settingsList.SetShowPagination(false)
	settingsList.DisableQuitKeybindings()

	fileList.DisableQuitKeybindings()

	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "enter a command..."
	ti.CharLimit = 256

	fi := textinput.New()
	fi.CharLimit = 256

	info := sysinfo.Fetch(dir)
	palettes := color.LoadPalettes(cfg.Ascii.PalettesFile)

	distroArtObj := ascii.LoadDistroArt(info.Distro, cfg.Ascii.Distro)
	theme := cfg.Ascii.Theme
	if theme == "" {
		theme = "default"
	}
	colorMode := cfg.Ascii.ColorMode
	if colorMode == "" {
		colorMode = "line"
	}
	distroLines := ascii.Colorize(distroArtObj, theme, colorMode, palettes)

	animeLines := ascii.LoadAnimeArt(cfg.Ascii.Anime)

	maxH := cfg.Ascii.MaxHeight
	if maxH == 0 {
		maxH = 16
	}
	distroLines = ascii.CropHeight(distroLines, maxH)
	animeLines = ascii.CropHeight(animeLines, maxH)

	aboutVP := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	shellVP := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	helpVP := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))

	infoStr := cfg.Statusbar.Info
	if infoStr == "" {
		infoStr = cfg.Info
	}

	muxCfg := config.LoadMux()

	m := &Model{
		dir:              dir,
		menuConf:         cfg.Menu,
		mainList:         mainList,
		fileList:         fileList,
		settingsList:     settingsList,
		cmdInput:         ti,
		fileInput:        fi,
		aboutView:        aboutVP,
		shellView:        shellVP,
		helpView:         helpVP,
		distroArt:        distroLines,
		animeArt:         animeLines,
		infoItems:        parseInfoItems(infoStr),
		statusColors:     loadStatusColors(cfg.Colors),
		uiColors:         loadUIColors(cfg.UI),
		extInfo:          sysinfo.FetchExtended(),
		aliases:          cfg.Aliases,
		autorun:          cfg.Autorun,
		builtinMode:      cfg.BuiltinMode,
		jobControl:       jobleader.Supported && config.BoolDefault(cfg.JobControl, true),
		startInShell:     cfg.StartInShell,
		disableMinishell: cfg.DisableMinishell,
		historySearch:    config.BoolDefault(cfg.HistorySearch, true),
	}
	m.applyMuxConfig(muxCfg)
	m.shell.loadFromDisk()
	m.shell.searchHistory = m.historySearch
	m.initMux()
	return m
}

func parseMuxBarInfo(s string) []string {
	if s == "" {
		return []string{"version", "user", "ram", "uptime", "time"}
	}
	var items []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func expandEnv(s string) string {
	return os.ExpandEnv(s)
}

func parseInfoItems(info string) []string {
	if info == "" {
		info = config.DefaultInfo
	}
	var items []string
	for _, group := range strings.Split(info, "|") {
		if len(items) > 0 {
			items = append(items, "|")
		}
		for _, s := range strings.Split(group, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				items = append(items, s)
			}
		}
	}
	if len(items) == 0 {
		return strings.Split(config.DefaultInfo, ",")
	}
	return items
}

func buildMenuItems(entries []config.Entry) []list.Item {
	items := make([]list.Item, 0, len(entries))
	for _, e := range entries {
		e := e
		if len(e.Items) > 0 {
			items = append(items, menuItem{
				title: e.Title,
				desc:  e.Desc,
				action: func(m *Model) tea.Cmd {
					m.pushMenu(e.Title, buildMenuItems(e.Items))
					return nil
				},
			})
		} else if e.Cmd != "" {
			items = append(items, menuItem{
				title: e.Title,
				desc:  e.Desc,
				action: func(m *Model) tea.Cmd {
					return m.menuCommand(e.Cmd)
				},
			})
		} else {
			items = append(items, menuItem{
				title:  e.Title,
				desc:   e.Desc,
				action: func(m *Model) tea.Cmd { return nil },
			})
		}
	}
	return items
}

func buildRootItems(dir string, entries []config.Entry) []list.Item {
	items := buildMenuItems(entries)

	if justfile.Present(dir) {
		var recipeItems []list.Item
		if recipes, err := justfile.Recipes(dir); err == nil {
			for _, r := range recipes {
				r := r
				recipeItems = append(recipeItems, menuItem{
					title: r,
					desc:  "Run just recipe",
					action: func(m *Model) tea.Cmd {
						return m.launch("just", r)
					},
				})
			}
		}
		if len(recipeItems) > 0 {
			items = append(items, menuItem{
				title: "Justfile",
				desc:  "Run just recipes",
				action: func(m *Model) tea.Cmd {
					m.pushMenu("Justfile", recipeItems)
					return nil
				},
			})
		}
	}

	if gradle.Present(dir) {
		items = append(items, menuItem{
			title: "Gradle",
			desc:  "Run gradle tasks",
			action: func(m *Model) tea.Cmd {
				var taskItems []list.Item
				if tasks, err := gradle.Tasks(m.dir); err == nil {
					for _, t := range tasks {
						t := t
						taskItems = append(taskItems, menuItem{
							title: t,
							desc:  "Run gradle task",
							action: func(m *Model) tea.Cmd {
								w := filepath.Join(m.dir, "gradlew")
								if _, err := os.Stat(w); err != nil {
									return m.launch("gradle", t)
								}
								return m.launch(w, t)
							},
						})
					}
				}
				if len(taskItems) == 0 {
					m.status = "no gradle tasks found"
					return nil
				}
				m.pushMenu("Gradle", taskItems)
				return nil
			},
		})
	}

	if mkf.Present(dir) {
		targets := mkf.Targets(dir)
		if len(targets) > 0 {
			var makeItems []list.Item
			for _, t := range targets {
				t := t
				makeItems = append(makeItems, menuItem{
					title: t,
					desc:  "Run make target",
					action: func(m *Model) tea.Cmd {
						return m.launch("make", t)
					},
				})
			}
			items = append(items, menuItem{
				title: "Makefile",
				desc:  "Run make targets",
				action: func(m *Model) tea.Cmd {
					m.pushMenu("Makefile", makeItems)
					return nil
				},
			})
		}
	}

	for _, mgr := range pkgmanager.All() {
		mgr := mgr
		if mgr.Available() {
			items = append(items, menuItem{
				title: mgr.Name,
				desc:  mgr.Desc,
				action: func(m *Model) tea.Cmd {
					m.pushMenu(mgr.Name, buildManagerItems(mgr))
					return nil
				},
			})
		}
	}

	items = append(items,
		menuItem{
			title: "Performance",
			desc:  "System monitoring and task management",
			action: func(m *Model) tea.Cmd {
				perfItems := []list.Item{
					menuItem{
						title: "Task Manager",
						desc:  "Unified monitor with htop, btop, and overview tabs",
						action: func(m *Model) tea.Cmd {
							m.enterProcMon(procmon.NewTM())
							return m.procViewer.Init()
						},
					},
					menuItem{
						title: "htop",
						desc:  "Interactive process viewer with kill, search, filter",
						action: func(m *Model) tea.Cmd {
							m.enterProcMon(procmon.NewHtop())
							return m.procViewer.Init()
						},
					},
					menuItem{
						title: "btop",
						desc:  "Resource monitor with CPU, memory, disk, network panels",
						action: func(m *Model) tea.Cmd {
							m.enterProcMon(procmon.NewBtop())
							return m.procViewer.Init()
						},
					},
					menuItem{
						title: "top",
						desc:  "Classic process overview",
						action: func(m *Model) tea.Cmd {
							m.enterProcMon(procmon.NewTop())
							return m.procViewer.Init()
						},
					},
				}
				m.pushMenu("Performance", perfItems)
				return nil
			},
		},
		menuItem{
			title: "Games",
			desc:  "Battleship, Minesweeper and Chess",
			action: func(m *Model) tea.Cmd {
				gamesItems := []list.Item{
					menuItem{
						title: "Battleship",
						desc:  "Sink the computer's fleet on a 10×10 grid",
						action: func(m *Model) tea.Cmd {
							m.enterProcMon(battleship.New(battleship.Options{}))
							return m.procViewer.Init()
						},
					},
					menuItem{
						title: "Mines",
						desc:  "Minesweeper, beginner 9×9",
						action: func(m *Model) tea.Cmd {
							m.enterProcMon(mines.New(mines.Levels[0]))
							return m.procViewer.Init()
						},
					},
					menuItem{
						title: "Chess",
						desc:  "Chess against the computer",
						action: func(m *Model) tea.Cmd {
							game, err := builtins.ChessGame(nil)
							if err != nil {
								m.status = err.Error()
								return nil
							}
							m.enterProcMon(game)
							return m.procViewer.Init()
						},
					},
				}
				m.pushMenu("Games", gamesItems)
				return nil
			},
		},
		menuItem{
			title: "TLDR pages",
			desc:  "Quick command references",
			action: func(m *Model) tea.Cmd {
				return m.enterTldrMenu()
			},
		},
		menuItem{
			title: "Man pages",
			desc:  "Full manual pages",
			action: func(m *Model) tea.Cmd {
				return m.enterManMenu()
			},
		},
		menuItem{
			title: "Fullscreen shell",
			desc:  "Expanded shell with full output history",
			action: func(m *Model) tea.Cmd {
				m.enterFullShell()
				return nil
			},
		},
		menuItem{
			title: "Browse files",
			desc:  "Navigate, open, create, and delete files",
			action: func(m *Model) tea.Cmd {
				return m.enterFileBrowser()
			},
		},
		menuItem{
			title: "Settings",
			desc:  "Configure misa",
			action: func(m *Model) tea.Cmd {
				return m.enterSettings()
			},
		},
		menuItem{
			title: "System Information",
			desc:  "Additional system information",
			action: func(m *Model) tea.Cmd {
				return m.enterAbout()
			},
		},
	)

	return items
}

func (m *Model) refreshRootMenu() {
	items := buildRootItems(m.dir, m.menuConf)
	m.mainList.SetItems(items)
}

func (m *Model) pushMenu(title string, items []list.Item) {
	m.menuStack = append(m.menuStack, menuState{
		title:      title,
		items:      m.mainList.Items(),
		index:      m.mainList.Index(),
		filterable: m.mainList.FilteringEnabled(),
	})
	m.mainList.SetItems(items)
	m.mainList.Select(0)
}

func (m *Model) pushFilterableMenu(title string, items []list.Item) {
	m.pushMenu(title, items)
	m.mainList.SetFilteringEnabled(true)
}

func (m *Model) popMenu() {
	if len(m.menuStack) == 0 {
		return
	}
	prev := m.menuStack[len(m.menuStack)-1]
	m.menuStack = m.menuStack[:len(m.menuStack)-1]
	m.mainList.SetItems(prev.items)
	m.mainList.Select(prev.index)
	m.mainList.SetFilteringEnabled(prev.filterable)
}

func (m *Model) atRoot() bool {
	return len(m.menuStack) == 0
}

func (m *Model) Init() tea.Cmd {
	if m.nestedMode {
		return tea.Batch(m.startNested(), tickCmd(), cursorBlinkCmd())
	}
	var start tea.Cmd
	if m.muxAutoPane {
		start = m.addMuxPane()
	}
	return tea.Batch(start, tickCmd(), cursorBlinkCmd(), requestHostColors())
}

// startNested opens the first screen and runs autorun. Ticks are left to the
// caller: the mux root forwards its own ticks to every pane.
func (m *Model) startNested() tea.Cmd {
	if needsOnboarding() {
		m.enterOnboarding()
	} else if m.startInShell {
		m.enterFullShell()
	}
	var cmds []tea.Cmd
	for _, cmd := range m.autorun {
		m.shellBuffer = append(m.shellBuffer, m.renderPrompt()+cmd)
		expanded := m.expandAlias(cmd)
		if handled, c := m.handleBuiltinCmd(expanded); handled {
			cmds = append(cmds, c)
		} else {
			cmds = append(cmds, runCaptured(m.dir, m.planCommand(expanded)))
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) updateMuxRoot(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a, inner, mail, ok := agentMsgOf(msg); ok {
		return m.routeAgentMsg(a, inner, mail)
	}
	if recordHostColor(msg) {
		for _, p := range m.allMuxPanes() {
			if p.misa != nil {
				p.misa.sendAgentMail(msg)
			}
		}
		return m, nil
	}
	if m.mux.ctrlWPending {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			m.mux.ctrlWPending = false
			return m.handleMuxCommand(keyMsg.String())
		}
	}

	if m.mux.resizeMode {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleResizeMode(keyMsg.String())
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tickMsg:
		m.muxBarReady = false
		for _, p := range m.mux.panes {
			if p.kind == paneMisa && p.misa != nil {
				p.misa.Update(msg)
			}
		}
		return m, tickCmd()

	case cursorBlinkMsg:
		m.cursorBlink = !m.cursorBlink
		for _, p := range m.mux.panes {
			if p.kind == paneMisa && p.misa != nil {
				p.misa.cursorBlink = m.cursorBlink
				if p.misa.terminal != nil && !p.misa.terminal.InAltScreen() && p.misa.screen == screenShell {
					p.misa.updateFullShellVPWithTerm()
				}
			}
		}
		return m, cursorBlinkCmd()

	case termData:
		if m.ownsTerminal(msg.t) {
			if msg.t.Stopped() {
				msg.t.Continue() // shell panes have no job control to return to
			}
			return m, waitTermUpdate(msg.t)
		}
		return m, nil

	case termExited:
		m.handleTermExit(msg.t, msg.exitCode)
		return m, nil

	case paneMsg:
		if c, ok := msg.msg.(muxConfigChanged); ok {
			return m.Update(c)
		}
		return m.updateOwnedPane(msg)

	case muxConfigChanged:
		m.applyMuxConfig(msg.cfg)
		m.resize()
		return m, nil

	case launcher.Finished:
		if p := m.activePane(); p != nil {
			return m.updateSecondaryPane(msg)
		}
		return m, nil

	case shellOutput:
		if p := m.activePane(); p != nil {
			return m.updateSecondaryPane(msg)
		}
		return m, nil

	case tldrReady:
		if p := m.activePane(); p != nil {
			return m.updateSecondaryPane(msg)
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMuxMouse(fromTea(msg))

	case tea.PasteMsg:
		return m.updateSecondaryPane(msg)

	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+w" {
			m.mux.ctrlWPending = true
			return m, nil
		}

		if handled, ret, cmd := m.handleGlobalMuxKey(key); handled {
			return ret, cmd
		}

		if p := m.activePane(); p != nil {
			return m.updateSecondaryPane(msg)
		}

		switch key {
		case "ctrl+c", "q", "esc":
			if !m.anyTagHasPanes() {
				return m, tea.Quit
			}
		}
		return m, nil

	default:
		if p := m.activePane(); p != nil && p.kind == paneMisa && p.misa != nil {
			return m.updateMuxMisa(p, msg)
		}
	}

	return m, nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.nestedMode {
		focus := m.mux.activeIdx
		model, cmd := m.updateMuxRoot(msg)
		m.frame.reuse = isInertHover(msg) && m.mux.activeIdx == focus && !m.muxSelectionActive()
		return model, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case agentMail:
		return m.updateAgentMsg(msg.a, msg.msg, true)

	case agentResult:
		return m.updateAgentMsg(msg.a, msg.msg, false)

	case tickMsg:
		return m, tickCmd()

	case cursorBlinkMsg:
		m.cursorBlink = !m.cursorBlink
		if m.terminal != nil && !m.terminal.InAltScreen() && m.screen == screenShell {
			m.updateFullShellVPWithTerm()
		}
		return m, cursorBlinkCmd()

	case termData:
		if m.terminal != nil && msg.t == m.terminal {
			if m.terminal.Stopped() {
				m.suspendForeground()
				return m, waitTermUpdate(msg.t)
			}
			if m.screen == screenShell && !m.terminal.InAltScreen() {
				m.updateFullShellVPWithTerm()
			}
			return m, waitTermUpdate(m.terminal)
		}
		if m.jobFor(msg.t) != nil {
			return m, waitTermUpdate(msg.t)
		}
		return m, nil

	case termExited:
		if j := m.jobFor(msg.t); j != nil {
			m.finishJob(j, msg.exitCode)
			return m, nil
		}
		if m.terminal != nil && msg.t == m.terminal {
			if m.menuLaunch {
				m.finishMenuLaunch(msg.exitCode)
				return m, nil
			}
			termLines := m.terminal.Lines()
			m.shellBuffer = append(m.shellBuffer, termLines...)
			m.terminal.Close()
			m.terminal = nil
			m.resize()
			m.lastExitCode = msg.exitCode
			if resp := companion.Respond(msg.exitCode); resp != "" {
				ps := lipgloss.NewStyle().Foreground(m.uiColors.prompt).Bold(true)
				for _, line := range strings.Split(resp, "\n") {
					m.shellBuffer = append(m.shellBuffer, ps.Render(line))
				}
			}
			m.thefuckFix = ""
			if msg.exitCode != 0 && m.shell.lastCmd != "" {
				output := strings.Join(termLines, "\n")
				if fix := thefuck.Match(m.shell.lastCmd, output, msg.exitCode); fix != "" {
					m.thefuckFix = fix
					fixStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f")).Bold(true)
					m.shellBuffer = append(m.shellBuffer, fixStyle.Render("fix → ")+fix+fixStyle.Render("  [ctrl+f]"))
				}
			}
			if m.screen == screenShell {
				m.updateFullShellVP()
			} else {
				m.updateShellView()
			}
			m.cmdInput.SetValue("")
		}
		return m, nil

	case launcher.Finished:
		if msg.Err != nil {
			m.status = "error: " + msg.Err.Error()
		}
		if m.screen == screenFileBrowser {
			return m, m.reloadFileList()
		}
		return m, nil

	case shellOutput:
		m.lastExitCode = msg.exitCode
		output := msg.output
		if msg.err != nil && output == "" {
			output = msg.err.Error()
		}
		if output != "" {
			lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
			m.shellBuffer = append(m.shellBuffer, lines...)
		}
		if resp := companion.Respond(msg.exitCode); resp != "" {
			ps := lipgloss.NewStyle().Foreground(m.uiColors.prompt).Bold(true)
			for _, line := range strings.Split(resp, "\n") {
				m.shellBuffer = append(m.shellBuffer, ps.Render(line))
			}
		}
		m.thefuckFix = ""
		if msg.exitCode != 0 && m.shell.lastCmd != "" {
			if fix := thefuck.Match(m.shell.lastCmd, output, msg.exitCode); fix != "" {
				m.thefuckFix = fix
				fixStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f")).Bold(true)
				m.shellBuffer = append(m.shellBuffer, fixStyle.Render("fix → ")+fix+fixStyle.Render("  [ctrl+f]"))
			}
		}
		m.shell.lastOutput = output
		m.updateShellView()
		if m.screen == screenShell {
			m.updateFullShellVP()
		}
		return m, nil

	case tldrReady:
		m.status = ""
		return m, m.enterTldrMenu()

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" && m.screen != screenEditor && m.terminal == nil && m.session == nil {
			m.wantsClose = true
			return m, nil
		}
	}

	switch m.screen {
	case screenFileBrowser:
		return m.updateFileBrowser(msg)
	case screenNewFile:
		return m.updateNewFile(msg)
	case screenConfirmDelete:
		return m.updateConfirmDelete(msg)
	case screenSettings:
		return m.updateSettings(msg)
	case screenAbout:
		return m.updateAbout(msg)
	case screenHelp:
		return m.updateHelp(msg)
	case screenPicker:
		return m.updatePicker(msg)
	case screenOnboarding:
		return m.updateOnboarding(msg)
	case screenShell:
		return m.updateFullShell(msg)
	case screenEditor:
		return m.updateEditor(msg)
	case screenProcMon:
		return m.updateProcMon(msg)
	default:
		return m.updateMenu(msg)
	}
}

func (m *Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mouseMsg, ok := msg.(mouseMsg); ok {
		return m.handleMenuMouse(mouseMsg)
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		if m.inputMode {
			var cmd tea.Cmd
			m.cmdInput, cmd = m.cmdInput.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.mainList, cmd = m.mainList.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "q":
		if !m.inputMode {
			if m.nestedMode {
				m.wantsClose = true
				return m, nil
			}
			return m, tea.Quit
		}

	case "tab":
		if m.inputMode {
			input := m.cmdInput.Value()
			if strings.TrimSpace(input) == "" {
				m.inputMode = false
				m.cmdInput.Blur()
				return m, nil
			}
			completed := m.shell.tabComplete(input, m.dir)
			if completed != input {
				m.cmdInput.SetValue(completed)
				m.cmdInput.SetCursor(len(completed))
			}
			return m, nil
		}
		if m.atRoot() && !m.disableMinishell {
			m.inputMode = true
			m.cmdInput.Focus()
			return m, nil
		}

	case "enter":
		if m.inputMode {
			val := strings.TrimSpace(m.cmdInput.Value())
			if val != "" {
				m.shell.addHistory(val)
				m.shell.resetHistory()
				m.shell.suggestion = ""
				m.thefuckFix = ""
				m.shellBuffer = append(m.shellBuffer, m.renderPrompt()+val)
				expanded := m.expandAlias(val)
				m.shell.lastCmd = expanded

				if native, nativeCmd := stripNativePrefix(expanded); native {
					m.cmdInput.SetValue("")
					m.shellBuffer = append(m.shellBuffer, m.renderPrompt()+"! "+nativeCmd)
					m.updateShellView()
					return m, m.execNative(nativeCmd)
				}

				if handled, cmd := m.handleBuiltinCmd(expanded); handled {
					m.cmdInput.SetValue("")
					m.updateShellView()
					return m, cmd
				}
				m.cmdInput.SetValue("")
				return m, m.execShellCmd(expanded)
			}
			return m, nil
		}
		if it, ok := m.mainList.SelectedItem().(menuItem); ok {
			return m, it.action(m)
		}
		return m, nil

	case "esc":
		if m.inputMode {
			m.inputMode = false
			m.cmdInput.Blur()
			return m, nil
		}
		if !m.atRoot() {
			m.popMenu()
			m.resize()
			return m, nil
		}
		return m, nil

	case "backspace":
		if m.inputMode {
			var cmd tea.Cmd
			m.cmdInput, cmd = m.cmdInput.Update(msg)
			return m, cmd
		}
		if !m.atRoot() {
			m.popMenu()
			m.resize()
			return m, nil
		}
		return m, nil
	}

	if m.inputMode {
		switch keyMsg.String() {
		case "ctrl+f":
			if m.thefuckFix != "" {
				fix := m.thefuckFix
				m.thefuckFix = ""
				m.shell.lastCmd = fix
				m.shellBuffer = append(m.shellBuffer, m.renderPrompt()+fix)
				if handled, cmd := m.handleBuiltinCmd(fix); handled {
					m.cmdInput.SetValue("")
					m.updateShellView()
					return m, cmd
				}
				m.cmdInput.SetValue("")
				return m, m.execShellCmd(fix)
			}
			return m, nil
		case "ctrl+v":
			text := m.registerText()
			if text == "" {
				text = readClipboard()
			}
			if text != "" {
				insertAtCursor(&m.cmdInput, text)
			}
			return m, nil
		case "shift+up", "pgup":
			m.shellView.ScrollUp(3)
			return m, nil
		case "shift+down", "pgdown":
			m.shellView.ScrollDown(3)
			return m, nil
		case "up":
			val := m.shell.historyUp(m.cmdInput.Value())
			m.cmdInput.SetValue(val)
			m.cmdInput.SetCursor(len(val))
			return m, nil
		case "down":
			val := m.shell.historyDown()
			m.cmdInput.SetValue(val)
			m.cmdInput.SetCursor(len(val))
			return m, nil
		case "right":
			// Accept suggestion on right arrow at end of input
			input := m.cmdInput.Value()
			if m.cmdInput.Position() >= utf8.RuneCountInString(input) && m.shell.suggestion != "" {
				accepted := m.shell.acceptSuggestion(input)
				if accepted != input {
					m.cmdInput.SetValue(accepted)
					m.cmdInput.SetCursor(len(accepted))
					return m, nil
				}
			}
		}
		prev := m.cmdInput.Value()
		var cmd tea.Cmd
		m.cmdInput, cmd = m.cmdInput.Update(msg)
		cur := m.cmdInput.Value()
		if cur != prev {
			m.shell.compInput = ""
			m.shell.completions = nil
			m.shell.complete(cur, m.dir)
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.mainList, cmd = m.mainList.Update(msg)
	return m, cmd
}

func (m *Model) menuListStartY() int {
	return m.headerHeight() + 2
}

func (m *Model) handleMenuMouse(msg mouseMsg) (tea.Model, tea.Cmd) {
	if isLeftClick(msg) {
		listStart := m.menuListStartY()
		if m.atRoot() && !m.disableMinishell {
			shellAreaY := listStart + m.mainList.Height()
			if msg.Y >= shellAreaY {
				if !m.inputMode {
					m.inputMode = true
					m.cmdInput.Focus()
				}
				return m, nil
			}
		}
		if m.inputMode && msg.Y < m.menuListStartY()+m.mainList.Height() {
			m.inputMode = false
			m.cmdInput.Blur()
		}
		idx := listClickIndex(m.mainList, msg.Y, listStart)
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
		if m.inputMode {
			if isWheelUp(msg) {
				m.shellView.ScrollUp(3)
			} else {
				m.shellView.ScrollDown(3)
			}
			return m, nil
		}
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

	return m, nil
}

func (m *Model) handleBuiltinCmd(input string) (bool, tea.Cmd) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false, nil
	}
	if fields[0] != "alias" && fields[0] != "unalias" && isCompound(input) {
		return false, nil
	}
	if isJobCommand(fields[0]) {
		return true, m.jobCommand(fields[0], fields[1:])
	}
	if fields[0] == "ugh" {
		if handled, cmd := m.startAgentSession(fields[1:]); handled {
			return true, cmd
		}
	}
	switch fields[0] {
	case "clear":
		m.shellBuffer = nil
		m.updateShellView()
		return true, nil
	case "exit":
		if m.nestedMode {
			m.wantsClose = true
			return true, nil
		}
		return true, tea.Quit
	case "misa":
		return true, nil
	}
	if fields[0] == "cd" {
		target := ""
		if len(fields) < 2 {
			home, err := os.UserHomeDir()
			if err != nil {
				m.shellBuffer = append(m.shellBuffer, "cd: "+err.Error())
				return true, nil
			}
			target = home
		} else {
			target = strings.TrimSpace(input[len("cd"):])
			target = strings.Trim(target, "\"'")
			if target == "~" {
				home, err := os.UserHomeDir()
				if err != nil {
					m.shellBuffer = append(m.shellBuffer, "cd: "+err.Error())
					return true, nil
				}
				target = home
			} else if strings.HasPrefix(target, "~/") || (onWindows && strings.HasPrefix(target, `~\`)) {
				home, err := os.UserHomeDir()
				if err != nil {
					m.shellBuffer = append(m.shellBuffer, "cd: "+err.Error())
					return true, nil
				}
				target = filepath.Join(home, target[2:])
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(m.dir, target)
			}
			target = filepath.Clean(target)
		}
		info, err := os.Stat(target)
		if err != nil {
			m.shellBuffer = append(m.shellBuffer, "cd: "+err.Error())
			return true, nil
		}
		if !info.IsDir() {
			m.shellBuffer = append(m.shellBuffer, "cd: not a directory: "+target)
			return true, nil
		}
		m.dir = target
		m.refreshRootMenu()
		return true, nil
	}

	if fields[0] == "alias" {
		if len(fields) == 1 {
			if len(m.aliases) == 0 {
				m.shellBuffer = append(m.shellBuffer, "no aliases defined")
			} else {
				for k, v := range m.aliases {
					m.shellBuffer = append(m.shellBuffer, fmt.Sprintf("alias %s='%s'", k, v))
				}
			}
			return true, nil
		}
		rest := strings.TrimSpace(input[len("alias"):])
		if eq := strings.IndexByte(rest, '='); eq > 0 {
			name := rest[:eq]
			val := strings.Trim(rest[eq+1:], "'\"")
			if m.aliases == nil {
				m.aliases = make(map[string]string)
			}
			m.aliases[name] = val
			return true, nil
		}
		m.shellBuffer = append(m.shellBuffer, "usage: alias name=value")
		return true, nil
	}

	if fields[0] == "unalias" {
		if len(fields) < 2 {
			m.shellBuffer = append(m.shellBuffer, "usage: unalias name")
			return true, nil
		}
		delete(m.aliases, fields[1])
		return true, nil
	}

	if fields[0] == "vi" || fields[0] == "edit" {
		filename := ""
		if len(fields) > 1 {
			filename = strings.Join(fields[1:], " ")
		}
		m.openInEditor(filename)
		m.cmdInput.SetValue("")
		m.updateShellView()
		return true, nil
	}

	switch fields[0] {
	case "top":
		m.enterProcMon(procmon.NewTop())
		m.cmdInput.SetValue("")
		return true, m.procViewer.Init()
	case "htop":
		m.enterProcMon(procmon.NewHtop())
		m.cmdInput.SetValue("")
		return true, m.procViewer.Init()
	case "btop", "bytop", "bashtop":
		m.enterProcMon(procmon.NewBtop())
		m.cmdInput.SetValue("")
		return true, m.procViewer.Init()
	case "tm":
		m.enterProcMon(procmon.NewTM())
		m.cmdInput.SetValue("")
		return true, m.procViewer.Init()
	case "mines":
		level, err := builtins.MinesLevel(fields[1:])
		if err != nil {
			return false, nil
		}
		m.enterProcMon(mines.New(level))
		m.cmdInput.SetValue("")
		return true, m.procViewer.Init()
	case "chess":
		game, err := builtins.ChessGame(fields[1:])
		if err != nil {
			return false, nil
		}
		m.enterProcMon(game)
		m.cmdInput.SetValue("")
		return true, m.procViewer.Init()
	case "battleship":
		game, err := builtins.BattleshipGame(fields[1:])
		if err != nil {
			return false, nil
		}
		m.enterProcMon(game)
		m.cmdInput.SetValue("")
		return true, m.procViewer.Init()
	}

	return false, nil
}

func (m *Model) renderPrompt() string {
	codeStr := fmt.Sprintf("E[%d]", m.lastExitCode)
	codeColor := m.uiColors.promptOK
	if m.lastExitCode != 0 {
		codeColor = m.uiColors.promptErr
	}

	return lipgloss.NewStyle().Foreground(codeColor).Render(codeStr) + " " +
		lipgloss.NewStyle().Foreground(m.uiColors.promptArrow).Render("➜") + " "
}

func (m *Model) updateShellView() {
	wasAtBottom := m.shellView.AtBottom()
	content := strings.Join(m.shellBuffer, "\n")
	m.shellView.SetContent(content)
	if wasAtBottom {
		m.shellView.GotoBottom()
	}
}

const shellHeight = 8
const bottomPanelHeight = 1 + shellHeight + 1 // divider + output viewport + prompt

func (m *Model) divider() string {
	w := m.mux.primaryW
	if w < 1 {
		w = m.width - 4
	}
	if w < 1 {
		w = 1
	}
	return lipgloss.NewStyle().Foreground(m.uiColors.divider).Render(strings.Repeat("─", w))
}

func (m *Model) resize() {
	m.resizeMux()
	if !m.nestedMode {
		return
	}
	pw, ph := m.mux.primaryW, m.mux.primaryH

	switch m.screen {
	case screenFileBrowser:
		m.fileList.SetSize(pw, ph)
		return
	case screenSettings:
		m.settingsList.SetSize(pw, ph-4)
		return
	case screenAbout:
		m.aboutView.SetWidth(pw)
		m.aboutView.SetHeight(ph - 2)
		return
	case screenHelp:
		m.helpView.SetWidth(pw)
		m.helpView.SetHeight(ph - 2)
		return
	case screenPicker:
		if m.picker != nil {
			m.picker.pList.SetSize(pw, ph-4)
		}
		return
	case screenOnboarding:
		if m.onboarding != nil {
			if m.onboarding.previewing {
				m.onboarding.previewVP.SetWidth(pw)
				m.onboarding.previewVP.SetHeight(ph - 6)
			} else if isCompanionListStep(m.onboarding.step) {
				m.mainList.SetSize(pw, ph-8)
			}
		}
		return
	case screenNewFile, screenConfirmDelete:
		return
	case screenShell:
		m.queueAgentSize()
		atBottom := m.fullShellVP.AtBottom()
		m.fullShellVP.SetWidth(pw)
		m.fullShellVP.SetHeight(ph - 2)
		if m.terminal != nil {
			m.terminal.Resize(ph, pw)
			m.fullShellVP.SetHeight(ph)
			if !m.terminal.InAltScreen() {
				m.updateFullShellVPWithTerm()
			}
		}
		if atBottom {
			m.fullShellVP.GotoBottom()
		}
		return
	case screenEditor:
		if m.viEditor != nil {
			if m.hasSplit() {
				m.viEditor.Resize(pw, ph)
			} else {
				m.viEditor.Resize(m.width, m.height)
			}
		}
		return
	case screenProcMon:
		if m.procViewer != nil {
			if m.hasSplit() {
				m.procViewer.Resize(pw, ph)
			} else {
				m.procViewer.Resize(m.width, m.height)
			}
		}
		return
	}

	reserve := m.headerHeight() + 2
	if m.atRoot() && !m.disableMinishell {
		reserve += bottomPanelHeight
		m.shellView.SetWidth(pw)
		m.shellView.SetHeight(shellHeight)
	}
	m.mainList.SetSize(pw, ph-reserve)
}

// View is the program's frame. Panes compose each other through render.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

func (m *Model) render() string {
	if !m.nestedMode {
		return m.frame.render(m.muxView)
	}
	if m.screen == screenEditor {
		return m.editorView()
	}
	if m.screen == screenProcMon {
		return m.procViewer.View()
	}
	return m.renderPrimaryContent()
}

func (m *Model) renderPrimaryContent() string {
	switch m.screen {
	case screenFileBrowser:
		body := m.fileBrowserView()
		if m.status != "" {
			body += "\n" + lipgloss.NewStyle().Foreground(m.uiColors.status).Render(m.status)
		}
		return body
	case screenNewFile:
		return m.newFileView()
	case screenConfirmDelete:
		return m.confirmDeleteView()
	case screenSettings:
		return m.settingsView()
	case screenAbout:
		return m.aboutView.View()
	case screenHelp:
		return m.helpView.View()
	case screenPicker:
		return m.pickerContent()
	case screenOnboarding:
		return m.onboardingContent()
	case screenShell:
		return m.fullShellContent()
	case screenEditor:
		return m.editorView()
	case screenProcMon:
		return m.procViewer.View()
	}

	header := m.renderHeader()
	div := m.divider()
	listView := m.mainList.View()

	if m.status != "" {
		listView += "\n" + lipgloss.NewStyle().Foreground(m.uiColors.status).Render(m.status)
	}

	if !m.atRoot() || m.disableMinishell {
		return header + "\n" + div + "\n" + listView
	}

	prompt := m.renderPrompt()
	inputView := renderInput(m.cmdInput)
	ghost := ""
	if m.inputMode && m.shell.suggestion != "" {
		input := strings.TrimSpace(m.cmdInput.Value())
		if input != "" && strings.HasPrefix(m.shell.suggestion, input) {
			ghost = lipgloss.NewStyle().Foreground(m.uiColors.ghost).Render(m.shell.suggestion[len(input):])
		}
	}
	bottomContent := m.shellView.View() +
		"\n" + fmt.Sprintf("%s%s%s", prompt, inputView, ghost)

	return header + "\n" + div + "\n" + listView + "\n" + div + "\n" + bottomContent
}

// ownsTerminal reports whether t belongs to one of the mux's own shell panes.
// Nested panes receive their terminal events through routeTo.
func (m *Model) ownsTerminal(t *term.Terminal) bool {
	for _, p := range m.allMuxPanes() {
		if p.terminal == t {
			return true
		}
	}
	return false
}

func (m *Model) handleTermExit(t *term.Terminal, exitCode int) {
	for _, p := range m.allMuxPanes() {
		if p.terminal == t {
			termLines := p.terminal.Lines()
			p.shellBuf = append(p.shellBuf, termLines...)
			p.terminal.Close()
			p.terminal = nil
			p.exitCode = exitCode
			if resp := companion.Respond(exitCode); resp != "" {
				ps := lipgloss.NewStyle().Foreground(m.uiColors.prompt).Bold(true)
				for _, line := range strings.Split(resp, "\n") {
					p.shellBuf = append(p.shellBuf, ps.Render(line))
				}
			}
			m.updateMuxShellVP(p)
			p.input.Focus()
			return
		}
	}
}

func (m *Model) execShellCmd(command string) tea.Cmd {
	return m.execPlan(m.planCommand(command))
}

// execNative runs a `! command` through the system shell, bypassing builtins.
func (m *Model) execNative(command string) tea.Cmd {
	return m.execPlan(shellPlan(command))
}

// launch runs an external program inside this pane's PTY (never suspending the
// mux) and returns to the current screen when it exits.
func (m *Model) launch(argv ...string) tea.Cmd {
	fromMenu := m.screen != screenShell
	cmd := m.execArgv(argv...)
	m.menuLaunch = cmd != nil && fromMenu
	return cmd
}

func (m *Model) finishMenuLaunch(exitCode int) {
	m.terminal.Close()
	m.terminal = nil
	m.menuLaunch = false
	m.lastExitCode = exitCode
	m.exitFullShell()
	if exitCode != 0 {
		m.status = fmt.Sprintf("exited with status %d", exitCode)
	}
}

func (m *Model) execArgv(argv ...string) tea.Cmd {
	return m.execPlan(commandPlan{argv: argv})
}

func (m *Model) execPlan(p commandPlan) tea.Cmd {
	if m.screen != screenShell {
		m.enterFullShell()
	}
	rows, cols := m.mux.primaryH, m.mux.primaryW
	if rows < 2 {
		rows = 24
	}
	if cols < 2 {
		cols = 80
	}
	t, cmd, err := m.startPlan(p, rows, cols)
	if err != nil {
		m.shellBuffer = append(m.shellBuffer, "error: "+err.Error())
		m.updateFullShellVP()
		return nil
	}
	m.terminal = t
	m.resize()
	return cmd
}

// expandAlias replaces the first word only, leaving the rest of the line
// (quoting, spacing) for the shell to parse.
func (m *Model) expandAlias(input string) string {
	trimmed := strings.TrimLeft(input, " \t")
	var name, rest string
	if i := strings.IndexAny(trimmed, " \t"); i >= 0 {
		name, rest = trimmed[:i], trimmed[i:]
	} else {
		name, rest = trimmed, ""
	}
	if expanded, ok := m.aliases[name]; ok {
		return expanded + rest
	}
	return input
}

func (m *Model) registerText() string {
	if len(m.viRegister) == 0 {
		return ""
	}
	return strings.Join(m.viRegister, " ")
}

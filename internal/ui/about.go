package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/sysinfo"
)

var (
	aboutLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#d3869b")).Bold(true).Width(12).Align(lipgloss.Right)
	aboutValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
)

func (m *Model) enterAbout() tea.Cmd {
	m.screen = screenAbout
	info := sysinfo.FetchExtended()
	basic := sysinfo.Fetch(m.dir)

	type row struct {
		label, value string
	}
	rows := []row{
		{"Title", info.Title},
		{"OS", info.OS},
		{"Host", info.Host},
		{"Kernel", basic.Kernel},
		{"Uptime", basic.Uptime},
		{"Packages", info.Packages},
		{"Shell", info.Shell},
		{"Editor", info.Editor},
		{"DE", info.DE},
		{"WM", info.WM},
		{"Terminal", info.Terminal},
		{"CPU", info.CPU},
		{"GPU", info.GPU},
		{"RAM", basic.RAM},
		{"Swap", basic.Swap},
		{"Storage", info.Storage},
		{"Monitors", info.Monitors},
		{"Audio", info.Audio},
		{"Local IP", info.LocalIP},
		{"Battery", info.Battery},
		{"Power", info.PowerAdapter},
		{"Bluetooth", info.Bluetooth},
		{"Gamepad", info.Gamepad},
		{"Login Mgr", info.LoginManager},
	}

	var b strings.Builder
	b.WriteString("\n")
	for _, r := range rows {
		if r.value == "" {
			continue
		}
		// Handle multi-line values (storage, etc.)
		lines := strings.Split(r.value, "\n")
		for i, line := range lines {
			label := r.label
			if i > 0 {
				label = ""
			}
			b.WriteString(aboutLabelStyle.Render(label))
			b.WriteString("  ")
			b.WriteString(aboutValueStyle.Render(line))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString("   \033[40m   \033[41m   \033[42m   \033[43m   \033[44m   \033[45m   \033[46m   \033[47m\033[0m\n")
	b.WriteString("   \033[100m   \033[101m   \033[102m   \033[103m   \033[104m   \033[105m   \033[106m   \033[107m\033[0m\n")
	b.WriteString("\n")
	b.WriteString(settingHelpStyle.Render("  esc: back"))

	m.aboutView.SetContent(b.String())
	m.resize()
	return nil
}

func (m *Model) updateAbout(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "backspace", "q":
			m.screen = screenMenu
			m.resize()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.aboutView, cmd = m.aboutView.Update(msg)
	return m, cmd
}

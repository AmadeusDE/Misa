package ui

import (
	"image/color"

	"net"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/ascii"
	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/gitinfo"
	"github.com/AmadeusDE/Misa/internal/hostpath"
	"github.com/AmadeusDE/Misa/internal/sysinfo"
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type cursorBlinkMsg time.Time

func cursorBlinkCmd() tea.Cmd {
	return tea.Tick(530*time.Millisecond, func(t time.Time) tea.Msg {
		return cursorBlinkMsg(t)
	})
}

type statusColors struct {
	version  color.Color
	time     color.Color
	terminal color.Color
	cwd      color.Color
	ram      color.Color
	swap     color.Color
	uptime   color.Color
	kernel   color.Color
	distro   color.Color
	anime    color.Color
	git      color.Color
	localip  color.Color
	user     color.Color
}

func resolveColor(configured string, fallback color.Color) color.Color {
	if configured == "" {
		return fallback
	}
	return lipgloss.Color(configured)
}

type uiColors struct {
	prompt      color.Color
	promptArrow color.Color
	promptOK    color.Color
	promptErr   color.Color
	divider     color.Color
	ghost       color.Color
	status      color.Color
	output      color.Color
}

func loadStatusColors(cfg config.StatusColors) statusColors {
	return statusColors{
		version:  resolveColor(cfg.Version, lipgloss.Color("#d3869b")),
		time:     resolveColor(cfg.Time, lipgloss.Color("#83a598")),
		terminal: resolveColor(cfg.Terminal, lipgloss.Color("#a89984")),
		cwd:      resolveColor(cfg.CWD, lipgloss.Color("#fe8019")),
		ram:      resolveColor(cfg.RAM, lipgloss.Color("#fabd2f")),
		swap:     resolveColor(cfg.Swap, lipgloss.Color("#d79921")),
		uptime:   resolveColor(cfg.Uptime, lipgloss.Color("#b8bb26")),
		kernel:   resolveColor(cfg.Kernel, lipgloss.Color("#a89984")),
		distro:   resolveColor(cfg.Distro, lipgloss.Color("#fb4934")),
		anime:    resolveColor(cfg.Anime, lipgloss.Color("#d3869b")),
		git:      resolveColor(cfg.Git, lipgloss.Color("#8ec07c")),
		localip:  resolveColor(cfg.LocalIP, lipgloss.Color("#83a598")),
		user:     resolveColor(cfg.User, lipgloss.Color("#fabd2f")),
	}
}

func loadUIColors(cfg config.UIColors) uiColors {
	return uiColors{
		prompt:      resolveColor(cfg.Prompt, lipgloss.Color("#d3869b")),
		promptArrow: resolveColor(cfg.PromptArrow, lipgloss.Color("#d3869b")),
		promptOK:    resolveColor(cfg.PromptOK, lipgloss.Color("#b8bb26")),
		promptErr:   resolveColor(cfg.PromptErr, lipgloss.Color("#fb4934")),
		divider:     resolveColor(cfg.Divider, lipgloss.Color("#665c54")),
		ghost:       resolveColor(cfg.Ghost, lipgloss.Color("#504945")),
		status:      resolveColor(cfg.Status, lipgloss.Color("#fb4934")),
		output:      resolveColor(cfg.Output, lipgloss.Color("#a89984")),
	}
}

func colorize(s string, fg color.Color) string {
	return lipgloss.NewStyle().Foreground(fg).Render(s)
}

func (m *Model) contentWidth() int {
	if m.mux.primaryW > 0 {
		return m.mux.primaryW
	}
	if m.nestedMode {
		return m.width
	}
	return m.width - 4
}

func (m *Model) showLeftArt() bool  { return m.contentWidth() >= 56 }
func (m *Model) showRightArt() bool { return m.contentWidth() >= 96 }

type headerLayout struct {
	showLeft  bool
	showRight bool
	artWidth  int
	artHeight int
	textWidth int
	rows      []string
}

func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagLoopback) != 0 || (iface.Flags&net.FlagUp) == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					return ip4.String()
				}
			}
		}
	}
	return ""
}

func (m *Model) resolveInfoItem(key string, info sysinfo.Info) string {
	sc := m.statusColors
	switch key {
	case "version":
		return colorize("misa 0.1.0", sc.version)
	case "time":
		return colorize(info.Time, sc.time)
	case "terminal", "term":
		if info.Terminal == "" {
			return ""
		}
		return colorize(info.Terminal, sc.terminal)
	case "user":
		u := hostpath.Username()
		if u == "" {
			return ""
		}
		return colorize(u, sc.user)
	case "cwd":
		cwd := info.CWD
		if home, err := os.UserHomeDir(); err == nil {
			cwd = strings.Replace(cwd, home, "~", 1)
		}
		return colorize(cwd, sc.cwd)
	case "ram", "memory":
		return colorize(info.RAM, sc.ram)
	case "swap":
		if info.Swap == "" {
			return ""
		}
		return colorize(info.Swap, sc.swap)
	case "uptime":
		return colorize(info.Uptime, sc.uptime)
	case "kernel":
		return colorize(info.Kernel, sc.kernel)
	case "git":
		git := gitinfo.Fetch(info.CWD)
		if !git.InRepo {
			return ""
		}
		return colorize(git.String(), sc.git)
	case "localip", "ip":
		ip := getLocalIP()
		if ip == "" {
			return ""
		}
		return colorize(ip, sc.localip)
	case "host":
		if m.extInfo.Host == "" {
			return ""
		}
		return colorize(m.extInfo.Host, sc.kernel)
	case "os", "distro_name":
		if m.extInfo.OS == "" {
			return ""
		}
		return colorize(m.extInfo.OS, sc.distro)
	case "cpu":
		if m.extInfo.CPU == "" {
			return ""
		}
		return colorize(m.extInfo.CPU, sc.ram)
	case "gpu":
		if m.extInfo.GPU == "" {
			return ""
		}
		return colorize(m.extInfo.GPU, sc.ram)
	case "pkgs", "packages":
		if m.extInfo.Packages == "" {
			return ""
		}
		return colorize(m.extInfo.Packages, sc.uptime)
	case "shell":
		if m.extInfo.Shell == "" {
			return ""
		}
		return colorize(m.extInfo.Shell, sc.terminal)
	case "editor":
		if m.extInfo.Editor == "" {
			return ""
		}
		return colorize(m.extInfo.Editor, sc.terminal)
	case "wm":
		if m.extInfo.WM == "" {
			return ""
		}
		return colorize(m.extInfo.WM, sc.version)
	case "de":
		if m.extInfo.DE == "" {
			return ""
		}
		return colorize(m.extInfo.DE, sc.version)
	case "monitors":
		if m.extInfo.Monitors == "" {
			return ""
		}
		return colorize(strings.SplitN(m.extInfo.Monitors, "\n", 2)[0], sc.time)
	case "audio":
		if m.extInfo.Audio == "" {
			return ""
		}
		return colorize(m.extInfo.Audio, sc.time)
	case "battery":
		if m.extInfo.Battery == "" {
			return ""
		}
		return colorize(m.extInfo.Battery, sc.uptime)
	case "bluetooth", "bt":
		if m.extInfo.Bluetooth == "" {
			return ""
		}
		return colorize(m.extInfo.Bluetooth, sc.time)
	case "storage", "disk":
		if m.extInfo.Storage == "" {
			return ""
		}
		return colorize(strings.SplitN(m.extInfo.Storage, "\n", 2)[0], sc.time)
	case "login", "lm":
		if m.extInfo.LoginManager == "" {
			return ""
		}
		return colorize(m.extInfo.LoginManager, sc.version)
	case "power", "poweradapter", "ac":
		if m.extInfo.PowerAdapter == "" {
			return ""
		}
		return colorize(m.extInfo.PowerAdapter, sc.uptime)
	case "gamepad", "gp":
		if m.extInfo.Gamepad == "" {
			return ""
		}
		return colorize(m.extInfo.Gamepad, sc.time)
	case "title":
		if m.extInfo.Title == "" {
			return ""
		}
		return colorize(m.extInfo.Title, sc.user)
	}
	return ""
}

func (m *Model) buildInfoRows(info sysinfo.Info, level int) []string {
	items := m.infoItems
	if len(items) == 0 {
		items = strings.Split(config.DefaultInfo, ",")
	}

	hasBreaks := false
	for _, item := range items {
		if item == "|" {
			hasBreaks = true
			break
		}
	}

	if hasBreaks {
		var rows []string
		var cur []string
		for _, item := range items {
			if item == "|" {
				if len(cur) > 0 {
					rows = append(rows, strings.Join(cur, "  "))
				}
				cur = nil
				continue
			}
			val := m.resolveInfoItem(item, info)
			if val != "" {
				cur = append(cur, val)
			}
		}
		if len(cur) > 0 {
			rows = append(rows, strings.Join(cur, "  "))
		}
		if len(rows) == 0 {
			return []string{""}
		}
		return rows
	}

	var allParts []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		val := m.resolveInfoItem(item, info)
		if val != "" {
			allParts = append(allParts, val)
		}
	}

	if len(allParts) == 0 {
		return []string{""}
	}

	perRow := 3
	if level >= 1 {
		perRow = 4
	}
	if level >= 3 {
		perRow = len(allParts)
	}

	var rows []string
	for i := 0; i < len(allParts); i += perRow {
		end := i + perRow
		if end > len(allParts) {
			end = len(allParts)
		}
		rows = append(rows, strings.Join(allParts[i:end], "  "))
	}
	return rows
}

func maxRowWidth(rows []string) int {
	w := 0
	for _, r := range rows {
		rw := lipgloss.Width(r)
		if rw > w {
			w = rw
		}
	}
	return w
}

const gap = 3 // " │ "

func (m *Model) computeLayout(info sysinfo.Info) headerLayout {
	avail := m.contentWidth()

	leftW := ascii.ArtWidth(m.distroArt)
	rightW := ascii.ArtWidth(m.animeArt)
	artW := leftW
	if rightW > artW {
		artW = rightW
	}

	leftH := len(m.distroArt)
	rightH := len(m.animeArt)
	artH := leftH
	if rightH > artH {
		artH = rightH
	}
	if artH < 3 {
		artH = 3
	}

	type candidate struct {
		left, right bool
		level       int
	}
	candidates := []candidate{
		{true, true, 0},
		{true, true, 1},
		{true, true, 2},
		{true, true, 3},
		{false, true, 0},
		{false, true, 3},
		{true, false, 0},
		{true, false, 3},
		{false, false, 0},
		{false, false, 3},
	}

	for _, c := range candidates {
		textAvail := avail
		if c.left {
			textAvail -= artW + gap
		}
		if c.right {
			textAvail -= artW + gap
		}
		if textAvail < 10 {
			continue
		}

		rows := m.buildInfoRows(info, c.level)
		needed := maxRowWidth(rows)
		if needed <= textAvail {
			h := artH
			if !c.left && !c.right {
				h = len(rows)
			}
			return headerLayout{
				showLeft:  c.left,
				showRight: c.right,
				artWidth:  artW,
				artHeight: h,
				textWidth: textAvail,
				rows:      rows,
			}
		}
	}

	rows := m.buildInfoRows(info, 3)
	return headerLayout{
		textWidth: avail,
		rows:      rows,
	}
}

func (m *Model) renderHeader() string {
	info := sysinfo.Fetch(m.dir)
	lay := m.computeLayout(info)
	sc := m.statusColors

	hdrHeight := lay.artHeight
	if len(lay.rows) > hdrHeight {
		hdrHeight = len(lay.rows)
	}

	leftStart := (hdrHeight - len(m.distroArt)) / 2
	rightStart := (hdrHeight - len(m.animeArt)) / 2

	leftArtW := ascii.ArtWidth(m.distroArt)
	rightArtW := ascii.ArtWidth(m.animeArt)
	leftHPad := (lay.artWidth - leftArtW) / 2
	rightHPad := (lay.artWidth - rightArtW) / 2

	distroFg := lipgloss.NewStyle().Foreground(sc.distro)
	animeFg := lipgloss.NewStyle().Foreground(sc.anime)
	sep := lipgloss.NewStyle().Foreground(m.uiColors.divider).Render("│")

	var output []string
	for i := 0; i < hdrHeight; i++ {
		var line strings.Builder

		if lay.showLeft {
			artIdx := i - leftStart
			artLine := ""
			if artIdx >= 0 && artIdx < len(m.distroArt) {
				artLine = m.distroArt[artIdx]
			}
			styled := artLine
			if !strings.Contains(artLine, "\033[") {
				styled = distroFg.Render(artLine)
			}
			line.WriteString(strings.Repeat(" ", leftHPad))
			line.WriteString(ascii.PadLine(styled, leftArtW))
			rightRemain := lay.artWidth - leftHPad - leftArtW
			if rightRemain > 0 {
				line.WriteString(strings.Repeat(" ", rightRemain))
			}
			line.WriteString(" ")
			line.WriteString(sep)
			line.WriteString(" ")
		}

		centerText := ""
		if i < len(lay.rows) {
			centerText = lay.rows[i]
		}
		centerVis := lipgloss.Width(centerText)
		leftPad := (lay.textWidth - centerVis) / 2
		if leftPad < 0 {
			leftPad = 0
		}
		rightPad := lay.textWidth - centerVis - leftPad
		if rightPad < 0 {
			rightPad = 0
		}
		line.WriteString(strings.Repeat(" ", leftPad))
		line.WriteString(centerText)
		line.WriteString(strings.Repeat(" ", rightPad))

		if lay.showRight {
			line.WriteString(" ")
			line.WriteString(sep)
			line.WriteString(" ")
			artIdx := i - rightStart
			artLine := ""
			if artIdx >= 0 && artIdx < len(m.animeArt) {
				artLine = m.animeArt[artIdx]
			}
			styled := animeFg.Render(artLine)
			line.WriteString(strings.Repeat(" ", rightHPad))
			line.WriteString(ascii.PadLine(styled, rightArtW))
			rightRemain := lay.artWidth - rightHPad - rightArtW
			if rightRemain > 0 {
				line.WriteString(strings.Repeat(" ", rightRemain))
			}
		}

		output = append(output, line.String())
	}

	return strings.Join(output, "\n")
}

func (m *Model) headerHeight() int {
	info := sysinfo.Fetch(m.dir)
	lay := m.computeLayout(info)
	h := lay.artHeight
	if len(lay.rows) > h {
		h = len(lay.rows)
	}
	return h
}

package builtins

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ascii"
	"github.com/AmadeusDE/Misa/internal/color"
	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/sysinfo"
)

const fetchSpec = "(info):(theme):(color-mode):(ascii):(ascii-type):(ascii-dir):(palettes-file):e(emotes)(list-info)(list-ascii)(list-themes)"

var fetchSpecParsed *Spec

func init() {
	var err error
	fetchSpecParsed, err = ParseSpec(fetchSpec)
	if err != nil {
		panic("builtins: bad fetch spec: " + err.Error())
	}
	Register(Command{Name: "fetch", Run: runFetch})
}

var defaultFetchInfo = "title,os,host,kernel,uptime,pkgs,memory,shell,editor,wm,de,cpu,gpu,storage,swap,monitors,audio,login,terminal,localip,gamepad,battery,poweradapter,bluetooth,colors"

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func runFetch(ctx *Context) int {
	flags, err := fetchSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "fetch:", err)
		return 1
	}

	mainCfg := config.Load()
	fetchCfg := config.LoadFetch()

	if flags.Bool("list-info") {
		items := []string{
			"title", "os", "host", "kernel", "uptime", "pkgs", "memory",
			"shell", "editor", "wm", "de", "cpu", "gpu", "storage",
			"swap", "monitors", "audio", "login", "terminal", "localip", "gamepad",
			"battery", "poweradapter", "bluetooth", "colors",
		}
		fmt.Fprintln(ctx.Stdout, "Available info modules:")
		for _, item := range items {
			fmt.Fprintf(ctx.Stdout, "  %s\n", item)
		}
		return 0
	}

	theme := mainCfg.Ascii.Theme
	if fetchCfg.Theme != "" {
		theme = fetchCfg.Theme
	}
	if theme == "" {
		theme = "default"
	}
	if v, ok := flags.String("theme"); ok {
		theme = v
	}

	colorMode := mainCfg.Ascii.ColorMode
	if fetchCfg.ColorMode != "" {
		colorMode = fetchCfg.ColorMode
	}
	if colorMode == "" {
		colorMode = "line"
	}
	if v, ok := flags.String("color-mode"); ok {
		colorMode = v
	}

	palettesFile := mainCfg.Ascii.PalettesFile
	if fetchCfg.PalettesFile != "" {
		palettesFile = fetchCfg.PalettesFile
	}
	if v, ok := flags.String("palettes-file"); ok {
		palettesFile = v
	}
	palettes := color.LoadPalettes(palettesFile)

	if flags.Bool("list-themes") {
		names := color.ListThemes(palettes)
		fmt.Fprintln(ctx.Stdout, "Available themes:")
		for _, n := range names {
			fmt.Fprintf(ctx.Stdout, "  %s\n", n)
		}
		return 0
	}

	if flags.Bool("list-ascii") {
		names := ascii.ListDistroNames()
		fmt.Fprintln(ctx.Stdout, "Available ASCII art:")
		for _, n := range names {
			fmt.Fprintf(ctx.Stdout, "  %s\n", n)
		}
		return 0
	}

	forcedAscii := mainCfg.Ascii.Distro
	if fetchCfg.Ascii != "" {
		forcedAscii = fetchCfg.Ascii
	}
	if v, ok := flags.String("ascii"); ok {
		forcedAscii = v
	}

	info := sysinfo.Fetch(ctx.Dir)
	distroQuery := info.Distro
	if forcedAscii != "" {
		distroQuery = forcedAscii
	}

	art := ascii.LoadDistroArt(distroQuery, forcedAscii)
	lines := ascii.Colorize(art, theme, colorMode, palettes)

	maxH := mainCfg.Ascii.MaxHeight
	if fetchCfg.MaxHeight > 0 {
		maxH = fetchCfg.MaxHeight
	}
	if maxH == 0 {
		maxH = 16
	}
	lines = ascii.CropHeight(lines, maxH)

	artWidth := 0
	for _, line := range lines {
		w := len(stripANSI(line))
		if w > artWidth {
			artWidth = w
		}
	}

	infoStr := defaultFetchInfo
	if fetchCfg.Info != "" {
		infoStr = fetchCfg.Info
	}
	if v, ok := flags.String("info"); ok {
		infoStr = v
	}

	enabledItems := strings.Split(infoStr, ",")
	for i := range enabledItems {
		enabledItems[i] = strings.TrimSpace(strings.ToLower(enabledItems[i]))
	}

	emotes := fetchCfg.Emotes || flags.Bool("e")

	ext := sysinfo.FetchExtended()
	basic := sysinfo.Fetch(ctx.Dir)
	infoRows := buildFetchRows(enabledItems, ext, basic, art, palettes, theme, emotes, distroQuery)

	maxRows := len(lines)
	if len(infoRows) > maxRows {
		maxRows = len(infoRows)
	}

	for i := 0; i < maxRows; i++ {
		artCol := ""
		if i < len(lines) {
			artCol = lines[i]
		}
		visLen := len(stripANSI(artCol))
		padding := artWidth - visLen + 4
		if padding < 2 {
			padding = 2
		}
		row := ""
		if i < len(infoRows) {
			row = infoRows[i]
		}
		fmt.Fprintf(ctx.Stdout, "%s%s%s\n", artCol, strings.Repeat(" ", padding), row)
	}

	return 0
}

func distroIcon(query string) string {
	q := strings.ToLower(query)
	if strings.Contains(q, "/") {
		q = strings.SplitN(q, "/", 2)[1]
	}
	switch {
	case strings.Contains(q, "arch"), strings.Contains(q, "cachy"), strings.Contains(q, "endeavour"):
		return "󰣇"
	case strings.Contains(q, "ubuntu"):
		return ""
	case strings.Contains(q, "fedora"):
		return ""
	case strings.Contains(q, "debian"):
		return ""
	case strings.Contains(q, "mint"):
		return ""
	case strings.Contains(q, "gentoo"):
		return ""
	case strings.Contains(q, "centos"):
		return ""
	case strings.Contains(q, "redhat"), strings.Contains(q, "rhel"):
		return ""
	case strings.Contains(q, "alpine"):
		return ""
	case strings.Contains(q, "void"):
		return ""
	case strings.Contains(q, "nixos"):
		return ""
	}
	return ""
}

func buildFetchRows(items []string, ext sysinfo.ExtendedInfo, basic sysinfo.Info, art *ascii.AsciiArt, palettes color.Palettes, theme string, emotes bool, asciiQuery string) []string {
	var labelColor, titleColor string

	if theme != "" && theme != "default" {
		if pal, ok := palettes[strings.ToLower(theme)]; ok && len(pal) > 0 {
			if c, err := color.HexToANSI(pal[0]); err == nil {
				labelColor = c
			}
			if len(pal) > 1 {
				if c, err := color.HexToANSI(pal[1]); err == nil {
					titleColor = c
				}
			}
			if titleColor == "" {
				titleColor = labelColor
			}
		}
	}

	if labelColor == "" {
		if len(art.Colors) > 0 {
			if c := color.TranslateColor(art.Colors[0]); c != "" {
				labelColor = c
			}
		}
		if labelColor == "" {
			labelColor = "\033[34m"
		}
		titleColor = labelColor
	}

	infoMap := map[string]string{
		"title":        ext.Title,
		"os":           ext.OS,
		"host":         ext.Host,
		"kernel":       basic.Kernel,
		"uptime":       basic.Uptime,
		"pkgs":         ext.Packages,
		"memory":       basic.RAM,
		"shell":        ext.Shell,
		"editor":       ext.Editor,
		"wm":           ext.WM,
		"de":           ext.DE,
		"cpu":          ext.CPU,
		"gpu":          ext.GPU,
		"storage":      ext.Storage,
		"swap":         basic.Swap,
		"monitors":     ext.Monitors,
		"audio":        ext.Audio,
		"login":        ext.LoginManager,
		"terminal":     ext.Terminal,
		"localip":      ext.LocalIP,
		"gamepad":      ext.Gamepad,
		"battery":      ext.Battery,
		"poweradapter": ext.PowerAdapter,
		"bluetooth":    ext.Bluetooth,
	}

	aliases := map[string]string{
		"disk": "storage", "lm": "login", "term": "terminal",
		"ip": "localip", "gp": "gamepad", "ac": "poweradapter", "bt": "bluetooth",
	}

	maxLabelLen := 0
	for _, item := range items {
		if item == "title" || item == "colors" || item == "colours" {
			continue
		}
		if len(item) > maxLabelLen {
			maxLabelLen = len(item)
		}
	}

	itemColors := map[string]string{
		"os": "\033[32m", "host": "\033[32m", "uptime": "\033[32m",
		"cpu": "\033[31m", "gpu": "\033[31m", "localip": "\033[31m",
		"storage": "\033[96m", "disk": "\033[96m",
		"memory": "\033[33m",
		"swap":   "\033[36m", "pkgs": "\033[36m",
		"monitors": "\033[93m",
		"audio":    "\033[34m", "gamepad": "\033[34m", "bluetooth": "\033[34m",
		"login": "\033[95m", "de": "\033[95m", "wm": "\033[95m",
		"kernel": "\033[37m", "shell": "\033[37m", "editor": "\033[37m",
		"terminal": "\033[37m", "battery": "\033[37m", "poweradapter": "\033[37m",
	}

	dIcon := distroIcon(asciiQuery)
	itemEmotes := map[string]string{
		"os": dIcon, "host": dIcon,
		"kernel":  "",
		"uptime":  "󰅐",
		"pkgs":    "󰏖",
		"memory":  "󰑭",
		"shell":   "",
		"editor":  "",
		"wm":      "",
		"de":      "",
		"cpu":     "󰻠",
		"gpu":     "󰍛",
		"storage": "", "disk": "",
		"swap":     "󰓡",
		"monitors": "󰍹",
		"audio":    "",
		"login":    "󰧨", "lm": "󰧨",
		"terminal": "", "term": "",
		"localip": "󰩟", "ip": "󰩟",
		"gamepad": "", "gp": "",
		"battery":      "",
		"poweradapter": "", "ac": "",
		"bluetooth": "", "bt": "",
	}

	var rows []string
	for _, item := range items {
		switch item {
		case "title":
			if v, ok := infoMap["title"]; ok && v != "" {
				parts := strings.SplitN(v, "@", 2)
				if len(parts) == 2 {
					rows = append(rows, fmt.Sprintf("%s\033[1m%s\033[0m@\033[1m%s%s", titleColor, parts[0], parts[1], color.Reset))
				} else {
					rows = append(rows, titleColor+v+color.Reset)
				}
			}
		case "colors", "colours":
			rows = append(rows, "")
			rows = append(rows, "   \033[40m   \033[41m   \033[42m   \033[43m   \033[44m   \033[45m   \033[46m   \033[47m\033[0m")
			rows = append(rows, "   \033[100m   \033[101m   \033[102m   \033[103m   \033[104m   \033[105m   \033[106m   \033[107m\033[0m")
		default:
			mapKey := item
			if alias, ok := aliases[item]; ok {
				mapKey = alias
			}
			val, ok := infoMap[mapKey]
			if !ok || val == "" {
				continue
			}
			col := itemColors[item]
			if col == "" {
				col = labelColor
			}
			for _, line := range strings.Split(val, "\n") {
				if emotes {
					icon := itemEmotes[item]
					if icon == "" {
						icon = itemEmotes[mapKey]
					}
					if icon == "" {
						icon = "•"
					}
					rows = append(rows, fmt.Sprintf("%s %s%s  %s%s", col, icon, color.Reset, line, color.Reset))
				} else {
					paddedLabel := mapKey + strings.Repeat(" ", maxLabelLen-len(mapKey))
					rows = append(rows, fmt.Sprintf("%s%s\033[0m  %s%s", col, paddedLabel, line, color.Reset))
				}
			}
		}
	}
	return rows
}

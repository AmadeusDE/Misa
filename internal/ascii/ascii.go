package ascii

import (
	"crypto/rand"
	"embed"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/color"
	"github.com/AmadeusDE/Misa/internal/hostpath"
)

//go:embed distro/pfetch/*.txt distro/neofetch/*.txt distro/fastfetch/*.txt
var distroFS embed.FS

type AsciiArt struct {
	Name    string
	Pattern string
	Colors  []string
	Lines   []string
	Source  string // "pfetch", "neofetch", "fastfetch"
}

func parseFile(content, name, source string) *AsciiArt {
	parts := strings.SplitN(content, "---", 3)
	art := content
	pattern := name
	var colors []string

	if len(parts) >= 3 {
		art = parts[2]
		for _, line := range strings.Split(parts[1], "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "pattern:") {
				pattern = strings.TrimSpace(line[len("pattern:"):])
			} else if strings.HasPrefix(line, "colors:") {
				for _, c := range strings.Split(line[len("colors:"):], ",") {
					colors = append(colors, strings.TrimSpace(c))
				}
			}
		}
	}

	art = strings.Trim(art, "\n")

	return &AsciiArt{
		Name:    name,
		Pattern: pattern,
		Colors:  colors,
		Lines:   strings.Split(art, "\n"),
		Source:  source,
	}
}

var sources = []string{"pfetch", "neofetch", "fastfetch"}

func externalDirs(subpath string) []string {
	return hostpath.SearchDirs(subpath)
}

func loadAllArts() []*AsciiArt {
	var all []*AsciiArt
	for _, src := range sources {
		dir := "distro/" + src
		files, err := distroFS.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".txt") {
				continue
			}
			fp, err := distroFS.Open(dir + "/" + f.Name())
			if err != nil {
				continue
			}
			data, err := io.ReadAll(fp)
			fp.Close()
			if err != nil {
				continue
			}
			name := strings.TrimSuffix(f.Name(), ".txt")
			all = append(all, parseFile(string(data), name, src))
		}
	}
	for _, dir := range externalDirs("ascii/distro") {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".txt") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, f.Name()))
			if err != nil {
				continue
			}
			name := strings.TrimSuffix(f.Name(), ".txt")
			all = append(all, parseFile(string(data), name, "custom"))
		}
	}
	return all
}

func LoadDistroArt(distroID, forced string) *AsciiArt {
	query := distroID
	if forced != "" {
		query = pickRandom(forced)
	}

	arts := loadAllArts()
	match := findMatch(arts, query)
	if match != nil {
		return match
	}

	return &AsciiArt{Name: "linux", Lines: fallbackDistro(), Source: "pfetch"}
}

func findMatch(arts []*AsciiArt, query string) *AsciiArt {
	queryLower := strings.ToLower(query)

	// Source-specific matching: "fastfetch/CachyOS_small"
	if strings.Contains(query, "/") {
		parts := strings.SplitN(query, "/", 2)
		srcReq := strings.ToLower(parts[0])
		nameReq := strings.ToLower(parts[1])

		for _, art := range arts {
			if strings.ToLower(art.Source) == srcReq && strings.ToLower(art.Name) == nameReq {
				return art
			}
		}
		for _, art := range arts {
			if strings.ToLower(art.Source) == srcReq && matchesPattern(art.Pattern, nameReq) {
				return art
			}
		}
	}

	// Exact name match (any source, pfetch first)
	for _, src := range sources {
		for _, art := range arts {
			if art.Source == src && strings.ToLower(art.Name) == queryLower {
				return art
			}
		}
	}

	// Pattern match (pfetch → neofetch → fastfetch)
	for _, src := range sources {
		for _, art := range arts {
			if art.Source == src && matchesPattern(art.Pattern, query) {
				return art
			}
		}
	}

	// Fallback: linux or generic
	for _, art := range arts {
		if strings.ToLower(art.Name) == "linux" {
			return art
		}
	}
	for _, art := range arts {
		if strings.ToLower(art.Name) == "generic" {
			return art
		}
	}

	if len(arts) > 0 {
		return arts[0]
	}
	return nil
}

func ListDistroNames() []string {
	arts := loadAllArts()
	var names []string
	for _, a := range arts {
		names = append(names, a.Source+"/"+a.Name)
	}
	sort.Strings(names)
	return names
}

func LoadDistroPreview(fullName string) []string {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	arts := loadAllArts()
	for _, a := range arts {
		if a.Source == parts[0] && a.Name == parts[1] {
			return stripColorTokens(a.Lines)
		}
	}
	return nil
}

// Anime art is never bundled: it comes from ascii/anime next to the binary or
// in ~/.config/misa, so users supply art they have the rights to.

func ListAnimeNames() []string {
	seen := make(map[string]bool)
	var names []string
	for _, path := range animeFiles() {
		n := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}

func animeFiles() []string {
	var paths []string
	for _, dir := range externalDirs("ascii/anime") {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && e.Name() != "credits.txt" {
				paths = append(paths, filepath.Join(dir, e.Name()))
			}
		}
	}
	return paths
}

// LoadAnimeArt returns the selected art, a random one for "" or "random", or
// nil when no art is installed.
func LoadAnimeArt(selection string) []string {
	if selection != "" && selection != "random" {
		picked := pickRandom(selection)
		for _, dir := range externalDirs("ascii/anime") {
			for _, ext := range []string{".ascii", ".txt", ""} {
				if lines := readArt(filepath.Join(dir, picked+ext)); lines != nil {
					return lines
				}
			}
		}
	}
	files := animeFiles()
	if len(files) == 0 {
		return nil
	}
	return readArt(files[randInt(len(files))])
}

func readArt(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func Colorize(art *AsciiArt, theme, colorMode string, palettes color.Palettes) []string {
	if theme == "" || theme == "default" {
		return stripColorTokens(art.Lines)
	}

	if theme == "native" {
		return colorizeDefault(art)
	}

	theme = pickRandom(theme)
	if theme == "default" {
		return stripColorTokens(art.Lines)
	}
	if theme == "native" {
		return colorizeDefault(art)
	}

	palette, ok := palettes[strings.ToLower(theme)]
	if !ok || len(palette) == 0 {
		return colorizeDefault(art)
	}

	var ansiPalette []string
	for _, hex := range palette {
		if code, err := color.HexToANSI(hex); err == nil {
			ansiPalette = append(ansiPalette, code)
		} else if code := color.TranslateColor(hex); code != "" {
			ansiPalette = append(ansiPalette, code)
		}
	}
	if len(ansiPalette) == 0 {
		return colorizeDefault(art)
	}

	clean := stripColorTokens(art.Lines)

	var result []string
	if colorMode == "char" {
		for _, line := range clean {
			var b strings.Builder
			j := 0
			for _, ch := range line {
				if ch == ' ' {
					b.WriteRune(ch)
				} else {
					b.WriteString(ansiPalette[j%len(ansiPalette)])
					b.WriteRune(ch)
					j++
				}
			}
			result = append(result, b.String()+color.Reset)
		}
	} else {
		for i, line := range clean {
			if strings.TrimSpace(line) == "" {
				result = append(result, line)
				continue
			}
			idx := (i * len(ansiPalette)) / len(clean)
			if idx >= len(ansiPalette) {
				idx = len(ansiPalette) - 1
			}
			result = append(result, ansiPalette[idx]+line+color.Reset)
		}
	}
	return result
}

func colorizeDefault(art *AsciiArt) []string {
	joined := strings.Join(art.Lines, "\n")

	for i := 1; i <= 8; i++ {
		ansiCode := "\033[37m"
		if i-1 < len(art.Colors) {
			if c := color.TranslateColor(art.Colors[i-1]); c != "" {
				ansiCode = c
			}
		}
		joined = strings.ReplaceAll(joined, fmt.Sprintf("${c%d}", i), ansiCode)
		joined = strings.ReplaceAll(joined, fmt.Sprintf("$%d", i), ansiCode)
	}

	lines := strings.Split(joined+color.Reset, "\n")
	return lines
}

func stripColorTokens(lines []string) []string {
	re1 := regexp.MustCompile(`\$\{c\d\}`)
	re2 := regexp.MustCompile(`\$\d`)
	result := make([]string, len(lines))
	for i, l := range lines {
		l = re1.ReplaceAllString(l, "")
		l = re2.ReplaceAllString(l, "")
		result[i] = l
	}
	return result
}

func CropHeight(lines []string, maxHeight int) []string {
	if maxHeight <= 0 || len(lines) <= maxHeight {
		return lines
	}
	return lines[:maxHeight]
}

func matchesPattern(pattern, query string) bool {
	parts := strings.Split(pattern, "|")
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		p = strings.ReplaceAll(p, "*", ".*")
		if !strings.HasPrefix(p, ".*") {
			p = "^" + p
		}
		if !strings.HasSuffix(p, ".*") {
			p = p + "$"
		}
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			continue
		}
		if re.MatchString(query) {
			return true
		}
	}
	return false
}

func pickRandom(csv string) string {
	parts := strings.Split(csv, ",")
	var items []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			items = append(items, p)
		}
	}
	if len(items) == 0 {
		return csv
	}
	if len(items) == 1 {
		return items[0]
	}
	return items[randInt(len(items))]
}

func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	big, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(big.Int64())
}

func fallbackDistro() []string {
	return []string{
		`    /\    `,
		`   /  \   `,
		`  /    \  `,
		`  \    /  `,
		`   \  /   `,
		`    \/    `,
	}
}

func ArtWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		lw := lipgloss.Width(l)
		if lw > w {
			w = lw
		}
	}
	return w
}

func PadLine(s string, width int) string {
	vis := lipgloss.Width(s)
	if vis >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vis)
}

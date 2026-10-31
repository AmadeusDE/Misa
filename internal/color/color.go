package color

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AmadeusDE/Misa/internal/hostpath"
)

//go:embed assets/presets.json
var embeddedPresetsFS embed.FS

type Palettes map[string][]string

const Reset = "\033[0m"

func LoadPalettes(customPath string) Palettes {
	var data []byte

	if customPath != "" {
		d, err := os.ReadFile(customPath)
		if err == nil {
			data = d
		}
	}

	if data == nil {
		if execPath, err := exec.LookPath(os.Args[0]); err == nil {
			if abs, err := filepath.Abs(execPath); err == nil {
				d, err := os.ReadFile(filepath.Join(filepath.Dir(abs), "presets.json"))
				if err == nil {
					data = d
				}
			}
		}
	}

	if data == nil {
		if dir := hostpath.ConfigDir(); dir != "" {
			if d, err := os.ReadFile(filepath.Join(dir, "presets.json")); err == nil {
				data = d
			}
		}
	}

	if data == nil {
		f, err := embeddedPresetsFS.Open("assets/presets.json")
		if err != nil {
			return make(Palettes)
		}
		defer f.Close()
		d, err := io.ReadAll(f)
		if err != nil {
			return make(Palettes)
		}
		data = d
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return make(Palettes)
	}

	p := make(Palettes)
	for name, rawVal := range raw {
		var colors []string
		if err := json.Unmarshal(rawVal, &colors); err == nil {
			p[name] = colors
			continue
		}
		var obj struct {
			Colors []string `json:"colors"`
		}
		if err := json.Unmarshal(rawVal, &obj); err == nil {
			p[name] = obj.Colors
		}
	}
	return p
}

func ListThemes(p Palettes) []string {
	var names []string
	for name := range p {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func HexToANSI(hex string) (string, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "", fmt.Errorf("invalid hex color: %s", hex)
	}
	var r, g, b uint8
	_, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b), nil
}

func TranslateColor(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}

	if strings.HasPrefix(val, "#") || len(val) == 6 {
		if ans, err := HexToANSI(val); err == nil {
			return ans
		}
	}

	if len(val) == 1 && val[0] >= '0' && val[0] <= '7' {
		return fmt.Sprintf("\033[3%sm", val)
	}

	isRawAnsi := true
	for _, char := range val {
		if !((char >= '0' && char <= '9') || char == ';') {
			isRawAnsi = false
			break
		}
	}
	if isRawAnsi && len(val) > 0 {
		return fmt.Sprintf("\033[%sm", val)
	}

	names := map[string]string{
		"black":          "\033[30m",
		"red":            "\033[31m",
		"green":          "\033[32m",
		"yellow":         "\033[33m",
		"blue":           "\033[34m",
		"magenta":        "\033[35m",
		"cyan":           "\033[36m",
		"white":          "\033[37m",
		"bright_black":   "\033[90m",
		"bright_red":     "\033[91m",
		"bright_green":   "\033[92m",
		"bright_yellow":  "\033[93m",
		"bright_blue":    "\033[94m",
		"bright_magenta": "\033[95m",
		"bright_cyan":    "\033[96m",
		"bright_white":   "\033[97m",
		"reset":          Reset,
	}
	if ans, ok := names[strings.ToLower(val)]; ok {
		return ans
	}
	return ""
}
